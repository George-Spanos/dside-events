package e2e

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// spec: MarkInterested, EventDetail, MyEvents, UpcomingAll
func TestInterested_CounterPublic_AppearsInMine(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	v := anon(t)
	if n := interestedCount(t, v.get(page).Body); n != 0 {
		t.Fatalf("fresh event counter = %d, want 0", n)
	}

	alice := newUser(t)
	r := alice.get(page)
	assertStatus(t, r, 200)
	assertForm(t, r, `action="`+page+`/interest"`, `value="interested"`)
	assertForm(t, r, `action="`+page+`/interest"`, `value="not_interested"`)
	assertNotContains(t, alice.get("/mine"), f.Title)

	assertRedirect(t, setInterest(alice, slug, "interested", page), page)

	r = v.get(page)
	assertStatus(t, r, 200)
	if n := interestedCount(t, r.Body); n != 1 {
		t.Errorf("public counter = %d, want 1", n)
	}
	assertContains(t, r, "1 interested")
	assertNotContains(t, r, "Nobody yet interested")

	r = alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertContains(t, r, `href="`+page+`"`)

	// The list row carries the count too.
	feed := v.get("/upcoming")
	assertStatus(t, feed, 200)
	row := rowFor(feed.Body, slug)
	if row == "" {
		t.Fatalf("/upcoming has no row for %q", f.Title)
	}
	if !strings.Contains(row, "1 interested") {
		t.Errorf("/upcoming row for %q lacks '1 interested':\n%s", f.Title, snippet(row))
	}

	// Redirect target is the back field; a foreign back falls back to the event.
	assertRedirect(t, setInterest(alice, slug, "interested", "/mine"), "/mine")
	assertRedirect(t, setInterest(alice, slug, "interested", "https://evil.example/"), page)
	assertRedirect(t, setInterest(alice, slug, "interested", ""), page)
}

// spec: MarkInterested, OneInterestPerAccountEvent
func TestInterested_IdempotentPerUser(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug
	alice := newUser(t)
	for i := 0; i < 3; i++ {
		assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	}
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter after three identical marks = %d, want 1", n)
	}
}

// spec: MarkInterested, Event.interested_count
func TestInterested_TwoUsersCountTwo(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug
	alice := newUser(t)
	bob := newUser(t)
	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	assertRedirect(t, setInterest(bob, slug, "interested", page), page)
	r := anon(t).get(page)
	if n := interestedCount(t, r.Body); n != 2 {
		t.Errorf("counter = %d, want 2", n)
	}
	assertContains(t, r, "2 interested")
	// A poster sees the same public number, nothing more.
	if n := interestedCount(t, asPoster(t, poster1).get(page).Body); n != 2 {
		t.Errorf("poster sees counter %d, want 2", n)
	}
}

// spec: MarkNotInterested, UpcomingAll, Home, EventDetail
func TestNotInterested_HiddenFromOwnFeed_InvisibleToOthers(t *testing.T) {
	f := validEvent(t, tomorrow())
	f.Tags = []string{"exhibition"}
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := newUser(t)
	bob := newUser(t)
	assertRedirect(t, follow(alice, "tag", "exhibition", "1", "/following"), "/following")
	assertListed(t, alice, "/upcoming", f.Title)
	assertListed(t, alice, "/upcoming?tag=exhibition", f.Title)
	assertListed(t, alice, "/following", f.Title)

	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)

	// Hidden from every list alice sees, the home page included.
	for _, path := range []string{"/upcoming", "/upcoming?tag=exhibition", "/following", "/", "/?tag=exhibition"} {
		assertNotListed(t, alice, path, f.Title)
	}
	// Her own event page says so and offers clear.
	r := alice.get(page)
	assertStatus(t, r, 200)
	assertContains(t, r, "Hidden from your feed")
	assertForm(t, r, `action="`+page+`/interest"`, `value="clear"`)

	// Everyone else sees the event untouched, counter unchanged, no trace.
	for name, c := range map[string]*client{"anon": anon(t), "bob": bob, "poster": asPoster(t, poster1)} {
		feed := c.get("/upcoming")
		if !strings.Contains(feed.Body, f.Title) {
			t.Errorf("%s: /upcoming lacks %q after alice hid it", name, f.Title)
		}
		r := c.get(page)
		assertStatus(t, r, 200)
		if n := interestedCount(t, r.Body); n != 0 {
			t.Errorf("%s: counter = %d, want 0", name, n)
		}
		if strings.Contains(r.Body, "Hidden from your feed") || containsFold(r.Body, "not interested by") {
			t.Errorf("%s: event page leaks alice's not_interested mark", name)
		}
	}
	// The poster's own list is untouched as well.
	assertContains(t, anon(t).get("/p/"+poster1.Slug), f.Title)
}

