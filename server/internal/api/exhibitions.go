package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/nafizahmed/vernissage/server/internal/gallery"
	"github.com/nafizahmed/vernissage/server/internal/live"
	"github.com/nafizahmed/vernissage/server/internal/pipeline"
	"github.com/nafizahmed/vernissage/server/internal/store"
)

// loadExhibition finds an exhibition the viewer may see: published ones,
// and the viewer's own drafts.
func (s *Server) loadExhibition(r *http.Request) (*store.Exhibition, error) {
	e, err := store.GetExhibition(r.Context(), s.pool, r.PathValue("ref"))
	if err != nil {
		return nil, err
	}
	u := userFrom(r)
	if e.Status != "published" && (u == nil || u.ID != e.OwnerID) {
		return nil, errNotFound
	}
	return e, nil
}

// ownExhibition finds an exhibition the viewer curates.
func (s *Server) ownExhibition(r *http.Request) (*store.Exhibition, error) {
	u := userFrom(r)
	if u == nil {
		return nil, errSignIn
	}
	e, err := s.loadExhibition(r)
	if err != nil {
		return nil, err
	}
	if e.OwnerID != u.ID {
		return nil, errNotYours
	}
	return e, nil
}

// detail is an exhibition with its hang and every work in it.
func (s *Server) detail(ctx context.Context, e *store.Exhibition, viewer *store.User, visitor string) (map[string]any, error) {
	views, err := s.exhibitionViews(ctx, []*store.Exhibition{e}, viewer)
	if err != nil {
		return nil, err
	}
	ps, err := store.Placements(ctx, s.pool, e.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(ps))
	for i, p := range ps {
		ids[i] = p.ArtworkID
	}
	works, err := store.GetArtworks(ctx, s.pool, ids)
	if err != nil {
		return nil, err
	}
	applauded := false
	if visitor != "" {
		if applauded, err = store.Applauded(ctx, s.pool, e.ID, visitor); err != nil {
			return nil, err
		}
	}
	room, _ := gallery.RoomByKey(e.Room)
	return map[string]any{
		"exhibition": views[0],
		"placements": ps,
		"works":      artworkViews(works),
		"problems":   room.Validate(ps, hangsOf(works)),
		"applauded":  applauded,
	}, nil
}

func hangsOf(works []*store.Artwork) map[int64]gallery.Hang {
	out := make(map[int64]gallery.Hang, len(works))
	for _, a := range works {
		out[a.ID] = a.Hang()
	}
	return out
}

func (s *Server) getExhibition(w http.ResponseWriter, r *http.Request) error {
	e, err := s.loadExhibition(r)
	if err != nil {
		return err
	}
	d, err := s.detail(r.Context(), e, userFrom(r), visitorFrom(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}

type cursor struct {
	At time.Time `json:"at"`
	ID int64     `json:"id"`
}

func (s *Server) listExhibitions(w http.ResponseWriter, r *http.Request) error {
	l := store.ExhibitionList{Published: true, Sort: r.URL.Query().Get("sort"), Limit: 24}
	if n := queryInt(r, "limit"); n != nil && *n > 0 && *n <= 50 {
		l.Limit = *n
	}
	if l.Sort == "popular" {
		if n := queryInt(r, "offset"); n != nil && *n >= 0 {
			l.Offset = *n
		}
	} else if c := r.URL.Query().Get("cursor"); c != "" {
		var cur cursor
		raw, err := base64.RawURLEncoding.DecodeString(c)
		if err != nil || json.Unmarshal(raw, &cur) != nil {
			return fail(http.StatusBadRequest, "That page link is broken.")
		}
		l.BeforeAt, l.BeforeID = &cur.At, cur.ID
	}
	list, err := store.ListExhibitions(r.Context(), s.pool, l)
	if err != nil {
		return err
	}
	views, err := s.exhibitionViews(r.Context(), list, userFrom(r))
	if err != nil {
		return err
	}
	resp := map[string]any{"items": views, "next": nil}
	if len(list) == l.Limit {
		if l.Sort == "popular" {
			resp["next"] = fmt.Sprint(l.Offset + l.Limit)
		} else if last := list[len(list)-1]; last.PublishedAt != nil {
			raw, _ := json.Marshal(cursor{*last.PublishedAt, last.ID})
			resp["next"] = base64.RawURLEncoding.EncodeToString(raw)
		}
	}
	return writeJSON(w, http.StatusOK, resp)
}

func (s *Server) myExhibitions(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if u == nil {
		return errSignIn
	}
	list, err := store.ListExhibitions(r.Context(), s.pool, store.ExhibitionList{OwnerID: u.ID, Limit: 50})
	if err != nil {
		return err
	}
	views, err := s.exhibitionViews(r.Context(), list, u)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"items": views})
}

