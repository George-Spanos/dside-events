package e2e

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	// homeListSize mirrors the server constant: a founder decision,
	// deliberately not configurable, so the number is hardcoded here too.
	homeListSize = 10

	linkAllUpcoming = "All upcoming events →"
	linkAllMine     = "All of mine →"
	headingMine     = "<h2>Mine</h2>"
	headingUpcoming = "<h2>Upcoming</h2>"
)

// farEvents creates n events by p on consecutive days about a thousand days
// out (well inside the three-year limit), tagged tag, so by date they come
// after everything else the suite posts. Returned in date order.
func farEvents(t testing.TB, p *client, n int, tag string) []eventForm {
	t.Helper()
	out := make([]eventForm, 0, n)
	for i := 0; i < n; i++ {
		f := validEvent(t, daysFromNow(1000+i))
		f.Title = uniqTitle(t, fmt.Sprintf("Far %02d", i))
		f.Tags = []string{tag}
		createEvent(t, p, f)
		out = append(out, f)
	}
	return out
}

// eventLinks counts the event rows in s by their detail links.
func eventLinks(s string) int { return strings.Count(s, `href="/e/`) }

// upcomingSection returns the home page's Upcoming column or fails.
func upcomingSection(t testing.TB, r resp) string {
	t.Helper()
	up := section(r.Body, "upcoming")
	if up == "" {
		t.Fatalf(`home page has no <section class="upcoming">…</section>`+"\nbody: %s", snippet(r.Body))
	}
	return up
}

// spec: Home, Home.TenAtMost, UpcomingAll, home_list_size, Visitor
func TestHome_UpcomingColumn_TenAtMost_LinksToAll(t *testing.T) {
	p := asPoster(t, poster1)
	far := farEvents(t, p, 12, "film")
	v := anon(t)

	r := v.get("/")
	assertStatus(t, r, 200)
	assertContains(t, r, `<div class="columns">`)
	assertContains(t, r, headingUpcoming)
	up := upcomingSection(t, r)
	// Twelve upcoming events exist at least, so the column is full: ten.
	if n := eventLinks(up); n != homeListSize {
		t.Errorf("home Upcoming column lists %d events, want exactly %d\nsection: %s", n, homeListSize, snippet(up))
	}
	if !strings.Contains(up, `<a href="/upcoming">`+linkAllUpcoming+`</a>`) {
		t.Errorf("Upcoming column lacks the footer link <a href=\"/upcoming\">%s</a>\nsection: %s", linkAllUpcoming, snippet(up))
	}
	// The two latest of the twelve are beyond the tenth whatever else exists.
	for _, f := range far[homeListSize:] {
		assertNotContains(t, r, f.Title)
	}

	// Filtered by tag: same cap, and the footer link keeps the filter.
	r = v.get("/?tag=film")
	assertStatus(t, r, 200)
	up = upcomingSection(t, r)
	if n := eventLinks(up); n != homeListSize {
		t.Errorf("home Upcoming column (tag=film) lists %d events, want exactly %d", n, homeListSize)
	}
	if !strings.Contains(up, `<a href="/upcoming?tag=film">`+linkAllUpcoming+`</a>`) {
		t.Errorf("filtered Upcoming column lacks <a href=\"/upcoming?tag=film\">%s</a>\nsection: %s", linkAllUpcoming, snippet(up))
	}
	for _, f := range far[homeListSize:] {
		assertNotContains(t, r, f.Title)
	}
	// No paging: one link to the full list, nothing else.
	if strings.Count(r.Body, linkAllUpcoming) != 1 {
		t.Errorf("home page shows %q %d times, want 1", linkAllUpcoming, strings.Count(r.Body, linkAllUpcoming))
	}
	for _, frag := range []string{"?page=", "?offset=", ">Next<", ">Previous<", ">More<"} {
		assertNotContains(t, r, frag)
	}
}

