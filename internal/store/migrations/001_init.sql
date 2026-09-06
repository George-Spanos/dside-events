-- 001_init: full schema for dside-events. Timestamps are unix seconds UTC.
--
-- Accounts are anonymous. `key` is the account's only credential (the
-- "secret link"): 32 random bytes, base64url. Posters additionally carry a
-- public name and slug.
CREATE TABLE accounts (
  id         INTEGER PRIMARY KEY,
  role       TEXT NOT NULL CHECK (role IN ('user', 'poster')),
  name       TEXT NOT NULL DEFAULT '',
  slug       TEXT UNIQUE,
  key        TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL
);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_account ON sessions(account_id);

CREATE TABLE events (
  id          INTEGER PRIMARY KEY,
  slug        TEXT NOT NULL UNIQUE,
  poster_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  starts_at   INTEGER NOT NULL,
  venue       TEXT NOT NULL,
  price       TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
CREATE INDEX events_starts_at ON events(starts_at);
CREATE INDEX events_poster_starts ON events(poster_id, starts_at);

CREATE TABLE event_links (
  event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  label    TEXT NOT NULL,
  url      TEXT NOT NULL,
  PRIMARY KEY (event_id, position)
);

CREATE TABLE event_tags (
  event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  tag      TEXT NOT NULL,
  PRIMARY KEY (event_id, tag)
);
CREATE INDEX event_tags_tag ON event_tags(tag, event_id);

CREATE TABLE follows_tags (
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  tag        TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (account_id, tag)
);

CREATE TABLE follows_posters (
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  poster_id  INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (account_id, poster_id)
);
CREATE INDEX follows_posters_poster ON follows_posters(poster_id);

-- An account's relation to an event: following (public, counted) or hidden
-- (private, keeps the event out of that account's feed).
CREATE TABLE follows_events (
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  event_id   INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  state      TEXT NOT NULL CHECK (state IN ('following', 'hidden')),
  created_at INTEGER NOT NULL,
  PRIMARY KEY (account_id, event_id)
);
CREATE INDEX follows_events_event_state ON follows_events(event_id, state);

-- Slugs of deleted events. A deleted event's URL answers 404 forever and the
-- slug is never handed to a new event.
CREATE TABLE retired_slugs (
  slug       TEXT PRIMARY KEY,
  retired_at INTEGER NOT NULL
);
