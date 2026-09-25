// Package jobs is a durable background queue kept in PostgreSQL.
//
// Enqueue inserts a row, in the caller's transaction if it has one, so a
// job exists exactly when the change that asked for it does. Workers claim
// rows with FOR UPDATE SKIP LOCKED, so any number of them, in any number of
// processes, never take the same job. A NOTIFY wakes idle workers the
// moment a job is queued; a slow poll catches anything a notification
// missed. A claimed job holds a lease; if its worker dies, the lease runs
// out and the job is handed out again. Failures retry with exponential
// backoff and jitter until max_attempts.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nafizahmed/vernissage/server/internal/db"
)

const channel = "vernissage_jobs"

// Handler runs one job. Returning an error schedules a retry; returning an
// error wrapped with Permanent fails the job at once.
type Handler func(ctx context.Context, payload json.RawMessage) error

type permanent struct{ error }

// Permanent marks an error that retrying won't fix.
func Permanent(err error) error { return permanent{err} }

// Options adjust a job at enqueue time.
type Options struct {
	// Dedupe keeps at most one queued or running job per key.
	Dedupe string
	// Priority runs higher first; the default is 0.
	Priority int
	// RunAt delays the job.
	RunAt time.Time
	// MaxAttempts defaults to 5.
	MaxAttempts int
}

// Enqueue adds a job. With a Dedupe key that already has a live job, it
// does nothing and reports false.
func Enqueue(ctx context.Context, q db.Querier, kind string, payload any, o Options) (bool, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 5
	}
	runAt := o.RunAt
	if runAt.IsZero() {
		runAt = time.Now()
	}
	var dedupe *string
	if o.Dedupe != "" {
		dedupe = &o.Dedupe
	}
	tag, err := q.Exec(ctx, `
		insert into jobs (kind, payload, dedupe, priority, max_attempts, run_at)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (dedupe) where dedupe is not null and status in ('queued', 'running') do nothing`,
		kind, body, dedupe, o.Priority, o.MaxAttempts, runAt)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	// Delivered on commit when q is a transaction.
	_, err = q.Exec(ctx, `select pg_notify($1, $2)`, channel, kind)
	return true, err
}

// Queue runs registered handlers on a pool of workers.
type Queue struct {
	pool     *pgxpool.Pool
	handlers map[string]Handler
	lease    time.Duration
	wake     chan struct{}
	log      *slog.Logger
}

func New(pool *pgxpool.Pool) *Queue {
	return &Queue{
		pool:     pool,
		handlers: map[string]Handler{},
		lease:    5 * time.Minute,
		wake:     make(chan struct{}, 1),
		log:      slog.With("component", "jobs"),
	}
}

// Handle registers the handler for a kind of job.
func (q *Queue) Handle(kind string, h Handler) { q.handlers[kind] = h }

type claimed struct {
	id          int64
	kind        string
	payload     json.RawMessage
	attempts    int
	maxAttempts int
}

// Run works jobs until ctx is cancelled, then waits for running jobs.
func (q *Queue) Run(ctx context.Context, workers int) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); q.listen(ctx) }()
	go func() { defer wg.Done(); q.reap(ctx) }()
	for range workers {
		wg.Add(1)
		go func() { defer wg.Done(); q.work(ctx) }()
	}
	wg.Wait()
}

func (q *Queue) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// listen turns NOTIFYs into wake-ups, reconnecting if the connection drops.
func (q *Queue) listen(ctx context.Context) {
	for ctx.Err() == nil {
		err := func() error {
			conn, err := q.pool.Acquire(ctx)
			if err != nil {
				return err
			}
			defer conn.Release()
			if _, err := conn.Exec(ctx, "listen "+channel); err != nil {
				return err
			}
			for {
				if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
					return err
				}
				q.poke()
			}
		}()
		if ctx.Err() == nil {
			q.log.Warn("listen interrupted", "err", err)
			sleep(ctx, 2*time.Second)
		}
	}
}

