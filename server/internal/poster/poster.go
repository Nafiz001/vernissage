// Package poster draws an exhibition's share card: its cover work hung in
// its frame on the exhibition's wall colour, lit from above, with the
// title set in Bodoni as vinyl lettering on the wall.
package poster

import (
	_ "embed"
	"fmt"
	"image"
	stdcolor "image/color"
	"image/draw"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/Nafiz001/vernissage/server/internal/color"
	"github.com/Nafiz001/vernissage/server/internal/gallery"
)

//go:embed fonts/BodoniModa-Regular.ttf
var bodoniTTF []byte

//go:embed fonts/BodoniModa-Italic.ttf
var bodoniItalicTTF []byte

//go:embed fonts/Archivo-Regular.ttf
var archivoTTF []byte

var (
	bodoni       = mustParse(bodoniTTF)
	bodoniItalic = mustParse(bodoniItalicTTF)
	archivo      = mustParse(archivoTTF)
)

func mustParse(b []byte) *opentype.Font {
	f, err := opentype.Parse(b)
	if err != nil {
		panic(err)
	}
	return f
}

func face(f *opentype.Font, size float64) font.Face {
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	return fc
}

const W, H = 1200, 630

// Card is what the poster shows.
type Card struct {
	Title   string
	Curator string
	Works   int
	Paint   string // hex
	Cover   image.Image
	Hang    gallery.Hang
}

// Render draws the card.
func Render(c Card) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	wall, err := color.ParseHex(c.Paint)
	if err != nil {
		wall, _ = color.ParseHex("#5b1d1f")
	}
	dark := wall.L < 0.62

	// The wall, brighter where the ceiling light falls, darker at the floor.
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			dx, dy := (float64(x)-850)/700, (float64(y)-120)/620
			light := 1.18 - 0.42*math.Sqrt(dx*dx+dy*dy)
			l := wall
			l.L = math.Max(0, math.Min(1, wall.L*light))
			r, g, b := l.RGB()
			img.Pix[img.PixOffset(x, y)+0], img.Pix[img.PixOffset(x, y)+1], img.Pix[img.PixOffset(x, y)+2], img.Pix[img.PixOffset(x, y)+3] = r, g, b, 255
		}
	}

	if c.Cover != nil {
		hangWork(img, c, 850, 300, 500, 440)
	}

	ink := stdcolor.RGBA{0xf3, 0xef, 0xe6, 0xff}
	soft := stdcolor.RGBA{0xf3, 0xef, 0xe6, 0xb0}
	if !dark {
		ink = stdcolor.RGBA{0x1c, 0x1a, 0x17, 0xff}
		soft = stdcolor.RGBA{0x1c, 0x1a, 0x17, 0xb0}
	}

	// Title: as large as fits in three lines.
	x0, maxW := 72, 470
	var lines []string
	size := 68.0
	for ; size > 34; size -= 4 {
		lines = wrap(face(bodoni, size), c.Title, maxW)
		if len(lines) <= 3 {
			break
		}
	}
	if len(lines) > 3 {
		lines = lines[:3]
		lines[2] = strings.TrimRight(lines[2], " ,.;:") + "…"
	}
	title := face(bodoni, size)
	y := 150 + int(size)
	for _, line := range lines {
		text(img, title, ink, x0, y, line)
		y += int(size * 1.08)
	}
	meta := face(archivo, 22)
	y += 26
	text(img, meta, soft, x0, y, "Curated by "+c.Curator)
	works := fmt.Sprintf("%d works", c.Works)
	if c.Works == 1 {
		works = "1 work"
	}
	text(img, meta, soft, x0, y+34, works)

	text(img, face(bodoniItalic, 30), ink, x0, H-64, "Vernissage")
	return img
}

