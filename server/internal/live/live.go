// Package live runs the rooms people walk through together.
//
// Each open exhibition is a room with the people in it. Visitors send where
// they stand and which way they face as often as they move; the room
// gathers those and sends everyone one batch of positions ten times a
// second, so a crowd of thirty costs each browser ten small messages a
// second rather than three hundred. Reactions, who is looking at which
// work, arrivals, departures and guestbook signatures go out at once.
//
// A visitor whose connection can't keep up misses position batches (the
// next one supersedes them anyway); one that falls further behind than
// that is disconnected rather than allowed to hold the room back.
package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
)

// Identity is who a connection belongs to.
type Identity struct {
	Visitor string // the visitor cookie
	UserID  *int64
	Name    string
	Hue     int
}

// Entry is a guestbook signature as the room broadcasts it.
type Entry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Hue       int       `json:"hue"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

// Guestbook persists signatures and reads the latest.
type Guestbook interface {
	Sign(ctx context.Context, exhibitionID int64, who Identity, message string) (Entry, error)
	Latest(ctx context.Context, exhibitionID int64) ([]Entry, error)
}

type Hub struct {
	mu        sync.Mutex
	rooms     map[int64]*room
	guestbook Guestbook
	log       *slog.Logger
	seq       uint64
}

func NewHub(g Guestbook) *Hub {
	return &Hub{rooms: map[int64]*room{}, guestbook: g, log: slog.With("component", "live")}
}

// Counts reports how many people are in each open room.
func (h *Hub) Counts() map[int64]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[int64]int, len(h.rooms))
	for id, r := range h.rooms {
		r.mu.Lock()
		out[id] = len(r.peers)
		r.mu.Unlock()
	}
	return out
}

type room struct {
	id    int64
	mu    sync.Mutex
	peers map[*peer]struct{}
	stop  chan struct{}
}

type peer struct {
	id   string
	who  Identity
	send chan []byte
	kick chan struct{}
	once sync.Once

	// Guarded by the room's lock.
	x, z, yaw float64
	looking   int64
	moved     bool

	// Owned by the read loop.
	lastPose  time.Time
	reactions float64
	reactedAt time.Time
	lastSign  time.Time
}

func (p *peer) close() { p.once.Do(func() { close(p.kick) }) }

// PeerInfo is a visitor as others see them.
type PeerInfo struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Hue     int     `json:"hue"`
	Member  bool    `json:"member"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Yaw     float64 `json:"yaw"`
	Looking int64   `json:"looking,omitempty"`
}

func (p *peer) info() PeerInfo {
	return PeerInfo{ID: p.id, Name: p.who.Name, Hue: p.who.Hue, Member: p.who.UserID != nil,
		X: p.x, Z: p.z, Yaw: p.yaw, Looking: p.looking}
}

func (h *Hub) join(exhibitionID int64, p *peer) *room {
	h.mu.Lock()
	defer h.mu.Unlock()
	r := h.rooms[exhibitionID]
	if r == nil {
		r = &room{id: exhibitionID, peers: map[*peer]struct{}{}, stop: make(chan struct{})}
		h.rooms[exhibitionID] = r
		go r.tick()
	}
	r.mu.Lock()
	r.peers[p] = struct{}{}
	r.mu.Unlock()
	return r
}

func (h *Hub) leave(r *room, p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r.mu.Lock()
	delete(r.peers, p)
	empty := len(r.peers) == 0
	r.mu.Unlock()
	if empty {
		close(r.stop)
		delete(h.rooms, r.id)
	}
}

// broadcast sends msg to everyone but skip. Must be called with r.mu held.
func (r *room) broadcast(msg []byte, skip *peer, droppable bool) {
	for p := range r.peers {
		if p == skip {
			continue
		}
		select {
		case p.send <- msg:
		default:
			if !droppable {
				p.close()
			}
		}
	}
}

func (r *room) tick() {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
		}
		r.mu.Lock()
		var poses [][4]any
		for p := range r.peers {
			if p.moved {
				poses = append(poses, [4]any{p.id, p.x, p.z, p.yaw})
				p.moved = false
			}
		}
		if len(poses) > 0 {
			msg, _ := json.Marshal(map[string]any{"t": "poses", "d": poses})
			r.broadcast(msg, nil, true)
		}
		r.mu.Unlock()
	}
}

// inbound is any message a visitor sends.
type inbound struct {
	T    string     `json:"t"`
	P    [2]float64 `json:"p"`
	R    float64    `json:"r"`
	E    string     `json:"e"`
	A    int64      `json:"a"`
	M    string     `json:"m"`
	Name string     `json:"name"`
}

var reactions = map[string]bool{"clap": true, "heart": true, "wow": true, "spark": true}

