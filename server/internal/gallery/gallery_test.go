package gallery

import (
	"fmt"
	"testing"
)

func f(v float64) *float64 { return &v }
func i(v int) *int         { return &v }

// The browser maps over every room's benches; none may be null in JSON.
func TestRoomsHaveBenchLists(t *testing.T) {
	for _, r := range Rooms {
		if r.Benches == nil {
			t.Errorf("%s has a nil bench list", r.Key)
		}
	}
}

func TestFramingFollowsTheWork(t *testing.T) {
	cases := []struct {
		name  string
		work  Work
		frame string
		mat   bool
	}{
		{"baroque canvas", Work{Kind: "painting", YearStart: i(1620), WidthCM: f(120), HeightCM: f(160)}, "gilt", false},
		{"impressionist", Work{Kind: "painting", YearStart: i(1889), WidthCM: f(92), HeightCM: f(73)}, "gilt", false},
		{"modern", Work{Kind: "painting", YearStart: i(1912), WidthCM: f(60), HeightCM: f(80)}, "oak", false},
		{"ukiyo-e", Work{Kind: "print", Culture: "Japan", YearStart: i(1831), WidthCM: f(37.9), HeightCM: f(25.7)}, "oak", true},
		{"etching", Work{Kind: "print", Culture: "Netherlands", YearStart: i(1650), WidthCM: f(20), HeightCM: f(15)}, "black", true},
		{"hanging scroll", Work{Kind: "painting", Department: "Chinese Art", YearStart: i(1500), WidthCM: f(50), HeightCM: f(160)}, "silk", false},
	}
	for _, c := range cases {
		h := HangFor(c.work)
		if h.Frame != c.frame || (h.Mat > 0) != c.mat {
			t.Errorf("%s: frame %s mat %.3f", c.name, h.Frame, h.Mat)
		}
		if h.W <= h.ArtW || h.H <= h.ArtH {
			t.Errorf("%s: frame %vx%v smaller than art %vx%v", c.name, h.W, h.H, h.ArtW, h.ArtH)
		}
	}
}

func TestHangKeepsAreaTakesPhotoProportions(t *testing.T) {
	// Measured 100 x 100 cm but photographed at 2:1: still one square metre.
	h := HangFor(Work{Kind: "painting", WidthCM: f(100), HeightCM: f(100), Aspect: f(2)})
	if got := h.ArtW * h.ArtH; got < 0.99 || got > 1.01 {
		t.Errorf("area %.3f m², want 1", got)
	}
	if r := h.ArtW / h.ArtH; r < 1.99 || r > 2.01 {
		t.Errorf("aspect %.3f, want 2", r)
	}
	if !HangFor(Work{Kind: "print", Aspect: f(1.5)}).Estimated {
		t.Error("a work without measurements isn't marked estimated")
	}
}

func items(n int, w, h float64) ([]Item, map[int64]Hang) {
	var out []Item
	hangs := map[int64]Hang{}
	for k := range n {
		hg := Hang{W: w, H: h}
		out = append(out, Item{ArtworkID: int64(k + 1), Hang: hg})
		hangs[int64(k+1)] = hg
	}
	return out, hangs
}

// Whatever AutoHang produces, Validate accepts.
func TestAutoHangIsValid(t *testing.T) {
	for _, room := range Rooms {
		for _, n := range []int{1, 2, 5, 9, 14} {
			t.Run(fmt.Sprintf("%s/%d", room.Key, n), func(t *testing.T) {
				its, hangs := items(n, 0.9, 1.1)
				// One big sightline piece.
				its[n-1].Hang = Hang{W: 2.2, H: 1.8}
				hangs[its[n-1].ArtworkID] = its[n-1].Hang
				ps, err := room.AutoHang(its)
				if err != nil {
					if room.Key == "cabinet" && n >= 9 {
						return // genuinely doesn't fit
					}
					t.Fatal(err)
				}
				if len(ps) != n {
					t.Fatalf("hung %d of %d", len(ps), n)
				}
				if probs := room.Validate(ps, hangs); len(probs) > 0 {
					t.Fatalf("auto-hang rejected: %+v", probs)
				}
				for _, p := range ps {
					if p.ArtworkID == its[n-1].ArtworkID && p.Wall != 0 {
						t.Errorf("largest work on wall %d, want the far wall", p.Wall)
					}
				}
			})
		}
	}
}

func TestAutoHangRefusesWhatWontFit(t *testing.T) {
	cabinet, _ := RoomByKey("cabinet")
	its, _ := items(20, 1.2, 1)
	if _, err := cabinet.AutoHang(its); err == nil {
		t.Error("hung 24 metres of frames in a 26-metre room with a door")
	}
	tall, _ := items(1, 1, 3.5)
	if _, err := cabinet.AutoHang(tall); err == nil {
		t.Error("hung a 3.5 m work under a 3.6 m ceiling")
	}
}

func TestValidateCatchesEachProblem(t *testing.T) {
	salon, _ := RoomByKey("salon")
	hangs := map[int64]Hang{1: {W: 1, H: 1}, 2: {W: 1, H: 1}}
	d0, _ := salon.DoorSpan()
	cases := map[string][]Placement{
		"off the wall":   {{ArtworkID: 1, Wall: 0, X: 0.2, Y: 1.5}},
		"in the doorway": {{ArtworkID: 1, Wall: 2, X: d0 + 0.2, Y: 1.5}},
		"on the floor":   {{ArtworkID: 1, Wall: 1, X: 5, Y: 0.4}},
		"overlapping":    {{ArtworkID: 1, Wall: 0, X: 5, Y: 1.5}, {ArtworkID: 2, Wall: 0, X: 5.5, Y: 1.6}},
		"hung twice":     {{ArtworkID: 1, Wall: 0, X: 3, Y: 1.5}, {ArtworkID: 1, Wall: 1, X: 3, Y: 1.5}},
		"unknown work":   {{ArtworkID: 9, Wall: 0, X: 3, Y: 1.5}},
		"no such wall":   {{ArtworkID: 1, Wall: 4, X: 3, Y: 1.5}},
	}
	for name, ps := range cases {
		if len(salon.Validate(ps, hangs)) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
	ok := []Placement{{ArtworkID: 1, Wall: 0, X: 5, Y: 1.5}, {ArtworkID: 2, Wall: 0, X: 6.2, Y: 1.5}}
	if probs := salon.Validate(ok, hangs); len(probs) > 0 {
		t.Errorf("rejected a good hang: %+v", probs)
	}
}
