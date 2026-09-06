package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Link is an external link attached to an event.
type Link struct {
	Label string
	URL   string
}

// Event is an event row joined with its poster, tags, links and the
// follower counter (plus the viewer's own follow state when known).
type Event struct {
	ID          int64
	Slug        string
	Title       string
	StartsAt    time.Time
	Venue       string
	Price       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	PosterID   int64
	PosterName string
	PosterSlug string

	Followers   int      // number of accounts following the event
	ViewerState string   // "", "following" or "hidden"
	Tags        []string // sorted
	Links       []Link   // only populated by EventBySlug

	SeriesID    int64 // shared by every date of a repeating event; 0 when the event stands alone
	SeriesCount int   // dates still in the series, this one included; 0 when SeriesID is 0
}

// EventInput is the validated payload for creating or updating an event.
type EventInput struct {
	Title       string
	StartsAt    time.Time
	Venue       string
	Price       string
	Description string
	Tags        []string
	Links       []Link
}

// eventSelect is shared by every event query. Bind the viewer id (0 for
// anonymous) as the first argument. The series count is computed per row so
// it stays right after single dates are deleted; a NULL series_id never
// matches, so a standalone row counts 0.
const eventSelect = `SELECT e.id, e.slug, e.title, e.starts_at, e.venue, e.price, e.description, e.created_at, e.updated_at,
  p.id, p.name, COALESCE(p.slug, ''),
  (SELECT COUNT(*) FROM follows_events f WHERE f.event_id = e.id AND f.state = 'following'),
  COALESCE((SELECT f.state FROM follows_events f WHERE f.event_id = e.id AND f.account_id = ?), ''),
  COALESCE((SELECT GROUP_CONCAT(tag, ',') FROM (SELECT t.tag FROM event_tags t WHERE t.event_id = e.id ORDER BY t.tag)), ''),
  COALESCE(e.series_id, 0),
  (SELECT COUNT(*) FROM events s WHERE s.series_id = e.series_id)
FROM events e JOIN accounts p ON p.id = e.poster_id `

func scanEvents(rows *sql.Rows) ([]Event, error) {
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var starts, created, updated int64
		var tags string
		if err := rows.Scan(&e.ID, &e.Slug, &e.Title, &starts, &e.Venue, &e.Price, &e.Description, &created, &updated,
			&e.PosterID, &e.PosterName, &e.PosterSlug, &e.Followers, &e.ViewerState, &tags, &e.SeriesID, &e.SeriesCount); err != nil {
			return nil, err
		}
		e.StartsAt, e.CreatedAt, e.UpdatedAt = toTime(starts), toTime(created), toTime(updated)
		if tags != "" {
			e.Tags = strings.Split(tags, ",")
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) queryEvents(ctx context.Context, where string, args ...any) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, eventSelect+where, args...)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// EventBySlug returns one event with its links.
func (s *Store) EventBySlug(ctx context.Context, slug string, viewerID int64) (*Event, error) {
	events, err := s.queryEvents(ctx, "WHERE e.slug = ?", viewerID, slug)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrNotFound
	}
	e := &events[0]
	rows, err := s.db.QueryContext(ctx, "SELECT label, url FROM event_links WHERE event_id = ? ORDER BY position", e.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.Label, &l.URL); err != nil {
			return nil, err
		}
		e.Links = append(e.Links, l)
	}
	return e, rows.Err()
}

// FeedOpts filters the public feed.
type FeedOpts struct {
	From     time.Time // include events starting at or after this instant
	Tag      string    // optional tag filter
	ViewerID int64     // 0 for anonymous; leaves out the viewer's hidden events
	Limit    int       // at most this many rows; 0 means all
}

// Feed lists upcoming events in chronological order.
func (s *Store) Feed(ctx context.Context, o FeedOpts) ([]Event, error) {
	where := `WHERE e.starts_at >= ?
  AND NOT EXISTS (SELECT 1 FROM follows_events f WHERE f.event_id = e.id AND f.account_id = ? AND f.state = 'hidden')`
	args := []any{o.ViewerID, o.From.Unix(), o.ViewerID}
	if o.Tag != "" {
		where += " AND EXISTS (SELECT 1 FROM event_tags t WHERE t.event_id = e.id AND t.tag = ?)"
		args = append(args, o.Tag)
	}
	where += " ORDER BY e.starts_at ASC, e.id ASC"
	if o.Limit > 0 {
		where += " LIMIT ?"
		args = append(args, o.Limit)
	}
	return s.queryEvents(ctx, where, args...)
}

