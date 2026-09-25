-- Curators, their sessions, the works they've set aside, and background jobs.

create table users (
  id            bigserial primary key,
  handle        text not null,
  name          text not null,
  email         text not null,
  password_hash text not null,
  hue           smallint not null,          -- their colour in rooms, 0–359
  created_at    timestamptz not null default now()
);
create unique index users_handle on users (lower(handle));
create unique index users_email on users (lower(email));

-- The cookie holds a random token; only its SHA-256 is stored, so a leaked
-- table can't be replayed as sessions.
create table sessions (
  token_hash   bytea primary key,
  user_id      bigint not null references users on delete cascade,
  created_at   timestamptz not null default now(),
  expires_at   timestamptz not null
);
create index sessions_user on sessions (user_id);
create index sessions_expiry on sessions (expires_at);

create table saved (
  user_id     bigint not null references users on delete cascade,
  artwork_id  bigint not null references artworks on delete cascade,
  created_at  timestamptz not null default now(),
  primary key (user_id, artwork_id)
);
create index saved_recent on saved (user_id, created_at desc);

-- A small, durable job queue. Workers claim rows with FOR UPDATE SKIP LOCKED
-- and are woken by NOTIFY; a row whose lease runs out is handed out again.
create table jobs (
  id            bigserial primary key,
  kind          text not null,
  payload       jsonb not null default '{}',
  dedupe        text,                        -- at most one live job per key
  priority      smallint not null default 0, -- higher runs first
  status        text not null default 'queued' check (status in ('queued', 'running', 'done', 'failed')),
  attempts      int not null default 0,
  max_attempts  int not null default 5,
  run_at        timestamptz not null default now(),
  leased_until  timestamptz,
  last_error    text,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now()
);
create index jobs_ready on jobs (priority desc, run_at, id) where status = 'queued';
create index jobs_leased on jobs (leased_until) where status = 'running';
create unique index jobs_dedupe on jobs (dedupe) where dedupe is not null and status in ('queued', 'running');
