package e2e

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var slugShape = regexp.MustCompile(`^[a-z0-9-]+-\d{4}-\d{2}-\d{2}(-\d+)?$`)

// spec: CreateEvent, EventComposer, Poster, Event.PostedByPoster
func TestNewEvent_UserForbidden(t *testing.T) {
	u := newUser(t)
	assertStatus(t, u.get("/new"), 403)
	f := validEvent(t, tomorrow())
	assertStatus(t, u.postForm("/new", f.values()), 403)
	assertNotListed(t, anon(t), "/upcoming", f.Title)
}

// spec: CreateEvent, Event, EventComposer, EventDetail, UpcomingAll, PosterPage
func TestCreateEvent_Success(t *testing.T) {
	p := asPoster(t, poster1)
	r := p.get("/new")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/new"`, `name="title"`)
	// max_links is enforced by the form itself: exactly three link slots, and
	// the contract has no fourth field.
	for i := 1; i <= 3; i++ {
		assertContains(t, r, fmt.Sprintf(`name="link_url_%d"`, i))
		assertContains(t, r, fmt.Sprintf(`name="link_label_%d"`, i))
	}
	assertNotContains(t, r, `name="link_url_4"`)
	// max_tags is a server rule (see the validation table); every tag is offered.
	for _, tag := range []string{"concert", "theater", "film", "exhibition"} {
		assertContains(t, r, `value="`+tag+`"`)
	}

	f := validEvent(t, tomorrow())
	f.Tags = []string{"concert", "theater"}
	slug := createEvent(t, p, f)
	if !slugShape.MatchString(slug) {
		t.Errorf("slug %q does not match %s", slug, slugShape)
	}
	if !strings.Contains(slug, "-"+f.Date) {
		t.Errorf("slug %q lacks the Athens date %s", slug, f.Date)
	}

	r = p.follow(resp{Status: 303, Location: "/e/" + slug})
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
	assertContains(t, r, f.Venue)
	assertContains(t, r, f.Price)
	assertContains(t, r, f.Description)
	assertContains(t, r, `href="/?tag=concert"`)
	assertContains(t, r, `href="/?tag=theater"`)
	assertContains(t, r, `href="`+f.LinkURLs[0]+`"`)
	assertContains(t, r, poster1.Name)

	v := anon(t)
	assertListed(t, v, "/upcoming", f.Title)
	assertListed(t, v, "/upcoming?tag=theater", f.Title)
	assertContains(t, v.get("/p/"+poster1.Slug), f.Title)
}

// spec: CreateEvent, EventComposer
func TestCreateEvent_ValidationErrors(t *testing.T) {
	p := asPoster(t, poster1)
	long := func(n int) string { return strings.Repeat("x", n) }
	cases := []struct {
		name string
		mut  func(f *eventForm)
	}{
		{"empty title", func(f *eventForm) { f.Title = "" }},
		{"blank title", func(f *eventForm) { f.Title = "   " }},
		{"title too long", func(f *eventForm) { f.Title = uniqTitle(t, long(121)) }},
		{"missing date", func(f *eventForm) { f.Date = "" }},
		{"malformed date", func(f *eventForm) { f.Date = "tomorrow" }},
		{"missing time", func(f *eventForm) { f.Time = "" }},
		{"malformed time", func(f *eventForm) { f.Time = "25:99" }},
		{"too far in the past", func(f *eventForm) { f.Date = daysFromNow(-2 * 366) }},
		{"too far in the future", func(f *eventForm) { f.Date = daysFromNow(4 * 366) }},
		{"empty venue", func(f *eventForm) { f.Venue = "" }},
		{"venue too long", func(f *eventForm) { f.Venue = long(121) }},
		{"price too long", func(f *eventForm) { f.Price = long(61) }},
		{"no tags", func(f *eventForm) { f.Tags = nil }},
		{"unknown tag", func(f *eventForm) { f.Tags = []string{"opera"} }},
		{"javascript link", func(f *eventForm) { f.LinkURLs = []string{"javascript:alert(1)"} }},
		{"ftp link", func(f *eventForm) { f.LinkURLs = []string{"ftp://example.test/x"} }},
		{"not a url", func(f *eventForm) { f.LinkURLs = []string{"not a url"} }},
		{"host-less link", func(f *eventForm) { f.LinkURLs = []string{"https://"} }},
		{"description too long", func(f *eventForm) { f.Description = long(4001) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validEvent(t, tomorrow())
			f.Venue = "Venue " + strings.ToUpper(uid())
			tc.mut(&f)
			r := p.postForm("/new", f.values())
			assertStatus(t, r, 422)
			// The form is re-rendered with the submitted values.
			if f.Title != "" && strings.TrimSpace(f.Title) != "" && len(f.Title) <= 120 {
				assertContains(t, r, f.Title)
			}
			if f.Venue != "" && len(f.Venue) <= 120 {
				assertContains(t, r, f.Venue)
			}
			assertContains(t, r, `role="alert"`)
			assertForm(t, r, `action="/new"`)
			// Nothing was created.
			if strings.TrimSpace(f.Title) != "" {
				assertNotListed(t, anon(t), "/upcoming", f.Title)
			}
		})
	}
}

