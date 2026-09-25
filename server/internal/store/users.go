package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/nafizahmed/vernissage/server/internal/db"
)

type User struct {
	ID           int64     `json:"id"`
	Handle       string    `json:"handle"`
	Name         string    `json:"name"`
	Email        string    `json:"-"`
	PasswordHash string    `json:"-"`
	Hue          int       `json:"hue"`
	CreatedAt    time.Time `json:"createdAt"`
}

var (
	ErrEmailTaken  = errors.New("email taken")
	ErrHandleTaken = errors.New("handle taken")
)

const userCols = `id, handle, name, email, password_hash, hue, created_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Handle, &u.Name, &u.Email, &u.PasswordHash, &u.Hue, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func CreateUser(ctx context.Context, q db.Querier, u *User) error {
	err := q.QueryRow(ctx, `insert into users (handle, name, email, password_hash, hue)
		values ($1, $2, $3, $4, $5) returning id, created_at`,
		u.Handle, u.Name, u.Email, u.PasswordHash, u.Hue).Scan(&u.ID, &u.CreatedAt)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		if pg.ConstraintName == "users_email" {
			return ErrEmailTaken
		}
		return ErrHandleTaken
	}
	return err
}

func UserByEmail(ctx context.Context, q db.Querier, email string) (*User, error) {
	return scanUser(q.QueryRow(ctx, `select `+userCols+` from users where lower(email) = lower($1)`, email))
}

func UserByHandle(ctx context.Context, q db.Querier, handle string) (*User, error) {
	return scanUser(q.QueryRow(ctx, `select `+userCols+` from users where lower(handle) = lower($1)`, handle))
}

func HandleTaken(ctx context.Context, q db.Querier, handle string) (bool, error) {
	var taken bool
	err := q.QueryRow(ctx, `select exists(select 1 from users where lower(handle) = lower($1))`, handle).Scan(&taken)
	return taken, err
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CreateSession stores a session for the token, which only the cookie holds.
func CreateSession(ctx context.Context, q db.Querier, userID int64, token string, ttl time.Duration) error {
	_, err := q.Exec(ctx, `insert into sessions (token_hash, user_id, expires_at) values ($1, $2, $3)`,
		hashToken(token), userID, time.Now().Add(ttl))
	return err
}

// SessionUser finds the signed-in user for a token, sliding the session's
// expiry forward when it's more than a day old.
func SessionUser(ctx context.Context, q db.Querier, token string, ttl time.Duration) (*User, error) {
	var u User
	var expires time.Time
	err := q.QueryRow(ctx, `
		select u.id, u.handle, u.name, u.email, u.password_hash, u.hue, u.created_at, s.expires_at
		from sessions s join users u on u.id = s.user_id
		where s.token_hash = $1 and s.expires_at > now()`, hashToken(token)).
		Scan(&u.ID, &u.Handle, &u.Name, &u.Email, &u.PasswordHash, &u.Hue, &u.CreatedAt, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if time.Until(expires) < ttl-24*time.Hour {
		q.Exec(ctx, `update sessions set expires_at = $2 where token_hash = $1`, hashToken(token), time.Now().Add(ttl))
	}
	return &u, nil
}

func DeleteSession(ctx context.Context, q db.Querier, token string) error {
	_, err := q.Exec(ctx, `delete from sessions where token_hash = $1`, hashToken(token))
	return err
}

func DeleteExpiredSessions(ctx context.Context, q db.Querier) (int64, error) {
	tag, err := q.Exec(ctx, `delete from sessions where expires_at < now()`)
	return tag.RowsAffected(), err
}

// SetSaved sets a work aside for a user, or puts it back.
func SetSaved(ctx context.Context, q db.Querier, userID, artworkID int64, saved bool) error {
	var err error
	if saved {
		_, err = q.Exec(ctx, `insert into saved (user_id, artwork_id) values ($1, $2) on conflict do nothing`, userID, artworkID)
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23503" {
			return ErrNotFound
		}
	} else {
		_, err = q.Exec(ctx, `delete from saved where user_id = $1 and artwork_id = $2`, userID, artworkID)
	}
	return err
}

// Saved lists the works a user has set aside, newest first.
func Saved(ctx context.Context, q db.Querier, userID int64) ([]*Artwork, error) {
	rows, err := q.Query(ctx, `select `+artworkCols+` from saved s join artworks a on a.id = s.artwork_id
		where s.user_id = $1 order by s.created_at desc limit 500`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Artwork
	for rows.Next() {
		a, err := scanArtwork(rows)
		if err != nil {
			return nil, err
		}
		a.Description = ""
		out = append(out, a)
	}
	return out, rows.Err()
}

// SavedIDs is the set of works a user has set aside.
func SavedIDs(ctx context.Context, q db.Querier, userID int64) ([]int64, error) {
	rows, err := q.Query(ctx, `select artwork_id from saved where user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}
