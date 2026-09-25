package color

import (
	"image"
	"math"
	"sort"
)

// Swatch is one colour of a palette and the share of the picture it covers.
type Swatch struct {
	Hex    string  `json:"hex"`
	L      float64 `json:"l"`
	A      float64 `json:"a"`
	B      float64 `json:"b"`
	Weight float64 `json:"w"`
}

// Lab returns the swatch's colour.
func (s Swatch) Lab() Lab { return Lab{s.L, s.A, s.B} }

// Analysis is what the collection knows about a picture's colour.
type Analysis struct {
	// Palette holds up to six colours, the most widespread first.
	Palette []Swatch
	// Lightness is the mean OKLab L over the picture, 0 to 1.
	Lightness float64
	// Colorfulness is Hasler and Süsstrunk's metric: about 0 for a grisaille,
	// 30 for a typical landscape, 80 and up for a Fauve canvas.
	Colorfulness float64
}

// Dominant is the most widespread colour, or "" for an empty palette.
func (a Analysis) Dominant() string {
	if len(a.Palette) == 0 {
		return ""
	}
	return a.Palette[0].Hex
}

const (
	sampleSide = 96    // pictures are clustered at this size
	clusters   = 8     // k for k-means, before merging
	mergeBelow = 0.045 // clusters closer than this are one colour to the eye
	minWeight  = 0.015 // colours covering less than this are dropped
	maxSwatch  = 6
)

// Analyze finds the picture's palette with k-means in OKLab. It is
// deterministic: the same picture always gives the same palette.
func Analyze(img image.Image) Analysis {
	px, stats := sample(img)
	if len(px) == 0 {
		return Analysis{}
	}
	centres, weights := kmeans(px, clusters)
	centres, weights = merge(centres, weights)

	total := 0.0
	for _, w := range weights {
		total += w
	}
	var out []Swatch
	for i, c := range centres {
		w := weights[i] / total
		if w < minWeight {
			continue
		}
		out = append(out, Swatch{Hex: c.Hex(), L: round4(c.L), A: round4(c.A), B: round4(c.B), Weight: round4(w)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Weight > out[j].Weight })
	if len(out) > maxSwatch {
		out = out[:maxSwatch]
	}
	// Re-normalise so the kept colours add up to the whole picture.
	sum := 0.0
	for _, s := range out {
		sum += s.Weight
	}
	for i := range out {
		out[i].Weight = round4(out[i].Weight / sum)
	}
	return Analysis{Palette: out, Lightness: round4(stats.lightness), Colorfulness: round4(stats.colorfulness)}
}

type sampleStats struct{ lightness, colorfulness float64 }

// sample box-filters the picture down to at most sampleSide pixels on its
// long side and converts every pixel to OKLab.
func sample(img image.Image) ([]Lab, sampleStats) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, sampleStats{}
	}
	scale := math.Max(1, float64(max(w, h))/sampleSide)
	sw, sh := max(1, int(float64(w)/scale)), max(1, int(float64(h)/scale))

	px := make([]Lab, 0, sw*sh)
	var sumL, sumRG, sumYB, sumRG2, sumYB2 float64
	for y := 0; y < sh; y++ {
		y0, y1 := b.Min.Y+y*h/sh, b.Min.Y+(y+1)*h/sh
		for x := 0; x < sw; x++ {
			x0, x1 := b.Min.X+x*w/sw, b.Min.X+(x+1)*w/sw
			var r, g, bl, n float64
			for yy := y0; yy < max(y1, y0+1); yy++ {
				for xx := x0; xx < max(x1, x0+1); xx++ {
					cr, cg, cb, _ := img.At(xx, yy).RGBA()
					// Average in linear light, as a lens would.
					r += linear[cr>>8]
					g += linear[cg>>8]
					bl += linear[cb>>8]
					n++
				}
			}
			r, g, bl = r/n, g/n, bl/n
			lab := fromLinear(r, g, bl)
			px = append(px, lab)
			sumL += lab.L

			R, G, B := float64(encode(r)), float64(encode(g)), float64(encode(bl))
			rg, yb := R-G, 0.5*(R+G)-B
			sumRG += rg
			sumYB += yb
			sumRG2 += rg * rg
			sumYB2 += yb * yb
		}
	}
	n := float64(len(px))
	mRG, mYB := sumRG/n, sumYB/n
	sdRG := math.Sqrt(math.Max(0, sumRG2/n-mRG*mRG))
	sdYB := math.Sqrt(math.Max(0, sumYB2/n-mYB*mYB))
	cf := math.Hypot(sdRG, sdYB) + 0.3*math.Hypot(mRG, mYB)
	return px, sampleStats{lightness: sumL / n, colorfulness: cf}
}

