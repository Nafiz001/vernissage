package colorindex

import (
	"testing"

	"github.com/nafizahmed/vernissage/server/internal/color"
)

func swatch(hex string, w float64) color.Swatch {
	c, _ := color.ParseHex(hex)
	return color.Swatch{Hex: hex, L: c.L, A: c.A, B: c.B, Weight: w}
}

func year(y int) *int { return &y }

func sample() *Index {
	x := New()
	x.Put(Work{ID: 1, Kind: "painting", Source: "met", Year: year(1830), Lightness: 0.7, Colorfulness: 40,
		Palette: []color.Swatch{swatch("#1f4f8f", 0.6), swatch("#e8e2d0", 0.4)}}) // mostly blue: the Great Wave
	x.Put(Work{ID: 2, Kind: "painting", Source: "cma", Year: year(1650), Lightness: 0.3, Colorfulness: 10,
		Palette: []color.Swatch{swatch("#3a2a1c", 0.85), swatch("#1f4f8f", 0.15)}}) // brown, a blue ribbon
	x.Put(Work{ID: 3, Kind: "print", Source: "met", Year: year(1890), Lightness: 0.8, Colorfulness: 70,
		Palette: []color.Swatch{swatch("#d8a02a", 0.7), swatch("#2a6a3a", 0.3)}}) // yellow and green
	x.Put(Work{ID: 4, Kind: "painting", Source: "cma", Year: year(1885), Lightness: 0.65, Colorfulness: 45,
		Palette: []color.Swatch{swatch("#2255a0", 0.5), swatch("#efe6d2", 0.5)}}) // blue and cream again
	x.mu.Lock()
	x.computeMoods()
	x.mu.Unlock()
	return x
}

func ids(hits []Hit) []int64 {
	var out []int64
	for _, h := range hits {
		out = append(out, h.ID)
	}
	return out
}

func TestMoreOfTheColourRanksHigher(t *testing.T) {
	x := sample()
	blue, _ := color.ParseHex("#1f4f8f")
	got := ids(x.Search(Query{Colors: []color.Lab{blue}}))
	if len(got) != 3 || got[0] != 1 || got[len(got)-1] != 2 {
		t.Errorf("blue search %v, want the wave first and the ribbon last, no yellow print", got)
	}
}

func TestNearbyShadeStillMatches(t *testing.T) {
	x := sample()
	nearBlue, _ := color.ParseHex("#2a5896")
	if got := ids(x.Search(Query{Colors: []color.Lab{nearBlue}})); len(got) == 0 || got[0] != 1 {
		t.Errorf("near-blue search %v", got)
	}
}

func TestEveryColourMustBePresent(t *testing.T) {
	x := sample()
	blue, _ := color.ParseHex("#1f4f8f")
	cream, _ := color.ParseHex("#ebe4d1")
	got := ids(x.Search(Query{Colors: []color.Lab{blue, cream}}))
	if len(got) != 2 {
		t.Errorf("blue+cream %v, want the two blue-and-cream works", got)
	}
}

func TestFilters(t *testing.T) {
	x := sample()
	blue, _ := color.ParseHex("#1f4f8f")
	if got := ids(x.Search(Query{Colors: []color.Lab{blue}, Source: "cma"})); len(got) != 2 {
		t.Errorf("cma only: %v", got)
	}
	if got := ids(x.Search(Query{Colors: []color.Lab{blue}, From: year(1800), To: year(1850)})); len(got) != 1 || got[0] != 1 {
		t.Errorf("1800–1850: %v", got)
	}
	if got := ids(x.Search(Query{Mood: "dark"})); len(got) != 1 || got[0] != 2 {
		t.Errorf("dark: %v", got)
	}
	if got := ids(x.Search(Query{Colors: []color.Lab{blue}, Allow: map[int64]bool{4: true}})); len(got) != 1 || got[0] != 4 {
		t.Errorf("allowed ids: %v", got)
	}
}

func TestSimilarPalettes(t *testing.T) {
	x := sample()
	got := x.Similar(1, 3)
	if len(got) == 0 || got[0].ID != 4 {
		t.Fatalf("similar to the wave: %v, want the other blue-and-cream work", ids(got))
	}
	for _, h := range got {
		if h.ID == 3 {
			t.Error("yellow print counted as similar to a blue wave")
		}
	}
}

// A pure pigment finds the muted version of itself that k-means leaves in a
// palette, but not a grey of the same lightness.
func TestMatchForgivesMutedNotGrey(t *testing.T) {
	ultramarine, _ := color.ParseHex("#2e45a0")
	mutedBlue, _ := color.ParseHex("#3d4f7c")
	grey, _ := color.ParseHex("#4f535c")
	brown, _ := color.ParseHex("#6b4a2f")
	if Match(ultramarine, mutedBlue) > Sigma {
		t.Errorf("muted blue is %.3f from ultramarine, want within %.3f", Match(ultramarine, mutedBlue), Sigma)
	}
	if Match(ultramarine, grey) < 1.6*Sigma {
		t.Errorf("grey is only %.3f from ultramarine", Match(ultramarine, grey))
	}
	if Match(ultramarine, brown) < 2*Sigma {
		t.Errorf("brown is only %.3f from ultramarine", Match(ultramarine, brown))
	}
}
