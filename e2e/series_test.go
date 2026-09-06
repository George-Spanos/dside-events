package e2e

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// Repeating events: one plain form POST to /new expands into ordinary events
// that share a series id and nothing else. Every request here is a plain
// form post, so this file doubles as the no-JS check of the feature.

var (
	allWeekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	dataSeries  = regexp.MustCompile(`^<li\b[^>]*\bdata-series="([^"]+)"`)
	inputTag    = regexp.MustCompile(`(?is)<input\b[^>]*>`)
)

// seriesOf returns the data-series value of a list row, or "".
func seriesOf(row string) string {
	if m := dataSeries.FindStringSubmatch(row); m != nil {
		return m[1]
	}
	return ""
}

// inputWith reports whether body has one <input> carrying every fragment.
func inputWith(body string, fragments ...string) bool {
	for _, in := range inputTag.FindAllString(body, -1) {
		ok := true
		for _, frag := range fragments {
			if !strings.Contains(in, frag) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// spec: CreateSeries, EventComposer, UpcomingAll, EventDetail, PosterPage, SlugsUnique
func TestCreateSeries_RowsRedirectAndDiscriminator(t *testing.T) {
	p := asPoster(t, poster1)

	// The New event form carries the repeat controls; the checkbox reveals the
	// rest in CSS, so they are in the markup whether or not it is ticked.
	r := p.get("/new")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/new"`, `name="repeats"`, `name="times"`, `name="until"`)
	if !inputWith(r.Body, `name="repeats"`, `value="1"`) {
		t.Errorf(`/new has no <input name="repeats" value="1">`)
	}
	for _, d := range allWeekdays {
		if !inputWith(r.Body, `name="weekday"`, `value="`+d+`"`) {
			t.Errorf(`/new has no <input name="weekday" value=%q>`, d)
		}
	}

	f := validSeries(t, 3)
	first, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series of 3 weeks made %d rows: %v", len(all), all)
	}
	if !strings.Contains(first, "-"+f.Date) {
		t.Errorf("303 went to /e/%s, want the earliest date %s", first, f.Date)
	}
	seen := map[string]bool{}
	for i, s := range all {
		if seen[s] {
			t.Errorf("slug %q used twice", s)
		}
		seen[s] = true
		if !slugShape.MatchString(s) {
			t.Errorf("slug %q does not match %s", s, slugShape)
		}
		if want := daysFromNow(1 + 7*i); !strings.Contains(s, "-"+want) {
			t.Errorf("date %d has slug %q, want the Athens date %s in it", i+1, s, want)
		}
	}

	// Every row carries the discriminator and the shared series id.
	v := anon(t)
	feed := v.get("/upcoming")
	assertStatus(t, feed, 200)
	var series string
	for _, s := range all {
		row := rowFor(feed.Body, s)
		if row == "" {
			t.Fatalf("/upcoming has no row for /e/%s", s)
		}
		if !strings.Contains(row, `<span class="series">One of 3 dates</span>`) {
			t.Errorf(`/upcoming row for /e/%s lacks <span class="series">One of 3 dates</span>:\n%s`, s, snippet(row))
		}
		id := seriesOf(row)
		if id == "" {
			t.Errorf("/upcoming row for /e/%s has no data-series attribute:\n%s", s, snippet(row))
		} else if series == "" {
			series = id
		} else if id != series {
			t.Errorf("/upcoming row for /e/%s has data-series=%q, its siblings %q", s, id, series)
		}
	}
	pp := v.get("/p/" + poster1.Slug)
	assertStatus(t, pp, 200)
	for _, s := range all {
		row := rowFor(pp.Body, s)
		if row == "" || !strings.Contains(row, "One of 3 dates") {
			t.Errorf("/p/%s row for /e/%s lacks 'One of 3 dates':\n%s", poster1.Slug, s, snippet(row))
		}
	}

	// So does every date's own page, together with the follow note.
	for _, s := range all {
		r := v.get("/e/" + s)
		assertStatus(t, r, 200)
		assertContains(t, r, f.Title)
		assertContains(t, r, `<small class="series">One of 3 dates</small>`)
		assertCopy(t, r, "Follow or hide applies to all 3 dates.")
	}

	// A single event shows neither.
	single := validEvent(t, tomorrow())
	slug := createEvent(t, p, single)
	row := rowFor(v.get("/upcoming").Body, slug)
	if row == "" {
		t.Fatalf("/upcoming has no row for the single event /e/%s", slug)
	}
	if strings.Contains(row, "dates") || seriesOf(row) != "" {
		t.Errorf("single event row carries series markup:\n%s", snippet(row))
	}
	r = v.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertNotContains(t, r, "One of ")
	assertNoCopy(t, r, "applies to all")
}

// spec: CreateSeries, UpcomingAll, SlugsUnique
func TestCreateSeries_SeveralTimesPerDay(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 2)
	f.Times = "21:00, 18:00"
	first, all := createSeries(t, p, f)
	if len(all) != 4 {
		t.Fatalf("2 weeks x 2 times made %d rows: %v", len(all), all)
	}

	// Feed order is by start: 18:00 then 21:00 on each of the two days.
	feed := anon(t).get("/upcoming")
	assertStatus(t, feed, 200)
	wantDay := []string{tomorrow(), tomorrow(), daysFromNow(8), daysFromNow(8)}
	wantClock := []string{"18:00", "21:00", "18:00", "21:00"}
	for i, s := range all {
		if !strings.Contains(s, "-"+wantDay[i]) {
			t.Errorf("row %d is /e/%s, want the date %s", i+1, s, wantDay[i])
		}
		row := rowFor(feed.Body, s)
		if !strings.Contains(row, ">"+wantClock[i]+"</time>") {
			t.Errorf("row %d (/e/%s) does not start at %s:\n%s", i+1, s, wantClock[i], snippet(row))
		}
		if !strings.Contains(row, "One of 4 dates") {
			t.Errorf("row %d (/e/%s) lacks 'One of 4 dates':\n%s", i+1, s, snippet(row))
		}
	}
	// The second start on one day gets its own slug, the -2 suffix.
	if all[1] != all[0]+"-2" || all[3] != all[2]+"-2" {
		t.Errorf("same-day slugs are %v, want base and base-2 per day", all)
	}
	// The redirect names the 18:00 of the first day.
	r := anon(t).get("/e/" + first)
	assertStatus(t, r, 200)
	assertContains(t, r, "18:00")
	assertNotContains(t, r, "21:00")
	// Both times keep the poster's own Start time out: it was replaced.
	assertNotContains(t, r, f.Time)
}

// spec: CreateSeries
func TestCreateSeries_FirstDateNotTickedIsSkipped(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, daysFromNow(1))
	f.Title = uniqTitle(t, "Series")
	f.Repeats = true
	f.Weekdays = []string{weekdayValue(daysFromNow(2))}
	f.Until = daysFromNow(15)
	first, all := createSeries(t, p, f)
	if len(all) != 2 {
		t.Fatalf("one weekday over days 2..15 made %d rows: %v", len(all), all)
	}
	for i, s := range all {
		if want := daysFromNow(2 + 7*i); !strings.Contains(s, "-"+want) {
			t.Errorf("row %d is /e/%s, want the date %s", i+1, s, want)
		}
		if strings.Contains(s, "-"+f.Date) {
			t.Errorf("row %d (/e/%s) is on the date field's day %s, whose weekday was not ticked", i+1, s, f.Date)
		}
	}
	if !strings.Contains(first, "-"+daysFromNow(2)) {
		t.Errorf("303 went to /e/%s, want the first ticked day %s", first, daysFromNow(2))
	}
}

