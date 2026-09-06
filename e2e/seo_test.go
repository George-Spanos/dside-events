package e2e

// What search engines are served: robots.txt, sitemap.xml, canonical links,
// meta descriptions, noindex on the private pages, and the schema.org Event
// that lets Google list an event on its own.

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var (
	canonicalHref = regexp.MustCompile(`<link rel="canonical" href="([^"]+)">`)
	metaDesc      = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	jsonLDBlock   = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	sitemapLoc    = regexp.MustCompile(`<loc>([^<]+)</loc>`)
)

// canonicalOf is the canonical URL the page names, or "" when it names none.
func canonicalOf(t testing.TB, r resp) string {
	t.Helper()
	m := canonicalHref.FindStringSubmatch(r.Body)
	if m == nil {
		return ""
	}
	return m[1]
}

// eventLD is the parsed schema.org Event of a page, or nil when it carries none.
func eventLD(t testing.TB, r resp) map[string]any {
	t.Helper()
	m := jsonLDBlock.FindStringSubmatch(r.Body)
	if m == nil {
		return nil
	}
	var ld map[string]any
	if err := json.Unmarshal([]byte(m[1]), &ld); err != nil {
		t.Fatalf("JSON-LD is not valid JSON: %v\n%s", err, m[1])
	}
	return ld
}

func TestRobots_AllowsCrawling_AndNamesTheSitemap(t *testing.T) {
	v := anon(t)
	r := v.get("/robots.txt")
	assertStatus(t, r, 200)
	assertHeader(t, r, "Content-Type", "text/plain")
	assertContains(t, r, "User-agent: *")
	assertContains(t, r, "Allow: /")
	assertContains(t, r, "Sitemap: ")
	assertContains(t, r, "/sitemap.xml")
	// Secret links are the one thing no crawler should ever fetch.
	assertContains(t, r, "Disallow: /k/")
	// The noindex pages must stay crawlable, or nobody reads their noindex.
	assertNotContains(t, r, "Disallow: /account")
	assertNotContains(t, r, "Disallow: /mine")
}

func TestSitemap_ListsTheListsTagsAndEvents(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Sitemap Event")
	slug := createEvent(t, p, f)

	v := anon(t)
	r := v.get("/sitemap.xml")
	assertStatus(t, r, 200)
	assertHeaderContains(t, r, "Content-Type", "xml")
	assertContains(t, r, `<?xml version="1.0"`)
	assertContains(t, r, "http://www.sitemaps.org/schemas/sitemap/0.9")

	locs := map[string]bool{}
	for _, m := range sitemapLoc.FindAllStringSubmatch(r.Body, -1) {
		locs[m[1]] = true
	}
	for _, want := range []string{"/", "/upcoming", "/upcoming?tag=concert",
		"/upcoming?tag=theater", "/upcoming?tag=film", "/upcoming?tag=exhibition", "/e/" + slug} {
		if !locs[v.abs(want)] {
			t.Errorf("sitemap has no <loc> for %q", want)
		}
	}
	// Absolute URLs only: a relative <loc> is invalid.
	for loc := range locs {
		if !strings.HasPrefix(loc, "http") {
			t.Errorf("sitemap <loc> %q is not absolute", loc)
		}
	}
}

func TestSitemap_SkipsThePagesThatPointElsewhere(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	f.Title = uniqTitle(t, "Sitemap Series")
	_, all := createSeries(t, p, f)
	if len(all) != 3 {
		t.Fatalf("series has %d dates, want 3", len(all))
	}

	v := anon(t)
	r := v.get("/sitemap.xml")
	assertStatus(t, r, 200)
	var listed []string
	for _, slug := range all {
		if strings.Contains(r.Body, "<loc>"+v.abs("/e/"+slug)+"</loc>") {
			listed = append(listed, slug)
		}
	}
	// One date stands for the run; the other two hand it their claim, so a
	// sitemap that carries canonical URLs only lists exactly one of them.
	if len(listed) != 1 {
		t.Errorf("sitemap lists %d of the 3 series dates (%v), want exactly the canonical one", len(listed), listed)
	}
	if len(listed) == 1 && listed[0] != all[0] {
		t.Errorf("sitemap lists %s, want the next upcoming date %s", listed[0], all[0])
	}
}

