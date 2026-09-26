package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Nafiz001/vernissage/server/internal/api"
	"github.com/Nafiz001/vernissage/server/internal/colorindex"
	"github.com/Nafiz001/vernissage/server/internal/config"
	"github.com/Nafiz001/vernissage/server/internal/imaging"
	"github.com/Nafiz001/vernissage/server/internal/live"
	"github.com/Nafiz001/vernissage/server/internal/pipeline"
	"github.com/Nafiz001/vernissage/server/internal/testdb"
)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	srv  *httptest.Server
	ids  []int64
}

func setup(t *testing.T) *env {
	return setupWith(t, config.Config{Origins: []string{"http://example.test"}})
}

func setupWith(t *testing.T, cfg config.Config) *env {
	pool := testdb.Open(t)
	ids := testdb.Insert(t, pool,
		testdb.Work{Title: "The Great Wave", Artist: "Katsushika Hokusai", Kind: "print", Year: 1831, WidthCM: 37.9, HeightCM: 25.7, Colors: []string{"#e8e2d0", "#1f4f8f"}},
		testdb.Work{Title: "Wheat Field with Cypresses", Artist: "Vincent van Gogh", Kind: "painting", Year: 1889, WidthCM: 93.4, HeightCM: 73, Colors: []string{"#c9a94e", "#3a6690", "#2f5f3a"},
			Description: "Van Gogh painted the wheat fields near Saint-Rémy."},
		testdb.Work{Title: "Portrait of a Man", Artist: "Rembrandt van Rijn", Kind: "painting", Year: 1632, WidthCM: 80, HeightCM: 100, Colors: []string{"#2a2018", "#8a6a4a"}},
		testdb.Work{Title: "Water Lilies", Artist: "Claude Monet", Kind: "painting", Year: 1915, WidthCM: 425, HeightCM: 200, Colors: []string{"#6a7a5a", "#9a8ab0"}},
	)
	index := colorindex.New()
	if err := index.Load(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	images, err := imaging.New(t.TempDir(), "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	hub := live.NewHub(api.Guestbook{Pool: pool})
	pipe := pipeline.New(pool, images, "test", 0)
	srv := httptest.NewServer(api.New(cfg, pool, images, index, hub, pipe).Handler())
	t.Cleanup(srv.Close)
	return &env{t: t, pool: pool, srv: srv, ids: ids}
}

// client is a browser: it keeps cookies and sends our Origin on writes.
type client struct {
	e    *env
	http *http.Client
}

func (e *env) browser() *client {
	jar, _ := cookiejar.New(nil)
	return &client{e: e, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, into any) int {
	c.e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if method != http.MethodGet {
		req.Header.Set("Origin", "http://example.test")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			c.e.t.Fatalf("%s %s: decoding: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func (c *client) signup(name, email string) {
	c.e.t.Helper()
	if code := c.do("POST", "/api/auth/signup", map[string]string{"name": name, "email": email, "password": "correct horse"}, nil); code != 201 {
		c.e.t.Fatalf("signup %s: %d", email, code)
	}
}

type detail struct {
	Exhibition struct {
		ID       int64  `json:"id"`
		Slug     string `json:"slug"`
		Status   string `json:"status"`
		CoverID  *int64 `json:"coverId"`
		Applause int    `json:"applause"`
		Visits   int    `json:"visits"`
	} `json:"exhibition"`
	Placements []struct {
		ArtworkID int64   `json:"artworkId"`
		Wall      int     `json:"wall"`
		X         float64 `json:"x"`
		Y         float64 `json:"y"`
		Label     string  `json:"label"`
	} `json:"placements"`
	Problems []struct {
		ArtworkID int64  `json:"artworkId"`
		Message   string `json:"message"`
	} `json:"problems"`
}

func TestAccounts(t *testing.T) {
	e := setup(t)
	c := e.browser()
	c.signup("Ada Curator", "ada@example.test")

	var me struct {
		User *struct{ Handle, Name string } `json:"user"`
	}
	c.do("GET", "/api/me", nil, &me)
	if me.User == nil || me.User.Handle != "ada-curator" {
		t.Fatalf("after signup /api/me says %+v", me.User)
	}

	// A second account with the same email is refused, and says why.
	var fail struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if code := e.browser().do("POST", "/api/auth/signup", map[string]string{"name": "Ada", "email": "ADA@example.test", "password": "correct horse"}, &fail); code != 409 || fail.Error.Fields["email"] == "" {
		t.Errorf("duplicate email: %d %+v", code, fail)
	}
	// Same name, new email: a different handle.
	d := e.browser()
	d.signup("Ada Curator", "ada2@example.test")
	d.do("GET", "/api/me", nil, &me)
	if me.User.Handle == "ada-curator" {
		t.Error("two accounts share a handle")
	}

	c.do("POST", "/api/auth/logout", nil, nil)
	c.do("GET", "/api/me", nil, &me)
	if me.User != nil {
		t.Error("still signed in after signing out")
	}
	if code := c.do("POST", "/api/auth/login", map[string]string{"email": "ada@example.test", "password": "wrong horse"}, nil); code != 401 {
		t.Errorf("wrong password: %d", code)
	}
	if code := c.do("POST", "/api/auth/login", map[string]string{"email": "nobody@example.test", "password": "correct horse"}, nil); code != 401 {
		t.Errorf("unknown email: %d", code)
	}
	if code := c.do("POST", "/api/auth/login", map[string]string{"email": "ada@example.test", "password": "correct horse"}, nil); code != 200 {
		t.Errorf("right password: %d", code)
	}
}

func TestCrossSiteWritesRefused(t *testing.T) {
	e := setup(t)
	for _, origin := range []string{"https://evil.test", "null"} {
		req, _ := http.NewRequest("POST", e.srv.URL+"/api/auth/signup", strings.NewReader(`{}`))
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("Origin %s: %d, want 403", origin, resp.StatusCode)
		}
	}
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/auth/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("cross-site fetch without Origin: %d", resp.StatusCode)
	}
}

func TestSearch(t *testing.T) {
	e := setup(t)
	c := e.browser()
	type page struct {
		Items []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
		Total int  `json:"total"`
		Fuzzy bool `json:"fuzzy"`
	}
	var p page
	c.do("GET", "/api/artworks?q=wheat+fields", nil, &p)
	if p.Total != 1 || p.Items[0].Title != "Wheat Field with Cypresses" {
		t.Errorf("'wheat fields' (stemmed) found %+v", p)
	}
	c.do("GET", "/api/artworks?q=saint-remy", nil, &p)
	if p.Total != 1 {
		t.Errorf("unaccented 'saint-remy' in the description found %d", p.Total)
	}
	c.do("GET", "/api/artworks?q=hokusia", nil, &p)
	if !p.Fuzzy || p.Total != 1 || p.Items[0].Title != "The Great Wave" {
		t.Errorf("misspelt 'hokusia' found %+v", p)
	}
	c.do("GET", "/api/artworks?color=%231f4f8f", nil, &p)
	if p.Total == 0 || p.Items[0].Title != "The Great Wave" {
		t.Errorf("blue search found %+v", p)
	}
	c.do("GET", "/api/artworks?color=%231f4f8f&kind=painting", nil, &p)
	for _, it := range p.Items {
		if it.Title == "The Great Wave" {
			t.Error("print returned for kind=painting")
		}
	}
	c.do("GET", "/api/artworks?q=wheat&color=%233a6690", nil, &p)
	if p.Total != 1 {
		t.Errorf("text and colour together: %d", p.Total)
	}
	if code := c.do("GET", "/api/artworks?color=blue", nil, nil); code != 400 {
		t.Errorf("bad colour: %d", code)
	}
}

func TestExhibitionLifecycle(t *testing.T) {
	e := setup(t)
	ada := e.browser()
	ada.signup("Ada", "ada@example.test")
	bob := e.browser()
	bob.signup("Bob", "bob@example.test")
	guest := e.browser()

	// Starting from a selection hangs it at once, validly.
	var d detail
	if code := ada.do("POST", "/api/exhibitions", map[string]any{"title": "Blue and Gold", "room": "salon", "artworkIds": e.ids}, &d); code != 201 {
		t.Fatalf("create: %d", code)
	}
	if len(d.Placements) != len(e.ids) || len(d.Problems) != 0 {
		t.Fatalf("auto-hung %d works with problems %+v", len(d.Placements), d.Problems)
	}
	slug := d.Exhibition.Slug
	if !strings.HasPrefix(slug, "blue-and-gold-") {
		t.Errorf("slug %q", slug)
	}

	// Drafts are private.
	if code := bob.do("GET", "/api/exhibitions/"+slug, nil, nil); code != 404 {
		t.Errorf("someone else's draft: %d", code)
	}
	if code := bob.do("PATCH", "/api/exhibitions/"+slug, map[string]string{"title": "Mine now"}, nil); code != 404 {
		t.Errorf("patch someone else's draft: %d", code)
	}

	// Two frames in one place are refused, and the server says which.
	bad := d.Placements
	bad[1].Wall, bad[1].X, bad[1].Y = bad[0].Wall, bad[0].X+0.1, bad[0].Y
	var fail struct {
		Error struct {
			Problems []struct{ ArtworkID int64 } `json:"problems"`
		} `json:"error"`
	}
	if code := ada.do("PUT", "/api/exhibitions/"+slug+"/placements", map[string]any{"placements": bad}, &fail); code != 422 || len(fail.Error.Problems) == 0 {
		t.Errorf("overlapping hang: %d %+v", code, fail)
	}

	// A labelled, valid hang saves.
	good := d.Placements[:2]
	good[0].Label, good[1].Wall, good[1].X, good[1].Y = "The sightline.", 1, 5, 1.48
	if code := ada.do("PUT", "/api/exhibitions/"+slug+"/placements", map[string]any{"placements": good}, &d); code != 200 {
		t.Fatalf("save hang: %d", code)
	}
	if len(d.Placements) != 2 || d.Exhibition.CoverID == nil {
		t.Errorf("after save: %d placements, cover %v", len(d.Placements), d.Exhibition.CoverID)
	}

	// Moving to a small room makes the Monet too big; the server reports it
	// rather than losing the hang, and won't open the doors until it's fixed.
	ada.do("PUT", "/api/exhibitions/"+slug+"/placements", map[string]any{"placements": []any{
		map[string]any{"artworkId": e.ids[3], "wall": 0, "x": 8, "y": 1.6},
	}}, &d)
	ada.do("PATCH", "/api/exhibitions/"+slug, map[string]string{"room": "cabinet"}, &d)
	if len(d.Problems) == 0 {
		t.Error("a 4.25 m painting fits a 7 m cabinet with room to spare")
	}
	if code := ada.do("POST", "/api/exhibitions/"+slug+"/publish", nil, nil); code != 422 {
		t.Errorf("publish with problems: %d", code)
	}
	ada.do("PATCH", "/api/exhibitions/"+slug, map[string]string{"room": "salon"}, &d)

	// Open the doors: visible to all, and a poster job is queued.
	if code := ada.do("POST", "/api/exhibitions/"+slug+"/publish", nil, &d); code != 200 || d.Exhibition.Status != "published" {
		t.Fatalf("publish: %d %s", code, d.Exhibition.Status)
	}
	var jobs int
	e.pool.QueryRow(context.Background(), `select count(*) from jobs where kind = 'poster'`).Scan(&jobs)
	if jobs == 0 {
		t.Error("no poster queued on publish")
	}
	if code := guest.do("GET", "/api/exhibitions/"+slug, nil, &d); code != 200 {
		t.Errorf("guest sees published: %d", code)
	}

	// Applause toggles per visitor; visits count once a day.
	var ap struct {
		Applause  int  `json:"applause"`
		Applauded bool `json:"applauded"`
	}
	guest.do("POST", "/api/exhibitions/"+slug+"/applause", nil, &ap)
	bob.do("POST", "/api/exhibitions/"+slug+"/applause", nil, &ap)
	if ap.Applause != 2 || !ap.Applauded {
		t.Errorf("two applauses: %+v", ap)
	}
	guest.do("POST", "/api/exhibitions/"+slug+"/applause", nil, &ap)
	if ap.Applause != 1 || ap.Applauded {
		t.Errorf("applause taken back: %+v", ap)
	}
	var v struct{ Visits int }
	guest.do("POST", "/api/exhibitions/"+slug+"/visit", nil, &v)
	guest.do("POST", "/api/exhibitions/"+slug+"/visit", nil, &v)
	bob.do("POST", "/api/exhibitions/"+slug+"/visit", nil, &v)
	if v.Visits != 2 {
		t.Errorf("visits %d, want 2 (the guest twice counts once)", v.Visits)
	}

	// Listed, newest first, with a working next-page cursor.
	var list struct {
		Items []struct{ Slug string } `json:"items"`
		Next  *string                 `json:"next"`
	}
	guest.do("GET", "/api/exhibitions?limit=1", nil, &list)
	if len(list.Items) != 1 || list.Items[0].Slug != slug {
		t.Errorf("listing: %+v", list)
	}

	if code := bob.do("DELETE", "/api/exhibitions/"+slug, nil, nil); code != 403 {
		t.Errorf("delete someone else's: %d", code)
	}
	if code := ada.do("DELETE", "/api/exhibitions/"+slug, nil, nil); code != 200 {
		t.Errorf("delete own: %d", code)
	}
}

// Two people in one room see each other arrive, move, react and sign.
func TestLiveRoom(t *testing.T) {
	e := setup(t)
	ada := e.browser()
	ada.signup("Ada", "ada@example.test")
	var d detail
	ada.do("POST", "/api/exhibitions", map[string]any{"title": "Together", "artworkIds": e.ids[:2]}, &d)
	ada.do("POST", "/api/exhibitions/"+d.Exhibition.Slug+"/publish", nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/ws/exhibitions/" + d.Exhibition.Slug
	dial := func(c *client) *websocket.Conn {
		u := e.srv.URL
		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPClient: c.http,
			HTTPHeader: http.Header{"Origin": {"http://example.test"}},
		})
		if err != nil {
			t.Fatalf("dial %s: %v", u, err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		return conn
	}
	read := func(c *websocket.Conn, want string) map[string]any {
		t.Helper()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %q: %v", want, err)
			}
			var m map[string]any
			json.Unmarshal(data, &m)
			if m["t"] == want {
				return m
			}
		}
	}
	send := func(c *websocket.Conn, m any) {
		b, _ := json.Marshal(m)
		if err := c.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}

	a := dial(ada)
	welcome := read(a, "welcome")
	if you := welcome["you"].(map[string]any); you["name"] != "Ada" || you["member"] != true {
		t.Errorf("Ada welcomed as %+v", you)
	}

	guest := e.browser()
	g := dial(guest)
	gw := read(g, "welcome")
	if len(gw["peers"].([]any)) != 1 {
		t.Errorf("guest sees %d others, want Ada", len(gw["peers"].([]any)))
	}
	join := read(a, "join")
	guestID := join["peer"].(map[string]any)["id"]
	if name := join["peer"].(map[string]any)["name"].(string); name == "" {
		t.Error("guest has no pigment name")
	}

	send(g, map[string]any{"t": "pose", "p": []float64{1.5, -2}, "r": 0.5})
	poses := read(a, "poses")
	found := false
	for _, p := range poses["d"].([]any) {
		row := p.([]any)
		if row[0] == guestID && row[1] == 1.5 && row[2] == -2.0 {
			found = true
		}
	}
	if !found {
		t.Errorf("Ada didn't see the guest move: %+v", poses)
	}

	send(g, map[string]any{"t": "react", "e": "clap"})
	if r := read(a, "react"); r["e"] != "clap" {
		t.Errorf("reaction %+v", r)
	}

	send(g, map[string]any{"t": "sign", "m": "  Lovely   room.\n"})
	signed := read(a, "signed")
	if msg := signed["entry"].(map[string]any)["message"]; msg != "Lovely room." {
		t.Errorf("signature %q", msg)
	}
	var n int
	e.pool.QueryRow(ctx, `select count(*) from guestbook`).Scan(&n)
	if n != 1 {
		t.Errorf("guestbook rows %d", n)
	}

	g.Close(websocket.StatusNormalClosure, "")
	if l := read(a, "leave"); l["id"] != guestID {
		t.Errorf("leave %+v", l)
	}

	// Another origin can't open a socket.
	_, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.test"}}})
	if err == nil {
		t.Error("cross-origin WebSocket accepted")
	}
}

// With Cloudinary configured, pictures redirect to its fetch CDN and deep
// zoom tiles are off.
func TestCloudinaryPictures(t *testing.T) {
	e := setupWith(t, config.Config{Origins: []string{"http://example.test"}, Cloudinary: "demo-cloud"})
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(fmt.Sprintf("%s/img/%d/800.jpg", e.srv.URL, e.ids[0]))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	want := "https://res.cloudinary.com/demo-cloud/image/fetch/c_limit,w_800,q_auto,f_auto/https://example.org/a.jpg"
	if resp.StatusCode != 302 || resp.Header.Get("Location") != want {
		t.Errorf("800 px: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp, _ = client.Get(fmt.Sprintf("%s/img/%d/2400.jpg", e.srv.URL, e.ids[0]))
	resp.Body.Close()
	if !strings.HasSuffix(resp.Header.Get("Location"), "/https://example.org/b.jpg") {
		t.Errorf("2400 px should come from the large picture: %s", resp.Header.Get("Location"))
	}
	resp, _ = client.Get(fmt.Sprintf("%s/dzi/%d.dzi", e.srv.URL, e.ids[0]))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("deep zoom descriptor: %d", resp.StatusCode)
	}
}

// A page on another domain gets a ticket through the API and opens the
// room with it; the room knows who they are without cookies.
func TestLiveTicket(t *testing.T) {
	e := setup(t)
	ada := e.browser()
	ada.signup("Ada", "ada@example.test")
	var d detail
	ada.do("POST", "/api/exhibitions", map[string]any{"title": "Draft", "artworkIds": e.ids[:1]}, &d)
	var tk struct{ Ticket string }
	ada.do("GET", "/api/live-ticket", nil, &tk)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/ws/exhibitions/" + d.Exhibition.Slug
	// No cookies at all: a plain dialer, as a browser on another site.
	c, _, err := websocket.Dial(ctx, url+"?ticket="+tk.Ticket, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://example.test"}}})
	if err != nil {
		t.Fatalf("with ticket (a draft only Ada may enter): %v", err)
	}
	defer c.CloseNow()
	_, data, err := c.Read(ctx)
	if err != nil || !strings.Contains(string(data), `"name":"Ada"`) {
		t.Errorf("welcome %s %v", data, err)
	}
	if _, _, err := websocket.Dial(ctx, url+"?ticket=forged.ticket", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"http://example.test"}}}); err == nil {
		t.Error("a forged ticket opened the room")
	}
}
