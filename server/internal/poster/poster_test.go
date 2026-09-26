package poster

import (
	"image"
	"image/color"
	"testing"

	"github.com/Nafiz001/vernissage/server/internal/gallery"
)

func TestRender(t *testing.T) {
	cover := image.NewRGBA(image.Rect(0, 0, 300, 200))
	for i := range cover.Pix {
		cover.Pix[i] = 200
	}
	img := Render(Card{
		Title:   "A title long enough that it has to wrap over more than one line of the card",
		Curator: "Ada", Works: 12, Paint: "#1e2e4a", Cover: cover,
		Hang: gallery.Hang{W: 1.2, H: 0.9, ArtW: 1.0, ArtH: 0.7, Frame: "gilt", Border: 0.1},
	})
	if img.Bounds() != image.Rect(0, 0, W, H) {
		t.Fatalf("size %v", img.Bounds())
	}
	// The cover sits in the middle of the frame on the right.
	if c := img.RGBAAt(850, 300); c.R < 150 {
		t.Errorf("cover not drawn: %v at its centre", c)
	}
	// The wall is the paint colour, lit or shaded.
	if c := img.RGBAAt(20, 600); c == (color.RGBA{}) || c.B < c.R {
		t.Errorf("wall at the bottom left is %v, want a Prussian blue", c)
	}
	if lines := wrap(face(bodoni, 40), "one two three four five six seven eight nine ten", 200); len(lines) < 2 {
		t.Errorf("wrap gave %v", lines)
	}
}
