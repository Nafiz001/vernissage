# Vernissage

Hang your own exhibition of the world's masterpieces, then open the doors.

Vernissage gathers over 8,000 paintings, woodblock prints and drawings that
The Metropolitan Museum of Art and the Cleveland Museum of Art have released
into the public domain. You browse them the way you'd walk a museum, save the
ones that stop you, and hang them on the walls of a room: a grand salon under
a skylight, a small cabinet, a white cube, a long gallery. Then you send the
link, and whoever opens it walks through your exhibition in 3D, in the
browser, alongside everyone else who's there.

A *vernissage* is an exhibition's opening night.

![The front page: Sargent's Madame X in its gilt frame on a Salon-red wall](docs/front.jpg)

- **Browse** a salon hang of the whole collection. Search by artist, title,
  subject or century, with typos forgiven; or by colour: pick vermilion and
  see the works that are full of it.
- **Look closely** at any work in a deep-zoom viewer, read its museum label,
  see the six colours it's made of, and see it at real size beside a person.
- **Hang** works in a studio that draws each wall to scale, with the door,
  the eye line and the gaps between frames measured. Every work hangs at its
  true size, framed the way a curator would frame it. Or press *Hang for me*.
- **Open the doors.** Visitors walk through the room with a mouse, keyboard
  or finger, click a work to stand in front of it, take a guided tour with
  an audio guide, see where everyone else is standing, applaud, and sign the
  guestbook on the way out.

![Walking into an exhibition, stopping at a Monet, then on to the next work](docs/walk.gif)

```console
$ make setup          # Go modules and npm packages
$ make db             # PostgreSQL 18 in Docker on :5491
$ make ingest         # read both museums (about 80 minutes; the Met is read slowly)
$ make dev            # API on :8790, site on :3790
$ make seed           # once the pictures are analysed: six exhibitions
```

Sign in as the demo curator, `demo@vernissage.local` with the password
`open-the-doors`, to try the studio without making an account.

<table>
<tr>
<td><img src="docs/walk.jpg" alt="Another visitor, Margot, in front of Monet's Water Lilies in the long gallery"></td>
<td><img src="docs/studio.jpg" alt="The studio: a wall drawn to scale with works on it"></td>
</tr>
<tr>
<td><img src="docs/collection.jpg" alt="The collection as a salon hang on a green wall"></td>
<td><img src="docs/artwork.jpg" alt="Van Gogh's Wheat Field with Cypresses on a wall mixed from its own colours"></td>
</tr>
</table>

## How it fits together

```
                 browser
   ┌───────────────┼────────────────────────┐
   │ pages         │ /api /img /dzi /poster │ /ws (live rooms)
   ▼               ▼                        ▼
 Next.js 16    ┌──────────────── Go server ─────────────────┐
 (React 19,    │ HTTP API · WebSocket hub · image service   │
 R3F, GSAP)    │ colour index (in memory, kept by NOTIFY)   │
               │ job workers ◄── NOTIFY ──┐                 │
               └───────┬──────────────────┼─────────────────┘
                       ▼                  │
                  PostgreSQL 18 ──────────┘      disk cache: masters, sizes,
                  (collection, people,           Deep Zoom pyramids, posters
                   exhibitions, jobs)
                       ▲
       The Met and Cleveland open-access APIs and image CDNs
```

`server/` is one Go binary: `vernissage serve` runs the API, the live rooms
and the background workers; `ingest`, `analyze`, `migrate` and `seed` are
subcommands. `web/` is a Next.js app. In development Next forwards `/api`,
`/img`, `/dzi` and `/poster` to Go so the browser sees one origin; in
production Caddy does the same routing (`docker compose --profile full up`).

## The collection

`internal/museum` reads the Cleveland Museum of Art's Open Access API (every
painting, the Japanese woodblock prints, the drawings it counts as
highlights) and The Met's Collection API (European paintings in full, and
highlights from Asian art, the American Wing, drawings and prints, and
modern art, keeping only what's in the public domain). Both become one
record: the artist and their dates, the medium, the size of the object
itself rather than its frame, and a web-size and a print-size image.