// spec: MarkNotInterested, MyEvents
func TestNotInterested_NotInMineUpcoming_ButInHidden(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	alice := newUser(t)
	assertRedirect(t, setInterest(alice, slug, "not_interested", "/mine"), "/mine")

	r := alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, "Hidden")
	assertBefore(t, r, "Hidden", f.Title)
	if i, j := strings.Index(r.Body, "Upcoming"), strings.Index(r.Body, f.Title); i >= 0 && j >= 0 {
		// If an Upcoming heading is rendered, the hidden event must not sit
		// between it and the Hidden heading.
		h := strings.Index(r.Body, "Hidden")
		if i < j && j < h {
			t.Errorf("hidden event %q listed under Upcoming", f.Title)
		}
	}
	assertForm(t, r, `action="/e/`+slug+`/interest"`, `value="clear"`)
}

// spec: MarkInterested, MarkNotInterested, OneInterestPerAccountEvent
func TestInterest_SwitchInterestedToNotInterested(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := newUser(t)
	v := anon(t)

	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}
	assertListed(t, alice, "/upcoming", f.Title)

	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 0 {
		t.Errorf("counter after switching = %d, want 0", n)
	}
	assertNotListed(t, alice, "/upcoming", f.Title)
	r := alice.get("/mine")
	assertBefore(t, r, "Hidden", f.Title)

	// And back again.
	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 1 {
		t.Errorf("counter after switching back = %d, want 1", n)
	}
	assertListed(t, alice, "/upcoming", f.Title)
	assertNotContains(t, alice.get(page), "Hidden from your feed")
}

// spec: ClearInterest, MyEvents, UpcomingAll
func TestInterest_Clear_RestoresDefault(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := newUser(t)
	v := anon(t)

	// not_interested → clear: back in the list, gone from /mine.
	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)
	assertNotListed(t, alice, "/upcoming", f.Title)
	assertRedirect(t, setInterest(alice, slug, "clear", "/mine"), "/mine")
	assertListed(t, alice, "/upcoming", f.Title)
	assertNotContains(t, alice.get("/mine"), f.Title)
	assertNotContains(t, alice.get(page), "Hidden from your feed")

	// interested → clear: counter drops, gone from /mine.
	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}
	assertRedirect(t, setInterest(alice, slug, "clear", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 0 {
		t.Errorf("counter after clear = %d, want 0", n)
	}
	assertNotContains(t, alice.get("/mine"), f.Title)

	// Clearing when nothing is marked is a harmless no-op.
	assertRedirect(t, setInterest(alice, slug, "clear", page), page)
}

// spec: ClearInterest, MarkInterested
func TestInterest_BadStateOrUnknownEvent(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	alice := newUser(t)
	assertStatus(t, setInterest(alice, slug, "maybe", ""), 400)
	assertStatus(t, setInterest(alice, slug, "", ""), 400)
	assertStatus(t, alice.postForm("/e/"+slug+"/interest", url.Values{"back": {"/"}}), 400)
	assertStatus(t, setInterest(alice, "no-such-event-2030-01-01", "interested", ""), 404)
	if n := interestedCount(t, anon(t).get("/e/"+slug).Body); n != 0 {
		t.Errorf("counter = %d after rejected posts, want 0", n)
	}
}

// spec: MyEvents, EventDetail, Event.is_upcoming
func TestMine_UpcomingVsPast(t *testing.T) {
	p := asPoster(t, poster1)
	up := validEvent(t, tomorrow())
	up.Title = uniqTitle(t, "Soon")
	past := validEvent(t, yesterday())
	past.Title = uniqTitle(t, "Gone")
	upSlug := createEvent(t, p, up)
	pastSlug := createEvent(t, p, past)
	alice := newUser(t)

	// A past event shows no toggles but still accepts the plain post.
	r := alice.get("/e/" + pastSlug)
	assertStatus(t, r, 200)
	assertContains(t, r, "This event has passed.")
	assertNotContains(t, r, `value="interested"`)
	assertNotContains(t, r, `value="not_interested"`)

	assertRedirect(t, setInterest(alice, upSlug, "interested", "/mine"), "/mine")
	assertRedirect(t, setInterest(alice, pastSlug, "interested", "/mine"), "/mine")

	r = alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, "Upcoming")
	assertContains(t, r, "Past")
	assertBefore(t, r, "Upcoming", up.Title)
	assertBefore(t, r, up.Title, "Past")
	assertBefore(t, r, "Past", past.Title)

	// Past tense on a past event's counter.
	r = anon(t).get("/e/" + pastSlug)
	assertContains(t, r, "1 were interested")
	if n := interestedCount(t, r.Body); n != 1 {
		t.Errorf("past counter = %d, want 1", n)
	}
	// The past event never reaches the upcoming lists even when someone is interested.
	assertNotListed(t, anon(t), "/upcoming", past.Title)
	assertNotListed(t, anon(t), "/", past.Title)
}

