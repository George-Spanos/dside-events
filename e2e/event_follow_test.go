package e2e

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// spec: FollowEvent, EventDetail, MyEvents, UpcomingAll, Event.follower_count, Account.followed_events
func TestFollow_CounterPublic_AppearsInMine(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	v := anon(t)
	if n := followerCount(t, v.get(page).Body); n != 0 {
		t.Fatalf("fresh event counter = %d, want 0", n)
	}

	alice := newUser(t)
	r := alice.get(page)
	assertStatus(t, r, 200)
	assertForm(t, r, `action="`+page+`/follow"`, `value="follow"`)
	assertForm(t, r, `action="`+page+`/follow"`, `value="hide"`)
	assertNotContains(t, alice.get("/mine"), f.Title)

	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)

	r = v.get(page)
	assertStatus(t, r, 200)
	if n := followerCount(t, r.Body); n != 1 {
		t.Errorf("public counter = %d, want 1", n)
	}
	assertContains(t, r, "1 following")
	assertNotContains(t, r, "Nobody following yet")

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
	if !strings.Contains(row, "1 following") {
		t.Errorf("/upcoming row for %q lacks '1 following':\n%s", f.Title, snippet(row))
	}

	// Redirect target is the back field; a foreign back falls back to the event.
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/mine"), "/mine")
	assertRedirect(t, setEventFollow(alice, slug, "follow", "https://evil.example/"), page)
	assertRedirect(t, setEventFollow(alice, slug, "follow", ""), page)
}

// spec: FollowEvent, OneFollowPerAccountEvent
func TestFollow_IdempotentPerUser(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug
	alice := newUser(t)
	for i := 0; i < 3; i++ {
		assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	}
	if n := followerCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter after three identical marks = %d, want 1", n)
	}
}

// spec: FollowEvent, Event.follower_count
func TestFollow_TwoUsersCountTwo(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	page := "/e/" + slug
	alice := newUser(t)
	bob := newUser(t)
	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	assertRedirect(t, setEventFollow(bob, slug, "follow", page), page)
	r := anon(t).get(page)
	if n := followerCount(t, r.Body); n != 2 {
		t.Errorf("counter = %d, want 2", n)
	}
	assertContains(t, r, "2 following")
	// A poster sees the same public number, nothing more.
	if n := followerCount(t, asPoster(t, poster1).get(page).Body); n != 2 {
		t.Errorf("poster sees counter %d, want 2", n)
	}
}

// spec: HideEvent, EventDetail.HiddenIsPrivate, UpcomingAll, Home, EventDetail, Account.hidden_events
func TestHide_HiddenFromOwnFeed_InvisibleToOthers(t *testing.T) {
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

	assertRedirect(t, setEventFollow(alice, slug, "hide", page), page)

	// Hidden from every list alice sees, the home page included.
	for _, path := range []string{"/upcoming", "/upcoming?tag=exhibition", "/following", "/", "/?tag=exhibition"} {
		assertNotListed(t, alice, path, f.Title)
	}
	// Her own event page says so and offers clear.
	r := alice.get(page)
	assertStatus(t, r, 200)
	assertContains(t, r, "Hidden from your feed")
	assertForm(t, r, `action="`+page+`/follow"`, `value="clear"`)

	// Everyone else sees the event untouched, counter unchanged, no trace.
	for name, c := range map[string]*client{"anon": anon(t), "bob": bob, "poster": asPoster(t, poster1)} {
		feed := c.get("/upcoming")
		if !strings.Contains(feed.Body, f.Title) {
			t.Errorf("%s: /upcoming lacks %q after alice hid it", name, f.Title)
		}
		r := c.get(page)
		assertStatus(t, r, 200)
		if n := followerCount(t, r.Body); n != 0 {
			t.Errorf("%s: counter = %d, want 0", name, n)
		}
		if strings.Contains(r.Body, "Hidden from your feed") || pressedHidden.MatchString(r.Body) {
			t.Errorf("%s: event page leaks alice's hidden state", name)
		}
	}
	// The poster's own list is untouched as well.
	assertContains(t, anon(t).get("/p/"+poster1.Slug), f.Title)
}