// spec: CreateEvent, EventComposer
func TestCreateEvent_DuplicateRejected(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)

	// Exactly the same title and day, different time.
	dup := f
	dup.Time = "21:00"
	r := p.postForm("/new", dup.values())
	assertStatus(t, r, 422)
	assertContains(t, r, "already posted")

	// Same normalised title: case and spacing differ.
	dup2 := f
	dup2.Title = "  " + strings.ToUpper(f.Title[:1]) + strings.ToLower(f.Title[1:]) + "  "
	dup2.Title = strings.Replace(dup2.Title, " ", "   ", 1)
	r = p.postForm("/new", dup2.values())
	assertStatus(t, r, 422)
	assertContains(t, r, "already posted")

	// The original is untouched and the list shows it once.
	r = anon(t).get("/upcoming")
	assertStatus(t, r, 200)
	if n := strings.Count(r.Body, `href="/e/`+slug+`"`); n != 1 {
		t.Errorf("/upcoming links to /e/%s %d times, want 1", slug, n)
	}
}

// spec: CreateEvent, SlugsUnique
func TestCreateEvent_SameTitleDifferentDayAllowed(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug1 := createEvent(t, p, f)
	f2 := f
	f2.Date = daysFromNow(2)
	slug2 := createEvent(t, p, f2)
	if slug1 == slug2 {
		t.Fatalf("both events got slug %q", slug1)
	}
	if !strings.Contains(slug2, "-"+f2.Date) {
		t.Errorf("slug %q lacks date %s", slug2, f2.Date)
	}
	v := anon(t)
	assertStatus(t, v.get("/e/"+slug1), 200)
	assertStatus(t, v.get("/e/"+slug2), 200)
}

// spec: CreateEvent, SlugsUnique
func TestCreateEvent_SameTitleOtherPosterAllowed(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug1 := createEvent(t, asPoster(t, poster1), f)
	slug2 := createEvent(t, asPoster(t, poster2), f)
	if slug1 == slug2 {
		t.Fatalf("two posters' events share slug %q", slug1)
	}
	if !slugShape.MatchString(slug2) {
		t.Errorf("slug %q does not match %s", slug2, slugShape)
	}
	v := anon(t)
	r := v.get("/e/" + slug1)
	assertStatus(t, r, 200)
	assertContains(t, r, poster1.Name)
	r = v.get("/e/" + slug2)
	assertStatus(t, r, 200)
	assertContains(t, r, poster2.Name)
}

// spec: CreateEvent, EventDetail
func TestCreateEvent_GreekTitleSlug(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, daysFromNow(37))
	f.Title = "Ταξίδι στη Χώρα των Ήχων"
	slug := createEvent(t, p, f)
	if !strings.HasPrefix(slug, "taxidi-sti-chora-ton-ichon-") {
		t.Errorf("slug = %q, want prefix taxidi-sti-chora-ton-ichon-", slug)
	}
	if !slugShape.MatchString(slug) {
		t.Errorf("slug %q does not match %s", slug, slugShape)
	}
	r := anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
}

