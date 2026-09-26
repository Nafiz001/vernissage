# Deploying for free: Vercel, Render, Neon, Cloudinary

| Piece | Service | Why |
| --- | --- | --- |
| Next.js site (`web/`) | Vercel | pages, and the proxy for `/api`, `/img`, `/poster` |
| Go server (`server/`) | Render, free web service | API, live rooms, posters |
| PostgreSQL | Neon, free | the collection, people, exhibitions |
| Pictures | Cloudinary, free | resizing and a CDN, which Render's 0.1 CPU can't do |
| Staying awake | UptimeRobot, free | Render's free tier sleeps after 15 idle minutes |

None of these need a card. Do the steps in order: each needs an address
from the one before.

## 1. Neon: the database

1. Sign up at https://neon.tech and create a project (Postgres 18 if it's
   offered, otherwise 17), in a region near you.
2. On the dashboard, open **Connect**, turn **Connection pooling off**, and
   copy the connection string. It looks like
   `postgresql://neondb_owner:…@ep-….neon.tech/neondb?sslmode=require`.
   The server listens for Postgres notifications, which need this direct
   connection, not the pooled one.
3. Copy your local collection into it, rather than reading the museums
   again (that takes over an hour). From the repository on the machine
   where you ran the ingest:

   ```bash
   export NEON_URL='postgresql://…'   # the string from step 2
   pg_dump --no-owner --no-privileges -h 127.0.0.1 -p 5491 -U vernissage vernissage | psql "$NEON_URL"
   ```

   Starting from nothing instead? Point `DATABASE_URL` at Neon and run
   `vernissage ingest`, then `vernissage serve` until the pictures are
   analysed, then `vernissage seed`, all on your own machine.

## 2. Cloudinary: the pictures

1. Sign up at https://cloudinary.com. Note the **cloud name** on the
   dashboard.
2. Settings → Security: under **Restricted image types**, make sure
   **Fetched URL** is *not* ticked, so Cloudinary may fetch the museums'
   pictures. Optionally list `openaccess-cdn.clevelandart.org` and
   `images.metmuseum.org` as allowed fetch domains.

## 3. Render: the Go server

1. Sign up at https://render.com with GitHub.
2. **New → Blueprint**, pick the `vernissage` repository. Render reads
   `render.yaml` and asks for three values:
   - `DATABASE_URL`: Neon's string from step 1.
   - `VERNISSAGE_CLOUDINARY`: your cloud name.
   - `VERNISSAGE_ORIGINS`: put `https://example.com` for now; you'll set
     the Vercel address in step 5.
3. Deploy. When it's live, note its address, like
   `https://vernissage-api.onrender.com`, and check
   `https://vernissage-api.onrender.com/api/health` says `{"ok":true}`.

## 4. Vercel: the site

1. Sign up at https://vercel.com with GitHub. **Add New → Project**, import
   `vernissage`.
2. **Root Directory**: `web`. Framework: Next.js (detected).
3. Environment variables:
   - `VERNISSAGE_API` = `https://vernissage-api.onrender.com`
   - `NEXT_PUBLIC_WS_ORIGIN` = `wss://vernissage-api.onrender.com`
   - `NEXT_PUBLIC_SITE_URL` = the Vercel address, e.g.
     `https://vernissage.vercel.app` (edit it after the first deploy if
     Vercel gives you a different one, then redeploy).
4. Deploy, and note the address.

## 5. Tell the server where the site is

In Render, service → **Environment**: set `VERNISSAGE_ORIGINS` to the
exact Vercel address, `https://vernissage.vercel.app` (no trailing
slash). Save; Render redeploys. The server refuses sign-ins, saves and
live rooms from any other origin.

## 6. UptimeRobot: keep it awake

Sign up at https://uptimerobot.com, **Add New Monitor** → HTTP(s),
URL `https://vernissage-api.onrender.com/api/health`, every 5 minutes.
Render's free tier gives 750 hours a month, enough for one service
running the whole month.

## Checking it

- The front page shows today's masterpiece; the collection loads pictures
  (from `res.cloudinary.com`, in the browser's network tab).
- Sign in with `demo@vernissage.local` / `open-the-doors`, open
  Exhibitions, walk into one: the top bar should say "1 here".
- Open the same room in a second browser: each sees the other.

## What's different from running it on one machine

- **Deep zoom** shows one 2400-pixel picture instead of tiles.
- **The picture cache and posters** live on Render's temporary disk and
  are redrawn after each deploy.
- **Periodic re-reads of the museums** are off; run `vernissage ingest`
  from your own machine against Neon when you want new works.
