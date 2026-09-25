// Package museum reads works from museums' open-access APIs and turns each
// into one common record.
package museum

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Record is one work as the collection stores it.
type Record struct {
	Source      string
	SourceID    string
	Title       string
	Artist      string
	ArtistBio   string
	DateText    string
	YearStart   *int
	YearEnd     *int
	Medium      string
	Kind        string // painting | print | drawing
	Culture     string
	Department  string
	Description string
	Credit      string
	SourceURL   string
	ImageURL    string // web size, for thumbnails and analysis
	LargeURL    string // print or original, for deep zoom
	WidthCM     *float64
	HeightCM    *float64
	Highlight   bool
}

// Source reads a museum's works and hands each usable one to emit.
type Source interface {
	Name() string
	Fetch(ctx context.Context, emit func(Record) error) error
}

// Museums names each source for people.
var Museums = map[string]string{
	"cma": "The Cleveland Museum of Art",
	"met": "The Metropolitan Museum of Art",
}

// client is a polite HTTP client: it identifies itself, retries transient
// failures with backoff, and waits between requests to one host.
type client struct {
	http      *http.Client
	userAgent string
	gap       time.Duration
	ticks     chan struct{}
}

func newClient(userAgent string, perSecond int) *client {
	c := &client{
		http:      &http.Client{Timeout: 60 * time.Second},
		userAgent: userAgent,
		ticks:     make(chan struct{}, 1),
	}
	c.gap = time.Second / time.Duration(perSecond)
	go func() {
		for {
			c.ticks <- struct{}{}
			time.Sleep(c.gap)
		}
	}()
	return c
}

var errNotFound = fmt.Errorf("not found")

func (c *client) getJSON(ctx context.Context, url string, into any) error {
	var last error
	throttled := false
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt)*time.Second + time.Duration(rand.IntN(500))*time.Millisecond
			if throttled {
				// A 403 or 429 from a museum's firewall means "slower", not
				// "try harder": wait it out.
				backoff = time.Duration(30*attempt) * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.ticks:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			last = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		resp.Body.Close()
		switch {
		case err != nil:
			last = err
		case resp.StatusCode == http.StatusNotFound:
			return errNotFound
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden:
			throttled = true
			last = fmt.Errorf("%s: %s", url, resp.Status)
		case resp.StatusCode >= 500:
			last = fmt.Errorf("%s: %s", url, resp.Status)
		case resp.StatusCode != http.StatusOK:
			return fmt.Errorf("%s: %s", url, resp.Status)
		default:
			return json.Unmarshal(body, into)
		}
	}
	return last
}

var (
	tags   = regexp.MustCompile(`<[^>]*>`)
	spaces = regexp.MustCompile(`[ \t]+`)
	breaks = regexp.MustCompile(`\n{3,}`)
)

// clean strips markup and tidies whitespace in museum prose.
func clean(s string) string {
	s = strings.ReplaceAll(s, "</p>", "\n\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = html.UnescapeString(tags.ReplaceAllString(s, ""))
	s = spaces.ReplaceAllString(s, " ")
	s = breaks.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func ptr[T any](v T) *T { return &v }

// kindOf maps a museum's classification onto the collection's three kinds.
func kindOf(classification string) string {
	c := strings.ToLower(classification)
	switch {
	case strings.Contains(c, "painting"):
		return "painting"
	case strings.Contains(c, "print") || strings.Contains(c, "woodblock"):
		return "print"
	case strings.Contains(c, "drawing") || strings.Contains(c, "watercolor"):
		return "drawing"
	}
	return ""
}
