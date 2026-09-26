// Package imaging fetches the museums' photographs once and serves every
// size the site needs from a disk cache: thumbnails, textures for the 3D
// rooms, and Deep Zoom tile pyramids for looking at brushstrokes.
//
// Three things keep it well behaved on a small machine:
//
//   - singleflight: a hundred browsers asking for one uncached picture
//     cause one download and one resize, not a hundred.
//   - a pixel budget: decoding a 30-megapixel photograph takes real memory,
//     so decodes wait on a weighted semaphore sized in pixels.
//   - a per-host limit on downloads, so the museums' CDNs see a polite
//     client.
package imaging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/sync/semaphore"
	"golang.org/x/sync/singleflight"
)

// Widths the service will make. Anything up to 1200 is cut from the
// master; larger sizes come from the museum's print-quality file.
var Widths = []int{200, 400, 800, 1200, 1600, 2400}

const (
	masterMax = 1200
	largeMax  = 3000
	// About 64 megapixels in flight at once, roughly 250 MB as RGBA, unless
	// the server is told otherwise.
	defaultPixelBudget = 64 << 20
)

// Source says where a work's pictures live.
type Source struct {
	ID       int64
	ImageURL string // web size
	LargeURL string // print or original
}

type Service struct {
	dir       string
	client    *http.Client
	userAgent string
	flight    singleflight.Group
	pixels    *semaphore.Weighted
	budget    int64
	hostMu    sync.Mutex
	hosts     map[string]*semaphore.Weighted
	log       *slog.Logger
}

// New opens the cache in dir. pixelBudget caps the pixels decoded at once;
// 0 means the default.
func New(dir, userAgent string, pixelBudget int64) (*Service, error) {
	if pixelBudget <= 0 {
		pixelBudget = defaultPixelBudget
	}
	for _, sub := range []string{"master", "large", "sized", "dzi", "posters"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Service{
		dir:       dir,
		client:    &http.Client{Timeout: 90 * time.Second},
		userAgent: userAgent,
		pixels:    semaphore.NewWeighted(pixelBudget),
		budget:    pixelBudget,
		hosts:     map[string]*semaphore.Weighted{},
		log:       slog.With("component", "imaging"),
	}, nil
}

// Dir is the cache's root.
func (s *Service) Dir() string { return s.dir }

func shard(id int64) string { return fmt.Sprintf("%02d", id%100) }

func (s *Service) path(kind string, id int64, suffix string) string {
	return filepath.Join(s.dir, kind, shard(id), strconv.FormatInt(id, 10)+suffix+".jpg")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Master is the work's picture at up to 1200 px, the source of thumbnails
// and of its palette.
func (s *Service) Master(ctx context.Context, src Source) (string, error) {
	p := s.path("master", src.ID, "")
	if exists(p) {
		return p, nil
	}
	_, err, _ := s.flight.Do(p, func() (any, error) {
		if exists(p) {
			return nil, nil
		}
		img, err := s.fetchDecode(ctx, src.ImageURL, masterMax)
		if err != nil {
			return nil, err
		}
		return nil, writeJPEG(p, img, 86)
	})
	return p, err
}

// Large is the work's picture at up to 3000 px, falling back to the master
// when the museum has nothing bigger.
func (s *Service) Large(ctx context.Context, src Source) (string, error) {
	p := s.path("large", src.ID, "")
	if exists(p) {
		return p, nil
	}
	_, err, _ := s.flight.Do(p, func() (any, error) {
		if exists(p) {
			return nil, nil
		}
		img, err := s.fetchDecode(ctx, src.LargeURL, largeMax)
		if err != nil {
			s.log.Warn("large picture unavailable, using master", "id", src.ID, "err", err)
			m, merr := s.Master(ctx, src)
			if merr != nil {
				return nil, merr
			}
			if img, err = decodeFile(m); err != nil {
				return nil, err
			}
		}
		return nil, writeJPEG(p, img, 88)
	})
	return p, err
}

// Sized is the work's picture at one of Widths (never enlarged).
func (s *Service) Sized(ctx context.Context, src Source, width int) (string, error) {
	p := s.path("sized", src.ID, "-"+strconv.Itoa(width))
	if exists(p) {
		return p, nil
	}
	_, err, _ := s.flight.Do(p, func() (any, error) {
		if exists(p) {
			return nil, nil
		}
		from, err := s.Master(ctx, src)
		if err == nil && width > masterMax {
			from, err = s.Large(ctx, src)
		}
		if err != nil {
			return nil, err
		}
		img, err := decodeFile(from)
		if err != nil {
			return nil, err
		}
		return nil, writeJPEG(p, fit(img, width, width*4), 84)
	})
	return p, err
}

// MasterImage decodes the master, for analysis and posters.
func (s *Service) MasterImage(ctx context.Context, src Source) (image.Image, error) {
	p, err := s.Master(ctx, src)
	if err != nil {
		return nil, err
	}
	return decodeFile(p)
}

func (s *Service) hostLimit(host string) *semaphore.Weighted {
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	if s.hosts[host] == nil {
		s.hosts[host] = semaphore.NewWeighted(6)
	}
	return s.hosts[host]
}

// ErrUnavailable means the museum no longer has the picture.
var ErrUnavailable = errors.New("picture unavailable")

// fetchDecode downloads a picture and decodes it scaled to fit maxSide.
func (s *Service) fetchDecode(ctx context.Context, rawURL string, maxSide int) (image.Image, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("%w: bad url %q", ErrUnavailable, rawURL)
	}
	body, err := s.download(ctx, u)
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	// Hold pixel budget for the decoded picture while we decode and shrink it.
	weight := min(int64(cfg.Width)*int64(cfg.Height), s.budget)
	if err := s.pixels.Acquire(ctx, weight); err != nil {
		return nil, err
	}
	defer s.pixels.Release(weight)
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return fit(img, maxSide, maxSide), nil
}

func (s *Service) download(ctx context.Context, u *url.URL) ([]byte, error) {
	lim := s.hostLimit(u.Host)
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*attempt)*2*time.Second + time.Duration(rand.IntN(800))*time.Millisecond):
			}
		}
		if err := lim.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		body, retry, err := s.get(ctx, u.String())
		lim.Release(1)
		if err == nil {
			return body, nil
		}
		if !retry {
			return nil, err
		}
		last = err
	}
	return nil, last
}