// spec: HideEvent, MyEvents, Account.hidden_events
func TestHide_NotInMineUpcoming_ButInHidden(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	alice := newUser(t)
	assertRedirect(t, setEventFollow(alice, slug, "hide", "/mine"), "/mine")

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
	assertCopy(t, r, copyMineHidden)
	assertContains(t, r, "Show again")
	assertForm(t, r, `action="/e/`+slug+`/follow"`, `value="clear"`)
}

// spec: FollowEvent, HideEvent, OneFollowPerAccountEvent
func TestEventFollow_SwitchFollowToHide(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := newUser(t)
	v := anon(t)

	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	if n := followerCount(t, v.get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}
	assertListed(t, alice, "/upcoming", f.Title)

	assertRedirect(t, setEventFollow(alice, slug, "hide", page), page)
	if n := followerCount(t, v.get(page).Body); n != 0 {
		t.Errorf("counter after switching = %d, want 0", n)
	}
	assertNotListed(t, alice, "/upcoming", f.Title)
	r := alice.get("/mine")
	assertBefore(t, r, "Hidden", f.Title)

	// And back again.
	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	if n := followerCount(t, v.get(page).Body); n != 1 {
		t.Errorf("counter after switching back = %d, want 1", n)
	}
	assertListed(t, alice, "/upcoming", f.Title)
	assertNotContains(t, alice.get(page), "Hidden from your feed")
}

// spec: UnfollowEvent, MyEvents, UpcomingAll
func TestEventFollow_Clear_RestoresDefault(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := newUser(t)
	v := anon(t)

	// hide → clear: back in the list, gone from /mine.
	assertRedirect(t, setEventFollow(alice, slug, "hide", page), page)
	assertNotListed(t, alice, "/upcoming", f.Title)
	assertRedirect(t, setEventFollow(alice, slug, "clear", "/mine"), "/mine")
	assertListed(t, alice, "/upcoming", f.Title)
	assertNotContains(t, alice.get("/mine"), f.Title)
	assertNotContains(t, alice.get(page), "Hidden from your feed")

	// follow → clear: counter drops, gone from /mine.
	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	if n := followerCount(t, v.get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}
	assertRedirect(t, setEventFollow(alice, slug, "clear", page), page)
	if n := followerCount(t, v.get(page).Body); n != 0 {
		t.Errorf("counter after clear = %d, want 0", n)
	}
	assertNotContains(t, alice.get("/mine"), f.Title)

	// Clearing when nothing is marked is a harmless no-op.
	assertRedirect(t, setEventFollow(alice, slug, "clear", page), page)
}

// spec: UnfollowEvent, FollowEvent, FollowState
func TestEventFollow_BadStateOrUnknownEvent(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	alice := newUser(t)
	assertStatus(t, setEventFollow(alice, slug, "maybe", ""), 400)
	assertStatus(t, setEventFollow(alice, slug, "", ""), 400)
	assertStatus(t, alice.postForm("/e/"+slug+"/follow", url.Values{"back": {"/"}}), 400)
	assertStatus(t, setEventFollow(alice, "no-such-event-2030-01-01", "follow", ""), 404)
	if n := followerCount(t, anon(t).get("/e/"+slug).Body); n != 0 {
		t.Errorf("counter = %d after rejected posts, want 0", n)
	}
}

// spec: MyEvents, EventDetail, Event.is_upcoming, Account.followed_events
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
	assertNotContains(t, r, `value="follow"`)
	assertNotContains(t, r, `value="hide"`)

	assertRedirect(t, setEventFollow(alice, upSlug, "follow", "/mine"), "/mine")
	assertRedirect(t, setEventFollow(alice, pastSlug, "follow", "/mine"), "/mine")

	r = alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, "Upcoming")
	assertContains(t, r, "Past")
	assertBefore(t, r, "Upcoming", up.Title)
	assertBefore(t, r, up.Title, "Past")
	assertBefore(t, r, "Past", past.Title)

	// Past tense on a past event's counter.
	r = anon(t).get("/e/" + pastSlug)
	assertContains(t, r, "1 followed")
	if n := followerCount(t, r.Body); n != 1 {
		t.Errorf("past counter = %d, want 1", n)
	}
	// The past event never reaches the upcoming lists even when someone follows it.
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