// spec: MyEvents
func TestMine_Empty(t *testing.T) {
	f := validEvent(t, tomorrow())
	createEvent(t, asPoster(t, poster1), f)
	c := newUser(t)
	r := c.get("/mine")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)
	assertNotContains(t, r, `href="/e/`)
	assertContains(t, r, `href="/`)
}

// spec: MarkInterested, StartAccount, EventDetail, MyEvents, Interest, Visitor
func TestInterested_AnonMarkStartsAccount(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	v := anon(t)
	r := setInterest(v, slug, "interested", "/mine")
	assertRedirect(t, r, "/mine")
	if v.cookie("session") == nil {
		t.Fatalf("anonymous Interested did not set a session cookie")
	}
	r = v.follow(r)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	r = v.get(page)
	assertStatus(t, r, 200)
	if n := interestedCount(t, r.Body); n != 1 {
		t.Errorf("counter = %d, want 1", n)
	}
	// Not interested from a fresh visitor works the same way and hides the
	// event from that visitor's lists only.
	w := anon(t)
	assertRedirect(t, setInterest(w, slug, "not_interested", page), page)
	if w.cookie("session") == nil {
		t.Fatalf("anonymous Not interested did not set a session cookie")
	}
	assertNotListed(t, w, "/upcoming", f.Title)
	assertContains(t, w.get(page), "Hidden from your feed")
	assertListed(t, anon(t), "/upcoming", f.Title)
	if n := interestedCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter after a not_interested = %d, want 1", n)
	}
}

// ---- row toggles --------------------------------------------------------------

var (
	pressedRowButton   = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="true"[^>]*>\s*Interested\s*</button>`)
	unpressedRowButton = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="false"[^>]*>\s*I'm interested\s*</button>`)
)