// reap returns jobs whose lease ran out to the queue.
func (q *Queue) reap(ctx context.Context) {
	for sleep(ctx, 30*time.Second) {
		tag, err := q.pool.Exec(ctx, `
			update jobs set status = 'queued', leased_until = null, updated_at = now(),
			       last_error = 'lease expired'
			where status = 'running' and leased_until < now()`)
		if err != nil {
			q.log.Warn("reap", "err", err)
		} else if tag.RowsAffected() > 0 {
			q.log.Info("requeued abandoned jobs", "count", tag.RowsAffected())
			q.poke()
		}
		// Keep a day of finished jobs for looking into, no more.
		q.pool.Exec(ctx, `delete from jobs where status = 'done' and updated_at < now() - interval '1 day'`)
	}
}

func (q *Queue) work(ctx context.Context) {
	for ctx.Err() == nil {
		j, err := q.claim(ctx)
		if err != nil {
			if ctx.Err() == nil {
				q.log.Warn("claim", "err", err)
				sleep(ctx, time.Second)
			}
			continue
		}
		if j == nil {
			// Nothing ready: wait for a notification or poll again.
			select {
			case <-ctx.Done():
			case <-q.wake:
			case <-time.After(5*time.Second + time.Duration(rand.IntN(1000))*time.Millisecond):
			}
			continue
		}
		q.run(ctx, j)
		// There may be more; let another idle worker look too.
		q.poke()
	}
}

func (q *Queue) claim(ctx context.Context) (*claimed, error) {
	kinds := make([]string, 0, len(q.handlers))
	for k := range q.handlers {
		kinds = append(kinds, k)
	}
	var j claimed
	err := q.pool.QueryRow(ctx, `
		update jobs set status = 'running', attempts = attempts + 1,
		       leased_until = now() + make_interval(secs => $2), updated_at = now()
		where id = (
			select id from jobs
			where status = 'queued' and run_at <= now() and kind = any($1)
			order by priority desc, run_at, id
			for update skip locked
			limit 1)
		returning id, kind, payload, attempts, max_attempts`,
		kinds, q.lease.Seconds()).Scan(&j.id, &j.kind, &j.payload, &j.attempts, &j.maxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &j, err
}

func (q *Queue) run(ctx context.Context, j *claimed) {
	start := time.Now()
	jctx, cancel := context.WithTimeout(ctx, q.lease-10*time.Second)
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		return q.handlers[j.kind](jctx, j.payload)
	}()
	cancel()

	// Record the outcome even if we're shutting down.
	bg, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer done()
	if err == nil {
		q.pool.Exec(bg, `update jobs set status = 'done', leased_until = null, updated_at = now() where id = $1`, j.id)
		q.log.Debug("done", "kind", j.kind, "id", j.id, "took", time.Since(start))
		return
	}
	var perm permanent
	if errors.As(err, &perm) || j.attempts >= j.maxAttempts {
		q.pool.Exec(bg, `update jobs set status = 'failed', leased_until = null, last_error = $2, updated_at = now() where id = $1`, j.id, err.Error())
		q.log.Warn("failed", "kind", j.kind, "id", j.id, "attempts", j.attempts, "err", err)
		return
	}
	q.pool.Exec(bg, `update jobs set status = 'queued', leased_until = null, last_error = $2,
		run_at = now() + make_interval(secs => $3), updated_at = now() where id = $1`, j.id, err.Error(), Backoff(j.attempts).Seconds())
	q.log.Info("retrying", "kind", j.kind, "id", j.id, "attempt", j.attempts, "err", err)
}

// Backoff is the wait before retry n: 10 s, 40 s, 2.5 min, 10 min... with
// a quarter of jitter so a burst of failures doesn't come back as a burst.
func Backoff(attempt int) time.Duration {
	base := 10 * time.Second << (2 * min(attempt-1, 6))
	return base + time.Duration(rand.Int64N(int64(base/4)))
}

func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// Stats counts jobs by kind and status, for the health endpoint.
func Stats(ctx context.Context, q db.Querier) (map[string]map[string]int, error) {
	rows, err := q.Query(ctx, `select kind, status, count(*) from jobs group by kind, status`)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]int{}
	for rows.Next() {
		var kind, status string
		var n int
		if err := rows.Scan(&kind, &status, &n); err != nil {
			return nil, err
		}
		if out[kind] == nil {
			out[kind] = map[string]int{}
		}
		out[kind][status] = n
	}
	return out, rows.Err()
}