// spec: FollowEvent, StartAccount, EventDetail, MyEvents, EventFollow, Visitor
func TestFollow_AnonFollowStartsAccount(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	v := anon(t)
	r := setEventFollow(v, slug, "follow", "/mine")
	assertRedirect(t, r, "/mine")
	if v.cookie("session") == nil {
		t.Fatalf("anonymous Follow did not set a session cookie")
	}
	r = v.follow(r)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	r = v.get(page)
	assertStatus(t, r, 200)
	if n := followerCount(t, r.Body); n != 1 {
		t.Errorf("counter = %d, want 1", n)
	}
	// Hide from a fresh visitor works the same way and hides the
	// event from that visitor's lists only.
	w := anon(t)
	assertRedirect(t, setEventFollow(w, slug, "hide", page), page)
	if w.cookie("session") == nil {
		t.Fatalf("anonymous Hide did not set a session cookie")
	}
	assertNotListed(t, w, "/upcoming", f.Title)
	assertContains(t, w.get(page), "Hidden from your feed")
	assertListed(t, anon(t), "/upcoming", f.Title)
	if n := followerCount(t, anon(t).get(page).Body); n != 1 {
		t.Errorf("counter after a hide = %d, want 1", n)
	}
}

// ---- row toggles --------------------------------------------------------------

var (
	pressedRowButton   = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="true"[^>]*>\s*Following\s*</button>`)
	unpressedRowButton = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="false"[^>]*>\s*Follow\s*</button>`)
)

// assertRowToggle checks one list row against the row-toggle contract: a
// POST form to /e/{slug}/follow with a hidden back field and a single
// Follow button that is either pressed (aria-pressed="true", the check mark is drawn by CSS,
// value="clear") or not (Follow, aria-pressed="false",
// value="follow"). Rows never offer hide; that lives on the event page.
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
	if !strings.Contains(row, `<form method="post" action="/e/`+slug+`/follow"`) {
		fail(`lacks <form method="post" action="/e/{slug}/follow"`)
		return
	}
	if !strings.Contains(row, `name="state"`) {
		fail(`lacks a name="state" button`)
	}
	if !strings.Contains(row, `name="back"`) {
		fail(`lacks the hidden name="back" field`)
	}
	// Every row also offers Hide (founder, 2026-09-06): a plain state=hide
	// button; hidden rows never appear in these lists, so it is never pressed.
	if !strings.Contains(row, `value="hide"`) || !strings.Contains(row, ">Hide</button>") {
		fail("lacks the Hide button")
	}
	if pressed {
		for _, want := range []string{`value="clear"`, `aria-pressed="true"`} {
			if !strings.Contains(row, want) {
				fail("is marked but lacks " + want)
			}
		}
		if !pressedRowButton.MatchString(row) {
			fail("has no pressed <button>Following</button>")
		}
		for _, no := range []string{`value="follow"`, `aria-pressed="false"`} {
			if strings.Contains(row, no) {
				fail("is marked but still has " + no)
			}
		}
	} else {
		for _, want := range []string{`value="follow"`, `aria-pressed="false"`} {
			if !strings.Contains(row, want) {
				fail("is unmarked but lacks " + want)
			}
		}
		if !unpressedRowButton.MatchString(row) {
			fail("has no <button>Follow</button>")
		}
		for _, no := range []string{`value="clear"`, `aria-pressed="true"`} {
			if strings.Contains(row, no) {
				fail("is unmarked but has " + no)
			}
		}
	}
}

