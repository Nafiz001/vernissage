package gallery

import (
	"fmt"
	"math"
	"sort"
)

const (
	// EyeLine is the height of the centre of a hung work: the museum
	// convention of 57 inches, give or take.
	EyeLine = 1.48
	// Corner is the clear space kept at each end of a wall.
	Corner = 0.35
	// MinGap is the least space between two frames.
	MinGap = 0.08
	// Floor and Ceiling are the clear space kept above the floor (for the
	// skirting and people's feet) and under the ceiling.
	FloorClear   = 0.35
	CeilingClear = 0.25
	// MaxWorks is the most works one exhibition can hang.
	MaxWorks = 40
)

// Placement is one work on a wall.
type Placement struct {
	ArtworkID int64   `json:"artworkId"`
	Wall      int     `json:"wall"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Label     string  `json:"label"`
}

// Problem is a reason a hang can't be saved, tied to the work at fault.
type Problem struct {
	ArtworkID int64  `json:"artworkId"`
	Message   string `json:"message"`
}

type span struct{ from, to float64 }

// segments are the stretches of wall w that can hold a frame edge to edge.
func (r Room) segments(w int) []span {
	l := r.WallLength(w)
	if w != 2 {
		return []span{{Corner, l - Corner}}
	}
	d0, d1 := r.DoorSpan()
	return []span{{Corner, d0 - Corner}, {d1 + Corner, l - Corner}}
}

// Validate checks every placement fits its wall, keeps clear of the door,
// the floor, the ceiling and the other frames.
func (r Room) Validate(ps []Placement, hangs map[int64]Hang) []Problem {
	var out []Problem
	if len(ps) > MaxWorks {
		out = append(out, Problem{Message: fmt.Sprintf("An exhibition can hang up to %d works.", MaxWorks)})
	}
	type box struct {
		id             int64
		wall           int
		x0, x1, y0, y1 float64
	}
	var boxes []box
	seen := map[int64]bool{}
	for _, p := range ps {
		h, ok := hangs[p.ArtworkID]
		switch {
		case !ok:
			out = append(out, Problem{p.ArtworkID, "This work isn't in the collection."})
			continue
		case seen[p.ArtworkID]:
			out = append(out, Problem{p.ArtworkID, "This work is hung twice."})
			continue
		case p.Wall < 0 || p.Wall > 3:
			out = append(out, Problem{p.ArtworkID, "This work is on a wall the room doesn't have."})
			continue
		}
		seen[p.ArtworkID] = true
		b := box{p.ArtworkID, p.Wall, p.X - h.W/2, p.X + h.W/2, p.Y - h.H/2, p.Y + h.H/2}
		// The corner margin only guides AutoHang; the wall's ends and the
		// doorway are the hard limits.
		const eps = 1e-6
		inside := b.x0 >= -eps && b.x1 <= r.WallLength(p.Wall)+eps
		if p.Wall == 2 {
			d0, d1 := r.DoorSpan()
			inside = inside && (b.x1 <= d0+eps || b.x0 >= d1-eps)
		}
		if !inside {
			if p.Wall == 2 {
				out = append(out, Problem{p.ArtworkID, "This work runs into the doorway or off the end of the wall."})
			} else {
				out = append(out, Problem{p.ArtworkID, "This work runs off the end of the wall."})
			}
			continue
		}
		if b.y0 < FloorClear-eps || b.y1 > r.Height-CeilingClear+eps {
			if h.H > r.Height-FloorClear-CeilingClear {
				out = append(out, Problem{p.ArtworkID, fmt.Sprintf("This work is too tall for the %s; try a taller room.", r.Name)})
			} else {
				out = append(out, Problem{p.ArtworkID, "This work hangs too close to the floor or the ceiling."})
			}
			continue
		}
		boxes = append(boxes, b)
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			if a.wall == b.wall && a.x0 < b.x1+MinGap && b.x0 < a.x1+MinGap && a.y0 < b.y1+MinGap && b.y0 < a.y1+MinGap {
				out = append(out, Problem{b.id, "This work overlaps another frame."})
			}
		}
	}
	return out
}

// Item is a work to hang and its framed size.
type Item struct {
	ArtworkID int64
	Hang      Hang
}

// AutoHang lays works out the way a curator would start: the largest work
// alone on the far wall, facing whoever comes through the door, the rest
// shared between the walls so each is about as full as the others, each
// wall symmetrical about its biggest piece, every centre on the eye line.
func (r Room) AutoHang(items []Item) ([]Placement, error) {
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > MaxWorks {
		return nil, fmt.Errorf("an exhibition can hang up to %d works", MaxWorks)
	}
	sorted := append([]Item(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Hang.W*sorted[i].Hang.H > sorted[j].Hang.W*sorted[j].Hang.H
	})
	for _, it := range sorted {
		if it.Hang.H > r.Height-FloorClear-CeilingClear {
			return nil, fmt.Errorf("a work is too tall for the %s", r.Name)
		}
	}

	type slot struct {
		wall  int
		span  span
		items []Item
		used  float64
	}
	var slots []*slot
	for _, w := range []int{0, 1, 3, 2} {
		for _, s := range r.segments(w) {
			if s.to-s.from > 0.3 {
				slots = append(slots, &slot{wall: w, span: s})
			}
		}
	}
	need := func(s *slot, it Item) float64 {
		return s.used + it.Hang.W + MinGap*3*float64(len(s.items)+1)
	}
	for i, it := range sorted {
		var best *slot
		bestFill := math.Inf(1)
		for _, s := range slots {
			length := s.span.to - s.span.from
			if need(s, it) > length {
				continue
			}
			fill := (s.used + it.Hang.W) / length
			if i == 0 && s.wall == 0 {
				fill = -1 // the sightline piece
			}
			if s.wall == 2 {
				fill += 0.15 // the door wall is seen last; fill it last
			}
			if fill < bestFill {
				best, bestFill = s, fill
			}
		}
		if best == nil {
			return nil, fmt.Errorf("there isn't enough wall for %d works in the %s; try a bigger room or fewer works", len(items), r.Name)
		}
		best.items = append(best.items, it)
		best.used += it.Hang.W
	}

	var out []Placement
	for _, s := range slots {
		if len(s.items) == 0 {
			continue
		}
		// Biggest in the middle, the rest alternating outwards.
		order := make([]Item, len(s.items))
		mid := (len(s.items) - 1) / 2
		for i, it := range s.items {
			off := (i + 1) / 2
			if i%2 == 1 {
				order[mid+off] = it
			} else {
				order[mid-off] = it
			}
		}
		// A lone work on a door-wall segment sits in the middle of it.
		gap := (s.span.to - s.span.from - s.used) / float64(len(order)+1)
		x := s.span.from + gap
		for _, it := range order {
			y := math.Max(EyeLine, FloorClear+it.Hang.H/2+0.2)
			y = math.Min(y, r.Height-CeilingClear-it.Hang.H/2)
			out = append(out, Placement{ArtworkID: it.ArtworkID, Wall: s.wall, X: r3(x + it.Hang.W/2), Y: r3(y)})
			x += it.Hang.W + gap
		}
	}
	return out, nil
}
