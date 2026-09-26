// Package api is the HTTP and WebSocket interface.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nafiz001/vernissage/server/internal/auth"
	"github.com/Nafiz001/vernissage/server/internal/colorindex"
	"github.com/Nafiz001/vernissage/server/internal/config"
	"github.com/Nafiz001/vernissage/server/internal/imaging"
	"github.com/Nafiz001/vernissage/server/internal/live"
	"github.com/Nafiz001/vernissage/server/internal/pipeline"
	"github.com/Nafiz001/vernissage/server/internal/store"
)

const (
	sessionCookie = "vernissage_session"
	visitorCookie = "vernissage_visitor"
	sessionTTL    = 30 * 24 * time.Hour
)

type Server struct {
	cfg    config.Config
	pool   *pgxpool.Pool
	images *imaging.Service
	index  *colorindex.Index
	hub    *live.Hub
	pipe   *pipeline.Pipeline

	authLimit  *auth.Limiter
	writeLimit *auth.Limiter
	readLimit  *auth.Limiter

	sources sync.Map // artwork id -> imaging.Source
	tickets tickets
	log     *slog.Logger
}

func New(cfg config.Config, pool *pgxpool.Pool, images *imaging.Service, index *colorindex.Index, hub *live.Hub, pipe *pipeline.Pipeline) *Server {
	return &Server{
		cfg: cfg, pool: pool, images: images, index: index, hub: hub, pipe: pipe,
		authLimit:  auth.NewLimiter(10, 10),
		writeLimit: auth.NewLimiter(120, 60),
		readLimit:  auth.NewLimiter(900, 300),
		tickets:    newTickets(cfg.Secret),
		log:        slog.With("component", "api"),
	}
}

// Handler is every route behind the middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h func(http.ResponseWriter, *http.Request) error) {
		mux.Handle(pattern, s.wrap(h))
	}

	route("GET /api/health", s.health)
	route("GET /api/stats", s.stats)
	route("GET /api/rooms", s.rooms)

	route("GET /api/artworks", s.listArtworks)
	route("GET /api/artworks/featured", s.featured)
	route("GET /api/artworks/{id}", s.getArtwork)
	route("GET /api/artworks/{id}/similar", s.similar)
	route("GET /api/suggest", s.suggest)

	route("POST /api/auth/signup", s.signup)
	route("POST /api/auth/login", s.login)
	route("POST /api/auth/logout", s.logout)
	route("GET /api/me", s.me)
	route("GET /api/me/saved", s.mySaved)
	route("PUT /api/me/saved/{id}", s.setSaved(true))
	route("DELETE /api/me/saved/{id}", s.setSaved(false))
	route("GET /api/me/exhibitions", s.myExhibitions)

	route("GET /api/exhibitions", s.listExhibitions)
	route("POST /api/exhibitions", s.createExhibition)
	route("GET /api/exhibitions/{ref}", s.getExhibition)
	route("PATCH /api/exhibitions/{ref}", s.updateExhibition)
	route("DELETE /api/exhibitions/{ref}", s.deleteExhibition)
	route("PUT /api/exhibitions/{ref}/placements", s.putPlacements)
	route("POST /api/exhibitions/{ref}/autohang", s.autoHang)
	route("POST /api/exhibitions/{ref}/publish", s.publish(true))
	route("POST /api/exhibitions/{ref}/unpublish", s.publish(false))
	route("POST /api/exhibitions/{ref}/applause", s.applaud)
	route("POST /api/exhibitions/{ref}/visit", s.visit)
	route("GET /api/users/{handle}", s.getUser)

	route("GET /api/live-ticket", s.liveTicket)
	route("GET /ws/exhibitions/{ref}", s.walk)

	route("GET /img/{id}/{file}", s.image)
	route("GET /dzi/{file}", s.dziDescriptor)
	route("GET /dzi/{name}/{level}/{tile}", s.dziTile)
	route("GET /poster/{file}", s.poster)

	return s.middleware(mux)
}

// httpError is an error with a status and a message fit for people.
type httpError struct {
	status  int
	message string
	extra   map[string]any
}

func (e *httpError) Error() string { return e.message }

func fail(status int, message string) error { return &httpError{status: status, message: message} }

func failWith(status int, message string, extra map[string]any) error {
	return &httpError{status: status, message: message, extra: extra}
}