// assertRowToggle checks one list row against the row-toggle contract: a
// POST form to /e/{slug}/interest with a hidden back field and a single
// state button that is either pressed (aria-pressed="true", the check mark is drawn by CSS,
// value="clear") or not (Interested, aria-pressed="false",
// value="interested"). Rows never offer not_interested.
func assertRowToggle(t testing.TB, page, row, slug string, pressed bool) {
	t.Helper()
	if row == "" {
		t.Errorf("%s: no row for /e/%s", page, slug)
		return
	}
	fail := func(what string) {
		t.Helper()
		t.Errorf("%s: row for /e/%s %s\nrow: %s", page, slug, what, snippet(row))
	}
	if !strings.Contains(row, `<form method="post" action="/e/`+slug+`/interest"`) {
		fail(`lacks <form method="post" action="/e/{slug}/interest"`)
		return
	}
	if !strings.Contains(row, `name="state"`) {
		fail(`lacks a name="state" button`)
	}
	if !strings.Contains(row, `name="back"`) {
		fail(`lacks the hidden name="back" field`)
	}
	if strings.Contains(row, "not_interested") || strings.Contains(row, "Not interested") {
		fail("offers not_interested; that belongs to the event page only")
	}
	if pressed {
		for _, want := range []string{`value="clear"`, `aria-pressed="true"`} {
			if !strings.Contains(row, want) {
				fail("is marked but lacks " + want)
			}
		}
		if !pressedRowButton.MatchString(row) {
			fail("has no pressed <button>Interested</button>")
		}
		for _, no := range []string{`value="interested"`, `aria-pressed="false"`} {
			if strings.Contains(row, no) {
				fail("is marked but still has " + no)
			}
		}
	} else {
		for _, want := range []string{`value="interested"`, `aria-pressed="false"`} {
			if !strings.Contains(row, want) {
				fail("is unmarked but lacks " + want)
			}
		}
		if !unpressedRowButton.MatchString(row) {
			fail("has no <button>I'm interested</button>")
		}
		for _, no := range []string{`value="clear"`, `aria-pressed="true"`} {
			if strings.Contains(row, no) {
				fail("is unmarked but has " + no)
			}
		}
	}
}

// spec: MarkInterested, ClearInterest, Home, UpcomingAll, MyEvents, PosterPage, Visitor
func TestRows_InterestToggleOnEveryList(t *testing.T) {
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Row")
	f.Tags = []string{"exhibition"}
	slug := createEvent(t, asPoster(t, poster2), f)
	alice := newUser(t)
	assertRedirect(t, follow(alice, "poster", poster2.Slug, "1", "/following"), "/following")

	// Full lists carry alice's row; the home page shows only the next ten,
	// so there the check is "every row is a toggle", whatever the rows are.
	fullLists := []string{"/upcoming", "/upcoming?tag=exhibition", "/following", "/p/" + poster2.Slug}
	pages := append([]string{"/", "/?tag=exhibition"}, fullLists...)

	for _, path := range pages {
		r := alice.get(path)
		assertStatus(t, r, 200)
		rs := eventRows(r.Body)
		if len(rs) == 0 {
			t.Errorf(`%s has no <ul class="events"> rows`, path)
			continue
		}
		for _, row := range rs {
			s := slugOf(row)
			if s == "" {
				t.Errorf("%s: row without an event link: %s", path, snippet(row))
				continue
			}
			// Alice marked nothing yet: every row is unpressed and posts
			// back to the page it is on.
			assertRowToggle(t, path, row, s, false)
			if !strings.Contains(row, `name="back" value="`+path+`"`) {
				t.Errorf("%s: row for /e/%s does not post back to %q\nrow: %s", path, s, path, snippet(row))
			}
		}
	}
	for _, path := range fullLists {
		assertRowToggle(t, path, rowFor(alice.get(path).Body, slug), slug, false)
	}
	assertNotContains(t, alice.get("/mine"), `href="/e/`+slug+`"`)

	// Pressing the row toggle posts back to the list it was on.
	assertRedirect(t, setInterest(alice, slug, "interested", "/upcoming?tag=exhibition"), "/upcoming?tag=exhibition")
	assertRedirect(t, setInterest(alice, slug, "interested", "/following"), "/following")
	assertRedirect(t, setInterest(alice, slug, "interested", "/p/"+poster2.Slug), "/p/"+poster2.Slug)
	assertRedirect(t, setInterest(alice, slug, "interested", "/"), "/")

	// Now every row of that event, on every list, is pressed.
	for _, path := range append(pages, "/mine") {
		r := alice.get(path)
		assertStatus(t, r, 200)
		rs := rowsFor(r.Body, slug)
		if len(rs) == 0 {
			if path == "/?tag=exhibition" {
				// The Upcoming column may be full of other events; the Mine
				// column carries the mark only on the unfiltered home page.
				continue
			}
			t.Errorf("%s: no row for the marked event /e/%s\nbody: %s", path, slug, snippet(r.Body))
			continue
		}
		for _, row := range rs {
			assertRowToggle(t, path, row, slug, true)
		}
	}
	// Other rows stay unpressed for alice, and alice's mark is invisible in
	// everyone else's rows.
	for _, row := range eventRows(alice.get("/upcoming").Body) {
		if s := slugOf(row); s != slug {
			assertRowToggle(t, "/upcoming", row, s, false)
		}
	}
	for name, c := range map[string]*client{"anon": anon(t), "bob": newUser(t), "poster": asPoster(t, poster2)} {
		assertRowToggle(t, name+" /upcoming", rowFor(c.get("/upcoming").Body, slug), slug, false)
	}

	// The pressed button posts value="clear": the row unpresses, /mine empties.
	assertRedirect(t, setInterest(alice, slug, "clear", "/following"), "/following")
	assertRowToggle(t, "/following", rowFor(alice.get("/following").Body, slug), slug, false)
	assertRowToggle(t, "/upcoming", rowFor(alice.get("/upcoming").Body, slug), slug, false)
	assertNotContains(t, alice.get("/mine"), `href="/e/`+slug+`"`)
	assertNotContains(t, alice.get("/"), "<h2>Mine</h2>")
}