// spec: EditEvent, EventEditor, EventDetail, UpcomingAll
func TestEditEvent_OwnerEdits_SlugImmutable(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)

	r := p.get("/e/" + slug + "/edit")
	assertStatus(t, r, 200)
	assertContains(t, r, `value="`+f.Title+`"`)
	assertContains(t, r, `value="`+f.Venue+`"`)
	assertContains(t, r, `value="`+formDate(f.Date)+`"`)
	assertContains(t, r, `value="`+f.Time+`"`)
	assertContains(t, r, f.Description)
	assertContains(t, r, `value="`+f.LinkURLs[0]+`"`)
	assertForm(t, r, `action="/e/`+slug+`/edit"`)

	edited := f
	edited.Title = uniqTitle(t, "Renamed")
	edited.Date = daysFromNow(3)
	edited.Time = "19:00"
	edited.Venue = "Kyttaro"
	edited.Tags = []string{"theater"}
	edited.LinkLabels = []string{"Tickets"}
	edited.LinkURLs = []string{"https://tickets.example.test/" + uid()}
	r = p.postForm("/e/"+slug+"/edit", edited.values())
	assertRedirect(t, r, "/e/"+slug)

	r = p.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, edited.Title)
	assertNotContains(t, r, f.Title)
	assertContains(t, r, edited.Venue)
	assertContains(t, r, edited.Time)
	assertContains(t, r, `href="/?tag=theater"`)
	assertNotContains(t, r, `href="/?tag=concert"`)
	assertContains(t, r, `href="`+edited.LinkURLs[0]+`"`)
	assertNotContains(t, r, f.LinkURLs[0])

	feed := anon(t).get("/upcoming")
	assertContains(t, feed, edited.Title)
	assertNotContains(t, feed, f.Title)
	if n := strings.Count(feed.Body, `href="/e/`+slug+`"`); n != 1 {
		t.Errorf("/upcoming links to /e/%s %d times, want 1", slug, n)
	}
}

// spec: EditEvent, EventEditor
func TestEditEvent_ValidationErrors(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)

	bad := f
	bad.Title = ""
	r := p.postForm("/e/"+slug+"/edit", bad.values())
	assertStatus(t, r, 422)
	assertForm(t, r, `action="/e/`+slug+`/edit"`)

	bad = f
	bad.Tags = nil
	assertStatus(t, p.postForm("/e/"+slug+"/edit", bad.values()), 422)

	// Editing into a duplicate of another own event is refused too.
	other := validEvent(t, tomorrow())
	createEvent(t, p, other)
	bad = f
	bad.Title = other.Title
	r = p.postForm("/e/"+slug+"/edit", bad.values())
	assertStatus(t, r, 422)
	assertContains(t, r, "already posted")

	// Nothing changed.
	r = anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
}

// spec: EditEvent, EventEditor
func TestEditEvent_OtherPosterForbidden(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	p2 := asPoster(t, poster2)
	assertStatus(t, p2.get("/e/"+slug+"/edit"), 403)
	hijack := validEvent(t, tomorrow())
	assertStatus(t, p2.postForm("/e/"+slug+"/edit", hijack.values()), 403)
	assertNotContains(t, anon(t).get("/e/"+slug), hijack.Title)
}

// spec: EditEvent, EventEditor
func TestEditEvent_UserForbidden(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	u := newUser(t)
	assertStatus(t, u.get("/e/"+slug+"/edit"), 403)
	assertStatus(t, u.postForm("/e/"+slug+"/edit", validEvent(t, tomorrow()).values()), 403)
}

// spec: EditEvent, EventEditor
func TestEditEvent_Unknown404(t *testing.T) {
	p := asPoster(t, poster1)
	assertStatus(t, p.get("/e/no-such-event-2030-01-01/edit"), 404)
	assertStatus(t, p.postForm("/e/no-such-event-2030-01-01/edit", validEvent(t, tomorrow()).values()), 404)
}

// spec: DeleteEvent, EventEditor, MyEvents
func TestDeleteEvent_OwnerDeletes_CascadesFollows(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)
	alice := newUser(t)
	assertRedirect(t, setEventFollow(alice, slug, "follow", "/e/"+slug), "/e/"+slug)
	assertContains(t, alice.get("/mine"), f.Title)

	assertRedirect(t, p.postForm("/e/"+slug+"/delete", nil), "/")

	v := anon(t)
	assertStatus(t, v.get("/e/"+slug), 404)
	assertStatus(t, p.get("/e/"+slug+"/edit"), 404)
	assertNotListed(t, v, "/upcoming", f.Title)
	assertNotContains(t, v.get("/p/"+poster1.Slug), f.Title)
	assertNotContains(t, alice.get("/mine"), f.Title)

	// The slug is not reused: reposting the same event gets a different URL.
	slug2 := createEvent(t, p, f)
	if slug2 == slug {
		t.Errorf("deleted slug %q was reused", slug)
	}
}