// spec: UpcomingAll, Home, Home.TenAtMost, Event.is_upcoming
func TestUpcoming_ListsBeyondTheTenth(t *testing.T) {
	p := asPoster(t, poster1)
	far := farEvents(t, p, 12, "exhibition")
	v := anon(t)

	home := v.get("/")
	assertStatus(t, home, 200)
	all := v.get("/upcoming")
	assertStatus(t, all, 200)
	for i, f := range far {
		assertContains(t, all, f.Title)
		if i >= homeListSize {
			assertNotContains(t, home, f.Title)
		}
	}
	assertBefore(t, all, far[0].Title, far[len(far)-1].Title)
	if n := eventLinks(all.Body); n <= homeListSize {
		t.Errorf("/upcoming lists %d events, want more than %d", n, homeListSize)
	}

	homeTag := v.get("/?tag=exhibition")
	assertStatus(t, homeTag, 200)
	allTag := v.get("/upcoming?tag=exhibition")
	assertStatus(t, allTag, 200)
	for i, f := range far {
		assertContains(t, allTag, f.Title)
		if i >= homeListSize {
			assertNotContains(t, homeTag, f.Title)
		}
	}
	// The full list is one column: no Mine section, no home footer link.
	assertNotContains(t, all, headingMine)
	assertNotContains(t, all, linkAllUpcoming)
}

// spec: Home, Home.SideBySide, MyEvents, Visitor
func TestHome_MineColumn_AbsentForAnonAndUnmarked(t *testing.T) {
	createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	for name, c := range map[string]*client{"anon": anon(t), "user with nothing marked": newUser(t)} {
		r := c.get("/")
		assertStatus(t, r, 200)
		for _, frag := range []string{headingMine, `<section class="mine">`, linkAllMine} {
			if strings.Contains(r.Body, frag) {
				t.Errorf("%s: home page shows a Mine column (%q)\nbody: %s", name, frag, snippet(r.Body))
			}
		}
		assertContains(t, r, `<div class="columns">`)
		assertContains(t, r, `<section class="upcoming">`)
		assertContains(t, r, headingUpcoming)
		assertContains(t, r, `<a href="/upcoming">`+linkAllUpcoming+`</a>`)
		// Without a Mine column there is no link to /mine anywhere: the nav
		// dropped it (founder, 2026-09-06), the column's footer carries it.
		assertNotContains(t, r, `href="/mine"`)
	}
}

// spec: Home, Home.SideBySide, FollowEvent, UnfollowEvent, HideEvent, MyEvents
func TestHome_MineColumn_ShowsFollowedUpcoming(t *testing.T) {
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Followed")
	slug := createEvent(t, asPoster(t, poster1), f)
	alice := newUser(t)

	// A row toggle posts back to the list it was pressed on.
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/"), "/")

	r := alice.get("/")
	assertStatus(t, r, 200)
	assertContains(t, r, `<div class="columns">`)
	assertBefore(t, r, `<section class="mine">`, `<section class="upcoming">`)
	assertBefore(t, r, headingMine, headingUpcoming)
	mine := section(r.Body, "mine")
	if mine == "" {
		t.Fatalf(`home page has no <section class="mine">…</section>`+"\nbody: %s", snippet(r.Body))
	}
	if !strings.Contains(mine, headingMine) {
		t.Errorf("Mine section lacks %s", headingMine)
	}
	if !strings.Contains(mine, f.Title) {
		t.Errorf("Mine column lacks %q\nsection: %s", f.Title, snippet(mine))
	}
	row := rowFor(mine, slug)
	if row == "" {
		t.Fatalf("Mine column has no row linking to /e/%s\nsection: %s", slug, snippet(mine))
	}
	assertRowToggle(t, "home Mine column", row, slug, true)
	if !strings.Contains(mine, `<a href="/mine">`+linkAllMine+`</a>`) {
		t.Errorf("Mine column lacks <a href=\"/mine\">%s</a>\nsection: %s", linkAllMine, snippet(mine))
	}
	// Both sections stand on one screen.
	assertContains(t, r, `<a href="/upcoming">`+linkAllUpcoming+`</a>`)
	// Every row of the followed event on the home page is pressed (it may
	// also sit in the Upcoming column).
	for _, row := range rowsFor(r.Body, slug) {
		assertRowToggle(t, "home", row, slug, true)
	}
	// Alice's follow is hers alone.
	assertNotContains(t, anon(t).get("/"), headingMine)
	assertNotContains(t, newUser(t).get("/"), headingMine)

	// Clearing empties the column and it goes away.
	assertRedirect(t, setEventFollow(alice, slug, "clear", "/"), "/")
	r = alice.get("/")
	assertStatus(t, r, 200)
	assertNotContains(t, r, headingMine)
	assertNotContains(t, r, `<section class="mine">`)
	assertNotContains(t, r, linkAllMine)

	// A hidden event is not "mine" either.
	assertRedirect(t, setEventFollow(alice, slug, "hide", "/"), "/")
	r = alice.get("/")
	assertStatus(t, r, 200)
	assertNotContains(t, r, headingMine)
	assertNotContains(t, r, f.Title)
}

