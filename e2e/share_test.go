package e2e

import (
	"strings"
	"testing"
)

// spec: EventDetail
func TestEventShare_CopiesTheEventURL(t *testing.T) {
	p := asPoster(t, poster1)
	slug := createEvent(t, p, validEvent(t, tomorrow()))

	a := anon(t)
	r := a.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, `data-copy="`+a.s.url+"/e/"+slug+`"`)
	assertContains(t, r, `>Share</button>`)

	// The clipboard has no form behind it, so the control ships hidden and
	// only script that finds a clipboard reveals it.
	if !strings.Contains(r.Body, "<span data-share hidden>") {
		t.Errorf("the share control is not hidden for a visitor without script\n%s", snippet(r.Body))
	}
}

// spec: EventDetail
func TestEventShare_NotOnPastEvents(t *testing.T) {
	p := asPoster(t, poster1)
	slug := createEvent(t, p, validEvent(t, yesterday()))

	r := anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, "This event has passed.")
	assertNotContains(t, r, "data-copy")
}

// A date of a series hands its canonical claim to the date that represents
// the run; Share still copies the page the visitor is on.
// spec: EventDetail, CreateSeries
func TestEventShare_SeriesCopiesThisDate(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	_, slugs := createSeries(t, p, f)
	if len(slugs) < 2 {
		t.Fatalf("expected several dates, got %v", slugs)
	}
	a := anon(t)
	for _, slug := range slugs {
		r := a.get("/e/" + slug)
		assertStatus(t, r, 200)
		assertContains(t, r, `data-copy="`+a.s.url+"/e/"+slug+`"`)
	}
}
