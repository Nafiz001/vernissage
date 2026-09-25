package api

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/mail"
	"strings"

	"github.com/nafizahmed/vernissage/server/internal/auth"
	"github.com/nafizahmed/vernissage/server/internal/live"
	"github.com/nafizahmed/vernissage/server/internal/store"
)

type userJSON struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Hue    int    `json:"hue"`
	Email  string `json:"email,omitempty"`
}

func meView(u *store.User) userJSON { return userJSON{u.Handle, u.Name, u.Hue, u.Email} }

func (s *Server) signup(w http.ResponseWriter, r *http.Request) error {
	if !s.authLimit.Allow("signup:" + clientIP(r)) {
		return fail(http.StatusTooManyRequests, "Too many attempts from here. Wait a minute and try again.")
	}
	var in struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Name = live.CleanText(in.Name, 40)
	in.Email = strings.TrimSpace(in.Email)
	fields := map[string]string{}
	if in.Name == "" {
		fields["name"] = "Tell us your name, as it should appear on your exhibitions."
	}
	if addr, err := mail.ParseAddress(in.Email); err != nil || addr.Address != in.Email || len(in.Email) > 254 {
		fields["email"] = "Enter an email address like name@example.com."
	}
	if err := auth.ValidatePassword(in.Password); err != nil {
		fields["password"] = err.Error()
	}
	if len(fields) > 0 {
		return failWith(http.StatusUnprocessableEntity, "Check the highlighted fields.", map[string]any{"fields": fields})
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}
	_, hue := live.Pigment(in.Email)
	u := &store.User{Name: in.Name, Email: in.Email, PasswordHash: hash, Hue: hue}
	base := store.Slugify(in.Name)
	if base == "" {
		base = "curator"
	}
	for attempt := 0; ; attempt++ {
		u.Handle = base
		if attempt > 0 {
			u.Handle = fmt.Sprintf("%s-%d", base, 10+rand.IntN(990))
		}
		err = store.CreateUser(r.Context(), s.pool, u)
		if !errors.Is(err, store.ErrHandleTaken) || attempt == 8 {
			break
		}
	}
	if errors.Is(err, store.ErrEmailTaken) {
		return failWith(http.StatusConflict, "There's already an account with that email.",
			map[string]any{"fields": map[string]string{"email": "There's already an account with that email. Sign in instead."}})
	}
	if err != nil {
		return err
	}
	token := auth.NewToken()
	if err := store.CreateSession(r.Context(), s.pool, u.ID, token, sessionTTL); err != nil {
		return err
	}
	s.setSession(w, token)
	return writeJSON(w, http.StatusCreated, map[string]any{"user": meView(u)})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Email = strings.TrimSpace(in.Email)
	if !s.authLimit.Allow("login:"+clientIP(r)) || !s.authLimit.Allow("login:"+strings.ToLower(in.Email)) {
		return fail(http.StatusTooManyRequests, "Too many sign-in attempts. Wait a minute and try again.")
	}
	u, err := store.UserByEmail(r.Context(), s.pool, in.Email)
	hash := ""
	if err == nil {
		hash = u.PasswordHash
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	// Checked even without a user, so both failures take as long.
	if !auth.CheckPassword(hash, in.Password) {
		return fail(http.StatusUnauthorized, "That email and password don't match an account.")
	}
	token := auth.NewToken()
	if err := store.CreateSession(r.Context(), s.pool, u.ID, token, sessionTTL); err != nil {
		return err
	}
	s.setSession(w, token)
	return writeJSON(w, http.StatusOK, map[string]any{"user": meView(u)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := store.DeleteSession(r.Context(), s.pool, c.Value); err != nil {
			return err
		}
	}
	s.clearSession(w)
	return writeJSON(w, http.StatusOK, map[string]any{"user": nil})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if u == nil {
		return writeJSON(w, http.StatusOK, map[string]any{"user": nil})
	}
	saved, err := store.SavedIDs(r.Context(), s.pool, u.ID)
	if err != nil {
		return err
	}
	if saved == nil {
		saved = []int64{}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"user": meView(u), "saved": saved})
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) error {
	u, err := store.UserByHandle(r.Context(), s.pool, r.PathValue("handle"))
	if err != nil {
		return err
	}
	list, err := store.ListExhibitions(r.Context(), s.pool, store.ExhibitionList{Published: true, OwnerID: u.ID, Limit: 50})
	if err != nil {
		return err
	}
	views, err := s.exhibitionViews(r.Context(), list, userFrom(r))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"user":        map[string]any{"handle": u.Handle, "name": u.Name, "hue": u.Hue, "createdAt": u.CreatedAt},
		"exhibitions": views,
	})
}
