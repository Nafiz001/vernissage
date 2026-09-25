package imaging

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Touch marks a cached file as recently used, at most once an hour, so the
// janitor evicts what nobody looks at rather than what was made first.
func Touch(p string) {
	if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) > time.Hour {
		now := time.Now()
		os.Chtimes(p, now, now)
	}
}

type entry struct {
	path  string
	size  int64
	mtime time.Time
	dir   bool
}

// Janitor keeps derived pictures (sizes, large files, pyramids and
// posters) under budget bytes, evicting the least recently used. Masters
// are what analysis ran on and are never evicted.
func (s *Service) Janitor(ctx context.Context, budget int64, every time.Duration) {
	for {
		if n, freed := s.sweep(budget); n > 0 {
			s.log.Info("cache trimmed", "removed", n, "freedMB", freed>>20)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func (s *Service) sweep(budget int64) (removed int, freed int64) {
	var entries []entry
	var total int64
	for _, sub := range []string{"sized", "large", "posters"} {
		filepath.WalkDir(filepath.Join(s.dir, sub), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if fi, err := d.Info(); err == nil {
				entries = append(entries, entry{p, fi.Size(), fi.ModTime(), false})
				total += fi.Size()
			}
			return nil
		})
	}
	// A pyramid is evicted whole; its size file's time is its last use.
	dirs, _ := os.ReadDir(filepath.Join(s.dir, "dzi"))
	for _, d := range dirs {
		p := filepath.Join(s.dir, "dzi", d.Name())
		var size int64
		filepath.WalkDir(p, func(_ string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				if fi, err := e.Info(); err == nil {
					size += fi.Size()
				}
			}
			return nil
		})
		mtime := time.Time{}
		if fi, err := os.Stat(filepath.Join(p, "size")); err == nil {
			mtime = fi.ModTime()
		}
		entries = append(entries, entry{p, size, mtime, true})
		total += size
	}
	if total <= budget {
		return 0, 0
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].mtime.Before(entries[j].mtime) })
	target := budget * 8 / 10
	for _, e := range entries {
		if total <= target {
			break
		}
		var err error
		if e.dir {
			err = os.RemoveAll(e.path)
		} else {
			err = os.Remove(e.path)
		}
		if err == nil {
			total -= e.size
			freed += e.size
			removed++
		}
	}
	return removed, freed
}