// FollowedEvents lists the events the account follows, narrowed to one tag
// when tag is set.
func (s *Store) FollowedEvents(ctx context.Context, accountID int64, tag string) ([]Event, error) {
	where := "JOIN follows_events f ON f.event_id = e.id AND f.account_id = ? AND f.state = 'following'"
	args := []any{accountID, accountID}
	if tag != "" {
		where += " WHERE EXISTS (SELECT 1 FROM event_tags t WHERE t.event_id = e.id AND t.tag = ?)"
		args = append(args, tag)
	}
	return s.queryEvents(ctx, where+" ORDER BY e.starts_at ASC, e.id ASC", args...)
}

// HiddenEvents lists the events the account hid.
func (s *Store) HiddenEvents(ctx context.Context, accountID int64) ([]Event, error) {
	return s.queryEvents(ctx, `JOIN follows_events f ON f.event_id = e.id AND f.account_id = ? AND f.state = 'hidden'
  ORDER BY e.starts_at ASC, e.id ASC`, accountID, accountID)
}

// EventsByPoster lists all events of a poster in chronological order.
func (s *Store) EventsByPoster(ctx context.Context, posterID, viewerID int64) ([]Event, error) {
	return s.queryEvents(ctx, "WHERE e.poster_id = ? ORDER BY e.starts_at ASC, e.id ASC", viewerID, posterID)
}

// EventsByPosterBetween lists a poster's events with from <= starts_at < to.
func (s *Store) EventsByPosterBetween(ctx context.Context, posterID int64, from, to time.Time) ([]Event, error) {
	return s.queryEvents(ctx, "WHERE e.poster_id = ? AND e.starts_at >= ? AND e.starts_at < ? ORDER BY e.id",
		0, posterID, from.Unix(), to.Unix())
}

// CreateEvent inserts an event, deriving a unique slug from baseSlug.
func (s *Store) CreateEvent(ctx context.Context, posterID int64, baseSlug string, in EventInput) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	slug, err := insertEvent(ctx, tx, posterID, baseSlug, in.StartsAt, 0, in)
	if err != nil {
		return "", err
	}
	return slug, tx.Commit()
}

// CreateSeries inserts one event per start, all sharing a fresh series id,
// in one transaction. starts is ascending with at least two instants (the
// caller validates); in.StartsAt is ignored. baseSlug names each date's
// slug base. Returns the slug of starts[0].
func (s *Store) CreateSeries(ctx context.Context, posterID int64, starts []time.Time, baseSlug func(time.Time) string, in EventInput) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	// Safe: the server holds the single write connection and the CLI never
	// writes events, so nobody else can mint the same id meanwhile.
	var seriesID int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(series_id), 0) + 1 FROM events").Scan(&seriesID); err != nil {
		return "", err
	}
	var first string
	for i, start := range starts {
		slug, err := insertEvent(ctx, tx, posterID, baseSlug(start), start, seriesID, in)
		if err != nil {
			return "", err
		}
		if i == 0 {
			first = slug
		}
	}
	return first, tx.Commit()
}

// insertEvent writes one event row with its tags and links inside tx,
// deriving a unique slug from baseSlug. A seriesID of 0 stores NULL.
func insertEvent(ctx context.Context, tx *sql.Tx, posterID int64, baseSlug string, start time.Time, seriesID int64, in EventInput) (string, error) {
	slug, err := uniqueSlug(ctx, tx, baseSlug, "events", "retired_slugs")
	if err != nil {
		return "", err
	}
	n := now()
	res, err := tx.ExecContext(ctx, `INSERT INTO events (slug, poster_id, title, starts_at, venue, price, description, created_at, updated_at, series_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0))`, slug, posterID, in.Title, start.Unix(), in.Venue, in.Price, in.Description, n, n, seriesID)
	if err != nil {
		return "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	if err := writeTagsAndLinks(ctx, tx, id, in); err != nil {
		return "", err
	}
	return slug, nil
}

