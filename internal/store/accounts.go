package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"
)

// Roles.
const (
	RoleUser   = "user"
	RolePoster = "poster"
)

// KeyLength is the size in bytes of an account key (the secret link).
const KeyLength = 32

// Account is a row of the accounts table. The key is deliberately not part
// of it: only CreateUser, CreatePoster, RotateKey and AccountKey hand it out.
type Account struct {
	ID        int64
	Role      string
	Name      string
	Slug      string
	CreatedAt time.Time
}

// IsPoster reports whether the account may post events.
func (a *Account) IsPoster() bool { return a != nil && a.Role == RolePoster }

const accountCols = "id, role, name, COALESCE(slug, ''), created_at"

func scanAccount(row interface{ Scan(...any) error }) (*Account, error) {
	var a Account
	var created int64
	if err := row.Scan(&a.ID, &a.Role, &a.Name, &a.Slug, &created); err != nil {
		return nil, notFound(err)
	}
	a.CreatedAt = toTime(created)
	return &a, nil
}

// newKey returns KeyLength random bytes as base64url (43 characters).
func newKey() (string, error) {
	b := make([]byte, KeyLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// AccountByID returns one account.
func (s *Store) AccountByID(ctx context.Context, id int64) (*Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE id = ?", id))
}

// AccountByKey resolves a secret key to its account.
func (s *Store) AccountByKey(ctx context.Context, key string) (*Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE key = ?", key))
}

// AccountKey returns the account's current secret key.
func (s *Store) AccountKey(ctx context.Context, id int64) (string, error) {
	var key string
	err := s.db.QueryRowContext(ctx, "SELECT key FROM accounts WHERE id = ?", id).Scan(&key)
	return key, notFound(err)
}

// CreateUser creates an anonymous account with role user and returns it with
// its fresh key.
func (s *Store) CreateUser(ctx context.Context) (*Account, string, error) {
	key, err := newKey()
	if err != nil {
		return nil, "", err
	}
	res, err := s.db.ExecContext(ctx, "INSERT INTO accounts (role, key, created_at) VALUES ('user', ?, ?)", key, now())
	if err != nil {
		return nil, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, "", err
	}
	a, err := s.AccountByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return a, key, nil
}

// CreatePoster creates a poster account with the given public name and slug
// and returns it with its fresh key. When the slug is already taken nothing
// changes: the existing account is returned with an empty key (use RotateKey
// to get a new link for it).
func (s *Store) CreatePoster(ctx context.Context, name, slug string) (*Account, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	existing, err := scanAccount(tx.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE slug = ?", slug))
	if err != nil && err != ErrNotFound {
		return nil, "", err
	}
	if existing != nil {
		return existing, "", tx.Commit()
	}
	key, err := newKey()
	if err != nil {
		return nil, "", err
	}
	res, err := tx.ExecContext(ctx,
		"INSERT INTO accounts (role, name, slug, key, created_at) VALUES ('poster', ?, ?, ?, ?)", name, slug, key, now())
	if err != nil {
		return nil, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, "", err
	}
	a, err := scanAccount(tx.QueryRowContext(ctx, "SELECT "+accountCols+" FROM accounts WHERE id = ?", id))
	if err != nil {
		return nil, "", err
	}
	return a, key, tx.Commit()
}

// RotateKey replaces the account's secret key and returns the new one. The
// old link stops working; existing sessions are untouched.
func (s *Store) RotateKey(ctx context.Context, id int64) (string, error) {
	key, err := newKey()
	if err != nil {
		return "", err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE accounts SET key = ? WHERE id = ?", key, id)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrNotFound
	}
	return key, nil
}

// PosterLink is one curator with the secret key that logs them in.
type PosterLink struct {
	Name, Slug, Key string
}

// Posters lists every curator with their current secret key, by name.
func (s *Store) Posters(ctx context.Context) ([]PosterLink, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT name, COALESCE(slug, ''), key FROM accounts WHERE role = 'poster' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PosterLink
	for rows.Next() {
		var pl PosterLink
		if err := rows.Scan(&pl.Name, &pl.Slug, &pl.Key); err != nil {
			return nil, err
		}
		out = append(out, pl)
	}
	return out, rows.Err()
}

// PosterBySlug returns the poster with the given slug.
func (s *Store) PosterBySlug(ctx context.Context, slug string) (*Account, error) {
	return scanAccount(s.db.QueryRowContext(ctx,
		"SELECT "+accountCols+" FROM accounts WHERE slug = ? AND role = 'poster'", slug))
}

// DeleteAccount removes the account; sessions and event follows cascade.
func (s *Store) DeleteAccount(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", id)
	return err
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
	return scanAccount(s.db.QueryRowContext(ctx, `SELECT a.id, a.role, a.name, COALESCE(a.slug, ''), a.created_at
		FROM sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, now()))
}

// DeleteSession removes a session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}
