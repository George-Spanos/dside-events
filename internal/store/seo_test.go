package store

import (
	"context"
	"testing"
	"time"
)

// seriesFixture creates a curator, a three-date series around now and one
// standalone event, and returns the series id with its slugs in date order.
func seriesFixture(t *testing.T, s *Store, starts []time.Time) (seriesID int64, slugs []string, solo string) {
	t.Helper()
	ctx := context.Background()
	poster, _, err := s.CreatePoster(ctx, "Maria P.", "maria-p")
	if err != nil {
		t.Fatal(err)
	}
	baseSlug := func(d time.Time) string { return "run-" + d.Format("2006-01-02") }
	in := EventInput{Title: "Run", Venue: "Stage", Tags: []string{"theater"}}
	if _, err := s.CreateSeries(ctx, poster.ID, starts, baseSlug, in); err != nil {
		t.Fatalf("create series: %v", err)
	}
	for _, d := range starts {
		slugs = append(slugs, baseSlug(d))
	}
	e, err := s.EventBySlug(ctx, slugs[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	solo, err = s.CreateEvent(ctx, poster.ID, "solo",
		EventInput{Title: "Solo", StartsAt: starts[0], Venue: "Bar"})
	if err != nil {
		t.Fatal(err)
	}
	return e.SeriesID, slugs, solo
}

func TestSeriesCanonicalSlug_PicksTheNextDate(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	now := time.Now().UTC()
	// One date passed, two still to come.
	starts := []time.Time{now.AddDate(0, 0, -7), now.AddDate(0, 0, 3), now.AddDate(0, 0, 10)}
	seriesID, slugs, _ := seriesFixture(t, s, starts)

	got, err := s.SeriesCanonicalSlug(ctx, seriesID, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != slugs[1] {
		t.Errorf("canonical slug is %q, want the next upcoming date %q", got, slugs[1])
	}
}

func TestSeriesCanonicalSlug_FallsBackToTheLastPastDate(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	now := time.Now().UTC()
	// The whole run has passed. Pointing at the earliest date would send
	// searchers to the stalest page of the three.
	starts := []time.Time{now.AddDate(0, 0, -30), now.AddDate(0, 0, -20), now.AddDate(0, 0, -10)}
	seriesID, slugs, _ := seriesFixture(t, s, starts)

	got, err := s.SeriesCanonicalSlug(ctx, seriesID, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != slugs[2] {
		t.Errorf("canonical slug is %q, want the last date %q", got, slugs[2])
	}
}

func TestSeriesCanonicalSlug_UnknownSeries(t *testing.T) {
	s := openTest(t)
	if _, err := s.SeriesCanonicalSlug(context.Background(), 999, time.Now()); err == nil {
		t.Error("SeriesCanonicalSlug(999) returned no error, want ErrNotFound")
	}
}

func TestIndexableEvents_OneRowPerSeries(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	now := time.Now().UTC()
	starts := []time.Time{now.AddDate(0, 0, 1), now.AddDate(0, 0, 8), now.AddDate(0, 0, 15)}
	_, slugs, solo := seriesFixture(t, s, starts)

	got, err := s.IndexableEvents(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, e := range got {
		if listed[e.Slug] {
			t.Errorf("%s listed twice", e.Slug)
		}
		listed[e.Slug] = true
		if e.UpdatedAt.IsZero() {
			t.Errorf("%s has no lastmod", e.Slug)
		}
	}
	// The standalone event and exactly one date of the run.
	if !listed[solo] {
		t.Errorf("standalone event %q is missing", solo)
	}
	if !listed[slugs[0]] {
		t.Errorf("the run's next date %q is missing", slugs[0])
	}
	for _, slug := range slugs[1:] {
		if listed[slug] {
			t.Errorf("%s points at %s but is still listed", slug, slugs[0])
		}
	}
	if len(got) != 2 {
		t.Errorf("listed %d events, want 2 (one standalone, one per series)", len(got))
	}
}

func TestPosterSlugs_ListsCuratorsWithoutTheirKeys(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	if _, _, err := s.CreatePoster(ctx, "Maria P.", "maria-p"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreatePoster(ctx, "Nikos K.", "nikos-k"); err != nil {
		t.Fatal(err)
	}
	// An anonymous account is not a curator and has no public page.
	if _, _, err := s.CreateUser(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.PosterSlugs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"maria-p", "nikos-k"}
	if len(got) != len(want) {
		t.Fatalf("PosterSlugs = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("PosterSlugs[%d] = %q, want %q", i, got[i], w)
		}
	}
}
