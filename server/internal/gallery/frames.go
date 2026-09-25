package gallery

import (
	"math"
	"strings"
)

// Work is what framing needs to know about an artwork.
type Work struct {
	Kind       string
	YearStart  *int
	Culture    string
	Department string
	WidthCM    *float64
	HeightCM   *float64
	Aspect     *float64 // of the museum's photograph
}

// Hang is a work as it goes on the wall: the picture at its real size and
// the frame a curator would give it. All lengths are metres.
type Hang struct {
	// W and H are the outside of the frame.
	W float64 `json:"w"`
	H float64 `json:"h"`
	// ArtW and ArtH are the picture itself.
	ArtW float64 `json:"artW"`
	ArtH float64 `json:"artH"`
	// Frame is gilt, oak, black or silk.
	Frame string `json:"frame"`
	// Border is the width of the frame moulding; Mat the card inside it.
	Border float64 `json:"border"`
	Mat    float64 `json:"mat"`
	// Estimated is true when the museum gave no measurements and the size
	// is a guess from the kind of work.
	Estimated bool `json:"estimated"`
}

// HangFor sizes and frames a work.
//
// The museums measure the object; their photographs are sometimes cropped a
// little differently. The picture keeps the object's area but takes the
// photograph's proportions, so nothing is stretched on the wall.
func HangFor(w Work) Hang {
	aspect := 0.0
	if w.Aspect != nil && *w.Aspect > 0 {
		aspect = *w.Aspect
	}
	var aw, ah float64
	estimated := false
	if w.WidthCM != nil && w.HeightCM != nil && *w.WidthCM > 0 && *w.HeightCM > 0 {
		aw, ah = *w.WidthCM/100, *w.HeightCM/100
		if aspect > 0 {
			area := aw * ah
			aw, ah = math.Sqrt(area*aspect), math.Sqrt(area/aspect)
		}
	} else {
		estimated = true
		if aspect == 0 {
			aspect = 0.8
		}
		long := map[string]float64{"painting": 0.9, "print": 0.38, "drawing": 0.45}[w.Kind]
		if long == 0 {
			long = 0.6
		}
		if aspect >= 1 {
			aw, ah = long, long/aspect
		} else {
			aw, ah = long*aspect, long
		}
	}
	// A handful of works are room-sized; keep them hangable.
	if long := math.Max(aw, ah); long > 4.5 {
		aw, ah = aw*4.5/long, ah*4.5/long
	}
	if long := math.Max(aw, ah); long < 0.12 {
		aw, ah = aw*0.12/long, ah*0.12/long
	}

	long := math.Max(aw, ah)
	h := Hang{ArtW: r3(aw), ArtH: r3(ah), Estimated: estimated}
	asian := containsAny(strings.ToLower(w.Culture+" "+w.Department), "japan", "china", "chinese", "korea", "asian")
	year := 1800
	if w.YearStart != nil {
		year = *w.YearStart
	}
	switch {
	case w.Kind == "print" || w.Kind == "drawing":
		// Works on paper go behind a card mat in a slim moulding.
		h.Mat = clamp(0.05+0.1*long, 0.05, 0.12)
		h.Border = 0.025
		h.Frame = "black"
		if asian {
			h.Frame = "oak"
		}
	case asian:
		// Scrolls and screens keep a mounting of silk brocade.
		h.Frame = "silk"
		h.Border = clamp(0.04+0.04*long, 0.04, 0.12)
	case year < 1650:
		h.Frame = "gilt"
		h.Border = clamp(0.06+0.05*long, 0.06, 0.16)
	case year < 1900:
		h.Frame = "gilt"
		h.Border = clamp(0.05+0.04*long, 0.05, 0.14)
	default:
		h.Frame = "oak"
		h.Border = 0.03
	}
	// However grand the moulding, a small work shouldn't drown in it.
	h.Border = math.Min(h.Border, 0.16*long)
	h.Border, h.Mat = r3(h.Border), r3(h.Mat)
	h.W = r3(h.ArtW + 2*(h.Border+h.Mat))
	h.H = r3(h.ArtH + 2*(h.Border+h.Mat))
	return h
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func r3(v float64) float64            { return math.Round(v*1000) / 1000 }