// spec: DeleteEvent, EventEditor
func TestDeleteEvent_OtherPosterOrUserForbidden(t *testing.T) {
	f := validEvent(t, tomorrow())
	slug := createEvent(t, asPoster(t, poster1), f)
	assertStatus(t, asPoster(t, poster2).postForm("/e/"+slug+"/delete", nil), 403)
	assertStatus(t, newUser(t).postForm("/e/"+slug+"/delete", nil), 403)
	assertStatus(t, asPoster(t, poster1).postForm("/e/no-such-event-2030-01-01/delete", nil), 404)
	r := anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
}

// spec: EventDetail, EventEditor
func TestEventDetail_EditDeleteOnlyForOwner(t *testing.T) {
	p := asPoster(t, poster1)
	slug := createEvent(t, p, validEvent(t, tomorrow()))
	edit, del := `href="/e/`+slug+`/edit"`, `action="/e/`+slug+`/delete"`

	r := p.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, edit)
	assertContains(t, r, del)

	for name, c := range map[string]*client{
		"other poster": asPoster(t, poster2),
		"user":         newUser(t),
		"anon":         anon(t),
	} {
		r := c.get("/e/" + slug)
		assertStatus(t, r, 200)
		if strings.Contains(r.Body, edit) || strings.Contains(r.Body, del) {
			t.Errorf("%s sees edit/delete controls on /e/%s", name, slug)
		}
	}
}

// spec: AddPoster, PosterSlugsUnique, PosterPage, OpenSecretLink, EventComposer, Poster, Account
// spec: Account, Account.name, Poster
func TestAddPoster_CLI_RejectsBlankAndOverlongNames(t *testing.T) {
	s := startServer(t)

	// The curator name is the one text no web form validates, and it lands on
	// every row this curator posts. Whitespace is not a name, and 120 is the
	// same ceiling an event title gets.
	for _, name := range []string{"   ", "\t\n "} {
		if out, err := runCLI(s, "add-poster", "-name", name); err == nil {
			t.Errorf("add-poster accepted the blank name %q\nstdout: %s", name, out)
		}
	}
	if out, err := runCLI(s, "add-poster", "-name", strings.Repeat("ω", 121)); err == nil {
		t.Errorf("add-poster accepted a 121-character name\nstdout: %s", out)
	}
	// 120 is inside the limit, and it is counted in runes: 120 Greek letters
	// are 240 bytes, and a byte-length check would have rejected them.
	p := addPoster(t, s, strings.Repeat("ω", 120), uniqSlug("longname"))
	assertStatus(t, newClient(t, s).get("/p/"+p.Slug), 200)

	// A name is trimmed rather than stored with its padding.
	p2 := addPoster(t, s, "  Padded P.  ", uniqSlug("padded"))
	r := newClient(t, s).get("/p/" + p2.Slug)
	assertStatus(t, r, 200)
	assertContains(t, r, ">Padded P.</h1>")
}

// spec: Event, Event.title, UpcomingAll
func TestEventRow_SurvivesExtremeTitles(t *testing.T) {
	p := asPoster(t, poster1)
	v := anon(t)

	// Real curators paste Greek, emoji and unbroken strings. None of these may
	// 500, and each has to come back as a reachable row.
	// Lengths leave room for the uniqueness suffix uniqTitle appends; the
	// 120-rune ceiling itself is covered by TestCreateEvent_ValidationErrors.
	for _, title := range []string{
		"Συναυλία στο Γκάζι 🎷 — «Μια Βραδιά»",
		strings.Repeat("Α", 100),
		"Ω" + strings.Repeat("ααα", 33),
	} {
		f := validEvent(t, tomorrow())
		f.Title = uniqTitle(t, title)
		slug := createEvent(t, p, f)
		r := v.get("/upcoming")
		assertStatus(t, r, 200)
		if rowFor(r.Body, slug) == "" {
			t.Errorf("/upcoming has no row for %q (slug %s)", title, slug)
		}
		assertStatus(t, v.get("/e/"+slug), 200)
	}
}

