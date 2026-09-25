// Package colorindex answers "which works have this colour in them?" from
// memory.
//
// Every analysed work contributes up to six palette colours with the share
// of the picture each covers. A work's score for a colour is how much of it
// is near that colour: the sum over its palette of share × exp(-(d/σ)²),
// d being the OKLab distance. A seascape that is 60% of the blue asked for
// outranks a portrait with a blue ribbon, and a colour half a shade away
// still counts. Several colours at once multiply, so every one must be
// present.
//
// At ten thousand works a full scan takes a couple of milliseconds, which
// is simpler and faster than any tree once the Gaussian falloff is
// involved. The index is loaded at start-up and kept current by
// PostgreSQL notifications, so every server instance sees new analyses.
package colorindex

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nafizahmed/vernissage/server/internal/color"
)

const (
	// Sigma is the colour-matching falloff: a palette colour 0.055 away in
	// OKLab (a clearly different shade of the same colour) counts for about
	// a third; one 0.11 away (another colour) for almost nothing.
	Sigma = 0.055
	// MinScore drops works in which the colour barely appears.
	MinScore = 0.05
	// Channel carries the ids of newly analysed works.
	Channel = "vernissage_analyzed"
)

type entry struct {
	id           int64
	kind         string
	source       string
	year         int
	colors       [6]color.Lab
	weights      [6]float64
	n            int
	lightness    float64
	colorfulness float64
}

// Moods are quartile bounds on lightness and colourfulness.
type Moods struct {
	BrightMin float64 `json:"brightMin"`
	DarkMax   float64 `json:"darkMax"`
	VividMin  float64 `json:"vividMin"`
	MutedMax  float64 `json:"mutedMax"`
}

type Index struct {
	mu      sync.RWMutex
	entries []entry
	pos     map[int64]int
	moods   Moods
	// stale means works were added since the mood bounds were computed.
	stale bool
}

func New() *Index { return &Index{pos: map[int64]int{}} }

// Query is a colour search with optional filters.
type Query struct {
	Colors   []color.Lab
	Kinds    []string
	Source   string
	From, To *int
	// Allow restricts results to these ids (from a text search), if set.
	Allow map[int64]bool
	// Mood is bright, dark, vivid or muted.
	Mood string
}

type Hit struct {
	ID    int64
	Score float64
}

// Len is the number of works indexed.
func (x *Index) Len() int {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return len(x.entries)
}

// Moods returns the current quartile bounds.
func (x *Index) Moods() Moods {
	x.refreshMoods()
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.moods
}

