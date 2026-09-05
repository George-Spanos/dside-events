package store

import (
	"context"
)

// Interest states.
const (
	Interested    = "interested"
	NotInterested = "not_interested"
)

// SetInterest records the account's state for an event (idempotent).
func (s *Store) SetInterest(ctx context.Context, accountID, eventID int64, state string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO interests (account_id, event_id, state, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, event_id) DO UPDATE SET state = excluded.state, created_at = excluded.created_at`,
		accountID, eventID, state, now())
	return err
}

// ClearInterest removes the account's state for an event (idempotent).
func (s *Store) ClearInterest(ctx context.Context, accountID, eventID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM interests WHERE account_id = ? AND event_id = ?", accountID, eventID)
	return err
}

// FollowTag and friends are idempotent.
func (s *Store) FollowTag(ctx context.Context, accountID int64, tag string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO follows_tags (account_id, tag, created_at) VALUES (?, ?, ?)", accountID, tag, now())
	return err
}

// UnfollowTag removes a tag follow.
func (s *Store) UnfollowTag(ctx context.Context, accountID int64, tag string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM follows_tags WHERE account_id = ? AND tag = ?", accountID, tag)
	return err
}

// FollowPoster records a poster follow.
func (s *Store) FollowPoster(ctx context.Context, accountID, posterID int64) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO follows_posters (account_id, poster_id, created_at) VALUES (?, ?, ?)", accountID, posterID, now())
	return err
}

// UnfollowPoster removes a poster follow.
func (s *Store) UnfollowPoster(ctx context.Context, accountID, posterID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM follows_posters WHERE account_id = ? AND poster_id = ?", accountID, posterID)
	return err
}

// Follows is everything an account follows.
type Follows struct {
	Tags    []string
	Posters []Account
}

// HasTag reports whether tag is followed.
func (f *Follows) HasTag(tag string) bool {
	for _, t := range f.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasPoster reports whether the poster is followed.
func (f *Follows) HasPoster(id int64) bool {
	for _, p := range f.Posters {
		if p.ID == id {
			return true
		}
	}
	return false
}

// Any reports whether anything at all is followed.
func (f *Follows) Any() bool { return len(f.Tags) > 0 || len(f.Posters) > 0 }

// Follows returns the account's followed tags (sorted) and posters (by name).
func (s *Store) Follows(ctx context.Context, accountID int64) (*Follows, error) {
	f := &Follows{}
	rows, err := s.db.QueryContext(ctx, "SELECT tag FROM follows_tags WHERE account_id = ? ORDER BY tag", accountID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return nil, err
		}
		f.Tags = append(f.Tags, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT `+accountCols+` FROM accounts
		WHERE id IN (SELECT poster_id FROM follows_posters WHERE account_id = ?) ORDER BY name, id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		f.Posters = append(f.Posters, *a)
	}
	return f, rows.Err()
}
