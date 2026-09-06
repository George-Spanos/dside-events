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
// anonymous) as the first argument.
const eventSelect = `SELECT e.id, e.slug, e.title, e.starts_at, e.venue, e.price, e.description, e.created_at, e.updated_at,
  p.id, p.name, COALESCE(p.slug, ''),
  (SELECT COUNT(*) FROM follows_events f WHERE f.event_id = e.id AND f.state = 'following'),
  COALESCE((SELECT f.state FROM follows_events f WHERE f.event_id = e.id AND f.account_id = ?), ''),
  COALESCE((SELECT GROUP_CONCAT(tag, ',') FROM (SELECT t.tag FROM event_tags t WHERE t.event_id = e.id ORDER BY t.tag)), '')
FROM events e JOIN accounts p ON p.id = e.poster_id `

func scanEvents(rows *sql.Rows) ([]Event, error) {
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var starts, created, updated int64
		var tags string
		if err := rows.Scan(&e.ID, &e.Slug, &e.Title, &starts, &e.Venue, &e.Price, &e.Description, &created, &updated,
			&e.PosterID, &e.PosterName, &e.PosterSlug, &e.Followers, &e.ViewerState, &tags); err != nil {
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

// FollowedEvents lists the events the account follows.
func (s *Store) FollowedEvents(ctx context.Context, accountID int64) ([]Event, error) {
	return s.queryEvents(ctx, `JOIN follows_events f ON f.event_id = e.id AND f.account_id = ? AND f.state = 'following'
  ORDER BY e.starts_at ASC, e.id ASC`, accountID, accountID)
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
	slug, err := uniqueSlug(ctx, tx, baseSlug, "events", "retired_slugs")
	if err != nil {
		return "", err
	}
	n := now()
	res, err := tx.ExecContext(ctx, `INSERT INTO events (slug, poster_id, title, starts_at, venue, price, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, slug, posterID, in.Title, in.StartsAt.Unix(), in.Venue, in.Price, in.Description, n, n)
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
	return slug, tx.Commit()
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
