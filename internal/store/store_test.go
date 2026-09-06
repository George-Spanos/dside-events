package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	// Migrations must be idempotent: reopening applies nothing new.
	s2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2.Close()
	return s
}

func TestEventsAndFollows(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	poster, _, err := s.CreatePoster(ctx, "Maria P.", "maria-p")
	if err != nil {
		t.Fatal(err)
	}
	u1, _, _ := s.CreateUser(ctx)
	u2, _, _ := s.CreateUser(ctx)
	if u1.Role != RoleUser || u1.ID == u2.ID {
		t.Fatalf("users: %+v %+v", u1, u2)
	}

	start := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	in := EventInput{Title: "Συναυλία", StartsAt: start, Venue: "Gagarin", Tags: []string{"concert", "party"},
		Links: []Link{{Label: "Tickets", URL: "https://example.com/t"}}}
	slug, err := s.CreateEvent(ctx, poster.ID, "synavlia-2026-09-07", in)
	if err != nil || slug != "synavlia-2026-09-07" {
		t.Fatalf("create: %v %q", err, slug)
	}
	slug2, err := s.CreateEvent(ctx, poster.ID, "synavlia-2026-09-07", in)
	if err != nil || slug2 != "synavlia-2026-09-07-2" {
		t.Fatalf("create dup slug: %v %q", err, slug2)
	}
	e, err := s.EventBySlug(ctx, slug, 0)
	if err != nil {
		t.Fatal(err)
	}
	if e.Followers != 0 || len(e.Tags) != 2 || e.Tags[0] != "concert" || len(e.Links) != 1 || e.PosterSlug != "maria-p" {
		t.Fatalf("event: %+v", e)
	}
	if !e.StartsAt.Equal(start) {
		t.Fatalf("starts_at %v != %v", e.StartsAt, start)
	}

	if err := s.SetEventFollow(ctx, u1.ID, e.ID, Following); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEventFollow(ctx, u1.ID, e.ID, Following); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := s.SetEventFollow(ctx, u2.ID, e.ID, Hidden); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEventFollow(ctx, u2.ID, e.ID, "maybe"); err == nil {
		t.Fatal("unknown state accepted by the CHECK constraint")
	}
	e, _ = s.EventBySlug(ctx, slug, u2.ID)
	if e.Followers != 1 || e.ViewerState != Hidden {
		t.Fatalf("counter/state: %+v", e)
	}
	feed, err := s.Feed(ctx, FeedOpts{From: time.Now(), ViewerID: u2.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range feed {
		if f.Slug == slug {
			t.Fatal("hidden event still in viewer feed")
		}
	}
	feed, _ = s.Feed(ctx, FeedOpts{From: time.Now(), ViewerID: 0, Tag: "concert"})
	if len(feed) != 2 {
		t.Fatalf("anon tag feed len = %d", len(feed))
	}
	mine, _ := s.FollowedEvents(ctx, u1.ID)
	hidden, _ := s.HiddenEvents(ctx, u2.ID)
	if len(mine) != 1 || len(hidden) != 1 {
		t.Fatalf("mine=%d hidden=%d", len(mine), len(hidden))
	}
	if err := s.ClearEventFollow(ctx, u2.ID, e.ID); err != nil {
		t.Fatal(err)
	}
	hidden, _ = s.HiddenEvents(ctx, u2.ID)
	if len(hidden) != 0 {
		t.Fatal("clear did not remove hidden")
	}

	in.Title = "Edited"
	in.Tags = []string{"theater"}
	in.Links = nil
	if err := s.UpdateEvent(ctx, e.ID, in); err != nil {
		t.Fatal(err)
	}
	e, _ = s.EventBySlug(ctx, slug, 0)
	if e.Title != "Edited" || len(e.Tags) != 1 || e.Tags[0] != "theater" || len(e.Links) != 0 {
		t.Fatalf("update: %+v", e)
	}
	if err := s.DeleteEvent(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EventBySlug(ctx, slug, 0); err != ErrNotFound {
		t.Fatalf("after delete: %v", err)
	}
	mine, _ = s.FollowedEvents(ctx, u1.ID)
	if len(mine) != 0 {
		t.Fatal("event follow did not cascade on event delete")
	}
	// The deleted slug is retired: the same base skips both it and the live -2.
	slug3, err := s.CreateEvent(ctx, poster.ID, "synavlia-2026-09-07", in)
	if err != nil || slug3 != "synavlia-2026-09-07-3" {
		t.Fatalf("slug after delete: %v %q (deleted slug must not be reused)", err, slug3)
	}
	if err := s.DeleteEvent(ctx, e.ID); err != ErrNotFound {
		t.Fatalf("delete twice: %v, want ErrNotFound", err)
	}
}

func TestFollowsAndFollowingFeed(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	p1, _, _ := s.CreatePoster(ctx, "One", "one")
	p2, _, _ := s.CreatePoster(ctx, "Two", "two")
	u, _, _ := s.CreateUser(ctx)
	start := time.Now().Add(24 * time.Hour)
	s.CreateEvent(ctx, p1.ID, "a", EventInput{Title: "A", StartsAt: start, Venue: "v", Tags: []string{"film"}})
	s.CreateEvent(ctx, p2.ID, "b", EventInput{Title: "B", StartsAt: start, Venue: "v", Tags: []string{"talk"}})
	s.CreateEvent(ctx, p2.ID, "c", EventInput{Title: "C", StartsAt: start, Venue: "v", Tags: []string{"film"}})

	following, _ := s.Feed(ctx, FeedOpts{From: time.Now(), ViewerID: u.ID, FollowingOnly: true})
	if len(following) != 0 {
		t.Fatalf("nothing followed but %d events", len(following))
	}
	if err := s.FollowTag(ctx, u.ID, "film"); err != nil {
		t.Fatal(err)
	}
	if err := s.FollowTag(ctx, u.ID, "film"); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := s.FollowPoster(ctx, u.ID, p2.ID); err != nil {
		t.Fatal(err)
	}
	f, err := s.Follows(ctx, u.ID)
	if err != nil || !f.HasTag("film") || !f.HasPoster(p2.ID) || f.HasPoster(p1.ID) {
		t.Fatalf("follows: %v %+v", err, f)
	}
	following, _ = s.Feed(ctx, FeedOpts{From: time.Now(), ViewerID: u.ID, FollowingOnly: true})
	if len(following) != 3 { // A by tag, B by poster, C by both (no duplicates)
		t.Fatalf("following feed len = %d", len(following))
	}
	s.UnfollowTag(ctx, u.ID, "film")
	s.UnfollowPoster(ctx, u.ID, p2.ID)
	f, _ = s.Follows(ctx, u.ID)
	if f.Any() {
		t.Fatalf("still following: %+v", f)
	}
}

func TestKeysAndPosters(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	u, key, err := s.CreateUser(ctx)
	if err != nil || len(key) != 43 {
		t.Fatalf("create user: %v key=%q", err, key)
	}
	got, err := s.AccountKey(ctx, u.ID)
	if err != nil || got != key {
		t.Fatalf("AccountKey: %v %q != %q", err, got, key)
	}
	a, err := s.AccountByKey(ctx, key)
	if err != nil || a.ID != u.ID || a.Role != RoleUser {
		t.Fatalf("AccountByKey: %v %+v", err, a)
	}
	if _, err := s.AccountByKey(ctx, "nope"); err != ErrNotFound {
		t.Fatalf("unknown key: %v", err)
	}
	rotated, err := s.RotateKey(ctx, u.ID)
	if err != nil || rotated == key || len(rotated) != 43 {
		t.Fatalf("rotate: %v %q", err, rotated)
	}
	if _, err := s.AccountByKey(ctx, key); err != ErrNotFound {
		t.Fatal("old key still opens the account")
	}
	if a, err := s.AccountByKey(ctx, rotated); err != nil || a.ID != u.ID {
		t.Fatalf("new key: %v", err)
	}
	if _, err := s.RotateKey(ctx, 9999); err != ErrNotFound {
		t.Fatalf("rotate unknown: %v", err)
	}

	p, pkey, err := s.CreatePoster(ctx, "Maria P.", "maria-p")
	if err != nil || p.Role != RolePoster || p.Name != "Maria P." || p.Slug != "maria-p" || len(pkey) != 43 {
		t.Fatalf("create poster: %v %+v %q", err, p, pkey)
	}
	// Re-running for an existing slug is a no-op: same account, nothing
	// changed, no key handed out.
	again, akey, err := s.CreatePoster(ctx, "Renamed", "maria-p")
	if err != nil || again.ID != p.ID || again.Name != "Maria P." || akey != "" {
		t.Fatalf("re-run: %v %+v %q", err, again, akey)
	}
	if a, err := s.AccountByKey(ctx, pkey); err != nil || a.ID != p.ID {
		t.Fatalf("poster key changed by re-run: %v", err)
	}
	if pb, err := s.PosterBySlug(ctx, "maria-p"); err != nil || pb.ID != p.ID {
		t.Fatalf("PosterBySlug: %v", err)
	}
	if _, err := s.PosterBySlug(ctx, "nobody"); err != ErrNotFound {
		t.Fatalf("unknown poster: %v", err)
	}
	// A user's slug is NULL, so many users never collide on the UNIQUE slug.
	if _, _, err := s.CreateUser(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	u, key, _ := s.CreateUser(ctx)
	s.CreateSession(ctx, "tok", u.ID, time.Now().Add(time.Hour))
	a, err := s.AccountBySession(ctx, "tok")
	if err != nil || a.ID != u.ID {
		t.Fatalf("session: %v", err)
	}
	s.CreateSession(ctx, "old", u.ID, time.Now().Add(-time.Hour))
	if _, err := s.AccountBySession(ctx, "old"); err != ErrNotFound {
		t.Fatal("expired session accepted")
	}
	s.DeleteSession(ctx, "tok")
	if _, err := s.AccountBySession(ctx, "tok"); err != ErrNotFound {
		t.Fatal("deleted session accepted")
	}
	s.CreateSession(ctx, "tok2", u.ID, time.Now().Add(time.Hour))
	s.FollowTag(ctx, u.ID, "film")
	s.DeleteAccount(ctx, u.ID)
	if _, err := s.AccountBySession(ctx, "tok2"); err != ErrNotFound {
		t.Fatal("session survived account delete")
	}
	if _, err := s.AccountByKey(ctx, key); err != ErrNotFound {
		t.Fatal("key survived account delete")
	}
	if f, err := s.Follows(ctx, u.ID); err != nil || f.Any() {
		t.Fatalf("follows survived account delete: %v %+v", err, f)
	}
}
