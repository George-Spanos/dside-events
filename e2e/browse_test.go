package e2e

import (
	"net/url"
	"testing"
)

// spec: UpcomingAll, Event.is_upcoming, Visitor
func TestUpcoming_AnonSeesUpcomingOnly_SortedAscending(t *testing.T) {
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

	r := anon(t).get("/upcoming")
	assertStatus(t, r, 200)
	assertContains(t, r, a.Title)
	assertContains(t, r, b.Title)
	assertNotContains(t, r, c.Title)
	assertBefore(t, r, a.Title, b.Title)
}

// spec: UpcomingAll, EventDetail
func TestUpcoming_EventLinksToDetail(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)

	r := anon(t).get("/upcoming")
	assertStatus(t, r, 200)
	assertContains(t, r, `href="/e/`+slug+`"`)
}

// spec: UpcomingAll, Home
func TestUpcoming_TagFilter(t *testing.T) {
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
	r := v.get("/upcoming?tag=theater")
	assertStatus(t, r, 200)
	assertContains(t, r, th.Title)
	assertNotContains(t, r, co.Title)

	r = v.get("/upcoming?tag=concert")
	assertStatus(t, r, 200)
	assertContains(t, r, co.Title)
	assertNotContains(t, r, th.Title)

	// The home page filters the same way (only the next ten, so no
	// "contains my title" here) and offers every tag.
	assertStatus(t, v.get("/?tag=theater"), 200)
	assertNotContains(t, v.get("/?tag=theater"), co.Title)
	assertNotContains(t, v.get("/?tag=concert"), th.Title)
	r = v.get("/")
	assertStatus(t, r, 200)
	for _, tag := range []string{"concert", "theater", "film", "exhibition"} {
		assertContains(t, r, `href="/?tag=`+tag+`"`)
	}

	r = v.get("/?tag=bogus")
	assertStatus(t, r, 404)
	assertStatus(t, v.get("/upcoming?tag=bogus"), 404)
}

// spec: EventDetail
func TestEventDetail_ShowsAllPublicFields(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Tags = []string{"concert", "film"}
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
	assertContains(t, r, `href="/?tag=film"`)
	assertContains(t, r, `href="`+f.LinkURLs[0]+`"`)
	assertContains(t, r, `href="`+f.LinkURLs[1]+`"`)
	assertContains(t, r, "Listen")
	// A link without a label is labelled with its host.
	assertContains(t, r, "music.example.test")
	assertContains(t, r, poster1.Name)
	assertContains(t, r, `href="/p/`+poster1.Slug+`"`)
	assertNotContains(t, r, `class="count"`)
	assertHeaderContains(t, r, "Cache-Control", "no-store")
}

// spec: EventDetail
func TestEventDetail_Unknown404(t *testing.T) {
	r := anon(t).get("/e/does-not-exist-2030-01-01")
	assertStatus(t, r, 404)
	assertContains(t, r, "Page not found")
}

// spec: EventDetail, UpcomingAll, Home, Event.is_upcoming
func TestEventDetail_PastEventStillReachable(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, yesterday())
	slug := createEvent(t, p, f)

	v := anon(t)
	assertNotListed(t, v, "/upcoming", f.Title)
	assertNotListed(t, v, "/", f.Title)

	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertContains(t, r, "This event has passed.")
	// Past: no buttons for anyone.
	assertNotContains(t, r, `value="follow"`)
	assertNotContains(t, r, `value="hide"`)
}

// spec: PosterPage, Poster
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
	assertNotContains(t, r, "@")
}

// spec: PosterPage
func TestPosterPage_Unknown404(t *testing.T) {
	v := anon(t)
	assertStatus(t, v.get("/p/nobody-here"), 404)

	// An event slug is not a poster slug.
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	assertStatus(t, v.get("/p/"+slug), 404)
}

// spec: EventDetail, StartAccount, Visitor, FollowEvent, HideEvent
func TestAnon_EventPageOffersFollow_NoOwnerControls(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug

	v := anon(t)
	r := v.get(page)
	assertStatus(t, r, 200)
	// The buttons are there for everyone; pressing one starts the account.
	assertForm(t, r, `action="`+page+`/follow"`, `value="follow"`)
	assertForm(t, r, `action="`+page+`/follow"`, `value="hide"`)
	assertNotContains(t, r, `class="count"`)
	assertNotContains(t, r, "Log in to follow")
	if hasLinkToPath(r.Body, "/login") {
		t.Errorf("anon event page still links to /login")
	}
	assertNotContains(t, r, `href="/e/`+slug+`/edit"`)
	assertNotContains(t, r, `action="/e/`+slug+`/delete"`)
	if v.cookie("session") != nil {
		t.Fatalf("GET %s set a session cookie", page)
	}

	// Anon and user see the same event page.
	u := newUser(t)
	ru := u.get(page)
	assertStatus(t, ru, 200)
	for _, frag := range []string{`value="follow"`, `value="hide"`, `class="count"`} {
		if (hasForm(r.Body, frag) || containsFold(r.Body, frag)) != (hasForm(ru.Body, frag) || containsFold(ru.Body, frag)) {
			t.Errorf("anon and user event pages differ on %q", frag)
		}
	}
}

// spec: UpcomingAll, MyEvents, AccountPage, EventComposer, EventEditor, DeleteEvent, Poster, Visitor
func TestAnon_ProtectedRoutes_403ForPosterOnly_200ForReads(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	v := anon(t)

	// Read pages: 200, no redirect, no cookie.
	for _, path := range []string{"/mine", "/upcoming", "/account"} {
		r := v.get(path)
		assertStatus(t, r, 200)
		if v.cookie("session") != nil || sessionSetCookie(r) != "" {
			t.Errorf("GET %s set a session cookie for an anonymous visitor", path)
		}
	}
	// Poster-only pages: 403 for anyone who is not a poster.
	for _, path := range []string{"/new", "/e/" + slug + "/edit"} {
		r := v.get(path)
		assertStatus(t, r, 403)
		if v.cookie("session") != nil {
			t.Errorf("GET %s set a session cookie for an anonymous visitor", path)
		}
	}

	hijack := validEvent(t, tomorrow())
	posts := []struct {
		path string
		form url.Values
	}{
		{"/new", hijack.values()},
		{"/e/" + slug + "/edit", hijack.values()},
		{"/e/" + slug + "/delete", url.Values{}},
	}
	for _, tc := range posts {
		r := v.postForm(tc.path, tc.form)
		if r.Status != 403 {
			t.Errorf("anon POST %s: got %d → %q, want 403\nbody: %s", tc.path, r.Status, r.Location, snippet(r.Body))
		}
		if v.cookie("session") != nil {
			t.Errorf("anon POST %s started an account", tc.path)
		}
	}
	// Nothing leaked through: the event is untouched and still public.
	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertNotContains(t, r, `class="count"`)
	assertNotListed(t, anon(t), "/upcoming", hijack.Title)
}
