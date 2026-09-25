package imaging

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// Deep Zoom: the picture at full size, then halved again and again down to
// a single pixel, each level cut into 254-pixel tiles overlapping by one.
// OpenSeadragon in the browser fetches only the tiles on screen at the
// zoom it's showing, so looking closely at a 3000-pixel photograph costs a
// few hundred kilobytes.
const (
	TileSize = 254
	Overlap  = 1
)

// DZI is a pyramid's size.
type DZI struct {
	Width, Height int
}

// Descriptor is the .dzi XML OpenSeadragon reads.
func (d DZI) Descriptor() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Image xmlns="http://schemas.microsoft.com/deepzoom/2008" Format="jpg" Overlap="%d" TileSize="%d"><Size Width="%d" Height="%d"/></Image>
`, Overlap, TileSize, d.Width, d.Height)
}

// MaxLevel is the level at which the picture is full size.
func (d DZI) MaxLevel() int {
	return int(math.Ceil(math.Log2(float64(max(d.Width, d.Height)))))
}

// LevelSize is the picture's size at a level.
func (d DZI) LevelSize(level int) (int, int) {
	scale := math.Pow(2, float64(d.MaxLevel()-level))
	return int(math.Ceil(float64(d.Width) / scale)), int(math.Ceil(float64(d.Height) / scale))
}

// TileRect is the part of a level a tile covers, overlap included.
func (d DZI) TileRect(level, col, row int) image.Rectangle {
	w, h := d.LevelSize(level)
	x0, y0 := col*TileSize, row*TileSize
	if col > 0 {
		x0 -= Overlap
	}
	if row > 0 {
		y0 -= Overlap
	}
	x1 := min(w, (col+1)*TileSize+Overlap)
	y1 := min(h, (row+1)*TileSize+Overlap)
	return image.Rect(x0, y0, x1, y1)
}

func (s *Service) dziDir(id int64) string {
	return filepath.Join(s.dir, "dzi", strconv.FormatInt(id, 10))
}

// Pyramid returns a work's Deep Zoom pyramid, building every level the
// first time it's asked for.
func (s *Service) Pyramid(ctx context.Context, src Source) (DZI, string, error) {
	dir := s.dziDir(src.ID)
	done := filepath.Join(dir, "size")
	if d, err := readSize(done); err == nil {
		return d, dir, nil
	}
	err := s.Once("dzi:"+dir, func() error {
		if _, err := readSize(done); err == nil {
			return nil
		}
		large, err := s.Large(ctx, src)
		if err != nil {
			return err
		}
		img, err := decodeFile(large)
		if err != nil {
			return err
		}
		b := img.Bounds()
		d := DZI{b.Dx(), b.Dy()}
		os.RemoveAll(dir)
		level := toRGBA(img)
		for l := d.MaxLevel(); l >= 0; l-- {
			w, h := d.LevelSize(l)
			if level.Bounds().Dx() != w || level.Bounds().Dy() != h {
				level = scaleTo(level, w, h)
			}
			for row := 0; row*TileSize < h; row++ {
				for col := 0; col*TileSize < w; col++ {
					tile := level.SubImage(d.TileRect(l, col, row))
					p := filepath.Join(dir, strconv.Itoa(l), fmt.Sprintf("%d_%d.jpg", col, row))
					if err := writeJPEG(p, tile, 84); err != nil {
						return err
					}
				}
			}
		}
		// Written last: its presence means the pyramid is complete.
		return os.WriteFile(done, []byte(fmt.Sprintf("%d %d", d.Width, d.Height)), 0o644)
	})
	if err != nil {
		return DZI{}, "", err
	}
	d, err := readSize(done)
	return d, dir, err
}

// Tile is the path of one tile, building the pyramid if needed.
func (s *Service) Tile(ctx context.Context, src Source, level, col, row int) (string, error) {
	d, dir, err := s.Pyramid(ctx, src)
	if err != nil {
		return "", err
	}
	if level < 0 || level > d.MaxLevel() {
		return "", os.ErrNotExist
	}
	return filepath.Join(dir, strconv.Itoa(level), fmt.Sprintf("%d_%d.jpg", col, row)), nil
}

func readSize(p string) (DZI, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return DZI{}, err
	}
	var d DZI
	_, err = fmt.Sscanf(string(b), "%d %d", &d.Width, &d.Height)
	return d, err
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok {
		return r
	}
	b := img.Bounds()
	return scaleTo(img, b.Dx(), b.Dy())
}