func (s *Server) createExhibition(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if u == nil {
		return errSignIn
	}
	var in struct {
		Title      string  `json:"title"`
		Room       string  `json:"room"`
		Paint      string  `json:"paint"`
		ArtworkIDs []int64 `json:"artworkIds"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Title = live.CleanText(in.Title, 90)
	if in.Title == "" {
		in.Title = "Untitled exhibition"
	}
	if _, ok := gallery.RoomByKey(in.Room); !ok {
		in.Room = "salon"
	}
	if !gallery.ValidPaint(in.Paint) {
		in.Paint = "oxblood"
	}
	var created *store.Exhibition
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		e, err := store.CreateExhibition(r.Context(), tx, u.ID, in.Title, in.Room, in.Paint)
		if err != nil {
			return err
		}
		created = e
		// Starting from a selection: hang it straight away.
		if len(in.ArtworkIDs) > 0 {
			ps, err := s.hangFor(r.Context(), in.Room, in.ArtworkIDs)
			if err != nil {
				return err
			}
			return store.ReplacePlacements(r.Context(), tx, e.ID, ps)
		}
		return nil
	})
	if err != nil {
		return err
	}
	d, err := s.detail(r.Context(), created, u, visitorFrom(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, d)
}

// hangFor auto-hangs works in a room.
func (s *Server) hangFor(ctx context.Context, roomKey string, ids []int64) ([]gallery.Placement, error) {
	room, ok := gallery.RoomByKey(roomKey)
	if !ok {
		return nil, fail(http.StatusBadRequest, "Choose one of the rooms.")
	}
	if len(ids) > gallery.MaxWorks {
		return nil, fail(http.StatusUnprocessableEntity, fmt.Sprintf("An exhibition can hang up to %d works.", gallery.MaxWorks))
	}
	works, err := store.GetArtworks(ctx, s.pool, ids)
	if err != nil {
		return nil, err
	}
	items := make([]gallery.Item, len(works))
	for i, a := range works {
		items[i] = gallery.Item{ArtworkID: a.ID, Hang: a.Hang()}
	}
	ps, err := room.AutoHang(items)
	if err != nil {
		return nil, fail(http.StatusUnprocessableEntity, capitalize(err.Error())+".")
	}
	return ps, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (s *Server) updateExhibition(w http.ResponseWriter, r *http.Request) error {
	e, err := s.ownExhibition(r)
	if err != nil {
		return err
	}
	var in struct {
		Title     *string          `json:"title"`
		Statement *string          `json:"statement"`
		Room      *string          `json:"room"`
		Paint     *string          `json:"paint"`
		Floor     *string          `json:"floor"`
		Light     *string          `json:"light"`
		CoverID   *int64           `json:"coverId"`
		OpeningAt *json.RawMessage `json:"openingAt"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var p store.ExhibitionPatch
	bad := func(msg string) error { return fail(http.StatusUnprocessableEntity, msg) }
	if in.Title != nil {
		t := live.CleanText(*in.Title, 90)
		if t == "" {
			return bad("Give the exhibition a title.")
		}
		p.Title = &t
	}
	if in.Statement != nil {
		st := strings.TrimSpace(*in.Statement)
		if len([]rune(st)) > 2000 {
			return bad("Keep the curator's statement under 2,000 characters.")
		}
		p.Statement = &st
	}
	if in.Room != nil {
		if _, ok := gallery.RoomByKey(*in.Room); !ok {
			return bad("Choose one of the rooms.")
		}
		p.Room = in.Room
	}
	if in.Paint != nil {
		if !gallery.ValidPaint(*in.Paint) {
			return bad("Choose one of the wall colours.")
		}
		p.Paint = in.Paint
	}
	if in.Floor != nil {
		if !gallery.ValidFloor(*in.Floor) {
			return bad("Choose one of the floors.")
		}
		p.Floor = in.Floor
	}
	if in.Light != nil {
		if !gallery.ValidLight(*in.Light) {
			return bad("Choose one of the lighting schemes.")
		}
		p.Light = in.Light
	}
	if in.CoverID != nil {
		var hung bool
		if err := s.pool.QueryRow(r.Context(), `select exists(select 1 from placements where exhibition_id = $1 and artwork_id = $2)`,
			e.ID, *in.CoverID).Scan(&hung); err != nil {
			return err
		}
		if !hung {
			return bad("The cover has to be one of the works on the walls.")
		}
		p.CoverID = in.CoverID
	}
	if in.OpeningAt != nil {
		if string(*in.OpeningAt) == "null" {
			p.ClearOpening = true
		} else {
			var t time.Time
			if json.Unmarshal(*in.OpeningAt, &t) != nil {
				return bad("Give the opening as a date and time.")
			}
			p.OpeningAt = &t
		}
	}
	err = pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := store.UpdateExhibition(r.Context(), tx, e, p); err != nil {
			return err
		}
		if e.Status == "published" {
			return pipeline.QueuePoster(r.Context(), tx, e.ID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.respondDetail(w, r, e.ID)
}

func (s *Server) respondDetail(w http.ResponseWriter, r *http.Request, id int64) error {
	e, err := store.GetExhibition(r.Context(), s.pool, fmt.Sprint(id))
	if err != nil {
		return err
	}
	d, err := s.detail(r.Context(), e, userFrom(r), visitorFrom(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}

func (s *Server) deleteExhibition(w http.ResponseWriter, r *http.Request) error {
	e, err := s.ownExhibition(r)
	if err != nil {
		return err
	}
	if err := store.DeleteExhibition(r.Context(), s.pool, e.ID); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) putPlacements(w http.ResponseWriter, r *http.Request) error {
	e, err := s.ownExhibition(r)
	if err != nil {
		return err
	}
	var in struct {
		Placements []gallery.Placement `json:"placements"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ids := make([]int64, len(in.Placements))
	for i := range in.Placements {
		ids[i] = in.Placements[i].ArtworkID
		in.Placements[i].Label = strings.TrimSpace(in.Placements[i].Label)
		if len([]rune(in.Placements[i].Label)) > 600 {
			return fail(http.StatusUnprocessableEntity, "Keep each wall label under 600 characters.")
		}
	}
	works, err := store.GetArtworks(r.Context(), s.pool, ids)
	if err != nil {
		return err
	}
	room, _ := gallery.RoomByKey(e.Room)
	if problems := room.Validate(in.Placements, hangsOf(works)); len(problems) > 0 {
		return failWith(http.StatusUnprocessableEntity, "Some works don't fit where they are.", map[string]any{"problems": problems})
	}
	err = pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		if err := store.ReplacePlacements(r.Context(), tx, e.ID, in.Placements); err != nil {
			return err
		}
		if e.Status == "published" {
			return pipeline.QueuePoster(r.Context(), tx, e.ID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.respondDetail(w, r, e.ID)
}

func (s *Server) autoHang(w http.ResponseWriter, r *http.Request) error {
	e, err := s.ownExhibition(r)
	if err != nil {
		return err
	}
	var in struct {
		ArtworkIDs []int64 `json:"artworkIds"`
		Room       string  `json:"room"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Room == "" {
		in.Room = e.Room
	}
	ps, err := s.hangFor(r.Context(), in.Room, in.ArtworkIDs)
	if err != nil {
		return err
	}
	// Keep the labels already written.
	old, err := store.Placements(r.Context(), s.pool, e.ID)
	if err != nil {
		return err
	}
	labels := map[int64]string{}
	for _, p := range old {
		labels[p.ArtworkID] = p.Label
	}
	for i := range ps {
		ps[i].Label = labels[ps[i].ArtworkID]
	}
	if ps == nil {
		ps = []gallery.Placement{}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"placements": ps})
}

func (s *Server) publish(on bool) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		e, err := s.ownExhibition(r)
		if err != nil {
			return err
		}
		if on {
			d, err := s.detail(r.Context(), e, userFrom(r), "")
			if err != nil {
				return err
			}
			if ps := d["placements"].([]gallery.Placement); len(ps) == 0 {
				return fail(http.StatusUnprocessableEntity, "Hang at least one work before opening the doors.")
			}
			if probs := d["problems"].([]gallery.Problem); len(probs) > 0 {
				return failWith(http.StatusUnprocessableEntity, "Some works don't fit where they are. Move them, then open the doors.", map[string]any{"problems": probs})
			}
		}
		err = pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
			if err := store.SetExhibitionStatus(r.Context(), tx, e.ID, on); err != nil {
				return err
			}
			if on {
				return pipeline.QueuePoster(r.Context(), tx, e.ID)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return s.respondDetail(w, r, e.ID)
	}
}

func (s *Server) applaud(w http.ResponseWriter, r *http.Request) error {
	e, err := s.loadExhibition(r)
	if err != nil {
		return err
	}
	if e.Status != "published" {
		return fail(http.StatusConflict, "Applause opens with the exhibition.")
	}
	count, on, err := store.ToggleApplause(r.Context(), s.pool, e.ID, visitorFrom(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"applause": count, "applauded": on})
}

func (s *Server) visit(w http.ResponseWriter, r *http.Request) error {
	e, err := s.loadExhibition(r)
	if err != nil {
		return err
	}
	if e.Status != "published" || (userFrom(r) != nil && userFrom(r).ID == e.OwnerID) {
		return writeJSON(w, http.StatusOK, map[string]any{"visits": e.VisitCount})
	}
	n, err := store.RecordVisit(r.Context(), s.pool, e.ID, visitorFrom(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"visits": n})
}
