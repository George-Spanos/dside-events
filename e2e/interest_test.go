package e2e

import (
	"net/url"
	"strings"
	"testing"
)

// spec: MarkInterested, EventDetail, EventDetailForUser, MyEvents, PublicFeed
func TestInterested_CounterPublic_AppearsInMine(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug

	v := anon(t)
	if n := interestedCount(t, v.get(page).Body); n != 0 {
		t.Fatalf("fresh event counter = %d, want 0", n)
	}

	alice := loginAs(t, uniqEmail(t, "alice"))
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

	// The feed row carries the count too.
	feed := v.get("/")
	assertStatus(t, feed, 200)
	i := strings.Index(feed.Body, f.Title)
	if i < 0 {
		t.Fatalf("feed lacks %q", f.Title)
	}
	if tail := feed.Body[i:min(len(feed.Body), i+600)]; !strings.Contains(tail, "1 interested") {
		t.Errorf("feed row for %q lacks '1 interested':\n%s", f.Title, snippet(tail))
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
	alice := loginAs(t, uniqEmail(t, "alice"))
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
	alice := loginAs(t, uniqEmail(t, "alice"))
	bob := loginAs(t, uniqEmail(t, "bob"))
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

// spec: MarkNotInterested, Feed, EventDetailForUser
func TestNotInterested_HiddenFromOwnFeed_InvisibleToOthers(t *testing.T) {
	f := validEvent(t, tomorrow())
	f.Tags = []string{"exhibition"}
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := loginAs(t, uniqEmail(t, "alice"))
	bob := loginAs(t, uniqEmail(t, "bob"))
	assertRedirect(t, follow(alice, "tag", "exhibition", "1", "/following"), "/following")
	assertContains(t, alice.get("/"), f.Title)
	assertContains(t, alice.get("/following"), f.Title)

	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)

	// Hidden from every list alice sees.
	assertNotContains(t, alice.get("/"), f.Title)
	assertNotContains(t, alice.get("/?tag=exhibition"), f.Title)
	assertNotContains(t, alice.get("/following"), f.Title)
	// Her own event page says so and offers clear.
	r := alice.get(page)
	assertStatus(t, r, 200)
	assertContains(t, r, "Hidden from your feed")
	assertForm(t, r, `action="`+page+`/interest"`, `value="clear"`)

	// Everyone else sees the event untouched, counter unchanged, no trace.
	for name, c := range map[string]*client{"anon": anon(t), "bob": bob, "poster": asPoster(t, poster1)} {
		feed := c.get("/")
		if !strings.Contains(feed.Body, f.Title) {
			t.Errorf("%s: feed lacks %q after alice hid it", name, f.Title)
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
	alice := loginAs(t, uniqEmail(t, "alice"))
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
	alice := loginAs(t, uniqEmail(t, "alice"))
	v := anon(t)

	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 1 {
		t.Fatalf("counter = %d, want 1", n)
	}
	assertContains(t, alice.get("/"), f.Title)

	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 0 {
		t.Errorf("counter after switching = %d, want 0", n)
	}
	assertNotContains(t, alice.get("/"), f.Title)
	r := alice.get("/mine")
	assertBefore(t, r, "Hidden", f.Title)

	// And back again.
	assertRedirect(t, setInterest(alice, slug, "interested", page), page)
	if n := interestedCount(t, v.get(page).Body); n != 1 {
		t.Errorf("counter after switching back = %d, want 1", n)
	}
	assertContains(t, alice.get("/"), f.Title)
	assertNotContains(t, alice.get(page), "Hidden from your feed")
}

// spec: ClearInterest, MyEvents, Feed
func TestInterest_Clear_RestoresDefault(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	page := "/e/" + slug
	alice := loginAs(t, uniqEmail(t, "alice"))
	v := anon(t)

	// not_interested → clear: back in the feed, gone from /mine.
	assertRedirect(t, setInterest(alice, slug, "not_interested", page), page)
	assertNotContains(t, alice.get("/"), f.Title)
	assertRedirect(t, setInterest(alice, slug, "clear", "/mine"), "/mine")
	assertContains(t, alice.get("/"), f.Title)
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
	alice := loginAs(t, uniqEmail(t, "alice"))
	assertStatus(t, setInterest(alice, slug, "maybe", ""), 400)
	assertStatus(t, setInterest(alice, slug, "", ""), 400)
	assertStatus(t, alice.postForm("/e/"+slug+"/interest", url.Values{"back": {"/"}}), 400)
	assertStatus(t, setInterest(alice, "no-such-event-2030-01-01", "interested", ""), 404)
	if n := interestedCount(t, anon(t).get("/e/"+slug).Body); n != 0 {
		t.Errorf("counter = %d after rejected posts, want 0", n)
	}
}

// spec: MyEvents, EventDetailForUser, Event.is_upcoming
func TestMine_UpcomingVsPast(t *testing.T) {
	p := asPoster(t, poster1)
	up := validEvent(t, tomorrow())
	up.Title = uniqTitle(t, "Soon")
	past := validEvent(t, yesterday())
	past.Title = uniqTitle(t, "Gone")
	upSlug := createEvent(t, p, up)
	pastSlug := createEvent(t, p, past)
	alice := loginAs(t, uniqEmail(t, "alice"))

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
	// The past event never reaches the feed even when someone is interested.
	assertNotContains(t, anon(t).get("/"), past.Title)
}

// spec: MyEvents
func TestMine_Empty(t *testing.T) {
	f := validEvent(t, tomorrow())
	createEvent(t, asPoster(t, poster1), f)
	c := loginAs(t, uniqEmail(t, "alice"))
	r := c.get("/mine")
	assertStatus(t, r, 200)
	assertNotContains(t, r, f.Title)
	assertNotContains(t, r, `href="/e/`)
	assertContains(t, r, `href="/`)
}