// UpdateEvent replaces the editable fields of an event. The slug never changes.
func (s *Store) UpdateEvent(ctx context.Context, id int64, in EventInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE events SET title = ?, starts_at = ?, venue = ?, price = ?, description = ?, updated_at = ?
		WHERE id = ?`, in.Title, in.StartsAt.Unix(), in.Venue, in.Price, in.Description, now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM event_tags WHERE event_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM event_links WHERE event_id = ?", id); err != nil {
		return err
	}
	if err := writeTagsAndLinks(ctx, tx, id, in); err != nil {
		return err
	}
	return tx.Commit()
}

func writeTagsAndLinks(ctx context.Context, tx *sql.Tx, id int64, in EventInput) error {
	for _, t := range in.Tags {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO event_tags (event_id, tag) VALUES (?, ?)", id, t); err != nil {
			return err
		}
	}
	for i, l := range in.Links {
		if _, err := tx.ExecContext(ctx, "INSERT INTO event_links (event_id, position, label, url) VALUES (?, ?, ?, ?)",
			id, i+1, l.Label, l.URL); err != nil {
			return err
		}
	}
	return nil
}

// DeleteEvent removes an event; follows, tags and links cascade. The slug
// is retired so no later event can take it.
func (s *Store) DeleteEvent(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var slug string
	if err := tx.QueryRowContext(ctx, "SELECT slug FROM events WHERE id = ?", id).Scan(&slug); err != nil {
		return notFound(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO retired_slugs (slug, retired_at) VALUES (?, ?)", slug, now()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM events WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetEvents removes every event so a seed file can be published again, and
// clears retired_slugs with them. Clearing the retirements is the point: a
// plain delete retires each slug, and the next seed of the same events would
// then land on "<slug>-2" and change every public URL. Links, tags and
// follows cascade. Accounts and sessions are untouched, so curators keep
// their secret links and attendees keep their devices.
func (s *Store) ResetEvents(ctx context.Context) (events, retired int64, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, "DELETE FROM events")
	if err != nil {
		return 0, 0, err
	}
	if events, err = res.RowsAffected(); err != nil {
		return 0, 0, err
	}
	res, err = tx.ExecContext(ctx, "DELETE FROM retired_slugs")
	if err != nil {
		return 0, 0, err
	}
	if retired, err = res.RowsAffected(); err != nil {
		return 0, 0, err
	}
	return events, retired, tx.Commit()
}

// CountEvents is how many events exist, so a destructive command can say what
// it is about to remove before it removes it.
func (s *Store) CountEvents(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&n)
	return n, err
}

// seriesPick orders a series' dates so the first row is the one that stands
// for the whole run in search: the next date at or after now, or the last one
// when they have all passed. Bind now twice, after the earlier arguments.
// alias is the events table's alias in the enclosing query.
func seriesPick(alias string) string {
	c := alias + ".starts_at"
	return "ORDER BY (" + c + " >= ?) DESC, CASE WHEN " + c + " >= ? THEN " + c + " END ASC, " + c + " DESC"
}

// SeriesCanonicalSlug returns the slug of the date that represents seriesID.
// Every other date of the run points at it with a canonical link, so search
// engines index the run once instead of once per date.
func (s *Store) SeriesCanonicalSlug(ctx context.Context, seriesID int64, now time.Time) (string, error) {
	var slug string
	err := s.db.QueryRowContext(ctx,
		"SELECT e.slug FROM events e WHERE e.series_id = ? "+seriesPick("e")+" LIMIT 1",
		seriesID, now.Unix(), now.Unix()).Scan(&slug)
	if err != nil {
		return "", notFound(err)
	}
	return slug, nil
}

// IndexableEvent is one event URL that stands for itself in search.
type IndexableEvent struct {
	Slug      string
	UpdatedAt time.Time
}

// IndexableEvents lists every event that belongs in the sitemap: standalone
// events, plus one date per series (see SeriesCanonicalSlug). The dates a
// canonical link points away from are left out, because a sitemap carries
// canonical URLs only.
func (s *Store) IndexableEvents(ctx context.Context, now time.Time) ([]IndexableEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug, updated_at FROM events WHERE series_id IS NULL
UNION ALL
SELECT e.slug, e.updated_at FROM events e WHERE e.series_id IS NOT NULL AND e.id =
  (SELECT x.id FROM events x WHERE x.series_id = e.series_id `+seriesPick("x")+` LIMIT 1)
ORDER BY 2 DESC`, now.Unix(), now.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IndexableEvent
	for rows.Next() {
		var e IndexableEvent
		var updated int64
		if err := rows.Scan(&e.Slug, &updated); err != nil {
			return nil, err
		}
		e.UpdatedAt = toTime(updated)
		out = append(out, e)
	}
	return out, rows.Err()
}
