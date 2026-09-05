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

func TestEventsAndInterests(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	poster, err := s.UpsertPoster(ctx, "p@example.com", "Maria P.", "maria-p")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.UpsertPoster(ctx, "p@example.com", "Maria P.", "maria-p")
	if err != nil || again.ID != poster.ID || again.Slug != "maria-p" {
		t.Fatalf("upsert not idempotent: %v %+v", err, again)
	}
	// Re-running for an existing poster is a no-op: name and slug are kept
	// even when a different name and an explicit slug are passed.
	same, err := s.UpsertPoster(ctx, "p@example.com", "Renamed", "renamed")
	if err != nil || same.ID != poster.ID || same.Name != "Maria P." || same.Slug != "maria-p" {
		t.Fatalf("re-run changed an existing poster: %v %+v", err, same)
	}
	if _, err := s.PosterBySlug(ctx, "renamed"); err != ErrNotFound {
		t.Fatalf("re-run created slug %q: %v", "renamed", err)
	}
	other, err := s.UpsertPoster(ctx, "q@example.com", "Maria P.", "maria-p")
	if err != nil || other.Slug != "maria-p-2" {
		t.Fatalf("slug suffix: %v %+v", err, other)
	}
	u1, _ := s.EnsureUser(ctx, "a@example.com")
	u2, _ := s.EnsureUser(ctx, "b@example.com")
	if u1.Role != RoleUser {
		t.Fatalf("role = %q", u1.Role)
	}
	// Promoting a user sets name and slug.
	promoted, err := s.UpsertPoster(ctx, "a@example.com", "A", "a")
	if err != nil || promoted.Role != RolePoster || promoted.ID != u1.ID || promoted.Name != "A" || promoted.Slug != "a" {
		t.Fatalf("promote: %v %+v", err, promoted)
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
	if e.Interested != 0 || len(e.Tags) != 2 || e.Tags[0] != "concert" || len(e.Links) != 1 || e.PosterSlug != "maria-p" {
		t.Fatalf("event: %+v", e)
	}
	if !e.StartsAt.Equal(start) {
		t.Fatalf("starts_at %v != %v", e.StartsAt, start)
	}

	if err := s.SetInterest(ctx, u1.ID, e.ID, Interested); err != nil {
		t.Fatal(err)
	}
	if err := s.SetInterest(ctx, u1.ID, e.ID, Interested); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := s.SetInterest(ctx, u2.ID, e.ID, NotInterested); err != nil {
		t.Fatal(err)
	}
	e, _ = s.EventBySlug(ctx, slug, u2.ID)
	if e.Interested != 1 || e.ViewerState != NotInterested {
		t.Fatalf("counter/state: %+v", e)
	}
	feed, err := s.Feed(ctx, FeedOpts{From: time.Now(), ViewerID: u2.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range feed {
		if f.Slug == slug {
			t.Fatal("not_interested event still in viewer feed")
		}
	}
	feed, _ = s.Feed(ctx, FeedOpts{From: time.Now(), ViewerID: 0, Tag: "concert"})
	if len(feed) != 2 {
		t.Fatalf("anon tag feed len = %d", len(feed))
	}
	mine, _ := s.InterestedEvents(ctx, u1.ID)
	hidden, _ := s.HiddenEvents(ctx, u2.ID)
	if len(mine) != 1 || len(hidden) != 1 {
		t.Fatalf("mine=%d hidden=%d", len(mine), len(hidden))
	}
	if err := s.ClearInterest(ctx, u2.ID, e.ID); err != nil {
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
	mine, _ = s.InterestedEvents(ctx, u1.ID)
	if len(mine) != 0 {
		t.Fatal("interest did not cascade on event delete")
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
	p1, _ := s.UpsertPoster(ctx, "p1@example.com", "One", "one")
	p2, _ := s.UpsertPoster(ctx, "p2@example.com", "Two", "two")
	u, _ := s.EnsureUser(ctx, "u@example.com")
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

func TestOTPAndSessions(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	exp := time.Now().Add(10 * time.Minute)
	if _, err := s.LatestOTP(ctx, "x@example.com"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	s.CreateOTP(ctx, "x@example.com", "h1", exp)
	s.CreateOTP(ctx, "x@example.com", "h2", exp)
	o, err := s.LatestOTP(ctx, "x@example.com")
	if err != nil || o.CodeHash != "h2" {
		t.Fatalf("supersede: %v %+v", err, o)
	}
	if n, _ := s.CountRecentOTPs(ctx, "x@example.com", time.Now().Add(-15*time.Minute)); n != 2 {
		t.Fatalf("recent = %d", n)
	}
	if a, _ := s.BumpOTPAttempts(ctx, o.ID); a != 1 {
		t.Fatalf("attempts = %d", a)
	}
	s.ConsumeOTP(ctx, o.ID)
	if _, err := s.LatestOTP(ctx, "x@example.com"); err != ErrNotFound {
		t.Fatal("consumed code still returned")
	}

	u, _ := s.EnsureUser(ctx, "x@example.com")
	s.CreateSession(ctx, "tok", u.ID, time.Now().Add(time.Hour))
	a, err := s.AccountBySession(ctx, "tok")
	if err != nil || a.ID != u.ID {
		t.Fatalf("session: %v", err)
	}
	s.CreateSession(ctx, "old", u.ID, time.Now().Add(-time.Hour))
	if _, err := s.AccountBySession(ctx, "old"); err != ErrNotFound {
		t.Fatal("expired session accepted")
	}
	s.DeleteAccount(ctx, u.ID)
	if _, err := s.AccountBySession(ctx, "tok"); err != ErrNotFound {
		t.Fatal("session survived account delete")
	}
}
