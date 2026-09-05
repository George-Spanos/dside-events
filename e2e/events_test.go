package e2e

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var slugShape = regexp.MustCompile(`^[a-z0-9-]+-\d{4}-\d{2}-\d{2}(-\d+)?$`)

// spec: CreateEvent, EventComposer, Poster, Event.PostedByPoster
func TestNewEvent_UserForbidden(t *testing.T) {
	u := loginAs(t, uniqEmail(t, "alice"))
	assertStatus(t, u.get("/new"), 403)
	f := validEvent(t, tomorrow())
	assertStatus(t, u.postForm("/new", f.values()), 403)
	assertNotContains(t, anon(t).get("/"), f.Title)
}

// spec: CreateEvent, EventComposer, EventDetail, PublicFeed, PosterPage
func TestCreateEvent_Success(t *testing.T) {
	p := asPoster(t, poster1)
	r := p.get("/new")
	assertStatus(t, r, 200)
	assertForm(t, r, `action="/new"`, `name="title"`)
	// max_links is enforced by the form itself: exactly five link slots, and
	// the contract has no sixth field.
	for i := 1; i <= 5; i++ {
		assertContains(t, r, fmt.Sprintf(`name="link_url_%d"`, i))
		assertContains(t, r, fmt.Sprintf(`name="link_label_%d"`, i))
	}
	assertNotContains(t, r, `name="link_url_6"`)
	// max_tags is a server rule (see the validation table); every tag is offered.
	for _, tag := range []string{"concert", "theater", "film", "exhibition", "talk", "party", "dance", "workshop"} {
		assertContains(t, r, `value="`+tag+`"`)
	}

	f := validEvent(t, tomorrow())
	f.Tags = []string{"concert", "party"}
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
	assertContains(t, r, `href="/?tag=party"`)
	assertContains(t, r, `href="`+f.LinkURLs[0]+`"`)
	assertContains(t, r, poster1.Name)

	v := anon(t)
	assertContains(t, v.get("/"), f.Title)
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
		{"five tags", func(f *eventForm) { f.Tags = []string{"concert", "theater", "film", "talk", "party"} }},
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
				assertNotContains(t, anon(t).get("/"), f.Title)
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

	// The original is untouched and the feed shows it once.
	r = anon(t).get("/")
	assertStatus(t, r, 200)
	if n := strings.Count(r.Body, `href="/e/`+slug+`"`); n != 1 {
		t.Errorf("feed links to /e/%s %d times, want 1", slug, n)
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

// spec: EditEvent, EventEditor, EventDetail, PublicFeed
func TestEditEvent_OwnerEdits_SlugImmutable(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)

	r := p.get("/e/" + slug + "/edit")
	assertStatus(t, r, 200)
	assertContains(t, r, `value="`+f.Title+`"`)
	assertContains(t, r, `value="`+f.Venue+`"`)
	assertContains(t, r, `value="`+f.Date+`"`)
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

	feed := anon(t).get("/")
	assertContains(t, feed, edited.Title)
	assertNotContains(t, feed, f.Title)
	if n := strings.Count(feed.Body, `href="/e/`+slug+`"`); n != 1 {
		t.Errorf("feed links to /e/%s %d times, want 1", slug, n)
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
	u := loginAs(t, uniqEmail(t, "alice"))
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
func TestDeleteEvent_OwnerDeletes_CascadesInterest(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	slug := createEvent(t, p, f)
	alice := loginAs(t, uniqEmail(t, "alice"))
	assertRedirect(t, setInterest(alice, slug, "interested", "/e/"+slug), "/e/"+slug)
	assertContains(t, alice.get("/mine"), f.Title)

	assertRedirect(t, p.postForm("/e/"+slug+"/delete", nil), "/")

	v := anon(t)
	assertStatus(t, v.get("/e/"+slug), 404)
	assertStatus(t, p.get("/e/"+slug+"/edit"), 404)
	assertNotContains(t, v.get("/"), f.Title)
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
	assertStatus(t, loginAs(t, uniqEmail(t, "alice")).postForm("/e/"+slug+"/delete", nil), 403)
	assertStatus(t, asPoster(t, poster1).postForm("/e/no-such-event-2030-01-01/delete", nil), 404)
	r := anon(t).get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, f.Title)
}

// spec: EventDetailForUser, EventEditor
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
		"user":         loginAs(t, uniqEmail(t, "alice")),
		"anon":         anon(t),
	} {
		r := c.get("/e/" + slug)
		assertStatus(t, r, 200)
		if strings.Contains(r.Body, edit) || strings.Contains(r.Body, del) {
			t.Errorf("%s sees edit/delete controls on /e/%s", name, slug)
		}
	}
}

// spec: AddPoster, PromoteToPoster, PosterSlugsUnique, PosterPage
func TestAddPoster_CLI_PromotesAndIdempotent(t *testing.T) {
	email := uniqEmail(t, "carol")
	c := loginAs(t, email)
	assertStatus(t, c.get("/new"), 403)
	assertRedirect(t, follow(c, "tag", "film", "1", "/account"), "/account")

	// Promote while the server is running; the existing session sees it at once.
	p := addPoster(t, shared, email, "Carol C.")
	if p.Slug == "" {
		t.Fatalf("empty slug from add-poster")
	}
	assertStatus(t, c.get("/new"), 200)
	r := anon(t).get("/p/" + p.Slug)
	assertStatus(t, r, 200)
	assertContains(t, r, "Carol C.")
	// Follows survive the promotion.
	assertForm(t, c.get("/account"), `action="/follow"`, `value="tag"`, `value="film"`, `value="0"`)

	// Idempotent: same email → same slug.
	again := addPoster(t, shared, email, "Carol C.")
	if again.Slug != p.Slug {
		t.Errorf("second add-poster slug %q, want %q", again.Slug, p.Slug)
	}

	// A brand-new poster: creates the account, can post immediately.
	fresh := addPoster(t, shared, uniqEmail(t, "dave"), "Dave D.")
	assertStatus(t, anon(t).get("/p/"+fresh.Slug), 200)
	d := loginAs(t, fresh.Email)
	createEvent(t, d, validEvent(t, tomorrow()))

	// Slugs stay unique even when names collide.
	twin := addPoster(t, shared, uniqEmail(t, "twin"), poster1.Name)
	if twin.Slug == poster1.Slug {
		t.Errorf("two posters named %q share slug %q", poster1.Name, twin.Slug)
	}
	assertStatus(t, anon(t).get("/p/"+twin.Slug), 200)
}
