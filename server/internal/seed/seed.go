// Package seed hangs a handful of exhibitions so there is something to walk
// through on a fresh install. The works are chosen with the same searches a
// curator would use (words, colours, moods) and hung by AutoHang.
package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nafiz001/vernissage/server/internal/auth"
	"github.com/Nafiz001/vernissage/server/internal/color"
	"github.com/Nafiz001/vernissage/server/internal/colorindex"
	"github.com/Nafiz001/vernissage/server/internal/gallery"
	"github.com/Nafiz001/vernissage/server/internal/live"
	"github.com/Nafiz001/vernissage/server/internal/pipeline"
	"github.com/Nafiz001/vernissage/server/internal/store"
)

// DemoEmail and DemoPassword sign in as the first curator, to try the
// studio without making an account.
const (
	DemoEmail    = "demo@vernissage.local"
	DemoPassword = "open-the-doors"
)

type curator struct{ name, email string }

var curators = []curator{
	{"Ines Albrecht", DemoEmail},
	{"Yuki Morimoto", "yuki@vernissage.local"},
	{"Tomás Ferreira", "tomas@vernissage.local"},
	{"Clara Wensley", "clara@vernissage.local"},
}

type show struct {
	curator                   int
	title, statement          string
	room, paint, floor, light string
	query                     string
	kinds                     []string
	colors                    []string
	mood                      string
	to                        *int
	n, perArtist              int
	publishedAgo, opensIn     time.Duration
}

func year(y int) *int { return &y }

var shows = []show{
	{
		curator: 0, title: "The Colour of Water",
		statement: "Painters have never agreed on what colour water is. Some saw it burn at sunset, some carved it into claws, " +
			"some let it dissolve into the sky above it. This room hangs their answers side by side along one long walk, " +
			"so you can go from one kind of water to the next and decide for yourself.",
		room: "long", paint: "prussian", floor: "concrete", light: "daylight",
		query: "sea or river or water or wave or harbor or boats or lake", kinds: []string{"painting", "print"},
		colors: []string{"#3a6690"}, n: 14, perArtist: 2, publishedAgo: 9 * 24 * time.Hour,
	},
	{
		curator: 1, title: "Floating World",
		statement: "Ukiyo-e, pictures of the floating world, were the posters and postcards of Edo: bright, cheap, and printed by " +
			"the thousand from carved cherry-wood blocks. Some of the most famous images in art were sold for about the price " +
			"of a bowl of noodles. They hang here at their real size, which is smaller than most people expect.",
		room: "cabinet", paint: "sage", floor: "walnut", light: "gallery",
		query: "Hokusai or Hiroshige or Utamaro or Kuniyoshi or Harunobu or Eisen", kinds: []string{"print"},
		n: 12, perArtist: 4, publishedAgo: 6 * 24 * time.Hour,
	},
	{
		curator: 2, title: "After Dark",
		statement: "Before electric light, night was something a painter had to invent. These are pictures lit by the moon, by a " +
			"candle, by a lamp just out of frame, and by almost nothing at all. They reward standing close and giving your " +
			"eyes a moment to adjust.",
		room: "salon", paint: "lamp", floor: "walnut", light: "evening",
		query: "night or moon or moonlight or evening or candle or lamp or nocturne", kinds: []string{"painting"},
		mood: "dark", n: 12, perArtist: 2, publishedAgo: 3 * 24 * time.Hour,
	},
	{
		curator: 3, title: "Gold Ground",
		statement: "For two centuries of European painting, heaven was made of gold leaf, beaten thinner than paper, laid over red " +
			"clay and burnished until it shone. Saints stand on it instead of in a landscape. Look closely at the haloes: " +
			"their patterns were punched into the gold with tiny tools, one mark at a time.",
		room: "salon", paint: "oxblood", floor: "marble", light: "gallery",
		query: "madonna or virgin or saint or christ or altarpiece or angel or crucifixion", kinds: []string{"painting"},
		colors: []string{"#b8913f"}, to: year(1520),
		n: 11, perArtist: 2, publishedAgo: 2 * 24 * time.Hour,
	},
	{
		curator: 0, title: "Summer Fields",
		statement: "Wheat, hay, poppies and heat. A season's worth of fields, meadows and harvests, hung in a bright room with " +
			"nothing between the pictures but white wall.",
		room: "cube", paint: "chalk", floor: "oak", light: "daylight",
		query: "field or fields or harvest or wheat or meadow or hay or poppies", kinds: []string{"painting"},
		colors: []string{"#c9a94e"}, n: 12, perArtist: 2, publishedAgo: 20 * time.Hour, opensIn: 3*24*time.Hour + 7*time.Hour,
	},
	{
		curator: 2, title: "Sitters",
		statement: "Everyone in this room paid, or was paid, to sit still. For four hundred years portraits were the pictures " +
			"most often commissioned in Europe: records of marriages, promotions, inheritances and vanity. Look at the " +
			"hands; they often say more than the faces.",
		room: "salon", paint: "verdigris", floor: "oak", light: "gallery",
		query: "portrait", kinds: []string{"painting"}, mood: "muted",
		n: 12, perArtist: 1, publishedAgo: 5 * time.Hour,
	},
}

