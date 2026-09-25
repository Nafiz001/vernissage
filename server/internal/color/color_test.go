package color

import (
	"image"
	stdcolor "image/color"
	"math"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	for _, hex := range []string{"#000000", "#ffffff", "#5a1f1f", "#1f2f47", "#c9a45c", "#00ff00"} {
		c, err := ParseHex(hex)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Hex(); got != hex {
			t.Errorf("%s came back as %s", hex, got)
		}
	}
	if w, _ := ParseHex("#fff"); math.Abs(w.L-1) > 1e-3 {
		t.Errorf("white has L %.4f, want 1", w.L)
	}
	if _, err := ParseHex("#12345g"); err == nil {
		t.Error("accepted a bad hex digit")
	}
}

func TestDistanceIsPerceptual(t *testing.T) {
	// Two blues a painter would call the same are closer than a blue and a
	// green of the same lightness.
	navy, _ := ParseHex("#1f3a68")
	navy2, _ := ParseHex("#223d6c")
	green, _ := ParseHex("#1f6838")
	if Dist(navy, navy2) >= Dist(navy, green)/5 {
		t.Errorf("near-identical blues %.3f apart, blue and green %.3f", Dist(navy, navy2), Dist(navy, green))
	}
}

// A picture that is 70% one red and 30% one blue has exactly those two
// colours, in that order, in those proportions.
func TestAnalyzeFindsTheColours(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	red := stdcolor.RGBA{160, 30, 30, 255}
	blue := stdcolor.RGBA{30, 50, 140, 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			if x < 140 {
				img.Set(x, y, red)
			} else {
				img.Set(x, y, blue)
			}
		}
	}
	a := Analyze(img)
	if len(a.Palette) != 2 {
		t.Fatalf("palette %+v, want two colours", a.Palette)
	}
	if a.Palette[0].Hex != "#a01e1e" || a.Palette[1].Hex != "#1e328c" {
		t.Errorf("palette %s, %s", a.Palette[0].Hex, a.Palette[1].Hex)
	}
	if math.Abs(a.Palette[0].Weight-0.7) > 0.02 {
		t.Errorf("red covers %.3f, want 0.7", a.Palette[0].Weight)
	}
	if a.Colorfulness < 40 {
		t.Errorf("red and blue colourfulness %.1f, want vivid", a.Colorfulness)
	}

	// Deterministic.
	b := Analyze(img)
	for i := range a.Palette {
		if a.Palette[i] != b.Palette[i] {
			t.Fatal("two analyses of one picture differ")
		}
	}
}

func TestGreyIsNotColourful(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 256)
	}
	if a := Analyze(img); a.Colorfulness > 1 {
		t.Errorf("grey ramp colourfulness %.2f", a.Colorfulness)
	}
}

func TestBlurhash(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	// Flat white with 4x3 components. Sampled cosines with an odd index do
	// not sum to zero over the grid, so the AC terms are small but not
	// empty; this matches the reference TypeScript encoder byte for byte.
	want := "L9TSUA~qfQ~q~qoffQoffQfQfQfQ"
	if got := Blurhash(img, 4, 3); got != want {
		t.Errorf("white blurhash %q, want %q", got, want)
	}
}
