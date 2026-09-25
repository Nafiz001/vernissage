// Package pipeline is the background work: reading the museums, looking at
// every picture, and drawing posters.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nafizahmed/vernissage/server/internal/color"
	"github.com/nafizahmed/vernissage/server/internal/colorindex"
	"github.com/nafizahmed/vernissage/server/internal/db"
	"github.com/nafizahmed/vernissage/server/internal/gallery"
	"github.com/nafizahmed/vernissage/server/internal/imaging"
	"github.com/nafizahmed/vernissage/server/internal/jobs"
	"github.com/nafizahmed/vernissage/server/internal/museum"
	"github.com/nafizahmed/vernissage/server/internal/poster"
	"github.com/nafizahmed/vernissage/server/internal/store"
)

const (
	KindAnalyze = "analyze"
	KindPoster  = "poster"
	KindSync    = "sync"
)

type Pipeline struct {
	Pool      *pgxpool.Pool
	Images    *imaging.Service
	UserAgent string
	SyncEvery time.Duration
	log       *slog.Logger
}

func New(pool *pgxpool.Pool, images *imaging.Service, userAgent string, syncEvery time.Duration) *Pipeline {
	return &Pipeline{Pool: pool, Images: images, UserAgent: userAgent, SyncEvery: syncEvery, log: slog.With("component", "pipeline")}
}

// Register installs the pipeline's job handlers.
func (p *Pipeline) Register(q *jobs.Queue) {
	q.Handle(KindAnalyze, p.analyze)
	q.Handle(KindPoster, p.poster)
	q.Handle(KindSync, p.sync)
}

type idPayload struct {
	ID int64 `json:"id"`
}

// Source is where a work's pictures live.
func Source(a *store.Artwork) imaging.Source {
	return imaging.Source{ID: a.ID, ImageURL: a.ImageURL, LargeURL: a.LargeURL}
}

// analyze downloads a work's picture once, keeps it as the master, and
// records its palette, placeholder and proportions.
func (p *Pipeline) analyze(ctx context.Context, raw json.RawMessage) error {
	var in idPayload
	if err := json.Unmarshal(raw, &in); err != nil {
		return jobs.Permanent(err)
	}
	a, err := store.GetArtwork(ctx, p.Pool, in.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	img, err := p.Images.MasterImage(ctx, Source(a))
	if errors.Is(err, imaging.ErrUnavailable) {
		return jobs.Permanent(err)
	}
	if err != nil {
		return err
	}
	b := img.Bounds()
	aspect := float64(b.Dx()) / float64(b.Dy())
	x, y := 4, 3
	if aspect < 1 {
		x, y = 3, 4
	}
	an := color.Analyze(img)
	if err := store.SaveAnalysis(ctx, p.Pool, a.ID, an, aspect, color.Blurhash(img, x, y)); err != nil {
		return err
	}
	_, err = p.Pool.Exec(ctx, `select pg_notify($1, $2)`, colorindex.Channel, strconv.FormatInt(a.ID, 10))
	return err
}

// EnqueueAnalysis queues every unanalysed work.
func (p *Pipeline) EnqueueAnalysis(ctx context.Context) (int, error) {
	ids, err := store.Unanalyzed(ctx, p.Pool)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		ok, err := jobs.Enqueue(ctx, p.Pool, KindAnalyze, idPayload{id}, jobs.Options{Dedupe: fmt.Sprintf("analyze:%d", id), MaxAttempts: 4})
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// IngestStats counts what a sync did.
type IngestStats struct {
	Seen, Queued int64
}

// Ingest reads every source and upserts what it finds, queueing new or
// re-photographed works for analysis.
func (p *Pipeline) Ingest(ctx context.Context, sources []museum.Source) (IngestStats, error) {
	var st IngestStats
	for _, src := range sources {
		start := time.Now()
		var seen atomic.Int64
		err := src.Fetch(ctx, func(r museum.Record) error {
			id, needs, err := store.UpsertArtwork(ctx, p.Pool, r)
			if err != nil {
				return fmt.Errorf("%s %s: %w", r.Source, r.SourceID, err)
			}
			if n := seen.Add(1); n%250 == 0 {
				p.log.Info("ingesting", "source", src.Name(), "works", n)
			}
			if needs {
				if _, err := jobs.Enqueue(ctx, p.Pool, KindAnalyze, idPayload{id},
					jobs.Options{Dedupe: fmt.Sprintf("analyze:%d", id), MaxAttempts: 4}); err != nil {
					return err
				}
				st.Queued++
			}
			return nil
		})
		st.Seen += seen.Load()
		p.log.Info("ingested", "source", src.Name(), "works", seen.Load(), "took", time.Since(start).Round(time.Second))
		if err != nil {
			return st, err
		}
	}
	return st, nil
}

func (p *Pipeline) sync(ctx context.Context, _ json.RawMessage) error {
	// A full sync takes a while; give it its own clock, not the job lease.
	ctx = context.WithoutCancel(ctx)
	_, err := p.Ingest(ctx, []museum.Source{museum.NewCMA(p.UserAgent, 0), museum.NewMet(p.UserAgent, 0)})
	if p.SyncEvery > 0 {
		p.ScheduleSync(context.Background(), time.Now().Add(p.SyncEvery))
	}
	return err
}

// ScheduleSync queues the next sync, once.
func (p *Pipeline) ScheduleSync(ctx context.Context, at time.Time) error {
	_, err := jobs.Enqueue(ctx, p.Pool, KindSync, struct{}{}, jobs.Options{Dedupe: "sync", RunAt: at, MaxAttempts: 2})
	return err
}

// PosterPath is where an exhibition's current poster is kept.
func (p *Pipeline) PosterPath(e *store.Exhibition) string {
	return filepath.Join(p.Images.Dir(), "posters", fmt.Sprintf("%d-%d.jpg", e.ID, e.PosterVersion))
}

func (p *Pipeline) poster(ctx context.Context, raw json.RawMessage) error {
	var in idPayload
	if err := json.Unmarshal(raw, &in); err != nil {
		return jobs.Permanent(err)
	}
	e, err := store.GetExhibition(ctx, p.Pool, strconv.FormatInt(in.ID, 10))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = p.Poster(ctx, e)
	return err
}

// Poster returns the path of the exhibition's current poster, drawing it
// if needed.
func (p *Pipeline) Poster(ctx context.Context, e *store.Exhibition) (string, error) {
	path := p.PosterPath(e)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	err := p.Images.Once(path, func() error {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		card := poster.Card{Title: e.Title, Curator: e.Owner.Name, Works: e.WorkCount, Paint: "#5b1d1f"}
		for _, paint := range gallery.Paints {
			if paint.Key == e.Paint {
				card.Paint = paint.Hex
			}
		}
		if e.CoverID != nil {
			if a, err := store.GetArtwork(ctx, p.Pool, *e.CoverID); err == nil {
				if img, err := p.Images.MasterImage(ctx, Source(a)); err == nil {
					card.Cover, card.Hang = img, a.Hang()
				}
			}
		}
		return imaging.WriteJPEG(path, poster.Render(card), 88)
	})
	return path, err
}

// QueuePoster asks for the exhibition's poster to be redrawn.
func QueuePoster(ctx context.Context, q db.Querier, id int64) error {
	_, err := jobs.Enqueue(ctx, q, KindPoster, idPayload{id}, jobs.Options{Dedupe: fmt.Sprintf("poster:%d", id), Priority: 10})
	return err
}
