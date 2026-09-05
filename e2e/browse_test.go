package e2e

import (
	"net/url"
	"strings"
	"testing"
)

// spec: PublicFeed, Event.is_upcoming
func TestFeed_AnonSeesUpcomingOnly_SortedAscending(t *testing.T) {
	p := asPoster(t, poster1)
	a := validEvent(t, tomorrow())
	a.Title = uniqTitle(t, "Feed A")
	b := validEvent(t, daysFromNow(2))
	b.Title = uniqTitle(t, "Feed B")
	c := validEvent(t, yesterday())
	c.Title = uniqTitle(t, "Feed C")
	// Create out of order so ordering cannot come from insertion.
	createEvent(t, p, b)
	createEvent(t, p, c)
	createEvent(t, p, a)

	r := anon(t).get("/")
	assertStatus(t, r, 200)
	assertContains(t, r, a.Title)
	assertContains(t, r, b.Title)
	assertNotContains(t, r, c.Title)
	assertBefore(t, r, a.Title, b.Title)
}

// spec: PublicFeed, EventDetail
func TestFeed_EventLinksToDetail(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)

	r := anon(t).get("/")
	assertStatus(t, r, 200)
	assertContains(t, r, `href="/e/`+slug+`"`)
}

// spec: PublicFeed
func TestFeed_TagFilter(t *testing.T) {
	p := asPoster(t, poster1)
	th := validEvent(t, tomorrow())
	th.Title = uniqTitle(t, "Play")
	th.Tags = []string{"theater"}
	co := validEvent(t, tomorrow())
	co.Title = uniqTitle(t, "Gig")
	co.Tags = []string{"concert"}
	createEvent(t, p, th)
	createEvent(t, p, co)

	v := anon(t)
	r := v.get("/?tag=theater")
	assertStatus(t, r, 200)
	assertContains(t, r, th.Title)
	assertNotContains(t, r, co.Title)

	r = v.get("/?tag=concert")
	assertStatus(t, r, 200)
	assertContains(t, r, co.Title)
	assertNotContains(t, r, th.Title)

	r = v.get("/")
	assertStatus(t, r, 200)
	for _, tag := range []string{"concert", "theater", "film", "exhibition", "talk", "party", "dance", "workshop"} {
		assertContains(t, r, `href="/?tag=`+tag+`"`)
	}

	r = v.get("/?tag=bogus")
	assertStatus(t, r, 404)
}

// spec: EventDetail
func TestEventDetail_ShowsAllPublicFields(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Tags = []string{"concert", "talk"}
	f.Price = "12 EUR"
	f.Venue = "Technopolis"
	f.Description = "An evening of improvised sound. Arrive early."
	f.LinkLabels = []string{"Listen", ""}
	f.LinkURLs = []string{"https://example.test/listen/" + uid(), "https://music.example.test/" + uid()}
	slug := createEvent(t, p, f)

	r := anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertContains(t, r, f.Venue)
	assertContains(t, r, f.Price)
	assertContains(t, r, f.Description)
	assertContains(t, r, f.Time)
	assertContains(t, r, `href="/?tag=concert"`)
	assertContains(t, r, `href="/?tag=talk"`)
	assertContains(t, r, `href="`+f.LinkURLs[0]+`"`)
	assertContains(t, r, `href="`+f.LinkURLs[1]+`"`)
	assertContains(t, r, "Listen")
	// A link without a label is labelled with its host.
	assertContains(t, r, "music.example.test")
	assertContains(t, r, poster1.Name)
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)
	assertContains(t, r, "Nobody yet interested")
	assertHeaderContains(t, r, "Cache-Control", "no-store")
}

// spec: EventDetail
func TestEventDetail_Unknown404(t *testing.T) {
	r := anon(t).get("/e/does-not-exist-2030-01-01")
	assertStatus(t, r, 404)
	assertContains(t, r, "Page not found")
}

