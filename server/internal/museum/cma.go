package museum

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// CMA reads the Cleveland Museum of Art's Open Access API. Every work it
// returns with cc0=1 is in the public domain, with images at three sizes.
type CMA struct {
	c *client
	// Limit stops after this many works per query; 0 reads everything.
	Limit int
}

func NewCMA(userAgent string, limit int) *CMA {
	return &CMA{c: newClient(userAgent, 4), Limit: limit}
}

func (*CMA) Name() string { return "cma" }

type cmaQuery struct {
	params        url.Values
	highlightOnly bool
}

// The collection takes every painting, the Japanese woodblock prints, and
// the drawings the museum counts among its highlights.
var cmaQueries = []cmaQuery{
	{params: url.Values{"type": {"Painting"}}},
	{params: url.Values{"type": {"Print"}, "department": {"Japanese Art"}}},
	{params: url.Values{"type": {"Drawing"}}, highlightOnly: true},
}

const cmaFields = "id,accession_number,title,creation_date,creation_date_earliest,creation_date_latest," +
	"creators,culture,technique,department,type,description,did_you_know,wall_description," +
	"dimensions,images,url,is_highlight,creditline"

type cmaPage struct {
	Info struct {
		Total int `json:"total"`
	} `json:"info"`
	Data []cmaWork `json:"data"`
}

type cmaWork struct {
	ID              int      `json:"id"`
	Accession       string   `json:"accession_number"`
	Title           string   `json:"title"`
	CreationDate    string   `json:"creation_date"`
	Earliest        *int     `json:"creation_date_earliest"`
	Latest          *int     `json:"creation_date_latest"`
	Culture         []string `json:"culture"`
	Technique       string   `json:"technique"`
	Department      string   `json:"department"`
	Type            string   `json:"type"`
	Description     *string  `json:"description"`
	WallDescription *string  `json:"wall_description"`
	DidYouKnow      *string  `json:"did_you_know"`
	URL             string   `json:"url"`
	Highlight       bool     `json:"is_highlight"`
	Credit          string   `json:"creditline"`
	Creators        []struct {
		Description string `json:"description"`
		Role        string `json:"role"`
	} `json:"creators"`
	Dimensions map[string]struct {
		Height *float64 `json:"height"`
		Width  *float64 `json:"width"`
	} `json:"dimensions"`
	Images struct {
		Web   *cmaImage `json:"web"`
		Print *cmaImage `json:"print"`
	} `json:"images"`
}

type cmaImage struct {
	URL string `json:"url"`
}

func (s *CMA) Fetch(ctx context.Context, emit func(Record) error) error {
	const page = 500
	for _, q := range cmaQueries {
		emitted := 0
		for skip := 0; ; skip += page {
			p := url.Values{}
			for k, v := range q.params {
				p[k] = v
			}
			p.Set("cc0", "1")
			p.Set("has_image", "1")
			p.Set("limit", strconv.Itoa(page))
			p.Set("skip", strconv.Itoa(skip))
			p.Set("fields", cmaFields)
			var res cmaPage
			if err := s.c.getJSON(ctx, "https://openaccess-api.clevelandart.org/api/artworks/?"+p.Encode(), &res); err != nil {
				return fmt.Errorf("cma %v: %w", q.params, err)
			}
			for _, w := range res.Data {
				if q.highlightOnly && !w.Highlight {
					continue
				}
				r, ok := w.record()
				if !ok {
					continue
				}
				if err := emit(r); err != nil {
					return err
				}
				emitted++
				if s.Limit > 0 && emitted >= s.Limit {
					break
				}
			}
			if len(res.Data) < page || (s.Limit > 0 && emitted >= s.Limit) {
				break
			}
		}
	}
	return nil
}

func (w cmaWork) record() (Record, bool) {
	kind := kindOf(w.Type)
	if kind == "" || w.Images.Web == nil || w.Images.Web.URL == "" {
		return Record{}, false
	}
	large := w.Images.Web.URL
	if w.Images.Print != nil && w.Images.Print.URL != "" {
		large = w.Images.Print.URL
	}
	r := Record{
		Source:     "cma",
		SourceID:   strconv.Itoa(w.ID),
		Title:      clean(w.Title),
		DateText:   w.CreationDate,
		YearStart:  w.Earliest,
		YearEnd:    w.Latest,
		Medium:     capitalize(w.Technique),
		Kind:       kind,
		Department: w.Department,
		Credit:     w.Credit,
		SourceURL:  w.URL,
		ImageURL:   w.Images.Web.URL,
		LargeURL:   large,
		Highlight:  w.Highlight,
	}
	if len(w.Culture) > 0 {
		r.Culture = w.Culture[0]
	}
	for _, c := range w.Creators {
		if c.Role == "" || strings.EqualFold(c.Role, "artist") {
			r.Artist, r.ArtistBio = splitCreator(c.Description)
			break
		}
	}
	for _, d := range []*string{w.WallDescription, w.Description, w.DidYouKnow} {
		if d != nil && strings.TrimSpace(*d) != "" {
			r.Description = clean(*d)
			break
		}
	}
	// Prefer the work itself over its frame or mount, then take whatever
	// the museum measured ("No Extent Specified" is common for prints).
	keys := []string{"unframed", "overall", "image", "sheet", "painting"}
	var others []string
	for k := range w.Dimensions {
		if !slices.Contains(keys, k) && k != "framed" {
			others = append(others, k)
		}
	}
	sort.Strings(others)
	keys = append(keys, others...)
	for _, key := range append(keys, "framed") {
		if d, ok := w.Dimensions[key]; ok && d.Height != nil && d.Width != nil && *d.Height > 0 && *d.Width > 0 {
			h, wd := *d.Height*100, *d.Width*100
			if key == "framed" { // take off a typical frame
				h, wd = h*0.84, wd*0.84
			}
			r.HeightCM, r.WidthCM = ptr(h), ptr(wd)
			break
		}
	}
	if r.Title == "" {
		r.Title = "Untitled"
	}
	return r, true
}

// splitCreator turns "Claude Monet (French, 1840–1926)" into the name and
// the part in brackets.
func splitCreator(s string) (name, bio string) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " ("); i > 0 && strings.HasSuffix(s, ")") {
		return s[:i], s[i+2 : len(s)-1]
	}
	return s, ""
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