// kmeans clusters the pixels, seeding with k-means++ from a fixed-seed
// generator so the result depends only on the picture.
func kmeans(px []Lab, k int) ([]Lab, []float64) {
	if len(px) < k {
		k = len(px)
	}
	rng := splitmix(uint64(len(px))*0x9e3779b97f4a7c15 + 1)
	centres := make([]Lab, 0, k)
	centres = append(centres, px[rng.intn(len(px))])
	d2 := make([]float64, len(px))
	for i := range d2 {
		d2[i] = math.Inf(1)
	}
	for len(centres) < k {
		last := centres[len(centres)-1]
		total := 0.0
		for i, p := range px {
			if d := sq(p, last); d < d2[i] {
				d2[i] = d
			}
			total += d2[i]
		}
		if total == 0 {
			break
		}
		target := rng.float() * total
		pick := len(px) - 1
		for i, d := range d2 {
			target -= d
			if target <= 0 {
				pick = i
				break
			}
		}
		centres = append(centres, px[pick])
	}

	assign := make([]int, len(px))
	weights := make([]float64, len(centres))
	for iter := 0; iter < 24; iter++ {
		moved := 0
		for i, p := range px {
			best, bestD := 0, math.Inf(1)
			for j, c := range centres {
				if d := sq(p, c); d < bestD {
					best, bestD = j, d
				}
			}
			if assign[i] != best || iter == 0 {
				moved++
			}
			assign[i] = best
		}
		sums := make([]Lab, len(centres))
		for j := range weights {
			weights[j] = 0
		}
		for i, p := range px {
			j := assign[i]
			sums[j].L += p.L
			sums[j].A += p.A
			sums[j].B += p.B
			weights[j]++
		}
		for j := range centres {
			if weights[j] > 0 {
				centres[j] = Lab{sums[j].L / weights[j], sums[j].A / weights[j], sums[j].B / weights[j]}
			}
		}
		if moved == 0 {
			break
		}
	}
	return centres, weights
}

// merge joins clusters that look like the same colour, heaviest first.
func merge(centres []Lab, weights []float64) ([]Lab, []float64) {
	for {
		bi, bj, bd := -1, -1, mergeBelow
		for i := range centres {
			for j := i + 1; j < len(centres); j++ {
				if weights[i] == 0 || weights[j] == 0 {
					continue
				}
				if d := Dist(centres[i], centres[j]); d < bd {
					bi, bj, bd = i, j, d
				}
			}
		}
		if bi < 0 {
			break
		}
		wi, wj := weights[bi], weights[bj]
		t := wi + wj
		centres[bi] = Lab{
			(centres[bi].L*wi + centres[bj].L*wj) / t,
			(centres[bi].A*wi + centres[bj].A*wj) / t,
			(centres[bi].B*wi + centres[bj].B*wj) / t,
		}
		weights[bi], weights[bj] = t, 0
	}
	var c []Lab
	var w []float64
	for i := range centres {
		if weights[i] > 0 {
			c = append(c, centres[i])
			w = append(w, weights[i])
		}
	}
	return c, w
}

func sq(p, q Lab) float64 {
	dl, da, db := p.L-q.L, p.A-q.A, p.B-q.B
	return dl*dl + da*da + db*db
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

type splitmix uint64

func (s *splitmix) next() uint64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (s *splitmix) float() float64 { return float64(s.next()>>11) / (1 << 53) }
func (s *splitmix) intn(n int) int { return int(s.next() % uint64(n)) }
