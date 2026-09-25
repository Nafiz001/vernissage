// Package store reads and writes the database.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/nafizahmed/vernissage/server/internal/color"
	"github.com/nafizahmed/vernissage/server/internal/db"
	"github.com/nafizahmed/vernissage/server/internal/gallery"
	"github.com/nafizahmed/vernissage/server/internal/museum"
)

var ErrNotFound = errors.New("not found")

type Artwork struct {
	ID           int64
	Source       string
	SourceID     string
	Title        string
	Artist       string
	ArtistBio    string
	DateText     string
	YearStart    *int
	YearEnd      *int
	Medium       string
	Kind         string
	Culture      string
	Department   string
	Description  string
	Credit       string
	SourceURL    string
	ImageURL     string
	LargeURL     string
	WidthCM      *float64
	HeightCM     *float64
	Highlight    bool
	AnalyzedAt   *time.Time
	Aspect       *float64
	Palette      []color.Swatch
	Dominant     string
	Blurhash     string
	Lightness    *float64
	Colorfulness *float64
}

// Hang frames and sizes the work for a wall.
func (a *Artwork) Hang() gallery.Hang {
	return gallery.HangFor(gallery.Work{
		Kind: a.Kind, YearStart: a.YearStart, Culture: a.Culture, Department: a.Department,
		WidthCM: a.WidthCM, HeightCM: a.HeightCM, Aspect: a.Aspect,
	})
}

const artworkCols = `a.id, a.source, a.source_id, a.title, a.artist, a.artist_bio, a.date_text,
	a.year_start, a.year_end, a.medium, a.kind, a.culture, a.department, a.description, a.credit,
	a.source_url, a.image_url, a.large_url, a.width_cm, a.height_cm, a.highlight, a.analyzed_at,
	a.aspect, a.palette, coalesce(a.dominant, ''), coalesce(a.blurhash, ''), a.lightness, a.colorfulness`

func scanArtwork(row pgx.Row, extra ...any) (*Artwork, error) {
	var a Artwork
	var palette []byte
	var w, h, aspect, light, colorful *float32
	dest := []any{&a.ID, &a.Source, &a.SourceID, &a.Title, &a.Artist, &a.ArtistBio, &a.DateText,
		&a.YearStart, &a.YearEnd, &a.Medium, &a.Kind, &a.Culture, &a.Department, &a.Description, &a.Credit,
		&a.SourceURL, &a.ImageURL, &a.LargeURL, &w, &h, &a.Highlight, &a.AnalyzedAt,
		&aspect, &palette, &a.Dominant, &a.Blurhash, &light, &colorful}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.WidthCM, a.HeightCM, a.Aspect, a.Lightness, a.Colorfulness = f64(w), f64(h), f64(aspect), f64(light), f64(colorful)
	if len(palette) > 0 {
		if err := json.Unmarshal(palette, &a.Palette); err != nil {
			return nil, err
		}
	}
	return &a, nil
}

func f64(v *float32) *float64 {
	if v == nil {
		return nil
	}
	x := float64(*v)
	return &x
}

// UpsertArtwork records a work read from a museum and reports its id and
// whether its picture still needs looking at.
func UpsertArtwork(ctx context.Context, q db.Querier, r museum.Record) (id int64, needsAnalysis bool, err error) {
	var analyzed *time.Time
	var oldImage string
	err = q.QueryRow(ctx, `
		with prev as (select image_url from artworks where source = $1 and source_id = $2)
		insert into artworks (source, source_id, title, artist, artist_bio, date_text, year_start, year_end,
			medium, kind, culture, department, description, credit, source_url, image_url, large_url,
			width_cm, height_cm, highlight)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		on conflict (source, source_id) do update set
			title = excluded.title, artist = excluded.artist, artist_bio = excluded.artist_bio,
			date_text = excluded.date_text, year_start = excluded.year_start, year_end = excluded.year_end,
			medium = excluded.medium, kind = excluded.kind, culture = excluded.culture,
			department = excluded.department, description = excluded.description, credit = excluded.credit,
			source_url = excluded.source_url, image_url = excluded.image_url, large_url = excluded.large_url,
			width_cm = excluded.width_cm, height_cm = excluded.height_cm, highlight = excluded.highlight,
			updated_at = now()
		returning id, analyzed_at, coalesce((select image_url from prev), '')`,
		r.Source, r.SourceID, r.Title, r.Artist, r.ArtistBio, r.DateText, r.YearStart, r.YearEnd,
		r.Medium, r.Kind, r.Culture, r.Department, r.Description, r.Credit, r.SourceURL, r.ImageURL, r.LargeURL,
		r.WidthCM, r.HeightCM, r.Highlight).Scan(&id, &analyzed, &oldImage)
	if err != nil {
		return 0, false, err
	}
	// A new photograph means a new palette.
	return id, analyzed == nil || (oldImage != "" && oldImage != r.ImageURL), nil
}

