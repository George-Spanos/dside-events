-- 2026-09-06: the event action is "follow". The interests table becomes
-- follows_events with states following/hidden. Rebuilt rather than renamed
-- because SQLite cannot change the CHECK constraint in place.
CREATE TABLE follows_events (
  account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  event_id   INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  state      TEXT NOT NULL CHECK (state IN ('following', 'hidden')),
  created_at INTEGER NOT NULL,
  PRIMARY KEY (account_id, event_id)
);
INSERT INTO follows_events (account_id, event_id, state, created_at)
  SELECT account_id, event_id,
         CASE state WHEN 'interested' THEN 'following' ELSE 'hidden' END,
         created_at
  FROM interests;
DROP TABLE interests;
CREATE INDEX follows_events_event_state ON follows_events(event_id, state);
