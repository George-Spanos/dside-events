package store

import (
	"context"
)

// Event follow states.
const (
	Following = "following"
	Hidden    = "hidden"
)

// SetEventFollow records the account's state for an event (idempotent).
func (s *Store) SetEventFollow(ctx context.Context, accountID, eventID int64, state string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO follows_events (account_id, event_id, state, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(account_id, event_id) DO UPDATE SET state = excluded.state, created_at = excluded.created_at`,
		accountID, eventID, state, now())
	return err
}

// ClearEventFollow removes the account's state for an event (idempotent).
func (s *Store) ClearEventFollow(ctx context.Context, accountID, eventID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM follows_events WHERE account_id = ? AND event_id = ?", accountID, eventID)
	return err
}
