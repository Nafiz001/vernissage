// Package gallery knows the rooms works can hang in, how a curator would
// frame each work, and whether a hang fits on the walls.
//
// The server is the single source of truth: the browser draws rooms and
// frames from the numbers /api/rooms serves, and every hang a curator saves
// is checked here against the same geometry.
package gallery

// A room is a rectangle seen from above, centred on the origin: x runs
// across its width, z across its depth, y up. Walls are numbered clockwise
// from the far wall: 0 north (z = -depth/2), 1 east, 2 south, where the
// door is, 3 west. A position on a wall is measured in metres from the
// wall's left end as you stand in the room facing it.
type Room struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Blurb    string  `json:"blurb"`
	Width    float64 `json:"width"`
	Depth    float64 `json:"depth"`
	Height   float64 `json:"height"`
	Skylight bool    `json:"skylight"`
	Door     Door    `json:"door"`
	Benches  []Bench `json:"benches"`
}

// Door is the entrance, centred on the south wall.
type Door struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Bench is a seat in the room, by its centre and footprint.
type Bench struct {
	X     float64 `json:"x"`
	Z     float64 `json:"z"`
	Width float64 `json:"width"`
	Depth float64 `json:"depth"`
}

// Paint is a wall colour, named the way a museum's paint schedule would.
type Paint struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

// Option is a floor or a lighting scheme.
type Option struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

var Rooms = []Room{
	{
		Key: "salon", Name: "Grand salon", Width: 16, Depth: 10, Height: 6, Skylight: true,
		Blurb:   "A tall room under a skylight, for big paintings and a crowd.",
		Door:    Door{Width: 2.4, Height: 3.4},
		Benches: []Bench{{X: 0, Z: 0.5, Width: 3.2, Depth: 0.7}},
	},
	{
		Key: "cabinet", Name: "Cabinet", Width: 7, Depth: 6, Height: 3.6,
		Blurb: "A small, close room for prints, drawings and little panels.",
		Door:  Door{Width: 1.4, Height: 2.6},
	},
	{
		Key: "cube", Name: "White cube", Width: 12, Depth: 12, Height: 4.2,
		Blurb:   "Square and even-lit, with space around every work.",
		Door:    Door{Width: 2, Height: 3},
		Benches: []Bench{{X: 0, Z: 0, Width: 2.4, Depth: 0.6}},
	},
	{
		Key: "long", Name: "Long gallery", Width: 24, Depth: 8, Height: 5.5, Skylight: true,
		Blurb:   "A long walk past a long wall, for a story told in order.",
		Door:    Door{Width: 2.4, Height: 3.4},
		Benches: []Bench{{X: -6, Z: 0, Width: 2.8, Depth: 0.6}, {X: 6, Z: 0, Width: 2.8, Depth: 0.6}},
	},
}

var Paints = []Paint{
	{"oxblood", "Salon red", "#5b1d1f"},
	{"verdigris", "Gallery green", "#2d4a40"},
	{"prussian", "Prussian blue", "#1e2e4a"},
	{"ochre", "Pompeian ochre", "#a87a36"},
	{"plaster", "Plaster pink", "#cfae9f"},
	{"sage", "Sage", "#8e9b86"},
	{"chalk", "Chalk white", "#e9e7e1"},
	{"lamp", "Lamp black", "#262523"},
}

var Floors = []Option{
	{"oak", "Oak parquet"},
	{"walnut", "Walnut boards"},
	{"concrete", "Polished concrete"},
	{"marble", "Marble"},
}

var Lights = []Option{
	{"daylight", "Daylight"},
	{"gallery", "Gallery lights"},
	{"evening", "Opening night"},
}

// RoomByKey finds a room preset.
func RoomByKey(key string) (Room, bool) {
	for _, r := range Rooms {
		if r.Key == key {
			return r, true
		}
	}
	return Room{}, false
}

func hasKey[T any](list []T, key string, keyOf func(T) string) bool {
	for _, v := range list {
		if keyOf(v) == key {
			return true
		}
	}
	return false
}

func ValidPaint(key string) bool { return hasKey(Paints, key, func(p Paint) string { return p.Key }) }
func ValidFloor(key string) bool { return hasKey(Floors, key, func(o Option) string { return o.Key }) }
func ValidLight(key string) bool { return hasKey(Lights, key, func(o Option) string { return o.Key }) }

// WallLength is the length of wall w in metres.
func (r Room) WallLength(w int) float64 {
	if w%2 == 0 {
		return r.Width
	}
	return r.Depth
}

// DoorSpan is where the doorway sits along the south wall.
func (r Room) DoorSpan() (from, to float64) {
	mid := r.Width / 2
	return mid - r.Door.Width/2, mid + r.Door.Width/2
}
