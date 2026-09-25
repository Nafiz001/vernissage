-- The collection: works read from the museums' open-access APIs, and what
-- the server has learned by looking at their pictures.

create extension if not exists pg_trgm;
create extension if not exists unaccent;

-- unaccent() is only STABLE (its dictionary could change), so it can't be
-- used in a generated column. Pinning the dictionary makes it immutable.
create or replace function immutable_unaccent(text) returns text
  language sql immutable parallel safe strict
  as $$ select public.unaccent('public.unaccent'::regdictionary, $1) $$;

create table artworks (
  id               bigserial primary key,
  source           text not null,            -- 'cma' | 'met'
  source_id        text not null,
  title            text not null,
  artist           text not null default '',
  artist_bio       text not null default '', -- "Dutch, 1853–1890"
  date_text        text not null default '',
  year_start       int,
  year_end         int,
  medium           text not null default '',
  kind             text not null,            -- painting | print | drawing
  culture          text not null default '',
  department       text not null default '',
  description      text not null default '',
  credit           text not null default '',
  source_url       text not null,
  image_url        text not null,            -- the museum's web-size picture
  large_url        text not null,            -- its print or original
  width_cm         real,                     -- the object, unframed
  height_cm        real,
  highlight        boolean not null default false,

  -- Filled in by the analyze job.
  analyzed_at      timestamptz,
  aspect           real,                     -- picture width / height
  palette          jsonb,                    -- [{hex, l, a, b, w}], widest first
  dominant         text,
  blurhash         text,
  lightness        real,
  colorfulness     real,

  search tsvector generated always as (
    setweight(to_tsvector('english', immutable_unaccent(title)), 'A') ||
    setweight(to_tsvector('english', immutable_unaccent(artist)), 'A') ||
    setweight(to_tsvector('english', immutable_unaccent(culture || ' ' || medium || ' ' || department || ' ' || date_text)), 'B') ||
    setweight(to_tsvector('english', immutable_unaccent(description)), 'C')
  ) stored,

  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now(),
  unique (source, source_id)
);

create index artworks_search on artworks using gin (search);
create index artworks_artist_trgm on artworks using gin (lower(immutable_unaccent(artist)) gin_trgm_ops);
create index artworks_title_trgm on artworks using gin (lower(immutable_unaccent(title)) gin_trgm_ops);
create index artworks_kind_year on artworks (kind, year_start);
create index artworks_unanalyzed on artworks (id) where analyzed_at is null;
