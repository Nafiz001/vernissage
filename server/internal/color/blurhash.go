package color

import (
	"image"
	"math"
	"strings"
)

// Blurhash encodes a picture as a short string (Wolt's BlurHash) that the
// browser paints as a soft placeholder before the image arrives.
func Blurhash(img image.Image, xComp, yComp int) string {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return ""
	}
	// Components only need a small picture; 64 px is plenty.
	step := max(1, max(w, h)/64)
	type rgb struct{ r, g, b float64 }
	var px [][]rgb
	for y := b.Min.Y; y < b.Max.Y; y += step {
		var row []rgb
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			row = append(row, rgb{linear[r>>8], linear[g>>8], linear[bl>>8]})
		}
		px = append(px, row)
	}
	sh, sw := len(px), len(px[0])

	factors := make([]rgb, 0, xComp*yComp)
	for j := 0; j < yComp; j++ {
		for i := 0; i < xComp; i++ {
			norm := 2.0
			if i == 0 && j == 0 {
				norm = 1
			}
			var f rgb
			for y := 0; y < sh; y++ {
				cy := math.Cos(math.Pi * float64(j) * float64(y) / float64(sh))
				for x := 0; x < sw; x++ {
					basis := math.Cos(math.Pi*float64(i)*float64(x)/float64(sw)) * cy
					p := px[y][x]
					f.r += basis * p.r
					f.g += basis * p.g
					f.b += basis * p.b
				}
			}
			scale := norm / float64(sw*sh)
			factors = append(factors, rgb{f.r * scale, f.g * scale, f.b * scale})
		}
	}

	var sb strings.Builder
	encode83(&sb, (xComp-1)+(yComp-1)*9, 1)

	maxAC := 0.0
	for _, f := range factors[1:] {
		maxAC = math.Max(maxAC, math.Max(math.Abs(f.r), math.Max(math.Abs(f.g), math.Abs(f.b))))
	}
	quantMax := 0
	if len(factors) > 1 {
		quantMax = int(math.Max(0, math.Min(82, math.Floor(maxAC*166-0.5))))
		maxAC = float64(quantMax+1) / 166
	} else {
		maxAC = 1
	}
	encode83(&sb, quantMax, 1)

	dc := factors[0]
	encode83(&sb, int(encode(dc.r))<<16|int(encode(dc.g))<<8|int(encode(dc.b)), 4)
	for _, f := range factors[1:] {
		q := func(v float64) int {
			return int(math.Max(0, math.Min(18, math.Floor(signPow(v/maxAC, 0.5)*9+9.5))))
		}
		encode83(&sb, q(f.r)*19*19+q(f.g)*19+q(f.b), 2)
	}
	return sb.String()
}

const b83 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%*+,-.:;=?@[]^_{|}~"

func encode83(sb *strings.Builder, v, length int) {
	for i := 1; i <= length; i++ {
		d := (v / int(math.Pow(83, float64(length-i)))) % 83
		sb.WriteByte(b83[d])
	}
}

func signPow(v, e float64) float64 {
	return math.Copysign(math.Pow(math.Abs(v), e), v)
}