The Met's search returns only ids, so each work is a request of its own. The
API allows 80 a second, but its firewall turns away sustained bursts far
below that, so the reader takes three a second and treats a 403 or 429 as
"slow down": it waits thirty seconds and more before trying again, and
gives up only after twenty failures in a row. Records are upserted; a new
or re-photographed work gets an `analyze` job.

## Looking at every picture

The `analyze` job downloads a work's picture once, keeps it as a
1200-pixel master, and learns from it:

- **Its palette.** The picture is box-filtered to 96 pixels, averaging in
  linear light as a lens would, and every pixel is converted to
  [OKLab](https://bottosson.github.io/posts/oklab/), a colour space in which
  distance tracks how different two colours look. K-means with k = 8, seeded
  by k-means++ from a fixed-seed generator so the same picture always gives
  the same answer, finds the clusters; clusters closer than 0.045 (the same
  colour to the eye) merge; anything under 1.5% of the picture is dropped.
  What's left is up to six colours and the share of the picture each covers.
- **Its mood.** Mean OKLab lightness, and Hasler and Süsstrunk's
  colourfulness metric.
- **A placeholder.** A BlurHash, byte-for-byte what the reference
  TypeScript encoder produces, which the site turns into a blurred SVG so
  the server's first render and the browser's agree.
- **Its proportions**, which the museums don't always give.

## Searching by colour

`internal/colorindex` holds every palette in memory: at 8,000 works a full
scan takes a couple of milliseconds, simpler and faster than any tree once
a smooth falloff is involved. A work's score for a colour is how much of it
is near that colour:

    score = Σ share(i) · exp(−(d(query, colour i) / σ)²)

so a seascape that is 60% the blue asked for outranks a portrait with a
blue ribbon. Several colours multiply (geometric mean), so all must be
present.

The distance isn't plain OKLab distance. A palette colour is the average of
thousands of pixels, so it's always greyer than the paint on the canvas: the
ultramarine of a robe averages out to a muted blue, and plain distance
would say no painting contains ultramarine. So `Match` counts hue in full,
lightness at three quarters, and forgives a palette colour for being less
saturated than asked, down to about half the chroma. Below that it charges
steeply, so greys don't pass for blues. Tests pin both halves.

Moods (bright, dark, vivid, muted) are the quartiles of lightness and
colourfulness across the collection. The index loads at start-up and
follows a `NOTIFY` channel the analyze job writes to, so every server
instance sees a new palette the moment it's stored.

## Searching by words

Each work has a generated `tsvector`: title and artist weighted highest,
then culture, medium and department, then the museum's description, all
stemmed in English and unaccented. `unaccent` is only `STABLE`, so it's
wrapped in an `IMMUTABLE` function that pins the dictionary, which is what
a generated column needs. Results rank by `ts_rank_cd`, highlights a little
higher. When the words match nothing, the query falls back to trigram word
similarity on artists and titles, so "hokusia" finds Hokusai; the page says
it's showing close spellings. Suggestions as you type come from the same
trigram indexes.

## Pictures

`internal/imaging` serves every size the site needs from a disk cache:

- `/img/{id}/{200…2400}.jpg`: up to 1200 pixels from the master, larger
  from the museum's print-quality file (up to 3000 pixels).
- `/dzi/{id}.dzi` and its tiles: a [Deep Zoom](https://openseadragon.github.io/)
  pyramid, the picture halved again and again down to one pixel, cut into
  254-pixel tiles overlapping by one. It's built whole on first request, and
  a marker file written last means complete. OpenSeadragon fetches only the
  tiles on screen.

Three things keep it well behaved on a small machine. `singleflight`
collapses a hundred requests for an uncached picture into one download and
one resize. Decodes wait on a semaphore weighted in pixels (64 megapixels in
flight at most), because decoding a 30-megapixel photograph takes real
memory. Each museum host gets six connections. Files are written to a
temporary name and renamed, so no reader sees half a picture. A janitor
keeps derived files under a byte budget, evicting whatever was least
recently looked at; masters are never evicted.

## Rooms, frames and hanging

`internal/gallery` is the single source of truth for geometry: the browser
draws rooms and frames from the numbers `/api/rooms` serves, and every hang
is checked there.

**Real size.** A work keeps the area the museum measured but takes the
proportions of its photograph, so nothing is stretched. Without
measurements it's given a typical size for its kind and marked as a guess.

**Framing** follows what a curator would do. Old masters get gilt, wider
for earlier work. Works on paper go behind a card mat in a slim moulding,
oak for Japanese prints and black otherwise. Hanging scrolls keep a silk
mounting; twentieth-century paintings get a plain oak strip. No moulding is
wider than 16% of the work it holds.

**Validation** checks that each frame stays on its wall, clear of the
doorway, 35 cm off the floor and 25 cm under the ceiling, and at least
8 cm from any other frame. The studio runs the same checks in the browser
on every drag, so a frame turns red the moment it stops fitting; the server
still decides what's saved, and won't open the doors on a hang that fails.

**Hang for me** starts the way a curator does: the largest work alone on
the far wall, facing whoever walks in; the rest shared between the walls so
each is about as full as the others, the door wall last; each wall
symmetrical about its biggest piece; every centre on the eye line
(1.48 m, the museum convention of 57 inches). A test hangs from one to
fourteen works in every room and checks that Validate accepts each result.

## The job queue

`internal/jobs` is a small, durable queue in the database:

- `Enqueue` inserts in the caller's transaction, so publishing an exhibition
  and queueing its poster commit together or not at all. A dedupe key allows
  at most one live job per key, via a partial unique index.
- Workers claim with `UPDATE … WHERE id = (SELECT … FOR UPDATE SKIP LOCKED)`,
  so any number of them in any number of processes never take the same job.
  Higher priority runs first: a poster jumps ahead of 5,000 analyses.
- A `NOTIFY` wakes idle workers the moment something is queued; a slow poll
  catches anything a notification missed.
- A claimed job holds a lease. If its worker dies, a reaper hands it out
  again when the lease runs out.
- Failures retry with exponential backoff and jitter. Handlers can fail a
  job permanently (the museum no longer has the picture).

## Live rooms

Each open exhibition is a room in `internal/live`. Visitors send where they
stand and which way they face as they move, up to twenty times a second. The
room gathers these and sends everyone one batch ten times a second, so a
crowd of thirty costs each browser ten small messages a second rather than
three hundred; the browser eases each figure toward its latest position.
Reactions, which work someone is looking at, arrivals, departures and
guestbook signatures go out at once.

Every connection has its own write queue. One that can't keep up misses
position batches (the next one supersedes them); one that falls further
behind is disconnected rather than allowed to hold the room back. Each
visitor is rate-limited: reactions as a token bucket, signatures one every
ten seconds, text cleaned and capped. Guests are named after pigments
(Cobalt, Madder, Verdigris) chosen from their visitor cookie, so the same
guest is the same colour every time.

## Posters

When an exhibition opens, a job draws its share card in Go: the cover hung
in a shaded moulding on the exhibition's own paint, a pool of light above
it, the title set in Bodoni Moda and fitted to three lines. Links to an
exhibition unfurl with it.

![The Colour of Water's poster](docs/poster.jpg)

## Accounts and safety

- Passwords are argon2id at OWASP's minimum (19 MiB, two passes). A sign-in
  with an unknown email checks against a dummy hash, so it takes as long as
  a wrong password and doesn't reveal who has an account.
- Sessions are 256-bit random tokens in an `HttpOnly`, `SameSite=Lax`
  cookie. Only their SHA-256 is stored, so a leaked table can't be replayed.
  They slide for thirty days.
- Every write must come from one of the site's origins, checked against the
  browser's `Origin` or `Sec-Fetch-Site` header, and WebSockets check their
  origin too.
- Token buckets per IP limit reads, writes and sign-in attempts, with sign-in
  also limited per email.
- Guests get a random visitor cookie, so applause and visits count once per
  person (visits once a day) without an account.

## The site

The site is a building. Every page is a room painted a museum wall colour
(Salon red, Gallery green, Prussian blue, Chalk white) with a skirting board
along the bottom, and works hang on it in frames drawn in CSS from the
server's framing rules: a lit bevel, a dark sight-edge lip, a burnished bead
and the shadow the moulding casts onto the picture. Type is Bodoni Moda for
lettering, as vinyl on a gallery wall, and Archivo for labels, set the way
museums set them, one fact to a line.

- **The front page** opens in the dark, close enough to a masterpiece to see
  the brushstrokes, and steps back until the work hangs on the wall in its
  frame, the room's light comes up, and the lettering appears beside it.
  Any key, click or scroll hurries it along, and reduced-motion users see
  the end at once.
- **The collection** is one salon hang: justified rows that alternate
  between taller and shorter, loaded as you scroll. You can repaint the wall
  behind it.
- **A work's page** paints its wall a deep, quiet shade of the work's own
  most telling colour, so the picture is the brightest thing in view.
- **The 3D rooms** (React Three Fiber) are built from the server's room
  geometry, with plaster walls in the chosen paint. Floors are drawn on
  canvases at load time (oak parquet, walnut, concrete, marble), so there
  are no texture files, with blurred reflections on the harder ones. There's
  a laylight in the skylit rooms, benches, a track of spotlights, and the
  title lettered over the door. Frames are extruded, bevelled mouldings;
  gilt reflects a procedural environment. Pictures load small and sharpen
  as you approach. Choosing a work glides you to where its whole frame fits
  the screen beside the label panel, shifting the projection so it's
  centred in what's still visible. Quality steps down on slower machines.

## Tests

```console
$ make test
```

Unit tests cover the colour science, palette determinism, the BlurHash
encoder against the reference, colour ranking and matching, framing, the
hanging property test, Deep Zoom tile geometry, the museum mappings and
posters. With `VERNISSAGE_TEST_DATABASE_URL` set, integration tests reset a
real PostgreSQL database and drive the whole API over HTTP: accounts,
refused cross-site writes, stemmed, unaccented, fuzzy and colour search, and
an exhibition from draft through a refused hang, a room too small, opening,
applause and visits to deletion. Two people meet in a live room over real
WebSockets. The job queue is tested for dedupe, exactly-once delivery under
eight competing workers, retry scheduling and permanent failure. CI runs all
of it with the race detector against a PostgreSQL service, then lints,
typechecks and builds the site.

## Configuration

| Variable | Default | |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://vernissage:vernissage@127.0.0.1:5491/vernissage` | |
| `VERNISSAGE_ADDR` | `127.0.0.1:8790` | where the API listens |
| `VERNISSAGE_ORIGINS` | `http://localhost:3790,…` | origins allowed to write and open WebSockets |
| `VERNISSAGE_CACHE`, `VERNISSAGE_CACHE_MB` | `data/cache`, `2048` | picture cache and its budget |
| `VERNISSAGE_WORKERS` | `6` | background jobs at once |
| `VERNISSAGE_SYNC_EVERY` | off | re-read the museums, e.g. `168h` |
| `VERNISSAGE_SECURE_COOKIES` | `false` | set behind HTTPS |
| `VERNISSAGE_API` (web) | `http://127.0.0.1:8790` | where Next finds the API |

## Credits

Pictures and information come from
[The Metropolitan Museum of Art](https://www.metmuseum.org/about-the-met/policies-and-documents/open-access)
and the [Cleveland Museum of Art](https://openaccess-api.clevelandart.org/),
both released under Creative Commons Zero. Neither museum is involved in this
project, and no pictures are stored in the repository. Bodoni Moda and Archivo
are under the SIL Open Font License.

MIT licensed; see [LICENSE](LICENSE).