// Run creates the demo curators and their exhibitions, once.
func Run(ctx context.Context, pool *pgxpool.Pool, index *colorindex.Index) error {
	if _, err := store.UserByEmail(ctx, pool, DemoEmail); err == nil {
		slog.Info("already seeded")
		return nil
	}
	if index.Len() < 200 {
		return errors.New("analyse the collection first (vernissage ingest, then let serve work through the queue)")
	}
	hash, err := auth.HashPassword(DemoPassword)
	if err != nil {
		return err
	}
	var users []*store.User
	for _, c := range curators {
		u := &store.User{Name: c.name, Email: c.email, Handle: store.Slugify(c.name), PasswordHash: hash}
		_, u.Hue = live.Pigment(c.email)
		if err := store.CreateUser(ctx, pool, u); err != nil {
			return err
		}
		users = append(users, u)
	}
	for _, sh := range shows {
		if err := hang(ctx, pool, index, users[sh.curator], sh); err != nil {
			return fmt.Errorf("%s: %w", sh.title, err)
		}
	}
	slog.Info("seeded", "curators", len(users), "exhibitions", len(shows), "demo", DemoEmail, "password", DemoPassword)
	return nil
}

func pick(ctx context.Context, pool *pgxpool.Pool, index *colorindex.Index, sh show) ([]*store.Artwork, error) {
	f := store.Filter{Query: sh.query, Kinds: sh.kinds, To: sh.to, Limit: 100}
	var candidates []*store.Artwork
	if len(sh.colors) > 0 || sh.mood != "" {
		q := colorindex.Query{Kinds: sh.kinds, To: sh.to, Mood: sh.mood}
		for _, hex := range sh.colors {
			c, err := color.ParseHex(hex)
			if err != nil {
				return nil, err
			}
			q.Colors = append(q.Colors, c)
		}
		if sh.query != "" {
			ids, err := store.MatchingIDs(ctx, pool, f, 10000)
			if err != nil {
				return nil, err
			}
			q.Allow = map[int64]bool{}
			for _, id := range ids {
				q.Allow[id] = true
			}
		}
		var ids []int64
		for i, h := range index.Search(q) {
			if i == 150 {
				break
			}
			ids = append(ids, h.ID)
		}
		var err error
		if candidates, err = store.GetArtworks(ctx, pool, ids); err != nil {
			return nil, err
		}
	} else {
		var err error
		if candidates, _, _, err = store.ListArtworks(ctx, pool, f); err != nil {
			return nil, err
		}
	}
	perArtist := map[string]int{}
	var out []*store.Artwork
	for _, a := range candidates {
		if a.AnalyzedAt == nil || perArtist[a.Artist] >= sh.perArtist {
			continue
		}
		perArtist[a.Artist]++
		out = append(out, a)
		if len(out) == sh.n {
			break
		}
	}
	return out, nil
}

func hang(ctx context.Context, pool *pgxpool.Pool, index *colorindex.Index, u *store.User, sh show) error {
	works, err := pick(ctx, pool, index, sh)
	if err != nil {
		return err
	}
	room, _ := gallery.RoomByKey(sh.room)
	// Drop works until the rest fit the room.
	var placements []gallery.Placement
	for len(works) > 0 {
		items := make([]gallery.Item, len(works))
		for i, a := range works {
			items[i] = gallery.Item{ArtworkID: a.ID, Hang: a.Hang()}
		}
		if placements, err = room.AutoHang(items); err == nil {
			break
		}
		works = works[:len(works)-1]
	}
	if len(placements) == 0 {
		return errors.New("nothing to hang")
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		e, err := store.CreateExhibition(ctx, tx, u.ID, sh.title, sh.room, sh.paint)
		if err != nil {
			return err
		}
		statement, floor, light := sh.statement, sh.floor, sh.light
		p := store.ExhibitionPatch{Statement: &statement, Floor: &floor, Light: &light}
		if sh.opensIn > 0 {
			// Openings are in the evening.
			d := time.Now().Add(sh.opensIn)
			at := time.Date(d.Year(), d.Month(), d.Day(), 19, 0, 0, 0, time.Local)
			p.OpeningAt = &at
		}
		if err := store.UpdateExhibition(ctx, tx, e, p); err != nil {
			return err
		}
		if err := store.ReplacePlacements(ctx, tx, e.ID, placements); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update exhibitions set status = 'published', published_at = $2 where id = $1`,
			e.ID, time.Now().Add(-sh.publishedAgo)); err != nil {
			return err
		}
		slog.Info("hung", "title", sh.title, "works", len(placements), "room", sh.room)
		return pipeline.QueuePoster(ctx, tx, e.ID)
	})
}
