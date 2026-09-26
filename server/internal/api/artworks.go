package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Nafiz001/vernissage/server/internal/color"
	"github.com/Nafiz001/vernissage/server/internal/colorindex"
	"github.com/Nafiz001/vernissage/server/internal/gallery"
	"github.com/Nafiz001/vernissage/server/internal/jobs"
	"github.com/Nafiz001/vernissage/server/internal/store"
)

func (s *Server) stats(w http.ResponseWriter, r *http.Request) error {
	st, err := store.Stats(r.Context(), s.pool)
	if err != nil {
		return err
	}
	queue, err := jobs.Stats(r.Context(), s.pool)
	if err != nil {
		return err
	}
	people := 0
	counts := s.hub.Counts()
	for _, n := range counts {
		people += n
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"collection": st,
		"indexed":    s.index.Len(),
		"moods":      s.index.Moods(),
		"queue":      queue,
		"live":       map[string]int{"rooms": len(counts), "people": people},
	})
}

func (s *Server) rooms(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "public, max-age=300")
	return writeJSON(w, http.StatusOK, map[string]any{
		"rooms": gallery.Rooms, "paints": gallery.Paints, "floors": gallery.Floors, "lights": gallery.Lights,
		"rules": map[string]any{
			"eyeLine": gallery.EyeLine, "corner": gallery.Corner, "minGap": gallery.MinGap,
			"floorClear": gallery.FloorClear, "ceilingClear": gallery.CeilingClear, "maxWorks": gallery.MaxWorks,
		},
	})
}

var kinds = map[string]bool{"painting": true, "print": true, "drawing": true}

func (s *Server) listArtworks(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := store.Filter{
		Query:  strings.TrimSpace(q.Get("q")),
		From:   queryInt(r, "from"),
		To:     queryInt(r, "to"),
		Source: q.Get("museum"),
		Artist: q.Get("artist"),
		Limit:  40,
	}
	if len(f.Query) > 200 {
		f.Query = f.Query[:200]
	}
	for _, k := range strings.Split(q.Get("kind"), ",") {
		if kinds[k] {
			f.Kinds = append(f.Kinds, k)
		}
	}
	if n := queryInt(r, "limit"); n != nil && *n > 0 && *n <= 100 {
		f.Limit = *n
	}
	if n := queryInt(r, "offset"); n != nil && *n >= 0 && *n <= 20000 {
		f.Offset = *n
	}
	if n := queryInt(r, "seed"); n != nil {
		f.Seed = *n
	}
	mood := q.Get("mood")

	var colors []color.Lab
	for _, hex := range strings.Split(q.Get("color"), ",") {
		if hex == "" {
			continue
		}
		c, err := color.ParseHex(hex)
		if err != nil {
			return fail(http.StatusBadRequest, "Colours are written as hex, like #3a6ea5.")
		}
		colors = append(colors, c)
	}
	if len(colors) > 3 {
		return fail(http.StatusBadRequest, "Search for up to three colours at once.")
	}

	var items []*store.Artwork
	var total int
	fuzzy := false
	if len(colors) > 0 {
		// Colour search ranks in memory; a text query or artist narrows it
		// to the works the database matches.
		cq := colorindex.Query{Colors: colors, Kinds: f.Kinds, Source: f.Source, From: f.From, To: f.To, Mood: mood}
		if f.Query != "" || f.Artist != "" {
			ids, err := store.MatchingIDs(r.Context(), s.pool, f, 10000)
			if err != nil {
				return err
			}
			cq.Allow = make(map[int64]bool, len(ids))
			for _, id := range ids {
				cq.Allow[id] = true
			}
		}
		hits := s.index.Search(cq)
		total = len(hits)
		end := min(len(hits), f.Offset+f.Limit)
		var ids []int64
		for _, h := range hits[min(f.Offset, end):end] {
			ids = append(ids, h.ID)
		}
		var err error
		if items, err = store.GetArtworks(r.Context(), s.pool, ids); err != nil {
			return err
		}
		for _, a := range items {
			a.Description = ""
		}
	} else {
		m := s.index.Moods()
		switch mood {
		case "bright":
			f.LightMin = &m.BrightMin
		case "dark":
			f.LightMax = &m.DarkMax
		case "vivid":
			f.ColorMin = &m.VividMin
		case "muted":
			f.ColorMax = &m.MutedMax
		}
		var err error
		if items, total, fuzzy, err = store.ListArtworks(r.Context(), s.pool, f); err != nil {
			return err
		}
	}
	var next *int
	if f.Offset+len(items) < total {
		n := f.Offset + f.Limit
		next = &n
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"items": artworkViews(items), "total": total, "offset": f.Offset, "next": next, "fuzzy": fuzzy,
	})
}

