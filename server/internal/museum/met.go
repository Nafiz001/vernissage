package museum

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Met reads The Metropolitan Museum of Art's Collection API. A search
// returns only object IDs, so each work is a request of its own. The API
// allows 80 a second, but its firewall turns away sustained bursts well
// below that, so this reads three a second and skips the odd object that
// still fails.
type Met struct {
	c     *client
	Limit int
}

func NewMet(userAgent string, limit int) *Met {
	return &Met{c: newClient(userAgent, 3), Limit: limit}
}

func (*Met) Name() string { return "met" }

// European paintings in full; highlights from Asian art (with every
// Hokusai and Hiroshige print), the American Wing, and drawings and prints.
var metQueries = []url.Values{
	{"departmentId": {"11"}, "q": {"*"}},
	{"departmentId": {"6"}, "isHighlight": {"true"}, "q": {"*"}},
	{"departmentId": {"6"}, "q": {"Hokusai"}},
	{"departmentId": {"6"}, "q": {"Hiroshige"}},
	{"departmentId": {"1"}, "isHighlight": {"true"}, "q": {"*"}},
	{"departmentId": {"9"}, "isHighlight": {"true"}, "q": {"*"}},
	{"departmentId": {"21"}, "isHighlight": {"true"}, "q": {"*"}},
}

type metObject struct {
	ID             int    `json:"objectID"`
	IsHighlight    bool   `json:"isHighlight"`
	IsPublicDomain bool   `json:"isPublicDomain"`
	Primary        string `json:"primaryImage"`
	PrimarySmall   string `json:"primaryImageSmall"`
	Department     string `json:"department"`
	ObjectName     string `json:"objectName"`
	Title          string `json:"title"`
	Culture        string `json:"culture"`
	Period         string `json:"period"`
	ArtistName     string `json:"artistDisplayName"`
	ArtistBio      string `json:"artistDisplayBio"`
	ObjectDate     string `json:"objectDate"`
	BeginDate      int    `json:"objectBeginDate"`
	EndDate        int    `json:"objectEndDate"`
	Medium         string `json:"medium"`
	Credit         string `json:"creditLine"`
	Classification string `json:"classification"`
	ObjectURL      string `json:"objectURL"`
	Measurements   []struct {
		Element string             `json:"elementName"`
		Values  map[string]float64 `json:"elementMeasurements"`
	} `json:"measurements"`
}

func (s *Met) Fetch(ctx context.Context, emit func(Record) error) error {
	seen := map[int]bool{}
	var ids []int
	for _, q := range metQueries {
		p := url.Values{}
		for k, v := range q {
			p[k] = v
		}
		p.Set("hasImages", "true")
		var res struct {
			ObjectIDs []int `json:"objectIDs"`
		}
		if err := s.c.getJSON(ctx, "https://collectionapi.metmuseum.org/public/collection/v1/search?"+p.Encode(), &res); err != nil {
			return fmt.Errorf("met search %v: %w", q, err)
		}
		n := 0
		for _, id := range res.ObjectIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
				n++
			}
			if s.Limit > 0 && n >= s.Limit {
				break
			}
		}
	}

	// Two readers share the client's pace; emit is called from one
	// goroutine at a time. An object that can't be read is skipped, but a
	// long run of failures means the API is refusing us, and we stop.
	jobs := make(chan int)
	var mu sync.Mutex
	var firstErr error
	failures := 0
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				var o metObject
				err := s.c.getJSON(ctx, "https://collectionapi.metmuseum.org/public/collection/v1/objects/"+strconv.Itoa(id), &o)
				if errors.Is(err, errNotFound) {
					continue
				}
				mu.Lock()
				if err != nil && ctx.Err() == nil {
					failures++
					slog.Warn("met object skipped", "id", id, "err", err)
					if failures >= 20 && firstErr == nil {
						firstErr = fmt.Errorf("the Met API keeps refusing requests: %w", err)
						cancel()
					}
				} else if err == nil {
					failures = 0
					if r, ok := o.record(); ok {
						if err := emit(r); err != nil && firstErr == nil {
							firstErr = err
							cancel()
						}
					}
				}
				mu.Unlock()
			}
		}()
	}
feed:
	for _, id := range ids {
		select {
		case jobs <- id:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	return firstErr
}

func (o metObject) record() (Record, bool) {
	if !o.IsPublicDomain || o.PrimarySmall == "" || o.Primary == "" {
		return Record{}, false
	}
	kind := kindOf(o.Classification)
	if kind == "" {
		kind = kindOf(o.ObjectName)
	}
	if kind == "" {
		return Record{}, false
	}
	r := Record{
		Source:     "met",
		SourceID:   strconv.Itoa(o.ID),
		Title:      clean(o.Title),
		Artist:     o.ArtistName,
		ArtistBio:  o.ArtistBio,
		DateText:   o.ObjectDate,
		Medium:     o.Medium,
		Kind:       kind,
		Culture:    strings.TrimSpace(strings.Join(nonEmpty(o.Culture, o.Period), ", ")),
		Department: o.Department,
		Credit:     o.Credit,
		SourceURL:  o.ObjectURL,
		ImageURL:   o.PrimarySmall,
		LargeURL:   o.Primary,
		Highlight:  o.IsHighlight,
	}
	if o.BeginDate != 0 || o.EndDate != 0 {
		r.YearStart, r.YearEnd = ptr(o.BeginDate), ptr(o.EndDate)
	}
	for _, m := range o.Measurements {
		h, w := m.Values["Height"], m.Values["Width"]
		if h > 0 && w > 0 && (m.Element == "Overall" || m.Element == "Image" || m.Element == "Sheet" || r.HeightCM == nil) {
			r.HeightCM, r.WidthCM = ptr(h), ptr(w)
			if m.Element == "Overall" {
				break
			}
		}
	}
	if r.Title == "" {
		r.Title = "Untitled"
	}
	return r, true
}

func nonEmpty(s ...string) []string {
	var out []string
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