// spec: CreateSeries, EventComposer.OnePostPerEvent
func TestCreateSeries_DuplicateOnCoveredDayRefused(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3) // days 1, 8 and 15 from now
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series made %d rows: %v", len(all), all)
	}

	// A single event with the same title on a covered day is a duplicate.
	dup := validEvent(t, daysFromNow(8))
	dup.Title = f.Title
	r := p.postForm("/new", dup.values())
	assertStatus(t, r, 422)
	assertContains(t, r, "already posted")
	assertContains(t, r, `role="alert"`)

	// So is the same series again.
	r = p.postForm("/new", f.values())
	assertStatus(t, r, 422)
	assertContains(t, r, "already posted")

	// A day inside the range that the series does not cover is free.
	free := validEvent(t, daysFromNow(2))
	free.Title = f.Title
	slug := createEvent(t, p, free)

	// The series is untouched and the newcomer is not one of its dates.
	feed := anon(t).get("/upcoming")
	assertStatus(t, feed, 200)
	if got := seriesSlugs(t, anon(t), f.Title); len(got) != 4 {
		t.Errorf("rows titled %q: %v, want the 3 dates plus the single event", f.Title, got)
	}
	for _, s := range all {
		if row := rowFor(feed.Body, s); !strings.Contains(row, "One of 3 dates") {
			t.Errorf("series row /e/%s changed:\n%s", s, snippet(row))
		}
	}
	row := rowFor(feed.Body, slug)
	if row == "" || strings.Contains(row, "dates") || seriesOf(row) != "" {
		t.Errorf("single event /e/%s row is missing or carries series markup:\n%s", slug, snippet(row))
	}
}

