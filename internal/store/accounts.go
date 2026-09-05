package store

import (
	"context"
	"time"
)

// Roles.
const (
	RoleUser   = "user"
	RolePoster = "poster"
)

// Account is a row of the accounts table.
type Account struct {
	ID        int64
	Email     string
	Role      string
	Name      string
	Slug      string
	CreatedAt time.Time
}

// IsPoster reports whether the account may post events.
func (a *Account) IsPoster() bool { return a != nil && a.Role == RolePoster }

const accountCols = "id, email, role, name, COALESCE(slug, ''), created_at"

func scanAccount(row interface{ Scan(...any) error }) (*Account, error) {
	var a Account
	var created int64
	if err := row.Scan(&a.ID, &a.Email, &a.Role, &a.Name, &a.Slug, &created); err != nil {
		return nil, notFound(err)
	}
	a.CreatedAt = toTime(created)
	return &a, nil
}

// AccountByEmail returns the account with the given (normalised) email.
func (s *Store) AccountByEmail(ctx context.Context, email string) (*Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE email = ?", email))
}

// EnsureUser returns the account for email, creating it with role user when
// it does not exist. Existing posters keep their role.
func (s *Store) EnsureUser(ctx context.Context, email string) (*Account, error) {
	if _, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO accounts (email, role, created_at) VALUES (?, 'user', ?)", email, now()); err != nil {
		return nil, err
	}
	return s.AccountByEmail(ctx, email)
}

// UpsertPoster creates a poster account or promotes an existing user to
// poster, setting name and slug. It is idempotent: an account that is already
// a poster is returned unchanged (name and slug are kept, baseSlug is ignored),
// so re-running add-poster is a no-op. baseSlug is made unique with a numeric
// suffix when taken.
func (s *Store) UpsertPoster(ctx context.Context, email, name, baseSlug string) (*Account, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	existing, err := scanAccount(tx.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE email = ?", email))
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	switch {
	case existing == nil:
		slug, err := uniqueSlug(ctx, tx, baseSlug, "accounts")
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO accounts (email, role, name, slug, created_at) VALUES (?, 'poster', ?, ?, ?)",
			email, name, slug, now()); err != nil {
			return nil, err
		}
	case existing.Role == RolePoster:
		// Already a poster: nothing to do. The public /p/{slug} URL never changes.
		return existing, tx.Commit()
	default:
		slug, err := uniqueSlug(ctx, tx, baseSlug, "accounts")
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE accounts SET role = 'poster', name = ?, slug = ? WHERE id = ?", name, slug, existing.ID); err != nil {
			return nil, err
		}
	}
	acct, err := scanAccount(tx.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE email = ?", email))
	if err != nil {
		return nil, err
	}
	return acct, tx.Commit()
}

// PosterBySlug returns the poster with the given slug.
func (s *Store) PosterBySlug(ctx context.Context, slug string) (*Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx,
		"SELECT "+accountCols+" FROM accounts WHERE slug = ? AND role = 'poster'", slug))
}

// DeleteAccount removes the account; sessions, interests and follows cascade.
func (s *Store) DeleteAccount(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", id)
	return err
}

// OTP is a one-time login code row.
type OTP struct {
	ID        int64
	Email     string
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
}

// CreateOTP stores a new code for email, superseding any pending code, and
// purges codes that expired more than a day ago.
func (s *Store) CreateOTP(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	n := now()
	if _, err := tx.ExecContext(ctx,
		"UPDATE otp_codes SET consumed_at = ? WHERE email = ? AND consumed_at IS NULL", n, email); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO otp_codes (email, code_hash, expires_at, created_at) VALUES (?, ?, ?, ?)",
		email, codeHash, expiresAt.Unix(), n); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM otp_codes WHERE expires_at < ?", n-86400); err != nil {
		return err
	}
	return tx.Commit()
}

// LatestOTP returns the newest unconsumed code for email.
func (s *Store) LatestOTP(ctx context.Context, email string) (*OTP, error) {
	var o OTP
	var exp int64
	err := s.db.QueryRowContext(ctx, `SELECT id, email, code_hash, attempts, expires_at FROM otp_codes
		WHERE email = ? AND consumed_at IS NULL ORDER BY created_at DESC, id DESC LIMIT 1`, email).
		Scan(&o.ID, &o.Email, &o.CodeHash, &o.Attempts, &exp)
	if err != nil {
		return nil, notFound(err)
	}
	o.ExpiresAt = toTime(exp)
	return &o, nil
}

// BumpOTPAttempts records a failed attempt and returns the new count.
func (s *Store) BumpOTPAttempts(ctx context.Context, id int64) (int, error) {
	var attempts int
	err := s.db.QueryRowContext(ctx,
		"UPDATE otp_codes SET attempts = attempts + 1 WHERE id = ? RETURNING attempts", id).Scan(&attempts)
	return attempts, notFound(err)
}

// ConsumeOTP marks a code as used.
func (s *Store) ConsumeOTP(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE otp_codes SET consumed_at = ? WHERE id = ?", now(), id)
	return err
}

// CountRecentOTPs counts codes issued to email since the given time.
func (s *Store) CountRecentOTPs(ctx context.Context, email string, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM otp_codes WHERE email = ? AND created_at >= ?", email, since.Unix()).Scan(&n)
	return n, err
}

// CreateSession stores a session token hash for the account.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, accountID int64, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, account_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		tokenHash, accountID, now(), expiresAt.Unix())
	return err
}

// AccountBySession resolves an unexpired session to its account.
func (s *Store) AccountBySession(ctx context.Context, tokenHash string) (*Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, `SELECT a.id, a.email, a.role, a.name, COALESCE(a.slug, ''), a.created_at
		FROM sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now()))
}

// DeleteSession removes a session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}