// Masterpieces for the front page: what a person would cross a city to
// see. Matched by title and artist, since ids depend on ingest order.
var featured = []struct{ title, artist string }{
	{"Wheat Field with Cypresses", "Gogh"},
	{"Under the Wave off Kanagawa%", "Hokusai"},
	{"The Burning of the Houses of Lords and Commons%", "Turner"},
	{"The Harvesters", "Bruegel"},
	{"Water Lilies (Agapanthus)", "Monet"},
	{"Young Woman with a Water Pitcher", "Vermeer"},
	{"Bridge over a Pond of Water Lilies", "Monet"},
	{"The Large Plane Trees%", "Gogh"},
	{"Cypresses", "Gogh"},
	{"Madame X%", "Sargent"},
	{"The Crucifixion of Saint Andrew", "Caravaggio"},
	{"Aristotle with a Bust of Homer", "Rembrandt"},
}

func (s *Server) featured(w http.ResponseWriter, r *http.Request) error {
	var ids []int64
	for _, f := range featured {
		var id int64
		err := s.pool.QueryRow(r.Context(), `select id from artworks
			where title ilike $1 and artist ilike '%' || $2 || '%' and analyzed_at is not null
			order by highlight desc, id limit 1`, f.title, f.artist).Scan(&id)
		if err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) < 6 {
		// Before the masterpieces are analysed, the most colourful highlights.
		rows, err := s.pool.Query(r.Context(), `select id from artworks
			where highlight and kind = 'painting' and analyzed_at is not null and aspect between 0.6 and 1.7
			order by colorfulness desc limit $1`, 8-len(ids))
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
	}
	works, err := store.GetArtworks(r.Context(), s.pool, ids)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "public, max-age=120")
	return writeJSON(w, http.StatusOK, map[string]any{"items": artworkViews(works)})
}

func (s *Server) getArtwork(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	a, err := store.GetArtwork(r.Context(), s.pool, id)
	if err != nil {
		return err
	}
	resp := map[string]any{"artwork": artworkView(a)}

	more := []*store.Artwork{}
	if a.Artist != "" {
		list, _, _, err := store.ListArtworks(r.Context(), s.pool, store.Filter{Artist: a.Artist, Limit: 13})
		if err != nil {
			return err
		}
		for _, m := range list {
			if m.ID != a.ID && len(more) < 12 {
				more = append(more, m)
			}
		}
	}
	resp["moreByArtist"] = artworkViews(more)

	shows, err := store.ExhibitionsWith(r.Context(), s.pool, a.ID)
	if err != nil {
		return err
	}
	if resp["exhibitions"], err = s.exhibitionViews(r.Context(), shows, userFrom(r)); err != nil {
		return err
	}
	saved := false
	if u := userFrom(r); u != nil {
		ids, err := store.SavedIDs(r.Context(), s.pool, u.ID)
		if err != nil {
			return err
		}
		for _, sid := range ids {
			saved = saved || sid == a.ID
		}
	}
	resp["saved"] = saved
	return writeJSON(w, http.StatusOK, resp)
}

func (s *Server) similar(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var ids []int64
	for _, h := range s.index.Similar(id, 18) {
		ids = append(ids, h.ID)
	}
	works, err := store.GetArtworks(r.Context(), s.pool, ids)
	if err != nil {
		return err
	}
	for _, a := range works {
		a.Description = ""
	}
	return writeJSON(w, http.StatusOK, map[string]any{"items": artworkViews(works)})
}

func (s *Server) suggest(w http.ResponseWriter, r *http.Request) error {
	sg, err := store.Suggest(r.Context(), s.pool, r.URL.Query().Get("q"))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, sg)
}

func (s *Server) mySaved(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if u == nil {
		return errSignIn
	}
	list, err := store.Saved(r.Context(), s.pool, u.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"items": artworkViews(list)})
}

func (s *Server) setSaved(on bool) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		u := userFrom(r)
		if u == nil {
			return errSignIn
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			return errNotFound
		}
		if err := store.SetSaved(r.Context(), s.pool, u.ID, id, on); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return errNotFound
			}
			return err
		}
		return writeJSON(w, http.StatusOK, map[string]any{"saved": on})
	}
}