// SaveAnalysis stores what the analyze job learned from the picture.
func SaveAnalysis(ctx context.Context, q db.Querier, id int64, a color.Analysis, aspect float64, blurhash string) error {
	palette, err := json.Marshal(a.Palette)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		update artworks set analyzed_at = now(), aspect = $2, palette = $3, dominant = $4,
			blurhash = $5, lightness = $6, colorfulness = $7
		where id = $1`, id, aspect, palette, a.Dominant(), blurhash, a.Lightness, a.Colorfulness)
	return err
}

func GetArtwork(ctx context.Context, q db.Querier, id int64) (*Artwork, error) {
	return scanArtwork(q.QueryRow(ctx, `select `+artworkCols+` from artworks a where a.id = $1`, id))
}

// GetArtworks fetches works by id, in the order asked for, skipping any
// that don't exist.
func GetArtworks(ctx context.Context, q db.Querier, ids []int64) ([]*Artwork, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx, `select `+artworkCols+` from artworks a where a.id = any($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[int64]*Artwork{}
	for rows.Next() {
		a, err := scanArtwork(rows)
		if err != nil {
			return nil, err
		}
		byID[a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*Artwork, 0, len(ids))
	for _, id := range ids {
		if a, ok := byID[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// Filter narrows a listing of the collection.
type Filter struct {
	Query  string
	Kinds  []string
	From   *int
	To     *int
	Source string
	Artist string
	// Mood bounds, from the colour index's quartiles.
	LightMin, LightMax, ColorMin, ColorMax *float64
	// Seed shuffles the unsearched listing the same way every page.
	Seed   int
	Limit  int
	Offset int
}

func (f Filter) where(args *[]any) string {
	var conds []string
	add := func(cond string, v any) {
		*args = append(*args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(*args))))
	}
	if len(f.Kinds) > 0 {
		add("a.kind = any(?)", f.Kinds)
	}
	if f.From != nil {
		add("coalesce(a.year_end, a.year_start) >= ?", *f.From)
	}
	if f.To != nil {
		add("a.year_start <= ?", *f.To)
	}
	if f.Source != "" {
		add("a.source = ?", f.Source)
	}
	if f.Artist != "" {
		add("a.artist = ?", f.Artist)
	}
	if f.LightMin != nil {
		add("a.lightness >= ?", *f.LightMin)
	}
	if f.LightMax != nil {
		add("a.lightness <= ?", *f.LightMax)
	}
	if f.ColorMin != nil {
		add("a.colorfulness >= ?", *f.ColorMin)
	}
	if f.ColorMax != nil {
		add("a.colorfulness <= ?", *f.ColorMax)
	}
	if len(conds) == 0 {
		return "true"
	}
	return strings.Join(conds, " and ")
}

// ListArtworks pages through the collection. With a query it ranks by
// relevance, falling back to fuzzy matching of titles and artists when the
// words match nothing (so "van gohg" still finds Van Gogh). It reports the
// total and whether the fuzzy fallback was used.
func ListArtworks(ctx context.Context, q db.Querier, f Filter) (list []*Artwork, total int, fuzzy bool, err error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 40
	}
	var args []any
	where := f.where(&args)
	query := strings.TrimSpace(f.Query)

	run := func(sql string, args []any) ([]*Artwork, int, error) {
		rows, err := q.Query(ctx, sql, args...)
		if err != nil {
			return nil, 0, err
		}
		defer rows.Close()
		var out []*Artwork
		var n int
		for rows.Next() {
			a, err := scanArtwork(rows, &n)
			if err != nil {
				return nil, 0, err
			}
			a.Description = ""
			out = append(out, a)
		}
		return out, n, rows.Err()
	}

	page := fmt.Sprintf(" limit %d offset %d", f.Limit, max(0, f.Offset))
	if query == "" {
		a := append(args, f.Seed)
		list, total, err = run(`select `+artworkCols+`, count(*) over () from artworks a where `+where+`
			order by a.highlight desc, (a.analyzed_at is not null) desc,
			         hashtextextended(a.id::text, $`+fmt.Sprint(len(a))+`)`+page, a)
		return list, total, false, err
	}

	a := append(args, query)
	n := len(a)
	list, total, err = run(fmt.Sprintf(`select %s, count(*) over () from artworks a,
			websearch_to_tsquery('english', immutable_unaccent($%d)) tsq
		where %s and a.search @@ tsq
		order by ts_rank_cd(a.search, tsq, 32) * (case when a.highlight then 1.3 else 1 end) desc, a.id`,
		artworkCols, n, where)+page, a)
	if err != nil || total > 0 || f.Offset > 0 {
		return list, total, false, err
	}
	list, total, err = run(fmt.Sprintf(`select %s, count(*) over () from artworks a,
			lower(immutable_unaccent($%d)) needle
		where %s and (needle <%% lower(immutable_unaccent(a.artist)) or needle <%% lower(immutable_unaccent(a.title)))
		order by greatest(word_similarity(needle, lower(immutable_unaccent(a.artist))),
		                  word_similarity(needle, lower(immutable_unaccent(a.title)))) desc, a.highlight desc, a.id`,
		artworkCols, n, where)+page, a)
	return list, total, true, err
}

// MatchingIDs lists every work matching the filter, text included, for
// intersecting with a colour search. It stops at limit.
func MatchingIDs(ctx context.Context, q db.Querier, f Filter, limit int) ([]int64, error) {
	var args []any
	where := f.where(&args)
	sql := `select a.id from artworks a where ` + where
	if query := strings.TrimSpace(f.Query); query != "" {
		args = append(args, query)
		sql += fmt.Sprintf(` and a.search @@ websearch_to_tsquery('english', immutable_unaccent($%d))`, len(args))
	}
	rows, err := q.Query(ctx, sql+fmt.Sprintf(" limit %d", limit), args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// Suggestion is a search-as-you-type hint.
type Suggestion struct {
	Artists []ArtistCount `json:"artists"`
	Works   []WorkHint    `json:"works"`
}

type ArtistCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type WorkHint struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Blurhash string `json:"blurhash"`
}

// Suggest finds artists and titles that start with or closely resemble q.
func Suggest(ctx context.Context, q db.Querier, text string) (Suggestion, error) {
	s := Suggestion{Artists: []ArtistCount{}, Works: []WorkHint{}}
	text = strings.TrimSpace(text)
	if len(text) < 2 {
		return s, nil
	}
	rows, err := q.Query(ctx, `
		select artist, count(*) from artworks, lower(immutable_unaccent($1)) needle
		where artist <> '' and (lower(immutable_unaccent(artist)) like '%' || needle || '%'
		                        or needle <% lower(immutable_unaccent(artist)))
		group by artist
		order by max(word_similarity(needle, lower(immutable_unaccent(artist)))) desc, count(*) desc
		limit 5`, text)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var a ArtistCount
		if err := rows.Scan(&a.Name, &a.Count); err != nil {
			rows.Close()
			return s, err
		}
		s.Artists = append(s.Artists, a)
	}
	rows.Close()

	rows, err = q.Query(ctx, `
		select id, title, artist, coalesce(blurhash, '') from artworks, lower(immutable_unaccent($1)) needle
		where lower(immutable_unaccent(title)) like '%' || needle || '%'
		order by highlight desc, length(title), id
		limit 6`, text)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var w WorkHint
		if err := rows.Scan(&w.ID, &w.Title, &w.Artist, &w.Blurhash); err != nil {
			return s, err
		}
		s.Works = append(s.Works, w)
	}
	return s, rows.Err()
}

// CollectionStats are the headline numbers.
type CollectionStats struct {
	Works       int `json:"works"`
	Analyzed    int `json:"analyzed"`
	Artists     int `json:"artists"`
	Paintings   int `json:"paintings"`
	Prints      int `json:"prints"`
	Drawings    int `json:"drawings"`
	Earliest    int `json:"earliest"`
	Latest      int `json:"latest"`
	Exhibitions int `json:"exhibitions"`
}

func Stats(ctx context.Context, q db.Querier) (CollectionStats, error) {
	var s CollectionStats
	err := q.QueryRow(ctx, `
		select count(*), count(analyzed_at), count(distinct nullif(artist, '')),
		       count(*) filter (where kind = 'painting'), count(*) filter (where kind = 'print'),
		       count(*) filter (where kind = 'drawing'),
		       coalesce(percentile_disc(0.005) within group (order by year_start), 0),
		       coalesce(max(year_start), 0),
		       (select count(*) from exhibitions where status = 'published')
		from artworks`).Scan(&s.Works, &s.Analyzed, &s.Artists, &s.Paintings, &s.Prints, &s.Drawings,
		&s.Earliest, &s.Latest, &s.Exhibitions)
	return s, err
}

// Unanalyzed lists works whose pictures haven't been looked at.
func Unanalyzed(ctx context.Context, q db.Querier) ([]int64, error) {
	rows, err := q.Query(ctx, `select id from artworks where analyzed_at is null order by highlight desc, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}
