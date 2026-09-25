// Package color turns pictures into the few colours a person would name
// looking at them, and measures how far apart two colours look.
//
// Everything is done in OKLab (Björn Ottosson, 2020), a perceptual space in
// which straight-line distance tracks how different two colours appear far
// better than RGB or HSL does. A distance of about 0.02 is the smallest
// difference most people notice; 0.1 is "clearly another colour".
package color

import (
	"fmt"
	"math"
	"strings"
)

// Lab is a colour in OKLab. L runs from 0 (black) to 1 (white); A is
// green–red and B is blue–yellow, both roughly within ±0.4.
type Lab struct{ L, A, B float64 }

// linear[i] is sRGB channel value i/255 with the transfer curve removed.
var linear [256]float64

func init() {
	for i := range linear {
		c := float64(i) / 255
		if c <= 0.04045 {
			linear[i] = c / 12.92
		} else {
			linear[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
}

func encode(c float64) uint8 {
	if c <= 0.0031308 {
		c *= 12.92
	} else {
		c = 1.055*math.Pow(c, 1/2.4) - 0.055
	}
	return uint8(math.Round(math.Max(0, math.Min(1, c)) * 255))
}

// FromRGB converts an 8-bit sRGB colour.
func FromRGB(r, g, b uint8) Lab {
	return fromLinear(linear[r], linear[g], linear[b])
}

func fromLinear(r, g, b float64) Lab {
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return Lab{
		L: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		A: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		B: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// RGB converts back to 8-bit sRGB, clipping colours outside the gamut.
func (c Lab) RGB() (r, g, b uint8) {
	l := c.L + 0.3963377774*c.A + 0.2158037573*c.B
	m := c.L - 0.1055613458*c.A - 0.0638541728*c.B
	s := c.L - 0.0894841775*c.A - 1.2914855480*c.B
	l, m, s = l*l*l, m*m*m, s*s*s
	return encode(+4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		encode(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		encode(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s)
}

// Hex formats the colour as #rrggbb.
func (c Lab) Hex() string {
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// Chroma is how far the colour is from grey.
func (c Lab) Chroma() float64 { return math.Hypot(c.A, c.B) }

// Dist is the perceptual distance between two colours.
func Dist(p, q Lab) float64 {
	dl, da, db := p.L-q.L, p.A-q.A, p.B-q.B
	return math.Sqrt(dl*dl + da*da + db*db)
}

// ParseHex reads "#rrggbb", "rrggbb" or "#rgb".
func ParseHex(s string) (Lab, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return Lab{}, fmt.Errorf("color %q is not a hex colour", s)
	}
	var v [3]uint8
	for i := range v {
		hi, ok1 := hexDigit(s[2*i])
		lo, ok2 := hexDigit(s[2*i+1])
		if !ok1 || !ok2 {
			return Lab{}, fmt.Errorf("color %q is not a hex colour", s)
		}
		v[i] = hi<<4 | lo
	}
	return FromRGB(v[0], v[1], v[2]), nil
}

func hexDigit(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