func (s *Service) get(ctx context.Context, u string) (body []byte, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusForbidden:
		return nil, false, fmt.Errorf("%w: %s", ErrUnavailable, resp.Status)
	case resp.StatusCode != http.StatusOK:
		return nil, true, fmt.Errorf("%s: %s", u, resp.Status)
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, 96<<20))
	return body, err != nil, err
}

// fit scales img down to fit within w x h, keeping its proportions.
func fit(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	if b.Dx() <= w && b.Dy() <= h {
		return img
	}
	scale := min(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	return scaleTo(img, max(1, int(float64(b.Dx())*scale+0.5)), max(1, int(float64(b.Dy())*scale+0.5)))
}

// scaleTo resamples with Catmull-Rom; x/image/draw widens the kernel when
// shrinking, so large reductions stay free of aliasing.
func scaleTo(img image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

func decodeFile(p string) (image.Image, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// ImageSize reads a JPEG's dimensions from its header.
func ImageSize(p string) (int, int, error) {
	f, err := os.Open(p)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg.Width, cfg.Height, err
}

// writeJPEG writes atomically: to a temporary file, then renamed into place,
// so a reader never sees half a picture.
func writeJPEG(p string, img image.Image, quality int) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: quality}); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// WriteJPEG is writeJPEG for other packages (posters).
func WriteJPEG(p string, img image.Image, quality int) error { return writeJPEG(p, img, quality) }

// Once runs fn for key unless another caller already is, sharing the result.
func (s *Service) Once(key string, fn func() error) error {
	_, err, _ := s.flight.Do(key, func() (any, error) { return nil, fn() })
	return err
}
