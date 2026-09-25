package api

import (
	"context"
	"time"

	"github.com/nafizahmed/vernissage/server/internal/gallery"
	"github.com/nafizahmed/vernissage/server/internal/museum"
	"github.com/nafizahmed/vernissage/server/internal/store"
)

type museumJSON struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type imageJSON struct {
	Aspect   float64 `json:"aspect"`
	Blurhash string  `json:"blurhash,omitempty"`
	Dominant string  `json:"dominant,omitempty"`
	Ready    bool    `json:"ready"`
}

type swatchJSON struct {
	Hex    string  `json:"hex"`
	Weight float64 `json:"w"`
}

type sizeJSON struct {
	WidthCM  float64 `json:"wCm"`
	HeightCM float64 `json:"hCm"`
}

type artworkJSON struct {
	ID          int64        `json:"id"`
	Title       string       `json:"title"`
	Artist      string       `json:"artist"`
	ArtistBio   string       `json:"artistBio"`
	Date        string       `json:"date"`
	Year        *int         `json:"year,omitempty"`
	Medium      string       `json:"medium"`
	Kind        string       `json:"kind"`
	Culture     string       `json:"culture,omitempty"`
	Department  string       `json:"department,omitempty"`
	Description string       `json:"description,omitempty"`
	Credit      string       `json:"credit,omitempty"`
	Highlight   bool         `json:"highlight"`
	Museum      museumJSON   `json:"museum"`
	Image       imageJSON    `json:"image"`
	Palette     []swatchJSON `json:"palette"`
	Size        *sizeJSON    `json:"size,omitempty"`
	Hang        gallery.Hang `json:"hang"`
}

func artworkView(a *store.Artwork) artworkJSON {
	v := artworkJSON{
		ID: a.ID, Title: a.Title, Artist: a.Artist, ArtistBio: a.ArtistBio, Date: a.DateText, Year: a.YearStart,
		Medium: a.Medium, Kind: a.Kind, Culture: a.Culture, Department: a.Department,
		Description: a.Description, Credit: a.Credit, Highlight: a.Highlight,
		Museum:  museumJSON{Key: a.Source, Name: museum.Museums[a.Source], URL: a.SourceURL},
		Image:   imageJSON{Aspect: 0.8, Blurhash: a.Blurhash, Dominant: a.Dominant, Ready: a.AnalyzedAt != nil},
		Palette: []swatchJSON{},
		Hang:    a.Hang(),
	}
	if a.Aspect != nil {
		v.Image.Aspect = round3(*a.Aspect)
	} else if a.WidthCM != nil && a.HeightCM != nil && *a.HeightCM > 0 {
		v.Image.Aspect = round3(*a.WidthCM / *a.HeightCM)
	}
	for _, s := range a.Palette {
		v.Palette = append(v.Palette, swatchJSON{s.Hex, s.Weight})
	}
	if a.WidthCM != nil && a.HeightCM != nil {
		v.Size = &sizeJSON{round1(*a.WidthCM), round1(*a.HeightCM)}
	}
	return v
}

func artworkViews(list []*store.Artwork) []artworkJSON {
	out := make([]artworkJSON, 0, len(list))
	for _, a := range list {
		out = append(out, artworkView(a))
	}
	return out
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
func round3(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }

type ownerJSON struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Hue    int    `json:"hue"`
}

type previewJSON struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Aspect   float64 `json:"aspect"`
	Blurhash string  `json:"blurhash,omitempty"`
	Dominant string  `json:"dominant,omitempty"`
}

type exhibitionJSON struct {
	ID            int64         `json:"id"`
	Slug          string        `json:"slug"`
	Title         string        `json:"title"`
	Statement     string        `json:"statement"`
	Room          string        `json:"room"`
	Paint         string        `json:"paint"`
	PaintHex      string        `json:"paintHex"`
	Floor         string        `json:"floor"`
	Light         string        `json:"light"`
	Status        string        `json:"status"`
	CoverID       *int64        `json:"coverId"`
	OpeningAt     *time.Time    `json:"openingAt"`
	PublishedAt   *time.Time    `json:"publishedAt"`
	PosterVersion int           `json:"posterVersion"`
	Visits        int           `json:"visits"`
	Applause      int           `json:"applause"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Owner         ownerJSON     `json:"owner"`
	WorkCount     int           `json:"workCount"`
	Preview       []previewJSON `json:"preview"`
	Inside        int           `json:"inside"`
	Mine          bool          `json:"mine"`
}

// exhibitionViews renders a list, fetching every card's preview works in
// one query.
func (s *Server) exhibitionViews(ctx context.Context, list []*store.Exhibition, viewer *store.User) ([]exhibitionJSON, error) {
	var ids []int64
	for _, e := range list {
		ids = append(ids, e.Preview...)
	}
	works, err := store.GetArtworks(ctx, s.pool, ids)
	if err != nil {
		return nil, err
	}
	byID := map[int64]*store.Artwork{}
	for _, a := range works {
		byID[a.ID] = a
	}
	inside := s.hub.Counts()
	out := make([]exhibitionJSON, 0, len(list))
	for _, e := range list {
		v := exhibitionJSON{
			ID: e.ID, Slug: e.Slug, Title: e.Title, Statement: e.Statement, Room: e.Room, Paint: e.Paint,
			Floor: e.Floor, Light: e.Light, Status: e.Status, CoverID: e.CoverID, OpeningAt: e.OpeningAt,
			PublishedAt: e.PublishedAt, PosterVersion: e.PosterVersion, Visits: e.VisitCount, Applause: e.ApplauseCount,
			CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, PaintHex: paintHex(e.Paint),
			Owner:     ownerJSON{e.Owner.Handle, e.Owner.Name, e.Owner.Hue},
			WorkCount: e.WorkCount, Preview: []previewJSON{}, Inside: inside[e.ID],
			Mine: viewer != nil && viewer.ID == e.OwnerID,
		}
		for _, id := range e.Preview {
			if a, ok := byID[id]; ok {
				av := artworkView(a)
				v.Preview = append(v.Preview, previewJSON{a.ID, a.Title, av.Image.Aspect, a.Blurhash, a.Dominant})
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func paintHex(key string) string {
	for _, p := range gallery.Paints {
		if p.Key == key {
			return p.Hex
		}
	}
	return gallery.Paints[0].Hex
}