func TestEventPage_IsSelfCanonical_AndDescribesItself(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Canonical Event")
	f.Venue = "Floyd, Gazi"
	f.Price = "12 EUR"
	slug := createEvent(t, p, f)

	v := anon(t)
	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	if got, want := canonicalOf(t, r), v.abs("/e/"+slug); got != want {
		t.Errorf("canonical is %q, want %q", got, want)
	}
	// The title carries what people type: the event, the venue, the date.
	assertContains(t, r, "<title>"+f.Title+" · Floyd · ")
	m := metaDesc.FindStringSubmatch(r.Body)
	if m == nil {
		t.Fatal("event page has no meta description")
	}
	if !strings.Contains(m[1], "Floyd") || !strings.Contains(m[1], "12 EUR") {
		t.Errorf("meta description %q does not lead with the venue and price", m[1])
	}
}

func TestEventPage_StructuredDataDescribesTheEvent(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Structured Event")
	f.Venue = "Gazarte, Gazi"
	f.Price = "18€"
	slug := createEvent(t, p, f)

	v := anon(t)
	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	ld := eventLD(t, r)
	if ld == nil {
		t.Fatal("event page carries no JSON-LD")
	}
	if ld["@type"] != "Event" {
		t.Errorf("@type is %v, want Event", ld["@type"])
	}
	if ld["name"] != f.Title {
		t.Errorf("name is %v, want %q", ld["name"], f.Title)
	}
	if ld["url"] != v.abs("/e/"+slug) {
		t.Errorf("url is %v, want the canonical URL", ld["url"])
	}
	if start, _ := ld["startDate"].(string); !strings.HasPrefix(start, f.Date) {
		t.Errorf("startDate is %v, want it to start with %q", ld["startDate"], f.Date)
	}
	loc, _ := ld["location"].(map[string]any)
	if loc == nil || loc["name"] != f.Venue {
		t.Errorf("location is %v, want a Place named %q", ld["location"], f.Venue)
	}
	addr, _ := loc["address"].(map[string]any)
	if addr == nil || addr["addressLocality"] != "Athens" || addr["addressCountry"] != "GR" {
		t.Errorf("address is %v, want Athens, GR", loc["address"])
	}
	offers, _ := ld["offers"].(map[string]any)
	if offers == nil || offers["price"] != "18" || offers["priceCurrency"] != "EUR" {
		t.Errorf("offers is %v, want price 18 EUR", ld["offers"])
	}
}

func TestEventPage_UnreadablePriceShipsWithoutAnOffer(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Range Price Event")
	f.Price = "8-12 EUR"
	slug := createEvent(t, p, f)

	v := anon(t)
	r := v.get("/e/" + slug)
	assertStatus(t, r, 200)
	ld := eventLD(t, r)
	if ld == nil {
		t.Fatal("event page carries no JSON-LD")
	}
	// A range is not a price. Guessing one would put a number in front of
	// searchers that the page never claimed.
	if _, ok := ld["offers"]; ok {
		t.Errorf("offers is %v, want none for a price range", ld["offers"])
	}
	// The visible page still shows what the curator wrote.
	assertContains(t, r, "8-12 EUR")
}

func TestSeriesDates_PointAtOnePage(t *testing.T) {
	p := asPoster(t, poster1)
	f := validSeries(t, 3)
	f.Title = uniqTitle(t, "Canonical Series")
	first, all := createSeries(t, p, f)

	v := anon(t)
	want := v.abs("/e/" + first)
	for _, slug := range all {
		r := v.get("/e/" + slug)
		assertStatus(t, r, 200)
		if got := canonicalOf(t, r); got != want {
			t.Errorf("/e/%s canonical is %q, want %q", slug, got, want)
		}
		// Only the page that stands for the run makes the schema.org claim,
		// so the dates do not compete with each other for the same event.
		ld := eventLD(t, r)
		if slug == first && ld == nil {
			t.Errorf("/e/%s is the canonical date but carries no JSON-LD", slug)
		}
		if slug != first && ld != nil {
			t.Errorf("/e/%s points at %s but still claims to be the event", slug, first)
		}
	}
}

