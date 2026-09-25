// Package testdb gives integration tests an empty, migrated database.
//
// Tests that need PostgreSQL call Open; without VERNISSAGE_TEST_DATABASE_URL
// set they are skipped, so `go test ./...` works anywhere and CI runs them
// against a real server.
package testdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nafizahmed/vernissage/server/internal/color"
	"github.com/nafizahmed/vernissage/server/internal/db"
)

// Tests in different packages run in parallel processes against one
// database; each takes this lock for its whole run.
const lockKey = 90_210

var mu sync.Mutex

// Open returns a pool on a freshly reset schema, holding a lock so no other
// test package uses the database at the same time.
func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("VERNISSAGE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("VERNISSAGE_TEST_DATABASE_URL not set")
	}
	mu.Lock()
	t.Cleanup(mu.Unlock)
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1)`, lockKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Exec(context.Background(), `select pg_advisory_unlock($1)`, lockKey)
		conn.Release()
		pool.Close()
	})
	if _, err := pool.Exec(ctx, `drop schema public cascade; create schema public`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// Work is a fixture artwork, already analysed.
type Work struct {
	Title, Artist, Kind, Description string
	Year                             int
	WidthCM, HeightCM                float64
	Colors                           []string // hex, most widespread first
}

// Insert adds analysed works and returns their ids in order.
func Insert(t *testing.T, pool *pgxpool.Pool, works ...Work) []int64 {
	t.Helper()
	var ids []int64
	for i, w := range works {
		var palette []color.Swatch
		for j, hex := range w.Colors {
			c, err := color.ParseHex(hex)
			if err != nil {
				t.Fatal(err)
			}
			palette = append(palette, color.Swatch{Hex: hex, L: c.L, A: c.A, B: c.B, Weight: 1 / float64(j+1)})
		}
		sum := 0.0
		for _, s := range palette {
			sum += s.Weight
		}
		for j := range palette {
			palette[j].Weight /= sum
		}
		pj, _ := json.Marshal(palette)
		var id int64
		err := pool.QueryRow(context.Background(), `
			insert into artworks (source, source_id, title, artist, kind, description, year_start, year_end,
				source_url, image_url, large_url, width_cm, height_cm,
				analyzed_at, aspect, palette, dominant, blurhash, lightness, colorfulness)
			values ('met', $1, $2, $3, $4, $5, $6, $6, 'https://example.org', 'https://example.org/a.jpg',
				'https://example.org/b.jpg', $7, $8, now(), $9, $10, $11, 'L00000fQfQfQfQfQfQfQfQfQfQfQ', 0.5, 30)
			returning id`,
			fmt.Sprint(1000+i), w.Title, w.Artist, w.Kind, w.Description, w.Year, w.WidthCM, w.HeightCM,
			w.WidthCM/w.HeightCM, pj, w.Colors[0]).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}