var (
	errNotFound = fail(http.StatusNotFound, "That doesn't exist, or it isn't public.")
	errSignIn   = fail(http.StatusUnauthorized, "Sign in to do that.")
	errNotYours = fail(http.StatusForbidden, "Only the curator can change this exhibition.")
)

func (s *Server) wrap(h func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		var he *httpError
		switch {
		case errors.As(err, &he):
		case errors.Is(err, store.ErrNotFound):
			he = errNotFound.(*httpError)
		case errors.Is(err, context.Canceled):
			return
		default:
			s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
			he = &httpError{status: http.StatusInternalServerError, message: "Something went wrong on our side. Try again in a moment."}
		}
		body := map[string]any{"message": he.message}
		for k, v := range he.extra {
			body[k] = v
		}
		writeJSON(w, he.status, map[string]any{"error": body})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 256<<10))
	if err := dec.Decode(v); err != nil {
		return fail(http.StatusBadRequest, "The request body isn't valid JSON.")
	}
	return nil
}

type ctxKey int

const (
	userKey ctxKey = iota
	visitorKey
)

func userFrom(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func visitorFrom(r *http.Request) string {
	v, _ := r.Context().Value(visitorKey).(string)
	return v
}

func (s *Server) clientIP(r *http.Request) string {
	return forwardedFor(r, s.cfg.ProxyHops)
}

// forwardedFor finds the browser's address. Each proxy in front of the
// server appends the address it was connected from to X-Forwarded-For, so
// with n proxies of our own the browser is n entries from the end; anything
// before that came from the client and can't be trusted.
func forwardedFor(r *http.Request, hops int) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[max(0, len(parts)-hops)])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// internal is a request from the web server rendering a page (no proxy
// header, from this machine or the private network), which shouldn't
// share one rate-limit bucket on behalf of every visitor.
func internal(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(s int)           { r.status = s; r.ResponseWriter.WriteHeader(s) }
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: 200}
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic", "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
				writeJSON(rec, 500, map[string]any{"error": map[string]any{"message": "Something went wrong on our side."}})
			}
			level := slog.LevelInfo
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				level = slog.LevelDebug
			}
			s.log.Log(r.Context(), level, "request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "took", time.Since(start).Round(time.Microsecond))
		}()

		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		api := strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/")
		if !api {
			next.ServeHTTP(rec, r)
			return
		}
		h.Set("Cache-Control", "no-store")

		unsafe := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if unsafe && !s.sameOrigin(r) {
			writeJSON(rec, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "Cross-site request refused."}})
			return
		}
		limiter := s.readLimit
		if unsafe {
			limiter = s.writeLimit
		}
		if (unsafe || !internal(r)) && !limiter.Allow(s.clientIP(r)) {
			rec.Header().Set("Retry-After", "10")
			writeJSON(rec, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"message": "Too many requests. Wait a few seconds and try again."}})
			return
		}

		ctx := r.Context()
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			if u, err := store.SessionUser(ctx, s.pool, c.Value, sessionTTL); err == nil {
				ctx = context.WithValue(ctx, userKey, u)
			}
		}
		visitor := ""
		if c, err := r.Cookie(visitorCookie); err == nil && len(c.Value) >= 16 && len(c.Value) <= 64 {
			visitor = c.Value
		} else {
			visitor = auth.NewToken()[:22]
			http.SetCookie(rec, &http.Cookie{Name: visitorCookie, Value: visitor, Path: "/", MaxAge: 365 * 24 * 3600,
				HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.Secure})
		}
		ctx = context.WithValue(ctx, visitorKey, visitor)
		next.ServeHTTP(rec, r.WithContext(ctx))
	})
}

// sameOrigin is the CSRF check for writes: the browser's Origin must be
// one of ours. Cookies are SameSite=Lax as well, so this is belt and braces.
func (s *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Browsers always send Origin on cross-site writes; its absence
		// means a same-origin form or a non-browser client.
		site := r.Header.Get("Sec-Fetch-Site")
		return site == "" || site == "same-origin" || site == "none"
	}
	for _, o := range s.cfg.Origins {
		if strings.EqualFold(strings.TrimSpace(o), origin) {
			return true
		}
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

func (s *Server) setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.Secure})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.Secure})
}

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound
	}
	return id, nil
}

func queryInt(r *http.Request, name string) *int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return nil
	}
	return &v
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		return writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "database": err.Error()})
	}
	return writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
