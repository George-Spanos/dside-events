package store

import (
	"context"
)

// Event follow states.
const (
	Following = "following"
	Hidden    = "hidden"
)

// SetEventFollow records the account's state for an event and, when the
// event repeats, for every date of its series (idempotent).
func (s *Store) SetEventFollow(ctx context.Context, accountID, eventID int64, state string) error {
	// A standalone row has a NULL series_id, which the subselect never
	// matches, so only id = ? applies.
	_, err := s.db.ExecContext(ctx, `INSERT INTO follows_events (account_id, event_id, state, created_at)
		SELECT ?, id, ?, ? FROM events
		 WHERE id = ? OR series_id = (SELECT series_id FROM events WHERE id = ?)
		ON CONFLICT(account_id, event_id) DO UPDATE SET state = excluded.state, created_at = excluded.created_at`,
		accountID, state, now(), eventID, eventID)
	return err
}

// ClearEventFollow removes the account's state for an event and, when the
// event repeats, for every date of its series (idempotent).
func (s *Store) ClearEventFollow(ctx context.Context, accountID, eventID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM follows_events WHERE account_id = ?
		AND event_id IN (SELECT id FROM events WHERE id = ? OR series_id = (SELECT series_id FROM events WHERE id = ?))`,
		accountID, eventID, eventID)
	return err
}
