package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// A database created by the first production image (schema 1, table
// interests) must come out of Open with follows_events and its rows converted.
func TestMigrateFromSchema1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	init1, err := migrations.ReadFile("migrations/001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"CREATE TABLE schema_version (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)",
		string(init1),
		"INSERT INTO schema_version (version, applied_at) VALUES (1, 0)",
		"INSERT INTO accounts (id, role, name, slug, key, created_at) VALUES (1, 'poster', 'P', 'p', 'k1', 0), (2, 'user', '', NULL, 'k2', 0)",
		"INSERT INTO events (id, slug, poster_id, title, starts_at, venue, created_at, updated_at) VALUES (1, 'e-1', 1, 'E', 0, 'V', 0, 0)",
		"INSERT INTO interests (account_id, event_id, state, created_at) VALUES (1, 1, 'interested', 0), (2, 1, 'not_interested', 0)",
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("%s: %v", q[:40], err)
		}
	}
	raw.Close()

	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	var following, hidden int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM follows_events WHERE state = 'following'").Scan(&following); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM follows_events WHERE state = 'hidden'").Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if following != 1 || hidden != 1 {
		t.Fatalf("converted rows: following=%d hidden=%d, want 1/1", following, hidden)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name = 'interests'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("interests table still present (%d, %v)", n, err)
	}
	var v int
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&v); err != nil || v != 2 {
		t.Fatalf("schema_version = %d (%v), want 2", v, err)
	}
}