// Search ranks every matching work by how much of the colours it holds.
func (x *Index) Search(q Query) []Hit {
	x.refreshMoods()
	x.mu.RLock()
	defer x.mu.RUnlock()
	kinds := map[string]bool{}
	for _, k := range q.Kinds {
		kinds[k] = true
	}
	var hits []Hit
	for i := range x.entries {
		e := &x.entries[i]
		if len(kinds) > 0 && !kinds[e.kind] ||
			q.Source != "" && e.source != q.Source ||
			q.From != nil && (e.year == 0 || e.year < *q.From) ||
			q.To != nil && (e.year == 0 || e.year > *q.To) ||
			q.Allow != nil && !q.Allow[e.id] ||
			!x.moodOK(e, q.Mood) {
			continue
		}
		score := 1.0
		for _, c := range q.Colors {
			s := e.score(c)
			if s < MinScore {
				score = 0
				break
			}
			score *= s
		}
		if score > 0 {
			if len(q.Colors) > 1 {
				score = math.Pow(score, 1/float64(len(q.Colors)))
			}
			hits = append(hits, Hit{e.id, score})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
	return hits
}

func (x *Index) moodOK(e *entry, mood string) bool {
	switch mood {
	case "bright":
		return e.lightness >= x.moods.BrightMin
	case "dark":
		return e.lightness <= x.moods.DarkMax
	case "vivid":
		return e.colorfulness >= x.moods.VividMin
	case "muted":
		return e.colorfulness <= x.moods.MutedMax
	}
	return true
}

func (e *entry) score(c color.Lab) float64 {
	s := 0.0
	for i := 0; i < e.n; i++ {
		d := Match(c, e.colors[i]) / Sigma
		s += e.weights[i] * math.Exp(-d*d)
	}
	return s
}

// Match is how far a palette colour p is from what someone asked for, q.
//
// A palette colour is the average of a cluster of pixels, so it is always
// more muted than the paint on the canvas: the ultramarine of a Virgin's
// robe averages out to a greyer blue. Plain distance would say no painting
// has ultramarine in it. So hue counts in full, lightness a little less,
// and being less saturated than asked costs little, down to about half the
// chroma asked for. Past that it costs in full, so greys don't pass for
// blues.
func Match(q, p color.Lab) float64 {
	cq, cp := q.Chroma(), p.Chroma()
	da, db := q.A-p.A, q.B-p.B
	dC := cq - cp
	dH2 := math.Max(0, da*da+db*db-dC*dC)
	switch {
	case dC <= 0:
		dC *= 0.8 // more vivid than asked: fine, mostly
	case cp >= 0.45*cq:
		dC *= 0.4
	default:
		dC = 0.55*cq*0.4 + 1.5*(0.45*cq-cp)
	}
	dL := 0.75 * (q.L - p.L)
	return math.Sqrt(dL*dL + dC*dC + dH2)
}

// Similar ranks works by how alike their whole palettes are: each colour of
// one palette matched softly against every colour of the other, weighted by
// both shares, normalised so a work is exactly 1 similar to itself.
func (x *Index) Similar(id int64, limit int) []Hit {
	x.mu.RLock()
	defer x.mu.RUnlock()
	i, ok := x.pos[id]
	if !ok {
		return nil
	}
	me := &x.entries[i]
	self := overlap(me, me)
	if self == 0 {
		return nil
	}
	var hits []Hit
	for j := range x.entries {
		e := &x.entries[j]
		if e.id == id {
			continue
		}
		s := overlap(me, e) / math.Sqrt(self*overlap(e, e))
		if s > 0.2 {
			hits = append(hits, Hit{e.id, s})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func overlap(a, b *entry) float64 {
	s := 0.0
	for i := 0; i < a.n; i++ {
		for j := 0; j < b.n; j++ {
			d := color.Dist(a.colors[i], b.colors[j]) / Sigma
			s += a.weights[i] * b.weights[j] * math.Exp(-d*d)
		}
	}
	return s
}

// Work is one row as the index needs it.
type Work struct {
	ID           int64
	Kind         string
	Source       string
	Year         *int
	Palette      []color.Swatch
	Lightness    float64
	Colorfulness float64
}

// Put adds or replaces a work.
func (x *Index) Put(w Work) {
	e := entry{id: w.ID, kind: w.Kind, source: w.Source, lightness: w.Lightness, colorfulness: w.Colorfulness}
	if w.Year != nil {
		e.year = *w.Year
	}
	for i, s := range w.Palette {
		if i == len(e.colors) {
			break
		}
		e.colors[i] = s.Lab()
		e.weights[i] = s.Weight
		e.n++
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.stale = true
	if i, ok := x.pos[w.ID]; ok {
		x.entries[i] = e
	} else {
		x.pos[w.ID] = len(x.entries)
		x.entries = append(x.entries, e)
	}
}

// refreshMoods recomputes the mood bounds if works arrived since the last
// time. Sorting ten thousand numbers takes well under a millisecond.
func (x *Index) refreshMoods() {
	x.mu.RLock()
	stale := x.stale
	x.mu.RUnlock()
	if stale {
		x.mu.Lock()
		if x.stale {
			x.computeMoods()
			x.stale = false
		}
		x.mu.Unlock()
	}
}

// computeMoods sets mood bounds at the quartiles. Called with the lock held.
func (x *Index) computeMoods() {
	n := len(x.entries)
	if n == 0 {
		return
	}
	l := make([]float64, n)
	c := make([]float64, n)
	for i, e := range x.entries {
		l[i], c[i] = e.lightness, e.colorfulness
	}
	sort.Float64s(l)
	sort.Float64s(c)
	q := func(v []float64, p float64) float64 { return v[int(p*float64(len(v)-1))] }
	x.moods = Moods{BrightMin: q(l, 0.75), DarkMax: q(l, 0.25), VividMin: q(c, 0.75), MutedMax: q(c, 0.25)}
}

const loadSQL = `select id, kind, source, year_start, palette, lightness, colorfulness
	from artworks where analyzed_at is not null and palette is not null`

// Load reads every analysed work.
func (x *Index) Load(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, loadSQL)
	if err != nil {
		return err
	}
	defer rows.Close()
	fresh := New()
	for rows.Next() {
		w, err := scanWork(rows.Scan)
		if err != nil {
			return err
		}
		fresh.Put(w)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	x.mu.Lock()
	x.entries, x.pos = fresh.entries, fresh.pos
	x.computeMoods()
	x.mu.Unlock()
	return nil
}

func scanWork(scan func(...any) error) (Work, error) {
	var w Work
	var palette []byte
	var l, c float32
	if err := scan(&w.ID, &w.Kind, &w.Source, &w.Year, &palette, &l, &c); err != nil {
		return w, err
	}
	w.Lightness, w.Colorfulness = float64(l), float64(c)
	return w, json.Unmarshal(palette, &w.Palette)
}

// Follow keeps the index current: it listens for analysed works' ids and
// adds them, and reloads in full now and then to settle mood bounds.
func (x *Index) Follow(ctx context.Context, pool *pgxpool.Pool) {
	log := slog.With("component", "colorindex")
	lastFull := time.Now()
	for ctx.Err() == nil {
		err := func() error {
			conn, err := pool.Acquire(ctx)
			if err != nil {
				return err
			}
			defer conn.Release()
			if _, err := conn.Exec(ctx, "listen "+Channel); err != nil {
				return err
			}
			for {
				wctx, cancel := context.WithTimeout(ctx, time.Minute)
				n, err := conn.Conn().WaitForNotification(wctx)
				cancel()
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if time.Since(lastFull) > 10*time.Minute {
					if err := x.Load(ctx, pool); err != nil {
						log.Warn("reload", "err", err)
					}
					lastFull = time.Now()
				}
				if err != nil {
					continue // timeout: loop to check the reload clock
				}
				id, perr := strconv.ParseInt(n.Payload, 10, 64)
				if perr != nil {
					continue
				}
				w, err := scanWork(pool.QueryRow(ctx, loadSQL+` and id = $1`, id).Scan)
				if err != nil {
					log.Warn("load analysed work", "id", id, "err", err)
					continue
				}
				x.Put(w)
			}
		}()
		if ctx.Err() == nil {
			log.Warn("listen interrupted", "err", err)
			time.Sleep(2 * time.Second)
		}
	}
}
