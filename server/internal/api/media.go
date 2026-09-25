package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nafizahmed/vernissage/server/internal/gallery"
	"github.com/nafizahmed/vernissage/server/internal/imaging"
	"github.com/nafizahmed/vernissage/server/internal/live"
	"github.com/nafizahmed/vernissage/server/internal/pipeline"
	"github.com/nafizahmed/vernissage/server/internal/store"
)

// source looks up where a work's pictures live, remembering the answer:
// a page of thumbnails would otherwise be forty identical queries.
func (s *Server) source(r *http.Request, id int64) (imaging.Source, error) {
	if v, ok := s.sources.Load(id); ok {
		return v.(imaging.Source), nil
	}
	a, err := store.GetArtwork(r.Context(), s.pool, id)
	if err != nil {
		return imaging.Source{}, err
	}
	src := pipeline.Source(a)
	s.sources.Store(id, src)
	return src, nil
}

// serveCached sends a cached picture that never changes at its URL.
func serveCached(w http.ResponseWriter, r *http.Request, path, contentType string, maxAge time.Duration) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	imaging.Touch(path)
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", int(maxAge.Seconds())))
	// Pictures go into WebGL textures on other origins.
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("ETag", fmt.Sprintf(`"%x-%x"`, fi.ModTime().Unix(), fi.Size()))
	http.ServeContent(w, r, "", fi.ModTime(), f)
	return nil
}

func imageError(err error) error {
	if errors.Is(err, imaging.ErrUnavailable) || errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusNotFound, "The museum no longer has this picture.")
	}
	if errors.Is(err, os.ErrNotExist) {
		return errNotFound
	}
	return fail(http.StatusBadGateway, "The museum's image server didn't answer. Try again shortly.")
}

func (s *Server) image(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	width, err := strconv.Atoi(strings.TrimSuffix(r.PathValue("file"), ".jpg"))
	if err != nil || !slices.Contains(imaging.Widths, width) {
		return fail(http.StatusNotFound, fmt.Sprintf("Pictures come in widths %v.", imaging.Widths))
	}
	src, err := s.source(r, id)
	if err != nil {
		return imageError(err)
	}
	p, err := s.images.Sized(r.Context(), src, width)
	if err != nil {
		return imageError(err)
	}
	return serveCached(w, r, p, "image/jpeg", 365*24*time.Hour)
}

func (s *Server) dziDescriptor(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(strings.TrimSuffix(r.PathValue("file"), ".dzi"), 10, 64)
	if err != nil {
		return errNotFound
	}
	src, err := s.source(r, id)
	if err != nil {
		return imageError(err)
	}
	d, _, err := s.images.Pyramid(r.Context(), src)
	if err != nil {
		return imageError(err)
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, err = w.Write([]byte(d.Descriptor()))
	return err
}

func (s *Server) dziTile(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(strings.TrimSuffix(r.PathValue("name"), "_files"), 10, 64)
	if err != nil {
		return errNotFound
	}
	level, err := strconv.Atoi(r.PathValue("level"))
	if err != nil {
		return errNotFound
	}
	var col, row int
	if _, err := fmt.Sscanf(r.PathValue("tile"), "%d_%d.jpg", &col, &row); err != nil {
		return errNotFound
	}
	src, err := s.source(r, id)
	if err != nil {
		return imageError(err)
	}
	p, err := s.images.Tile(r.Context(), src, level, col, row)
	if err != nil {
		return imageError(err)
	}
	return serveCached(w, r, p, "image/jpeg", 30*24*time.Hour)
}

func (s *Server) poster(w http.ResponseWriter, r *http.Request) error {
	e, err := store.GetExhibition(r.Context(), s.pool, strings.TrimSuffix(r.PathValue("file"), ".jpg"))
	if err != nil {
		return err
	}
	if e.Status != "published" {
		return errNotFound
	}
	p, err := s.pipe.Poster(r.Context(), e)
	if err != nil {
		return err
	}
	age := 5 * time.Minute
	if r.URL.Query().Get("v") == strconv.Itoa(e.PosterVersion) {
		age = 30 * 24 * time.Hour
	}
	return serveCached(w, r, p, "image/jpeg", age)
}

// walk upgrades to a WebSocket and puts the visitor in the room.
func (s *Server) walk(w http.ResponseWriter, r *http.Request) error {
	e, err := s.loadExhibition(r)
	if err != nil {
		return err
	}
	var hosts []string
	for _, o := range s.cfg.Origins {
		o = strings.TrimSpace(o)
		o = strings.TrimPrefix(strings.TrimPrefix(o, "https://"), "http://")
		hosts = append(hosts, o)
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: hosts})
	if err != nil {
		return nil // Accept has already answered
	}
	defer c.CloseNow()

	who := live.Identity{Visitor: visitorFrom(r)}
	if u := userFrom(r); u != nil {
		who.UserID, who.Name, who.Hue = &u.ID, u.Name, u.Hue
	} else {
		who.Name, who.Hue = live.Pigment(who.Visitor)
	}
	room, _ := gallery.RoomByKey(e.Room)
	// Hijacked connections outlive the request's usual context handling.
	s.hub.Serve(r.Context(), c, e.ID, who, room.Width/2-0.4, room.Depth/2-0.4)
	c.Close(websocket.StatusNormalClosure, "")
	return nil
}

// Guestbook adapts the store to the live rooms.
type Guestbook struct{ Pool *pgxpool.Pool }

func (g Guestbook) Sign(ctx context.Context, exhibitionID int64, who live.Identity, message string) (live.Entry, error) {
	e, err := store.Sign(ctx, g.Pool, exhibitionID, who.UserID, who.Name, who.Hue, message)
	return live.Entry(e), err
}

func (g Guestbook) Latest(ctx context.Context, exhibitionID int64) ([]live.Entry, error) {
	list, err := store.Guestbook(ctx, g.Pool, exhibitionID, 40)
	out := make([]live.Entry, len(list))
	for i, e := range list {
		out[i] = live.Entry(e)
	}
	return out, err
}
