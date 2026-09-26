// Command vernissage serves the site and runs its background work.
//
//	vernissage serve     the API, the live rooms and the job workers
//	vernissage migrate   bring the database schema up to date
//	vernissage ingest    read the museums' collections and queue analysis
//	vernissage analyze   queue analysis for every work not yet looked at
//	vernissage seed      hang a few exhibitions to walk through
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nafiz001/vernissage/server/internal/api"
	"github.com/Nafiz001/vernissage/server/internal/colorindex"
	"github.com/Nafiz001/vernissage/server/internal/config"
	"github.com/Nafiz001/vernissage/server/internal/db"
	"github.com/Nafiz001/vernissage/server/internal/imaging"
	"github.com/Nafiz001/vernissage/server/internal/jobs"
	"github.com/Nafiz001/vernissage/server/internal/live"
	"github.com/Nafiz001/vernissage/server/internal/museum"
	"github.com/Nafiz001/vernissage/server/internal/pipeline"
	"github.com/Nafiz001/vernissage/server/internal/seed"
	"github.com/Nafiz001/vernissage/server/internal/store"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("VERNISSAGE_DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	var err error
	switch cmd {
	case "serve":
		err = serve(ctx, cfg)
	case "migrate":
		err = withDB(ctx, cfg, func(*pgxpool.Pool) error { return nil })
	case "ingest":
		err = ingest(ctx, cfg, args)
	case "analyze":
		err = withDB(ctx, cfg, func(pool *pgxpool.Pool) error {
			n, err := pipeline.New(pool, nil, cfg.UserAgent, 0).EnqueueAnalysis(ctx)
			slog.Info("queued for analysis", "works", n)
			return err
		})
	case "seed":
		err = withDB(ctx, cfg, func(pool *pgxpool.Pool) error {
			index := colorindex.New()
			if err := index.Load(ctx, pool); err != nil {
				return err
			}
			return seed.Run(ctx, pool, index)
		})
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q; use serve, migrate, ingest, analyze or seed\n", cmd)
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error(cmd+" failed", "err", err)
		os.Exit(1)
	}
}

// withDB opens the database, migrates it and runs fn.
func withDB(ctx context.Context, cfg config.Config, fn func(*pgxpool.Pool) error) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	return fn(pool)
}

func ingest(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	sources := fs.String("sources", "cma,met", "museums to read: cma, met")
	limit := fs.Int("limit", 0, "stop after this many works per query (0: all)")
	fs.Parse(args)
	return withDB(ctx, cfg, func(pool *pgxpool.Pool) error {
		var list []museum.Source
		for _, s := range strings.Split(*sources, ",") {
			switch strings.TrimSpace(s) {
			case "cma":
				list = append(list, museum.NewCMA(cfg.UserAgent, *limit))
			case "met":
				list = append(list, museum.NewMet(cfg.UserAgent, *limit))
			default:
				return fmt.Errorf("unknown source %q", s)
			}
		}
		st, err := pipeline.New(pool, nil, cfg.UserAgent, 0).Ingest(ctx, list)
		slog.Info("ingest finished", "works", st.Seen, "queued", st.Queued)
		return err
	})
}

func serve(ctx context.Context, cfg config.Config) error {
	return withDB(ctx, cfg, func(pool *pgxpool.Pool) error {
		images, err := imaging.New(cfg.CacheDir, cfg.UserAgent)
		if err != nil {
			return err
		}
		index := colorindex.New()
		if err := index.Load(ctx, pool); err != nil {
			return err
		}
		slog.Info("colour index loaded", "works", index.Len())

		pipe := pipeline.New(pool, images, cfg.UserAgent, cfg.SyncEvery)
		queue := jobs.New(pool)
		pipe.Register(queue)
		if cfg.SyncEvery > 0 {
			pipe.ScheduleSync(ctx, time.Now().Add(cfg.SyncEvery))
		}

		// Every request, WebSockets included, ends when bg does.
		bg, cancel := context.WithCancel(ctx)
		defer cancel()
		hub := live.NewHub(api.Guestbook{Pool: pool})
		srv := &http.Server{
			Addr:              cfg.Addr,
			Handler:           api.New(cfg, pool, images, index, hub, pipe).Handler(),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			BaseContext:       func(net.Listener) context.Context { return bg },
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			queue.Run(bg, cfg.Workers)
		}()
		go index.Follow(bg, pool)
		go images.Janitor(bg, cfg.CacheBudget, 10*time.Minute)
		go func() {
			for {
				if n, err := store.DeleteExpiredSessions(bg, pool); err == nil && n > 0 {
					slog.Info("expired sessions removed", "count", n)
				}
				select {
				case <-bg.Done():
					return
				case <-time.After(time.Hour):
				}
			}
		}()

		errc := make(chan error, 1)
		go func() {
			slog.Info("listening", "addr", cfg.Addr)
			errc <- srv.ListenAndServe()
		}()
		select {
		case err := <-errc:
			return err
		case <-ctx.Done():
		}
		slog.Info("shutting down")
		shut, done2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer done2()
		srv.Shutdown(shut)
		cancel()
		<-done
		return nil
	})
}