// spec: Home, MyEvents, Event.is_upcoming, FollowEvent
func TestHome_MineColumn_AbsentWhenOnlyPastFollowed(t *testing.T) {
	past := validEvent(t, yesterday())
	past.Title = uniqTitle(t, "Gone By")
	slug := createEvent(t, asPoster(t, poster1), past)
	alice := newUser(t)
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/mine"), "/mine")

	r := alice.get("/mine")
	assertStatus(t, r, 200)
	assertBefore(t, r, "Past", past.Title)

	r = alice.get("/")
	assertStatus(t, r, 200)
	assertNotContains(t, r, headingMine)
	assertNotContains(t, r, `<section class="mine">`)
	assertNotContains(t, r, linkAllMine)
	assertNotContains(t, r, past.Title)
	assertContains(t, r, headingUpcoming)
}

// spec: UpcomingAll, Visitor
func TestUpcoming_Page_TagFilter_Unknown404(t *testing.T) {
	p := asPoster(t, poster1)
	th := validEvent(t, tomorrow())
	th.Title = uniqTitle(t, "Stage Play")
	th.Tags = []string{"theater"}
	co := validEvent(t, tomorrow())
	co.Title = uniqTitle(t, "Live Gig")
	co.Tags = []string{"concert"}
	createEvent(t, p, th)
	createEvent(t, p, co)

	v := anon(t)
	r := v.get("/upcoming")
	assertStatus(t, r, 200)
	assertContains(t, r, "<h1>Upcoming</h1>")
	assertContains(t, r, "<title>Upcoming · dside events</title>")
	assertContains(t, r, th.Title)
	assertContains(t, r, co.Title)
	assertNotContains(t, r, `href="/mine"`)
	assertContains(t, r, `href="/account"`)
	assertHeaderContains(t, r, "Cache-Control", "no-store")
	if v.cookie("session") != nil || sessionSetCookie(r) != "" {
		t.Errorf("GET /upcoming set a session cookie")
	}

	r = v.get("/upcoming?tag=theater")
	assertStatus(t, r, 200)
	assertContains(t, r, "<h1>Upcoming</h1>")
	assertContains(t, r, th.Title)
	assertNotContains(t, r, co.Title)
	for _, row := range eventRows(r.Body) {
		s := slugOf(row)
		if !strings.Contains(v.get("/e/"+s).Body, `href="/?tag=theater"`) {
			t.Errorf("/upcoming?tag=theater lists /e/%s, which is not tagged theater", s)
		}
	}

	r = v.get("/upcoming?tag=concert")
	assertStatus(t, r, 200)
	assertContains(t, r, co.Title)
	assertNotContains(t, r, th.Title)

	r = v.get("/upcoming?tag=bogus")
	assertStatus(t, r, 404)
	assertContains(t, r, "Page not found")
	assertStatus(t, v.get("/upcoming/"), 404)
	assertStatus(t, v.get("/upcoming/extra"), 404)
}

var (
	dayHeading = regexp.MustCompile(`(?s)<h3 class="day">.*?</h3>`)
	weekdays   = "Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday"
	// A day heading that is not the contract's <h3 class="day">: an <h2>
	// or a class-less <h3> carrying a weekday.
	staleDayHeading = regexp.MustCompile(`<h[23]>(?:<b>)?(?:Today · |Tomorrow · )?(?:<b>)?(?:` + weekdays + `)\b`)
)

