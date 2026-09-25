-- Exhibitions: a room, the works hung in it, and the people who came.

create table exhibitions (
  id               bigserial primary key,
  owner_id         bigint not null references users on delete cascade,
  slug             text not null unique,
  title            text not null,
  statement        text not null default '',
  room             text not null default 'salon',
  paint            text not null default 'oxblood',
  floor            text not null default 'oak',
  light            text not null default 'gallery',
  status           text not null default 'draft' check (status in ('draft', 'published')),
  cover_id         bigint references artworks on delete set null,
  opening_at       timestamptz,
  published_at     timestamptz,
  poster_version   int not null default 0,
  visit_count      int not null default 0,
  applause_count   int not null default 0,
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now()
);
create index exhibitions_owner on exhibitions (owner_id, updated_at desc);
create index exhibitions_published on exhibitions (published_at desc, id desc) where status = 'published';

-- Where each work hangs: which wall, and the centre of its frame in metres
-- from the wall's left end and from the floor.
create table placements (
  exhibition_id  bigint not null references exhibitions on delete cascade,
  artwork_id     bigint not null references artworks on delete cascade,
  wall           smallint not null check (wall between 0 and 3),
  x              real not null,
  y              real not null,
  label          text not null default '',
  primary key (exhibition_id, artwork_id)
);
create index placements_artwork on placements (artwork_id);

-- Guests are known by a random visitor cookie, signed-in people by it too,
-- so neither can applaud twice or count twice in a day.
create table applause (
  exhibition_id  bigint not null references exhibitions on delete cascade,
  visitor        text not null,
  created_at     timestamptz not null default now(),
  primary key (exhibition_id, visitor)
);

create table visits (
  exhibition_id  bigint not null references exhibitions on delete cascade,
  visitor        text not null,
  day            date not null default current_date,
  primary key (exhibition_id, visitor, day)
);

create table guestbook (
  id             bigserial primary key,
  exhibition_id  bigint not null references exhibitions on delete cascade,
  user_id        bigint references users on delete set null,
  name           text not null,
  hue            smallint not null,
  message        text not null,
  created_at     timestamptz not null default now()
);
create index guestbook_recent on guestbook (exhibition_id, id desc);