// spec: MyEvents, PosterPage, Event.is_upcoming, MarkInterested, UpcomingAll
func TestRows_PastRowsHaveNoToggle(t *testing.T) {
	past := validEvent(t, yesterday())
	past.Title = uniqTitle(t, "Was")
	slug := createEvent(t, asPoster(t, poster2), past)
	alice := newUser(t)
	assertRedirect(t, setInterest(alice, slug, "interested", "/mine"), "/mine")

	for name, r := range map[string]resp{"/mine": alice.get("/mine"), "/p/{slug}": alice.get("/p/" + poster2.Slug)} {
		assertStatus(t, r, 200)
		assertBefore(t, r, "Past", past.Title)
		row := rowFor(r.Body, slug)
		if row == "" {
			t.Errorf("%s: no row for the past event /e/%s\nbody: %s", name, slug, snippet(r.Body))
			continue
		}
		if strings.Contains(row, "<form") || strings.Contains(row, "/interest") || strings.Contains(row, "aria-pressed") {
			t.Errorf("%s: past row for /e/%s carries an interest toggle\nrow: %s", name, slug, snippet(row))
		}
	}
	// Past rows are never upcoming rows.
	for _, path := range []string{"/upcoming", "/", "/following"} {
		assertNotListed(t, alice, path, past.Title)
	}
}

var (
	pressedInterested      = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="true"[^>]*>\s*Interested\s*</button>`)
	pressedNotInterested   = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="true"[^>]*>\s*Not interested\s*</button>`)
	unpressedInterested    = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="false"[^>]*>\s*I'm interested\s*</button>`)
	unpressedNotInterested = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="false"[^>]*>\s*Not interested\s*</button>`)
)

// spec: EventDetail, MarkInterested, MarkNotInterested, ClearInterest
func TestEventPage_PressedButtonShowsCheck(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug
	alice := newUser(t)

	check := func(state string, want ...*regexp.Regexp) {
		t.Helper()
		r := alice.get(page)
		assertStatus(t, r, 200)
		for _, re := range want {
			if !re.MatchString(r.Body) {
				t.Errorf("after %s: event page lacks a button matching %s\nbody: %s", state, re, snippet(r.Body))
			}
		}
		if n := strings.Count(r.Body, `aria-pressed="true"`); (state == "clear" && n != 0) || (state != "clear" && n != 1) {
			t.Errorf("after %s: event page has %d pressed buttons", state, n)
		}
	}
	check("clear", unpressedInterested, unpressedNotInterested)

	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	check("interested", pressedInterested, unpressedNotInterested)

	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)
	check("not_interested", pressedNotInterested, unpressedInterested)

	assertRedirect(t, setInterest(alice, slug, "clear", page), page)
	check("clear", unpressedInterested, unpressedNotInterested)

	// The pressed state is alice's alone.
	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	for name, c := range map[string]*client{"anon": anon(t), "bob": newUser(t)} {
		r := c.get(page)
		assertStatus(t, r, 200)
		if strings.Contains(r.Body, `aria-pressed="true"`) {
			t.Errorf("%s sees a pressed button on the event page after alice's mark", name)
		}
	}
}