// spec: CreateSeries, EventComposer, max_series_occurrences
func TestCreateSeries_ValidationErrors(t *testing.T) {
	p := asPoster(t, poster1)
	cases := []struct {
		name  string
		mut   func(f *eventForm)
		field string
		msg   string
	}{
		{"no weekday", func(f *eventForm) { f.Weekdays = nil }, "weekday", "Pick at least one weekday."},
		{"bad weekday", func(f *eventForm) { f.Weekdays = []string{"funday"} }, "weekday", "Pick weekdays from the list."},
		{"bad times", func(f *eventForm) { f.Times = "18:00, noon" }, "times", "Pick valid start times, like 18:00, 21:00."},
		{"duplicate times", func(f *eventForm) { f.Times = "18:00, 18:00" }, "times", "Each start time must be different."},
		{"until empty", func(f *eventForm) { f.Until = "" }, "until", "Pick an until date."},
		{"until invalid", func(f *eventForm) { f.Until = "someday" }, "until", "Pick a valid until date."},
		{"until before date", func(f *eventForm) { f.Until = daysFromNow(0) }, "until", "Until must be on or after the date."},
		{"until too far", func(f *eventForm) { f.Until = daysFromNow(4 * 366) }, "until", "Until must be within the next three years."},
		{"too many dates", func(f *eventForm) {
			// 7 weekdays x 3 times x 71 days = 213, above 200 and within three years.
			f.Weekdays = allWeekdays
			f.Times = "10:00, 11:00, 12:00"
			f.Until = daysFromNow(1 + 70)
		}, "until", "A repeating event can have at most 200 dates."},
		{"too few dates", func(f *eventForm) { f.Until = tomorrow() }, "until", "A repeating event needs at least two dates."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validSeries(t, 3)
			tc.mut(&f)
			r := p.postForm("/new", f.values())
			assertStatus(t, r, 422)
			assertContains(t, r, `role="alert"`)
			assertCopy(t, r, tc.msg)
			assertContains(t, r, `id="`+tc.field+`-err"`)
			assertForm(t, r, `action="/new"`)
			// The form is re-rendered with the submitted values.
			assertContains(t, r, f.Title)
			if !inputWith(r.Body, `name="repeats"`, `value="1"`, "checked") {
				t.Errorf("Repeats is not checked on re-render")
			}
			if tc.field != "weekday" {
				if !inputWith(r.Body, `name="weekday"`, `value="`+f.Weekdays[0]+`"`, "checked") {
					t.Errorf("weekday %q is not checked on re-render", f.Weekdays[0])
				}
			}
			if f.Times != "" {
				assertContains(t, r, `value="`+f.Times+`"`)
			}
			if f.Until != "" {
				assertContains(t, r, `value="`+f.Until+`"`)
			}
			// Nothing was created.
			assertNotListed(t, anon(t), "/upcoming", f.Title)
		})
	}
}