// spec: FollowEvent, UnfollowEvent, Home, UpcomingAll, MyEvents, PosterPage, Visitor
func TestRows_FollowToggleOnEveryList(t *testing.T) {
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
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/upcoming?tag=exhibition"), "/upcoming?tag=exhibition")
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/following"), "/following")
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/p/"+poster2.Slug), "/p/"+poster2.Slug)
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/"), "/")

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
	assertRedirect(t, setEventFollow(alice, slug, "clear", "/following"), "/following")
	assertRowToggle(t, "/following", rowFor(alice.get("/following").Body, slug), slug, false)
	assertRowToggle(t, "/upcoming", rowFor(alice.get("/upcoming").Body, slug), slug, false)
	assertNotContains(t, alice.get("/mine"), `href="/e/`+slug+`"`)
	assertNotContains(t, alice.get("/"), "<h2>Mine</h2>")
}

// spec: MyEvents, PosterPage, Event.is_upcoming, FollowEvent, UpcomingAll
func TestRows_PastRowsHaveNoToggle(t *testing.T) {
	past := validEvent(t, yesterday())
	past.Title = uniqTitle(t, "Was")
	slug := createEvent(t, asPoster(t, poster2), past)
	alice := newUser(t)
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/mine"), "/mine")

	for name, r := range map[string]resp{"/mine": alice.get("/mine"), "/p/{slug}": alice.get("/p/" + poster2.Slug)} {
		assertStatus(t, r, 200)
		assertBefore(t, r, "Past", past.Title)
		row := rowFor(r.Body, slug)
		if row == "" {
			t.Errorf("%s: no row for the past event /e/%s\nbody: %s", name, slug, snippet(r.Body))
			continue
		}
		if strings.Contains(row, "<form") || strings.Contains(row, "/follow") || strings.Contains(row, "aria-pressed") {
			t.Errorf("%s: past row for /e/%s carries a follow toggle\nrow: %s", name, slug, snippet(row))
		}
	}
	// Past rows are never upcoming rows.
	for _, path := range []string{"/upcoming", "/", "/following"} {
		assertNotListed(t, alice, path, past.Title)
	}
}

var (
	pressedFollowing = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="true"[^>]*>\s*Following\s*</button>`)
	pressedHidden    = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="true"[^>]*>\s*Hidden\s*</button>`)
	unpressedFollow  = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="false"[^>]*>\s*Follow\s*</button>`)
	unpressedHide    = regexp.MustCompile(`(?s)<button[^>]*aria-pressed="false"[^>]*>\s*Hide\s*</button>`)
)

// spec: EventDetail, EventDetail.HiddenIsPrivate, FollowEvent, HideEvent, UnfollowEvent
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
	check("clear", unpressedFollow, unpressedHide)

	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	check("follow", pressedFollowing, unpressedHide)

	assertRedirect(t, setEventFollow(alice, slug, "hide", page), page)
	check("hide", pressedHidden, unpressedFollow)

	assertRedirect(t, setEventFollow(alice, slug, "clear", page), page)
	check("clear", unpressedFollow, unpressedHide)

	// The pressed state is alice's alone.
	assertRedirect(t, setEventFollow(alice, slug, "follow", page), page)
	for name, c := range map[string]*client{"anon": anon(t), "bob": newUser(t)} {
		r := c.get(page)
		assertStatus(t, r, 200)
		if strings.Contains(r.Body, `aria-pressed="true"`) {
			t.Errorf("%s sees a pressed button on the event page after alice's mark", name)
		}
	}
}

// spec: HideEvent, Home, UpcomingAll, MyEvents
func TestRows_HideFromList_RemovesRowAndLandsInHidden(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)
	alice := newUser(t)
	assertListed(t, alice, "/upcoming", f.Title)

	// The row's own Hide button posts from the list and returns there.
	r := alice.postForm("/e/"+slug+"/follow", url.Values{"state": {"hide"}, "back": {"/upcoming"}})
	assertRedirect(t, r, "/upcoming")
	assertNotListed(t, alice, "/upcoming", f.Title)
	assertNotListed(t, alice, "/", f.Title)

	// It sits in Hidden on /mine with the way back, and others still see it.
	r = alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, "<h2>Hidden</h2>")
	assertContains(t, r, f.Title)
	assertContains(t, r, "Show again")
	assertListed(t, anon(t), "/upcoming", f.Title)
}