// Serve runs one visitor's connection until they leave. bounds are the
// room's half-width and half-depth; positions outside are clamped.
func (h *Hub) Serve(ctx context.Context, c *websocket.Conn, exhibitionID int64, who Identity, halfW, halfD float64) {
	c.SetReadLimit(4096)
	h.mu.Lock()
	h.seq++
	id := fmt.Sprintf("p%d", h.seq)
	h.mu.Unlock()

	p := &peer{id: id, who: who, send: make(chan []byte, 64), kick: make(chan struct{}), z: halfD - 1.2, yaw: 0}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	r := h.join(exhibitionID, p)
	defer func() {
		h.leave(r, p)
		msg, _ := json.Marshal(map[string]any{"t": "leave", "id": p.id})
		r.mu.Lock()
		r.broadcast(msg, nil, false)
		r.mu.Unlock()
	}()

	// Welcome: who you are, who's here, what they've written.
	book, err := h.guestbook.Latest(ctx, exhibitionID)
	if err != nil {
		h.log.Warn("guestbook", "err", err)
		book = []Entry{}
	}
	r.mu.Lock()
	peers := []PeerInfo{}
	for q := range r.peers {
		if q != p {
			peers = append(peers, q.info())
		}
	}
	you := p.info()
	r.mu.Unlock()
	welcome, _ := json.Marshal(map[string]any{"t": "welcome", "you": you, "peers": peers, "guestbook": book})
	p.send <- welcome
	join, _ := json.Marshal(map[string]any{"t": "join", "peer": you})
	r.mu.Lock()
	r.broadcast(join, p, false)
	r.mu.Unlock()

	// Writer.
	go func() {
		defer cancel()
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.kick:
				c.Close(websocket.StatusPolicyViolation, "connection too slow")
				return
			case msg := <-p.send:
				wctx, done := context.WithTimeout(ctx, 5*time.Second)
				err := c.Write(wctx, websocket.MessageText, msg)
				done()
				if err != nil {
					return
				}
			case <-ping.C:
				wctx, done := context.WithTimeout(ctx, 10*time.Second)
				err := c.Ping(wctx)
				done()
				if err != nil {
					return
				}
			}
		}
	}()

	// Reader.
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) && websocket.CloseStatus(err) == -1 {
				h.log.Debug("read", "peer", p.id, "err", err)
			}
			return
		}
		var m inbound
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		h.handle(ctx, r, p, m, halfW, halfD)
	}
}

func (h *Hub) handle(ctx context.Context, r *room, p *peer, m inbound, halfW, halfD float64) {
	now := time.Now()
	switch m.T {
	case "pose":
		// Twenty a second is more than enough; the rest are noise.
		if now.Sub(p.lastPose) < 45*time.Millisecond || !finite(m.P[0], m.P[1], m.R) {
			return
		}
		p.lastPose = now
		r.mu.Lock()
		p.x = round2(clamp(m.P[0], -halfW, halfW))
		p.z = round2(clamp(m.P[1], -halfD, halfD))
		p.yaw = round2(math.Remainder(m.R, 2*math.Pi))
		p.moved = true
		r.mu.Unlock()

	case "react":
		// A burst of four, then one a second.
		p.reactions = math.Min(4, p.reactions+now.Sub(p.reactedAt).Seconds())
		p.reactedAt = now
		if !reactions[m.E] || p.reactions < 1 {
			return
		}
		p.reactions--
		msg, _ := json.Marshal(map[string]any{"t": "react", "id": p.id, "e": m.E})
		r.mu.Lock()
		r.broadcast(msg, nil, true)
		r.mu.Unlock()

	case "look":
		r.mu.Lock()
		if p.looking != m.A {
			p.looking = m.A
			msg, _ := json.Marshal(map[string]any{"t": "look", "id": p.id, "a": m.A})
			r.broadcast(msg, nil, false)
		}
		r.mu.Unlock()

	case "name":
		// Guests may say who they are; members keep their account name.
		name := CleanText(m.Name, 24)
		if p.who.UserID != nil || name == "" {
			return
		}
		r.mu.Lock()
		p.who.Name = name
		msg, _ := json.Marshal(map[string]any{"t": "rename", "id": p.id, "name": name})
		r.broadcast(msg, nil, false)
		r.mu.Unlock()

	case "sign":
		text := CleanText(m.M, 280)
		if text == "" {
			return
		}
		if now.Sub(p.lastSign) < 10*time.Second {
			msg, _ := json.Marshal(map[string]any{"t": "error", "message": "Wait a few seconds before signing again."})
			select {
			case p.send <- msg:
			default:
			}
			return
		}
		p.lastSign = now
		r.mu.Lock()
		who := p.who
		r.mu.Unlock()
		e, err := h.guestbook.Sign(ctx, r.id, who, text)
		if err != nil {
			h.log.Warn("sign", "err", err)
			return
		}
		msg, _ := json.Marshal(map[string]any{"t": "signed", "entry": e, "id": p.id})
		r.mu.Lock()
		r.broadcast(msg, nil, false)
		r.mu.Unlock()
	}
}

// CleanText trims, collapses whitespace, drops control characters and
// caps the length in characters.
func CleanText(s string, limit int) string {
	var b strings.Builder
	space := false
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r == utf8.RuneError || (r < 32 && r != '\n') || r == 0x7f {
			continue
		}
		if r == ' ' || r == '\n' || r == '\t' {
			if space {
				continue
			}
			space = true
			r = ' '
		} else {
			space = false
		}
		if n == limit {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func round2(v float64) float64        { return math.Round(v*100) / 100 }

// Pigments name guests: a visitor is "Cobalt" or "Madder" rather than
// "Guest 4812", with the pigment's own hue as their colour in the room.
var Pigments = []struct {
	Name string
	Hue  int
}{
	{"Vermilion", 8}, {"Madder", 350}, {"Carmine", 345}, {"Cinnabar", 12},
	{"Sienna", 20}, {"Umber", 28}, {"Ochre", 38}, {"Gamboge", 45},
	{"Orpiment", 50}, {"Verdigris", 160}, {"Malachite", 140}, {"Viridian", 165},
	{"Terre verte", 110}, {"Cerulean", 200}, {"Cobalt", 220}, {"Smalt", 230},
	{"Lapis", 225}, {"Indigo", 240}, {"Ultramarine", 235}, {"Mauve", 280},
	{"Tyrian", 300}, {"Sepia", 30}, {"Bistre", 32}, {"Azurite", 210},
}

// Pigment picks a visitor's pigment from their cookie, so the same guest is
// the same colour every time.
func Pigment(visitor string) (string, int) {
	h := fnv.New32a()
	h.Write([]byte(visitor))
	p := Pigments[h.Sum32()%uint32(len(Pigments))]
	return p.Name, p.Hue
}
