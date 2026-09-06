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
	mine, _ := s.FollowedEvents(ctx, u1.ID, "")
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
	mine, _ = s.FollowedEvents(ctx, u1.ID, "")
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

func TestCreateSeries(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	poster, _, err := s.CreatePoster(ctx, "Maria P.", "maria-p")
	if err != nil {
		t.Fatal(err)
	}
	u1, _, _ := s.CreateUser(ctx)
	u2, _, _ := s.CreateUser(ctx)

	// Three dates, two of them on the same day: 18:00 and 21:00 the day
	// after tomorrow, 20:00 two days later.
	day := time.Now().UTC().Add(48 * time.Hour)
	d1 := time.Date(day.Year(), day.Month(), day.Day(), 18, 0, 0, 0, time.UTC)
	starts := []time.Time{d1, d1.Add(3 * time.Hour), d1.AddDate(0, 0, 2).Add(2 * time.Hour)}
	baseSlug := func(t time.Time) string { return "show-" + t.Format("2006-01-02") }
	in := EventInput{Title: "Show", Venue: "Stage", Tags: []string{"theater"},
		Links: []Link{{Label: "Tickets", URL: "https://example.com/t"}}}
	slug, err := s.CreateSeries(ctx, poster.ID, starts, baseSlug, in)
	if err != nil || slug != baseSlug(d1) {
		t.Fatalf("create series: %v %q", err, slug)
	}
	want := []string{baseSlug(d1), baseSlug(d1) + "-2", baseSlug(starts[2])}
	var members []*Event
	for i, w := range want {
		e, err := s.EventBySlug(ctx, w, 0)
		if err != nil {
			t.Fatalf("%s: %v", w, err)
		}
		if !e.StartsAt.Equal(starts[i]) || len(e.Tags) != 1 || len(e.Links) != 1 {
			t.Fatalf("%s: %+v", w, e)
		}
		members = append(members, e)
	}
	seriesID := members[0].SeriesID
	for _, e := range members {
		if seriesID == 0 || e.SeriesID != seriesID || e.SeriesCount != 3 {
			t.Fatalf("series fields: %+v", e)
		}
	}
	standaloneSlug, err := s.CreateEvent(ctx, poster.ID, "solo", EventInput{Title: "Solo", StartsAt: d1, Venue: "Bar"})
	if err != nil {
		t.Fatal(err)
	}
	solo, _ := s.EventBySlug(ctx, standaloneSlug, 0)
	if solo.SeriesID != 0 || solo.SeriesCount != 0 {
		t.Fatalf("standalone: %+v", solo)
	}
	// A second series mints its own id.
	other := func(t time.Time) string { return "other-" + t.Format("2006-01-02") }
	otherSlug, err := s.CreateSeries(ctx, poster.ID, starts[:2], other, EventInput{Title: "Other", Venue: "Bar"})
	if err != nil {
		t.Fatal(err)
	}
	if o, _ := s.EventBySlug(ctx, otherSlug, 0); o.SeriesID == seriesID || o.SeriesID == 0 || o.SeriesCount != 2 {
		t.Fatalf("second series: %+v", o)
	}

	// Following one date follows the whole series and nothing else.
	if err := s.SetEventFollow(ctx, u1.ID, members[2].ID, Following); err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		e, _ := s.EventBySlug(ctx, w, u1.ID)
		if e.ViewerState != Following || e.Followers != 1 {
			t.Fatalf("%s after follow: %+v", w, e)
		}
	}
	if mine, _ := s.FollowedEvents(ctx, u1.ID, ""); len(mine) != 3 {
		t.Fatalf("followed = %d, want 3", len(mine))
	}
	if solo, _ = s.EventBySlug(ctx, standaloneSlug, u1.ID); solo.Followers != 0 || solo.ViewerState != "" {
		t.Fatalf("standalone touched by series follow: %+v", solo)
	}
	if o, _ := s.EventBySlug(ctx, otherSlug, u1.ID); o.Followers != 0 {
		t.Fatalf("other series touched: %+v", o)
	}

	// Hiding one date hides the whole series for that viewer.
	if err := s.SetEventFollow(ctx, u2.ID, members[0].ID, Hidden); err != nil {
		t.Fatal(err)
	}
	feed, err := s.Feed(ctx, FeedOpts{From: time.Unix(0, 0), ViewerID: u2.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range feed {
		if f.SeriesID == seriesID {
			t.Fatalf("hidden series date still in feed: %s", f.Slug)
		}
	}
	if hidden, _ := s.HiddenEvents(ctx, u2.ID); len(hidden) != 3 {
		t.Fatalf("hidden = %d, want 3", len(hidden))
	}
	if err := s.ClearEventFollow(ctx, u2.ID, members[2].ID); err != nil {
		t.Fatal(err)
	}
	if hidden, _ := s.HiddenEvents(ctx, u2.ID); len(hidden) != 0 {
		t.Fatalf("hidden after clear = %d, want 0", len(hidden))
	}

	// Deleting one date leaves the others, which now count two.
	if err := s.DeleteEvent(ctx, members[1].ID); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{want[0], want[2]} {
		e, err := s.EventBySlug(ctx, w, 0)
		if err != nil || e.SeriesID != seriesID || e.SeriesCount != 2 {
			t.Fatalf("%s after delete: %v %+v", w, err, e)
		}
	}
	if mine, _ := s.FollowedEvents(ctx, u1.ID, ""); len(mine) != 2 {
		t.Fatalf("followed after delete = %d, want 2", len(mine))
	}
	// Following the standalone leaves the series untouched.
	if err := s.SetEventFollow(ctx, u2.ID, solo.ID, Following); err != nil {
		t.Fatal(err)
	}
	if mine, _ := s.FollowedEvents(ctx, u2.ID, ""); len(mine) != 1 || mine[0].ID != solo.ID {
		t.Fatalf("standalone follow: %d rows", len(mine))
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
	s.DeleteAccount(ctx, u.ID)
	if _, err := s.AccountBySession(ctx, "tok2"); err != ErrNotFound {
		t.Fatal("session survived account delete")
	}
	if _, err := s.AccountByKey(ctx, key); err != ErrNotFound {
		t.Fatal("key survived account delete")
	}
}