// spec: EditEvent, EventEditor, CreateSeries
func TestSeries_EditIsPerDate(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series made %d rows: %v", len(all), all)
	}
	target := all[1]

	// The edit form has no repeat controls.
	r := p.get("/e/" + target + "/edit")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/e/`+target+`/edit"`)
	for _, name := range []string{"repeats", "weekday", "times", "until"} {
		assertNotContains(t, r, `name="`+name+`"`)
	}

	// Posting repeat fields to the edit changes that one date and nothing else.
	edited := f
	edited.Title = uniqTitle(t, "Renamed")
	edited.Date = daysFromNow(8)
	edited.Time = "19:00"
	edited.Weekdays = allWeekdays
	edited.Times = "10:00, 11:00"
	edited.Until = daysFromNow(60)
	assertRedirect(t, p.postForm("/e/"+target+"/edit", edited.values()), "/e/"+target)

	v := anon(t)
	r = v.get("/e/" + target)
	assertStatus(t, r, 200)
	assertContains(t, r, edited.Title)
	assertContains(t, r, "19:00")
	assertContains(t, r, `<small class="series">One of 3 dates</small>`)
	for _, s := range []string{all[0], all[2]} {
		r := v.get("/e/" + s)
		assertStatus(t, r, 200)
		assertContains(t, r, f.Title)
		assertNotContains(t, r, edited.Title)
		assertContains(t, r, f.Time)
	}
	if got := seriesSlugs(t, v, edited.Title); len(got) != 1 || got[0] != target {
		t.Errorf("rows titled %q: %v, want just /e/%s", edited.Title, got, target)
	}
	if got := seriesSlugs(t, v, f.Title); len(got) != 2 || got[0] != all[0] || got[1] != all[2] {
		t.Errorf("rows titled %q: %v, want the two untouched dates %v", f.Title, got, []string{all[0], all[2]})
	}
}

// spec: DeleteEvent, EventDetail, CreateSeries
func TestSeries_DeleteIsPerDate(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series made %d rows: %v", len(all), all)
	}
	alice := newUser(t)
	assertRedirect(t, setEventFollow(alice, all[0], "follow", "/mine"), "/mine")

	assertRedirect(t, p.postForm("/e/"+all[1]+"/delete", nil), "/")

	v := anon(t)
	assertStatus(t, v.get("/e/"+all[1]), 404)
	feed := v.get("/upcoming")
	assertStatus(t, feed, 200)
	if row := rowFor(feed.Body, all[1]); row != "" {
		t.Errorf("/upcoming still lists the deleted date /e/%s", all[1])
	}
	for _, s := range []string{all[0], all[2]} {
		r := v.get("/e/" + s)
		assertStatus(t, r, 200)
		assertContains(t, r, `<small class="series">One of 2 dates</small>`)
		assertCopy(t, r, "Follow or hide applies to all 2 dates.")
		if row := rowFor(feed.Body, s); !strings.Contains(row, "One of 2 dates") {
			t.Errorf("/upcoming row for /e/%s does not say 'One of 2 dates':\n%s", s, snippet(row))
		}
	}
	// The siblings keep their follows.
	r := alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, `href="/e/`+all[0]+`"`)
	assertContains(t, r, `href="/e/`+all[2]+`"`)
	assertNotContains(t, r, `href="/e/`+all[1]+`"`)
}

// spec: FollowEvent, SeriesFollowStateUniform, MyEvents, UpcomingAll, PosterPage, EventDetail
func TestSeries_FollowOneFollowsAll(t *testing.T) {
	p := asPoster(t, poster2)
	f := validSeries(t, 3)
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series made %d rows: %v", len(all), all)
	}
	alice := newUser(t)
	bob := newUser(t)
	lists := []string{"/upcoming", "/p/" + poster2.Slug}

	for _, path := range lists {
		body := alice.get(path).Body
		for _, s := range all {
			assertRowToggle(t, path, rowFor(body, s), s, false)
		}
	}

	// One press on the middle date.
	assertRedirect(t, setEventFollow(alice, all[1], "follow", "/upcoming"), "/upcoming")

	for _, path := range append(lists, "/mine") {
		r := alice.get(path)
		assertStatus(t, r, 200)
		for _, s := range all {
			assertRowToggle(t, path, rowFor(r.Body, s), s, true)
		}
	}
	v := anon(t)
	feed := v.get("/upcoming")
	for _, s := range all {
		r := v.get("/e/" + s)
		assertStatus(t, r, 200)
		if n := followerCount(t, r.Body); n != 1 {
			t.Errorf("/e/%s counter = %d, want 1", s, n)
		}
		if row := rowFor(feed.Body, s); !strings.Contains(row, "1 following") {
			t.Errorf("/upcoming row for /e/%s lacks '1 following':\n%s", s, snippet(row))
		}
		if ra := alice.get("/e/" + s); !pressedFollowing.MatchString(ra.Body) {
			t.Errorf("alice's /e/%s has no pressed Following button", s)
		}
	}

	// Bob is untouched.
	body := bob.get("/upcoming").Body
	for _, s := range all {
		assertRowToggle(t, "bob /upcoming", rowFor(body, s), s, false)
	}
	assertNotContains(t, bob.get("/mine"), f.Title)
}