// spec: UpcomingAll, Home, MyEvents, PosterPage
func TestUpcoming_DayHeadings_WeekdayBold(t *testing.T) {
	p := asPoster(t, poster1)
	far := time.Now().In(athens).AddDate(0, 0, 45)
	a := validEvent(t, far.Format("2006-01-02"))
	a.Title = uniqTitle(t, "Day A")
	a.Time = "19:00"
	b := validEvent(t, far.Format("2006-01-02"))
	b.Title = uniqTitle(t, "Day B")
	b.Time = "22:00"
	today := validEvent(t, daysFromNow(0))
	today.Title = uniqTitle(t, "Tonight")
	today.Time = "23:59"
	tmrw := validEvent(t, tomorrow())
	tmrw.Title = uniqTitle(t, "Morrow")
	createEvent(t, p, a)
	createEvent(t, p, b)
	createEvent(t, p, today)
	createEvent(t, p, tmrw)

	v := anon(t)
	r := v.get("/upcoming")
	assertStatus(t, r, 200)
	heads := dayHeading.FindAllString(r.Body, -1)
	if len(heads) == 0 {
		t.Fatalf(`/upcoming has no <h3 class="day"> heading`+"\nbody: %s", snippet(r.Body))
	}
	for _, h := range heads {
		if !strings.Contains(h, "<b>") || !strings.Contains(h, "</b>") {
			t.Errorf("day heading without a bold weekday: %s", h)
		}
	}
	if m := staleDayHeading.FindString(r.Body); m != "" {
		t.Errorf("/upcoming still renders a day as %q; want <h3 class=\"day\">", m)
	}
	// A plain day: bold weekday, the rest normal; one heading for both events.
	farHead := `<h3 class="day"><b>` + far.Format("Monday") + `</b> ` + far.Format("2 January") + `</h3>`
	assertContains(t, r, farHead)
	if n := strings.Count(r.Body, farHead); n != 1 {
		// One heading per day; rows show the clock only.
		t.Errorf("%q appears %d times on /upcoming, want 1 (one heading per day)", farHead, n)
	}
	assertBefore(t, r, farHead, a.Title)
	assertBefore(t, r, a.Title, b.Title)
	// Today and tomorrow carry their prefix inside the heading.
	now := time.Now().In(athens)
	for _, tc := range []struct {
		label string
		d     time.Time
		title string
	}{{"Today", now, today.Title}, {"Tomorrow", now.AddDate(0, 0, 1), tmrw.Title}} {
		re := regexp.MustCompile(`<h3 class="day">(?:<b>)?` + tc.label + ` · (?:</b>)?(?:<b>)?` + tc.d.Format("Monday") + `</b> ` + tc.d.Format("2 January") + `</h3>`)
		if !re.MatchString(r.Body) {
			t.Errorf("/upcoming lacks a %s heading matching %s\nheadings: %s", tc.label, re, strings.Join(heads, " "))
		}
		assertBefore(t, r, tc.label+" · ", tc.title)
	}

	// The same heading markup serves the home page and the other lists.
	for _, path := range []string{"/", "/p/" + poster1.Slug} {
		r := v.get(path)
		assertStatus(t, r, 200)
		if len(dayHeading.FindAllString(r.Body, -1)) == 0 {
			t.Errorf(`%s has no <h3 class="day"> heading`, path)
		}
		if m := staleDayHeading.FindString(r.Body); m != "" {
			t.Errorf("%s still renders a day as %q", path, m)
		}
	}
	alice := newUser(t)
	assertRedirect(t, setEventFollow(alice, createEvent(t, p, validEvent(t, tomorrow())), "follow", "/mine"), "/mine")
	r = alice.get("/mine")
	if len(dayHeading.FindAllString(r.Body, -1)) == 0 {
		t.Errorf(`/mine has no <h3 class="day"> heading`)
	}
}
