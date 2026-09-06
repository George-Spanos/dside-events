-- Repeating events: one ordinary row per date, sharing a series_id and
-- nothing else. No series table, no foreign key: deleting one date leaves
-- the others.
ALTER TABLE events ADD COLUMN series_id INTEGER;
CREATE INDEX events_series ON events(series_id);