func TestAddPoster_CLI_PrintsLink_NoOpForExistingSlug(t *testing.T) {
	s := startServer(t)

	// A new poster: slug + link, the link logs in, the curator page exists.
	slug := uniqSlug("carol")
	p := addPoster(t, s, "Carol C.", slug)
	if p.Slug != slug {
		t.Errorf("add-poster -slug %s printed slug %q", slug, p.Slug)
	}
	if !strings.HasPrefix(p.Link, s.url+"/k/") {
		t.Errorf("link %q does not start with BASE_URL %s/k/", p.Link, s.url)
	}
	if len(p.Key) != 43 {
		t.Errorf("key %q has length %d, want 43", p.Key, len(p.Key))
	}
	r := newClient(t, s).get("/p/" + p.Slug)
	assertStatus(t, r, 200)
	assertContains(t, r, "Carol C.")
	c := openLink(t, s, p.Link)
	assertStatus(t, c.get("/new"), 200)
	createEvent(t, c, validEvent(t, tomorrow()))

	// Re-running for the same slug is a no-op: same poster line, no new link,
	// the old link still works and the name is not changed.
	out := addPosterRaw(t, s, "Somebody Else", slug)
	if m := posterLine.FindStringSubmatch(out); m == nil || m[1] != slug {
		t.Errorf("no-op add-poster printed %q, want 'poster %s'", strings.TrimSpace(out), slug)
	}
	if !strings.Contains(out, linkUnchanged) {
		t.Errorf("no-op add-poster stdout lacks %q:\n%s", linkUnchanged, out)
	}
	if linkKey.MatchString(out) {
		t.Errorf("no-op add-poster printed a key:\n%s", out)
	}
	assertStatus(t, openLink(t, s, p.Link).get("/new"), 200)
	r = newClient(t, s).get("/p/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, "Carol C.")
	assertNotContains(t, r, "Somebody Else")

	// Without -slug the slug derives from the name; the same name a second
	// time hits the existing slug and is the same no-op.
	d := addPoster(t, s, "Dave D.", "")
	if d.Slug == "" || d.Slug == slug {
		t.Fatalf("add-poster without -slug gave slug %q", d.Slug)
	}
	if !strings.HasPrefix(d.Slug, "dave") {
		t.Errorf("slug %q for \"Dave D.\" does not start with dave", d.Slug)
	}
	assertStatus(t, openLink(t, s, d.Link).get("/new"), 200)
	out = addPosterRaw(t, s, "Dave D.", "")
	if !strings.Contains(out, linkUnchanged) {
		t.Errorf("second add-poster for the same name printed:\n%s", out)
	}

	// Two posters never share a slug or a key; an explicit different slug
	// for the same name is a different poster.
	e := addPoster(t, s, "Dave D.", uniqSlug("dave"))
	if e.Slug == d.Slug || e.Key == d.Key {
		t.Errorf("posters %q and %q share slug or key", d.Slug, e.Slug)
	}
	assertStatus(t, newClient(t, s).get("/p/"+e.Slug), 200)
}

// spec: Event, Event.slug, CreateEvent, SlugsUnique
func TestResetEvents_CLI_ClearsEventsAndFreesSlugs(t *testing.T) {
	s := startServer(t)
	p := addPoster(t, s, "Reset R.", uniqSlug("reset"))
	c := openLink(t, s, p.Link)

	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Before Reset")
	slug := createEvent(t, c, f)
	v := newClient(t, s)
	assertStatus(t, v.get("/e/"+slug), 200)

	// Without -yes it refuses and changes nothing: this is the one command
	// that destroys data.
	out, err := runCLI(s, "reset-events")
	if err == nil {
		t.Errorf("reset-events ran without -yes\nstdout: %s", out)
	}
	assertStatus(t, v.get("/e/"+slug), 200)

	// Delete it through the app first: that retires the slug, so republishing
	// on its own would land on "<slug>-2" and change the public URL. This is
	// the case reset has to undo, and the reason it clears retired_slugs.
	assertRedirect(t, c.postForm("/e/"+slug+"/delete", url.Values{"confirm": {"1"}}), "/")
	assertStatus(t, v.get("/e/"+slug), 404)
	if got := createEvent(t, c, f); got == slug {
		t.Fatalf("slug %q was reused straight after a delete: it should have been retired", slug)
	}

	if out, err = runCLI(s, "reset-events", "-yes"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cleared 1 retired slugs") {
		t.Errorf("reset-events did not clear the retirement\nstdout: %s", out)
	}
	assertStatus(t, v.get("/e/"+slug), 404)

	// The curator survives with the same link, and republishing now reclaims
	// the original slug.
	c2 := openLink(t, s, p.Link)
	if got := createEvent(t, c2, f); got != slug {
		t.Errorf("republished event got slug %q, want the original %q", got, slug)
	}
	assertStatus(t, v.get("/e/"+slug), 200)
}