func TestTagPages_AreLandingPagesForTheirQuery(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Tagged Gig")
	f.Tags = []string{"concert"}
	createEvent(t, p, f)

	v := anon(t)
	r := v.get("/upcoming?tag=concert")
	assertStatus(t, r, 200)
	assertContains(t, r, "<title>Concerts in Athens · dside events</title>")
	// The full tag list is the page that should rank, not a truncated view
	// of it, so it names itself.
	if got, want := canonicalOf(t, r), v.abs("/upcoming?tag=concert"); got != want {
		t.Errorf("/upcoming?tag=concert canonical is %q, want %q", got, want)
	}
	if m := metaDesc.FindStringSubmatch(r.Body); m == nil || !strings.Contains(m[1], "Athens") {
		t.Errorf("tag page description is %v, want it to mention Athens", m)
	}

	// Home with the same tag shows the first ten of that list, so it hands
	// its claim over rather than competing.
	r = v.get("/?tag=concert")
	assertStatus(t, r, 200)
	if got, want := canonicalOf(t, r), v.abs("/upcoming?tag=concert"); got != want {
		t.Errorf("/?tag=concert canonical is %q, want %q", got, want)
	}
}

func TestListPages_NameThemselves(t *testing.T) {
	v := anon(t)
	for path, want := range map[string]string{
		"/":         "/",
		"/upcoming": "/upcoming",
	} {
		r := v.get(path)
		assertStatus(t, r, 200)
		if got := canonicalOf(t, r); got != v.abs(want) {
			t.Errorf("%s canonical is %q, want %q", path, got, v.abs(want))
		}
		if metaDesc.FindStringSubmatch(r.Body) == nil {
			t.Errorf("%s has no meta description", path)
		}
	}
	assertContains(t, v.get("/"), "<title>Events in Athens · dside events</title>")
}

func TestPrivatePages_StayOutOfSearch(t *testing.T) {
	v := anon(t)
	for _, path := range []string{"/account", "/mine", "/offline"} {
		r := v.get(path)
		assertStatus(t, r, 200)
		assertContains(t, r, `<meta name="robots" content="noindex, follow">`)
		if c := canonicalOf(t, r); c != "" {
			t.Errorf("%s names a canonical URL (%q); a noindex page should not", path, c)
		}
	}
	// A curator's own pages are private too.
	p := asPoster(t, poster1)
	assertContains(t, p.get("/new"), `<meta name="robots" content="noindex, follow">`)

	// The public pages must not carry it.
	assertNotContains(t, v.get("/"), `content="noindex`)
	assertNotContains(t, v.get("/upcoming"), `content="noindex`)
}

func TestErrorPages_StayOutOfSearch(t *testing.T) {
	v := anon(t)
	r := v.get("/no-such-page")
	assertStatus(t, r, 404)
	assertContains(t, r, `<meta name="robots" content="noindex, follow">`)
	assertContains(t, r, "<title>Page not found · dside events</title>")
}

func TestPosterPage_DescribesTheCurator(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Curated Event")
	createEvent(t, p, f)

	v := anon(t)
	r := v.get("/p/" + poster1.Slug)
	assertStatus(t, r, 200)
	if got, want := canonicalOf(t, r), v.abs("/p/"+poster1.Slug); got != want {
		t.Errorf("poster page canonical is %q, want %q", got, want)
	}
	if m := metaDesc.FindStringSubmatch(r.Body); m == nil || !strings.Contains(m[1], poster1.Name) {
		t.Errorf("poster page description is %v, want it to name the curator", m)
	}
}