// spec: HideEvent, UpcomingAll, Home, MyEvents, EventDetail, SeriesFollowStateUniform
func TestSeries_HideOneHidesAll(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series made %d rows: %v", len(all), all)
	}
	alice := newUser(t)
	assertListed(t, alice, "/upcoming", f.Title)

	// The row's Hide button on the last date.
	r := alice.postForm("/e/"+all[2]+"/follow", url.Values{"state": {"hide"}, "back": {"/upcoming"}})
	assertRedirect(t, r, "/upcoming")

	assertNotListed(t, alice, "/upcoming", f.Title)
	assertNotListed(t, alice, "/", f.Title)
	for _, s := range all {
		r := alice.get("/e/" + s)
		assertStatus(t, r, 200)
		assertContains(t, r, "Hidden from your feed")
		if !pressedHidden.MatchString(r.Body) {
			t.Errorf("alice's /e/%s has no pressed Hidden button", s)
		}
	}

	// All three sit in Hidden on /mine, each with its way back.
	r = alice.get("/mine")
	assertStatus(t, r, 200)
	assertContains(t, r, "<h2>Hidden</h2>")
	assertCopy(t, r, copyMineHidden)
	if n := strings.Count(r.Body, ">"+f.Title+"</a>"); n != 3 {
		t.Errorf("/mine lists %q %d times, want 3", f.Title, n)
	}
	for _, s := range all {
		assertForm(t, r, `action="/e/`+s+`/follow"`, `value="clear"`, "Show again")
	}

	// Everyone else still sees every date.
	feed := anon(t).get("/upcoming")
	assertStatus(t, feed, 200)
	for _, s := range all {
		if rowFor(feed.Body, s) == "" {
			t.Errorf("anon /upcoming lacks /e/%s after alice hid the run", s)
		}
	}
}

// spec: UnfollowEvent, MyEvents, UpcomingAll, SeriesFollowStateUniform
func TestSeries_ClearOneClearsAll(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series made %d rows: %v", len(all), all)
	}
	alice := newUser(t)
	v := anon(t)

	// Follow one date, clear another: nothing is followed any more.
	assertRedirect(t, setEventFollow(alice, all[0], "follow", "/mine"), "/mine")
	assertContains(t, alice.get("/mine"), f.Title)
	assertRedirect(t, setEventFollow(alice, all[2], "clear", "/mine"), "/mine")
	assertNotContains(t, alice.get("/mine"), f.Title)
	body := alice.get("/upcoming").Body
	for _, s := range all {
		if n := followerCount(t, v.get("/e/"+s).Body); n != 0 {
			t.Errorf("/e/%s counter after clear = %d, want 0", s, n)
		}
		assertRowToggle(t, "/upcoming", rowFor(body, s), s, false)
	}

	// Hide one date, clear another: the whole run is visible again.
	assertRedirect(t, setEventFollow(alice, all[1], "hide", "/upcoming"), "/upcoming")
	assertNotListed(t, alice, "/upcoming", f.Title)
	assertRedirect(t, setEventFollow(alice, all[0], "clear", "/mine"), "/mine")
	body = alice.get("/upcoming").Body
	for _, s := range all {
		if rowFor(body, s) == "" {
			t.Errorf("alice's /upcoming lacks /e/%s after clearing the run", s)
		}
		assertNotContains(t, alice.get("/e/"+s), "Hidden from your feed")
	}
	assertNotContains(t, alice.get("/mine"), f.Title)
}
