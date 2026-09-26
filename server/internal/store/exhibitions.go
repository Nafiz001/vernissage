package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"github.com/Nafiz001/vernissage/server/internal/db"
	"github.com/Nafiz001/vernissage/server/internal/gallery"
)

type Exhibition struct {
	ID            int64
	OwnerID       int64
	Slug          string
	Title         string
	Statement     string
	Room          string
	Paint         string
	Floor         string
	Light         string
	Status        string
	CoverID       *int64
	OpeningAt     *time.Time
	PublishedAt   *time.Time
	PosterVersion int
	VisitCount    int
	ApplauseCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Joined for display.
	Owner     User
	WorkCount int
	// Up to four works, cover first, for cards.
	Preview []int64
}

const exhibitionCols = `e.id, e.owner_id, e.slug, e.title, e.statement, e.room, e.paint, e.floor, e.light,
	e.status, e.cover_id, e.opening_at, e.published_at, e.poster_version, e.visit_count, e.applause_count,
	e.created_at, e.updated_at, u.id, u.handle, u.name, u.hue,
	(select count(*) from placements p where p.exhibition_id = e.id),
	array(select p.artwork_id from placements p where p.exhibition_id = e.id
	      order by (p.artwork_id = e.cover_id) desc, p.wall, p.x limit 4)`

const exhibitionFrom = ` from exhibitions e join users u on u.id = e.owner_id `

func scanExhibition(row pgx.Row) (*Exhibition, error) {
	var e Exhibition
	err := row.Scan(&e.ID, &e.OwnerID, &e.Slug, &e.Title, &e.Statement, &e.Room, &e.Paint, &e.Floor, &e.Light,
		&e.Status, &e.CoverID, &e.OpeningAt, &e.PublishedAt, &e.PosterVersion, &e.VisitCount, &e.ApplauseCount,
		&e.CreatedAt, &e.UpdatedAt, &e.Owner.ID, &e.Owner.Handle, &e.Owner.Name, &e.Owner.Hue,
		&e.WorkCount, &e.Preview)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &e, err
}

// Slugify makes a URL-safe slug from a title: "Rêverie à Giverny" becomes
// "reverie-a-giverny".
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// drop accents
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 48 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

func slugFor(title string) string {
	base := Slugify(title)
	if base == "" {
		base = "exhibition"
	}
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	var buf [4]byte
	rand.Read(buf[:])
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return base + "-" + string(buf[:])
}

func CreateExhibition(ctx context.Context, q db.Querier, ownerID int64, title, room, paint string) (*Exhibition, error) {
	var id int64
	err := q.QueryRow(ctx, `insert into exhibitions (owner_id, slug, title, room, paint) values ($1, $2, $3, $4, $5) returning id`,
		ownerID, slugFor(title), title, room, paint).Scan(&id)
	if err != nil {
		return nil, err
	}
	return GetExhibition(ctx, q, fmt.Sprint(id))
}