// hangWork draws the cover in its frame, fitted to a box centred on cx, cy,
// with a soft shadow and a pool of light above.
func hangWork(img *image.RGBA, c Card, cx, cy, boxW, boxH int) {
	h := c.Hang
	if h.W == 0 || h.H == 0 {
		b := c.Cover.Bounds()
		h = gallery.Hang{W: 1, H: float64(b.Dy()) / float64(b.Dx()), ArtW: 0.9, ArtH: 0.9 * float64(b.Dy()) / float64(b.Dx()), Frame: "gilt", Border: 0.05}
	}
	scale := math.Min(float64(boxW)/h.W, float64(boxH)/h.H)
	fw, fh := int(h.W*scale), int(h.H*scale)
	outer := image.Rect(cx-fw/2, cy-fh/2, cx-fw/2+fw, cy-fh/2+fh)
	border := max(3, int(h.Border*scale))
	mat := int(h.Mat * scale)

	// Light wash on the wall above the work.
	wash(img, cx, outer.Min.Y-40, float64(fw)*0.9, float64(fh)*0.9)
	// Shadow: the frame stands off the wall and the light is above.
	for i := 18; i > 0; i-- {
		a := uint8(9)
		r := outer.Add(image.Pt(0, 10)).Inset(-i)
		draw.DrawMask(img, r, image.NewUniform(stdcolor.RGBA{0, 0, 0, 255}), image.Point{}, image.NewUniform(stdcolor.Alpha{a}), image.Point{}, draw.Over)
	}
	moulding(img, outer, border, h.Frame)
	inner := outer.Inset(border)
	if mat > 0 {
		draw.Draw(img, inner, image.NewUniform(stdcolor.RGBA{0xf2, 0xee, 0xe4, 0xff}), image.Point{}, draw.Src)
		// The bevelled edge of the mat's window.
		art := inner.Inset(mat)
		draw.Draw(img, art.Inset(-2), image.NewUniform(stdcolor.RGBA{0xfb, 0xf9, 0xf4, 0xff}), image.Point{}, draw.Src)
		inner = art
	}
	xdraw.CatmullRom.Scale(img, inner, c.Cover, c.Cover.Bounds(), xdraw.Src, nil)
}

var frameColours = map[string][3]float64{
	"gilt":  {0.78, 0.62, 0.33},
	"oak":   {0.60, 0.46, 0.31},
	"black": {0.11, 0.105, 0.10},
	"silk":  {0.80, 0.73, 0.60},
}

// moulding shades the frame like a carved moulding lit from the top left:
// each side's tone set by which way it faces, each profile rounded.
func moulding(img *image.RGBA, r image.Rectangle, border int, kind string) {
	base, ok := frameColours[kind]
	if !ok {
		base = frameColours["gilt"]
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dl, dr, dt, db := x-r.Min.X, r.Max.X-1-x, y-r.Min.Y, r.Max.Y-1-y
			d := min(dl, dr, dt, db)
			if d >= border || !image.Pt(x, y).In(img.Bounds()) {
				continue
			}
			side := 1.0
			switch d {
			case dt:
				side = 1.15
			case dl:
				side = 1.0
			case dr:
				side = 0.78
			case db:
				side = 0.66
			}
			t := (float64(d) + 0.5) / float64(border)
			profile := 0.72 + 0.45*math.Sin(math.Pi*t)
			if kind == "gilt" {
				// A bright bead inside the moulding.
				profile += 0.25 * math.Exp(-math.Pow((t-0.7)/0.08, 2))
			}
			k := side * profile
			i := img.PixOffset(x, y)
			img.Pix[i+0] = clamp8(base[0] * k)
			img.Pix[i+1] = clamp8(base[1] * k)
			img.Pix[i+2] = clamp8(base[2] * k)
			img.Pix[i+3] = 255
		}
	}
}

// wash brightens the wall in an ellipse, as a spotlight would.
func wash(img *image.RGBA, cx, cy int, rx, ry float64) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dx, dy := float64(x-cx)/rx, float64(y-cy)/(ry*1.1)
			k := math.Exp(-(dx*dx + dy*dy) * 1.6)
			if k < 0.01 {
				continue
			}
			i := img.PixOffset(x, y)
			for c := 0; c < 3; c++ {
				v := float64(img.Pix[i+c])
				img.Pix[i+c] = clamp8((v + (255-v)*0.10*k) / 255 * (1 + 0.12*k))
			}
		}
	}
}

func clamp8(v float64) uint8 { return uint8(math.Max(0, math.Min(255, math.Round(v*255)))) }

func text(img *image.RGBA, f font.Face, c stdcolor.Color, x, y int, s string) {
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// wrap breaks s into lines no wider than width.
func wrap(f font.Face, s string, width int) []string {
	var lines []string
	var line string
	for _, w := range strings.Fields(s) {
		try := w
		if line != "" {
			try = line + " " + w
		}
		if font.MeasureString(f, try).Ceil() > width && line != "" {
			lines = append(lines, line)
			line = w
		} else {
			line = try
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