// spec: EventDetail, PublicFeed, Event.is_upcoming
func TestEventDetail_PastEventStillReachable(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, yesterday())
	slug := createEvent(t, p, f)

	v := anon(t)
	r := v.get("/")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)

	r = v.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertContains(t, r, "This event has passed.")
}

// spec: PosterPage
func TestPosterPage_NameAndOwnEventsOnly(t *testing.T) {
	p1 := asPoster(t, poster1)
	p2 := asPoster(t, poster2)
	up := validEvent(t, tomorrow())
	up.Title = uniqTitle(t, "Mine Upcoming")
	past := validEvent(t, yesterday())
	past.Title = uniqTitle(t, "Mine Past")
	other := validEvent(t, tomorrow())
	other.Title = uniqTitle(t, "Theirs")
	createEvent(t, p1, up)
	createEvent(t, p1, past)
	createEvent(t, p2, other)

	r := anon(t).get("/p/" + poster1.Slug)
	assertStatus(t, r, 200)
	assertContains(t, r, poster1.Name)
	assertContains(t, r, up.Title)
	assertContains(t, r, past.Title)
	assertNotContains(t, r, other.Title)
	// Upcoming list first, then Past.
	assertBefore(t, r, up.Title, past.Title)
	// A visitor sees no follow control.
	assertNoForm(t, r, `action="/follow"`)
}

// spec: PosterPage
func TestPosterPage_Unknown404(t *testing.T) {
	v := anon(t)
	assertStatus(t, v.get("/p/nobody-here"), 404)

	// An event slug is not a poster slug.
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	assertStatus(t, v.get("/p/"+slug), 404)
}

// spec: EventDetail, PosterPage, Login
func TestAnon_NoFollowOrInterestControls(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))

	v := anon(t)
	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertNotContains(t, r, `action="/e/`+slug+`/interest"`)
	assertNotContains(t, r, `action="/follow"`)
	assertNotContains(t, r, `href="/e/`+slug+`/edit"`)
	assertNotContains(t, r, `action="/e/`+slug+`/delete"`)
	if !hasLoginLink(r.Body, "/e/"+slug) {
		t.Errorf("anon event page lacks a link to /login?next=/e/%s\nbody: %s", slug, snippet(r.Body))
	}

	r = v.get("/p/" + poster1.Slug)
	assertStatus(t, r, 200)
	assertNotContains(t, r, `action="/follow"`)

	r = v.get("/")
	assertStatus(t, r, 200)
	assertNotContains(t, r, `action="/follow"`)
}

// spec: Feed, MyEvents, AccountPage, EventComposer, EventEditor, Login
func TestAnon_ProtectedRoutesRedirectToLogin(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	v := anon(t)

	for _, path := range []string{"/mine", "/account", "/new", "/following", "/e/" + slug + "/edit"} {
		r := v.get(path)
		assertLoginRedirect(t, r, path)
		if v.cookie("session") != nil {
			t.Errorf("GET %s set a session cookie for an anonymous visitor", path)
		}
	}

	posts := []struct {
		path string
		form url.Values
	}{
		{"/e/" + slug + "/interest", url.Values{"state": {"interested"}}},
		{"/follow", url.Values{"kind": {"tag"}, "key": {"concert"}, "on": {"1"}}},
		{"/account/delete", url.Values{"confirm": {"1"}}},
		{"/logout", url.Values{}},
		{"/new", validEvent(t, tomorrow()).values()},
		{"/e/" + slug + "/edit", validEvent(t, tomorrow()).values()},
		{"/e/" + slug + "/delete", url.Values{}},
	}
	for _, tc := range posts {
		r := v.postForm(tc.path, tc.form)
		if r.Status != 303 || !strings.HasPrefix(r.Location, "/login") {
			t.Errorf("anon POST %s: got %d → %q, want 303 → /login…\nbody: %s", tc.path, r.Status, r.Location, snippet(r.Body))
		}
	}
	// Nothing leaked through: the event is untouched and still public.
	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, "Nobody yet interested")
}