// GetExhibition finds an exhibition by numeric id or by slug.
func GetExhibition(ctx context.Context, q db.Querier, ref string) (*Exhibition, error) {
	col := "e.slug"
	if isDigits(ref) {
		col = "e.id::text"
	}
	return scanExhibition(q.QueryRow(ctx, `select `+exhibitionCols+exhibitionFrom+` where `+col+` = $1`, ref))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ExhibitionPatch changes the fields that are set.
type ExhibitionPatch struct {
	Title     *string
	Statement *string
	Room      *string
	Paint     *string
	Floor     *string
	Light     *string
	CoverID   *int64
	OpeningAt *time.Time
	// ClearOpening removes the opening time.
	ClearOpening bool
}

func UpdateExhibition(ctx context.Context, q db.Querier, e *Exhibition, p ExhibitionPatch) error {
	sets := []string{"updated_at = now()", "poster_version = poster_version + 1"}
	args := []any{e.ID}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.Title != nil {
		set("title", *p.Title)
		// Drafts follow their title; a published link never changes.
		if e.Status == "draft" && Slugify(*p.Title) != Slugify(e.Title) {
			set("slug", slugFor(*p.Title))
		}
	}
	if p.Statement != nil {
		set("statement", *p.Statement)
	}
	if p.Room != nil {
		set("room", *p.Room)
	}
	if p.Paint != nil {
		set("paint", *p.Paint)
	}
	if p.Floor != nil {
		set("floor", *p.Floor)
	}
	if p.Light != nil {
		set("light", *p.Light)
	}
	if p.CoverID != nil {
		set("cover_id", *p.CoverID)
	}
	if p.OpeningAt != nil {
		set("opening_at", *p.OpeningAt)
	} else if p.ClearOpening {
		sets = append(sets, "opening_at = null")
	}
	_, err := q.Exec(ctx, `update exhibitions set `+strings.Join(sets, ", ")+` where id = $1`, args...)
	return err
}

func SetExhibitionStatus(ctx context.Context, q db.Querier, id int64, published bool) error {
	var err error
	if published {
		_, err = q.Exec(ctx, `update exhibitions set status = 'published', published_at = coalesce(published_at, now()),
			poster_version = poster_version + 1, updated_at = now() where id = $1`, id)
	} else {
		_, err = q.Exec(ctx, `update exhibitions set status = 'draft', updated_at = now() where id = $1`, id)
	}
	return err
}

func DeleteExhibition(ctx context.Context, q db.Querier, id int64) error {
	_, err := q.Exec(ctx, `delete from exhibitions where id = $1`, id)
	return err
}

func Placements(ctx context.Context, q db.Querier, exhibitionID int64) ([]gallery.Placement, error) {
	rows, err := q.Query(ctx, `select artwork_id, wall, x, y, label from placements
		where exhibition_id = $1 order by wall, x`, exhibitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []gallery.Placement{}
	for rows.Next() {
		var p gallery.Placement
		var x, y float32
		if err := rows.Scan(&p.ArtworkID, &p.Wall, &x, &y, &p.Label); err != nil {
			return nil, err
		}
		// Stored as real; round away float32 noise (1.48 comes back as 1.4800000190734863).
		p.X, p.Y = math.Round(float64(x)*1000)/1000, math.Round(float64(y)*1000)/1000
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplacePlacements swaps the whole hang in one statement batch; the
// caller runs it in a transaction.
func ReplacePlacements(ctx context.Context, tx pgx.Tx, exhibitionID int64, ps []gallery.Placement) error {
	if _, err := tx.Exec(ctx, `delete from placements where exhibition_id = $1`, exhibitionID); err != nil {
		return err
	}
	rows := make([][]any, len(ps))
	for i, p := range ps {
		rows[i] = []any{exhibitionID, p.ArtworkID, p.Wall, p.X, p.Y, p.Label}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"placements"},
		[]string{"exhibition_id", "artwork_id", "wall", "x", "y", "label"}, pgx.CopyFromRows(rows)); err != nil {
		return err
	}
	// Keep a cover that is still hung; otherwise take the biggest-looking
	// first work.
	_, err := tx.Exec(ctx, `
		update exhibitions e set updated_at = now(), poster_version = poster_version + 1,
		cover_id = case when exists (select 1 from placements p where p.exhibition_id = e.id and p.artwork_id = e.cover_id)
		                then e.cover_id
		                else (select artwork_id from placements p where p.exhibition_id = e.id order by wall, x limit 1) end
		where e.id = $1`, exhibitionID)
	return err
}

// ExhibitionList pages exhibitions newest first, by keyset: pass the last
// row's published time and id to get the next page.
type ExhibitionList struct {
	Published bool
	OwnerID   int64
	Sort      string // "recent" or "popular"
	BeforeAt  *time.Time
	BeforeID  int64
	Offset    int
	Limit     int
}

func ListExhibitions(ctx context.Context, q db.Querier, l ExhibitionList) ([]*Exhibition, error) {
	if l.Limit <= 0 || l.Limit > 50 {
		l.Limit = 24
	}
	var conds []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if l.Published {
		conds = append(conds, "e.status = 'published'")
	}
	if l.OwnerID != 0 {
		conds = append(conds, "e.owner_id = "+arg(l.OwnerID))
	}
	order := "e.updated_at desc, e.id desc"
	page := ""
	switch {
	case l.Published && l.Sort == "popular":
		order = "(e.applause_count * 3 + e.visit_count) desc, e.id desc"
		page = fmt.Sprintf(" offset %d", max(0, l.Offset))
	case l.Published:
		order = "e.published_at desc, e.id desc"
		if l.BeforeAt != nil {
			conds = append(conds, "(e.published_at, e.id) < ("+arg(*l.BeforeAt)+", "+arg(l.BeforeID)+")")
		}
	}
	where := "true"
	if len(conds) > 0 {
		where = strings.Join(conds, " and ")
	}
	rows, err := q.Query(ctx, `select `+exhibitionCols+exhibitionFrom+` where `+where+
		` order by `+order+fmt.Sprintf(" limit %d", l.Limit)+page, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Exhibition
	for rows.Next() {
		e, err := scanExhibition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExhibitionsWith lists published exhibitions that hang a work.
func ExhibitionsWith(ctx context.Context, q db.Querier, artworkID int64) ([]*Exhibition, error) {
	rows, err := q.Query(ctx, `select `+exhibitionCols+exhibitionFrom+`
		where e.status = 'published' and exists (select 1 from placements p where p.exhibition_id = e.id and p.artwork_id = $1)
		order by e.applause_count desc, e.id desc limit 6`, artworkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Exhibition
	for rows.Next() {
		e, err := scanExhibition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ToggleApplause adds or takes back a visitor's applause and returns the
// new count and whether they are now applauding.
func ToggleApplause(ctx context.Context, q db.Querier, exhibitionID int64, visitor string) (count int, on bool, err error) {
	tag, err := q.Exec(ctx, `insert into applause (exhibition_id, visitor) values ($1, $2) on conflict do nothing`, exhibitionID, visitor)
	if err != nil {
		return 0, false, err
	}
	on = tag.RowsAffected() == 1
	if !on {
		if _, err := q.Exec(ctx, `delete from applause where exhibition_id = $1 and visitor = $2`, exhibitionID, visitor); err != nil {
			return 0, false, err
		}
	}
	err = q.QueryRow(ctx, `update exhibitions set applause_count = (select count(*) from applause where exhibition_id = $1)
		where id = $1 returning applause_count`, exhibitionID).Scan(&count)
	return count, on, err
}

func Applauded(ctx context.Context, q db.Querier, exhibitionID int64, visitor string) (bool, error) {
	var on bool
	err := q.QueryRow(ctx, `select exists(select 1 from applause where exhibition_id = $1 and visitor = $2)`, exhibitionID, visitor).Scan(&on)
	return on, err
}

// RecordVisit counts a visitor once a day.
func RecordVisit(ctx context.Context, q db.Querier, exhibitionID int64, visitor string) (int, error) {
	var n int
	err := q.QueryRow(ctx, `
		with v as (insert into visits (exhibition_id, visitor) values ($1, $2) on conflict do nothing returning 1)
		update exhibitions set visit_count = visit_count + (select count(*) from v) where id = $1
		returning visit_count`, exhibitionID, visitor).Scan(&n)
	return n, err
}

type GuestbookEntry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Hue       int       `json:"hue"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func Sign(ctx context.Context, q db.Querier, exhibitionID int64, userID *int64, name string, hue int, message string) (GuestbookEntry, error) {
	e := GuestbookEntry{Name: name, Hue: hue, Message: message}
	err := q.QueryRow(ctx, `insert into guestbook (exhibition_id, user_id, name, hue, message)
		values ($1, $2, $3, $4, $5) returning id, created_at`, exhibitionID, userID, name, hue, message).Scan(&e.ID, &e.CreatedAt)
	return e, err
}

func Guestbook(ctx context.Context, q db.Querier, exhibitionID int64, limit int) ([]GuestbookEntry, error) {
	rows, err := q.Query(ctx, `select id, name, hue, message, created_at from guestbook
		where exhibition_id = $1 order by id desc limit $2`, exhibitionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GuestbookEntry{}
	for rows.Next() {
		var g GuestbookEntry
		if err := rows.Scan(&g.ID, &g.Name, &g.Hue, &g.Message, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
