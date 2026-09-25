package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nafizahmed/vernissage/server/internal/jobs"
	"github.com/nafizahmed/vernissage/server/internal/testdb"
)

func TestDedupe(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		jobs.Enqueue(ctx, pool, "analyze", map[string]int{"id": 1}, jobs.Options{Dedupe: "analyze:1"})
	}
	var n int
	pool.QueryRow(ctx, `select count(*) from jobs`).Scan(&n)
	if n != 1 {
		t.Errorf("three enqueues with one key made %d jobs", n)
	}
}

// Many workers, many jobs: every job runs exactly once.
func TestEachJobRunsOnce(t *testing.T) {
	pool := testdb.Open(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const total = 60
	for i := 0; i < total; i++ {
		if _, err := jobs.Enqueue(ctx, pool, "count", map[string]int{"n": i}, jobs.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	seen := map[int]int{}
	var done atomic.Int32
	q := jobs.New(pool)
	q.Handle("count", func(ctx context.Context, raw json.RawMessage) error {
		var p struct{ N int }
		json.Unmarshal(raw, &p)
		mu.Lock()
		seen[p.N]++
		mu.Unlock()
		done.Add(1)
		return nil
	})
	go q.Run(ctx, 8)
	deadline := time.Now().Add(15 * time.Second)
	for done.Load() < total && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if len(seen) != total {
		t.Fatalf("ran %d distinct jobs of %d", len(seen), total)
	}
	for n, c := range seen {
		if c != 1 {
			t.Errorf("job %d ran %d times", n, c)
		}
	}
}

// A failing job is retried later; a permanent failure stops at once.
func TestRetryAndPermanentFailure(t *testing.T) {
	pool := testdb.Open(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs.Enqueue(ctx, pool, "flaky", struct{}{}, jobs.Options{})
	jobs.Enqueue(ctx, pool, "doomed", struct{}{}, jobs.Options{})
	var flaky, doomed atomic.Int32
	q := jobs.New(pool)
	q.Handle("flaky", func(context.Context, json.RawMessage) error {
		flaky.Add(1)
		return errors.New("the museum is down")
	})
	q.Handle("doomed", func(context.Context, json.RawMessage) error {
		doomed.Add(1)
		return jobs.Permanent(errors.New("no such picture"))
	})
	go q.Run(ctx, 2)
	deadline := time.Now().Add(10 * time.Second)
	for (flaky.Load() == 0 || doomed.Load() == 0) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	cancel()

	var status string
	var attempts int
	var runAt time.Time
	pool.QueryRow(context.Background(), `select status, attempts, run_at from jobs where kind = 'flaky'`).Scan(&status, &attempts, &runAt)
	if status != "queued" || attempts != 1 || time.Until(runAt) < 5*time.Second {
		t.Errorf("flaky job: %s after %d attempts, next run in %v", status, attempts, time.Until(runAt).Round(time.Second))
	}
	pool.QueryRow(context.Background(), `select status, attempts from jobs where kind = 'doomed'`).Scan(&status, &attempts)
	if status != "failed" || attempts != 1 || doomed.Load() != 1 {
		t.Errorf("doomed job: %s after %d attempts", status, attempts)
	}
}

func TestBackoffGrows(t *testing.T) {
	prev := time.Duration(0)
	for a := 1; a <= 5; a++ {
		b := jobs.Backoff(a)
		if b <= prev {
			t.Errorf("backoff %d = %v, not more than %v", a, b, prev)
		}
		prev = b
	}
}
