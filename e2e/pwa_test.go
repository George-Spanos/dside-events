package e2e

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// spec: PwaAssets
func TestManifest_ContentTypeAndShape(t *testing.T) {
	r := anon(t).get("/manifest.webmanifest")
	assertStatus(t, r, 200)
	assertHeader(t, r, "Content-Type", "application/manifest+json")

	var m struct {
		Name     string `json:"name"`
		StartURL string `json:"start_url"`
		Display  string `json:"display"`
		Icons    []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		} `json:"icons"`
	}
	if err := json.Unmarshal([]byte(r.Body), &m); err != nil {
		t.Fatalf("manifest is not JSON: %v\n%s", err, snippet(r.Body))
	}
	if m.Name == "" || m.StartURL == "" || m.Display == "" {
		t.Errorf("manifest lacks name/start_url/display: %+v", m)
	}
	if !strings.HasPrefix(m.StartURL, "/") {
		t.Errorf("start_url = %q, want a local path", m.StartURL)
	}
	if len(m.Icons) == 0 {
		t.Fatalf("manifest has no icons")
	}
	v := anon(t)
	for _, ic := range m.Icons {
		if ic.Src == "" || ic.Sizes == "" || ic.Type == "" {
			t.Errorf("icon lacks src/sizes/type: %+v", ic)
			continue
		}
		ir := v.get(ic.Src)
		assertStatus(t, ir, 200)
		assertHeader(t, ir, "Content-Type", ic.Type)
	}
}

// spec: PwaAssets
func TestServiceWorker_ContentType(t *testing.T) {
	r := anon(t).get("/sw.js")
	assertStatus(t, r, 200)
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/javascript") && !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript or application/javascript", ct)
	}
	assertHeaderContains(t, r, "Cache-Control", "no-cache")
	assertContains(t, r, "'install'")
	assertContains(t, r, "'fetch'")
	assertContains(t, r, "/offline")
}

// spec: PwaAssets
func TestPages_DeclareManifestViewportAndSW(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	u := newUser(t)
	pages := []struct {
		name string
		r    resp
	}{
		{"/", anon(t).get("/")},
		{"/upcoming", anon(t).get("/upcoming")},
		{"/following", anon(t).get("/following")},
		{"/e/{slug}", anon(t).get("/e/" + slug)},
		{"/p/{slug}", anon(t).get("/p/" + poster1.Slug)},
		{"/mine", u.get("/mine")},
		{"/mine (anon)", anon(t).get("/mine")},
		{"/account", u.get("/account")},
		{"/account (anon)", anon(t).get("/account")},
		{"/offline", anon(t).get("/offline")},
		{"404", anon(t).get("/definitely-not-here")},
	}
	for _, p := range pages {
		if p.name != "404" {
			assertStatus(t, p.r, 200)
		}
		assertHeader(t, p.r, "Content-Type", "text/html")
		for _, want := range []string{`rel="manifest"`, `name="viewport"`, `theme-color`, `/static/app.js`, `rel="icon"`} {
			if !strings.Contains(p.r.Body, want) {
				t.Errorf("%s lacks %s", p.name, want)
			}
		}
		if p.name != "/offline" {
			assertHeaderContains(t, p.r, "Cache-Control", "no-store")
		}
	}
}

// spec: PwaAssets
func TestStaticCSS_Served(t *testing.T) {
	v := anon(t)
	r := v.get("/static/style.css")
	assertStatus(t, r, 200)
	assertHeader(t, r, "Content-Type", "text/css")

	r = v.get("/static/style.css?v=e2e")
	assertStatus(t, r, 200)
	assertHeaderContains(t, r, "Cache-Control", "immutable")

	r = v.get("/static/app.js")
	assertStatus(t, r, 200)
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/javascript") && !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("app.js Content-Type = %q", ct)
	}
	assertContains(t, r, "serviceWorker")

	// Pages reference the stylesheet.
	assertContains(t, v.get("/"), "/static/style.css")
	assertStatus(t, v.get("/static/nope.css"), 404)
}

// spec: PwaAssets
func TestIcons_Served(t *testing.T) {
	v := anon(t)
	r := v.get("/icon.svg")
	assertStatus(t, r, 200)
	assertHeader(t, r, "Content-Type", "image/svg+xml")
	assertContains(t, r, "<svg")
	for _, p := range []string{"/icon-180.png", "/icon-192.png", "/icon-512.png"} {
		r := v.get(p)
		assertStatus(t, r, 200)
		assertHeader(t, r, "Content-Type", "image/png")
		if !strings.HasPrefix(r.Body, "\x89PNG") {
			t.Errorf("%s is not a PNG", p)
		}
	}
}

// spec: PwaAssets
func TestHealthz(t *testing.T) {
	r := anon(t).get("/healthz")
	assertStatus(t, r, 200)
	assertHeader(t, r, "Content-Type", "text/plain")
	if strings.TrimSpace(r.Body) != "ok" {
		t.Errorf("body = %q, want ok", r.Body)
	}
}

// spec: PwaAssets
func TestNotFound_CustomPage(t *testing.T) {
	v := anon(t)
	for _, p := range []string{"/nope", "/e/", "/p/", "/static/", "/e/missing-2030-01-01/edit/extra"} {
		r := v.get(p)
		if r.Status != 404 {
			t.Errorf("GET %s: status %d, want 404", p, r.Status)
			continue
		}
		assertContains(t, r, "Page not found")
		assertContains(t, r, `href="/"`)
	}
}

// spec: PwaAssets, ForgetDevice, RotateSecretLink
func TestMethodNotAllowed(t *testing.T) {
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	u := newUser(t)
	for _, p := range []string{"/follow", "/account/delete", "/account/key", "/forget", "/e/" + slug + "/interest", "/e/" + slug + "/delete"} {
		r := u.get(p)
		if r.Status != 405 {
			t.Errorf("GET %s: status %d, want 405", p, r.Status)
		}
	}
	// Nothing happened as a side effect.
	assertStatus(t, u.get("/account"), 200)
	assertStatus(t, anon(t).get("/e/"+slug), 200)
}

// spec: PwaAssets, StartAccount, CreateEvent, EditEvent, DeleteEvent, MarkInterested, FollowTag, RotateSecretLink, ForgetDevice, DeleteAccount, OpenSecretLink
func TestNoJS_AllMutationsAre303Redirects(t *testing.T) {
	u := anon(t) // its first POST below starts the account
	p := asPoster(t, poster1)
	var slug string
	f := validEvent(t, tomorrow())

	steps := []struct {
		route string
		run   func() resp
	}{
		{"POST /new", func() resp {
			r := p.postForm("/new", f.values())
			slug = strings.TrimPrefix(r.Location, "/e/")
			return r
		}},
		{"POST /e/{slug}/edit", func() resp {
			g := f
			g.Venue = "Elsewhere"
			return p.postForm("/e/"+slug+"/edit", g.values())
		}},
		{"POST /e/{slug}/interest", func() resp { return setInterest(u, slug, "interested", "/e/"+slug) }},
		{"POST /follow", func() resp { return follow(u, "tag", "concert", "1", "/account") }},
		{"POST /account/key", func() resp { return u.postForm("/account/key", nil) }},
		{"POST /e/{slug}/delete", func() resp { return p.postForm("/e/"+slug+"/delete", nil) }},
		{"POST /forget", func() resp {
			// Forget a throwaway device rather than u, which still has to delete itself.
			return newUser(t).postForm("/forget", nil)
		}},
		{"POST /account/delete", func() resp { return u.postForm("/account/delete", url.Values{"confirm": {"1"}}) }},
		// The secret link is the one state-changing GET (it creates a session):
		// a plain <a href> a browser follows, so it redirects the same way.
		{"GET /k/{key}", func() resp { return anon(t).get(keyPath(poster2.Link)) }},
	}
	for _, s := range steps {
		r := s.run()
		if r.Status != 303 {
			t.Fatalf("%s: status %d, want 303\nbody: %s", s.route, r.Status, snippet(r.Body))
		}
		if r.Location == "" || !strings.HasPrefix(r.Location, "/") {
			t.Errorf("%s: Location = %q, want a local path", s.route, r.Location)
		}
		if ct := r.Header.Get("Content-Type"); strings.Contains(ct, "json") {
			t.Errorf("%s: Content-Type %q, want no JSON", s.route, ct)
		}
	}
}

// spec: PwaAssets, EventDetail, EventComposer, EventEditor, AccountPage, MyEvents, PosterPage, Home, UpcomingAll, SecretLink
func TestNoJS_FormsAreWellFormed(t *testing.T) {
	p := asPoster(t, poster1)
	slug := createEvent(t, p, validEvent(t, tomorrow()))
	hidden := createEvent(t, p, validEvent(t, tomorrow()))
	u := newUser(t)
	assertRedirect(t, setInterest(u, hidden, "not_interested", "/mine"), "/mine")
	assertRedirect(t, follow(u, "poster", poster1.Slug, "1", "/following"), "/following")

	pages := []struct {
		name string
		r    resp
	}{
		{"/e/{slug} (anon)", anon(t).get("/e/" + slug)},
		{"/e/{slug} (user)", u.get("/e/" + slug)},
		{"/e/{slug} (owner)", p.get("/e/" + slug)},
		{"/e/{slug} (hidden)", u.get("/e/" + hidden)},
		{"/new", p.get("/new")},
		{"/e/{slug}/edit", p.get("/e/" + slug + "/edit")},
		{"/account (user)", u.get("/account")},
		{"/account (poster)", p.get("/account")},
		{"/mine", u.get("/mine")},
		{"/p/{slug} (anon)", anon(t).get("/p/" + poster1.Slug)},
		{"/p/{slug} (user)", u.get("/p/" + poster1.Slug)},
		{"/ (anon)", anon(t).get("/")},
		{"/?tag=concert (user)", u.get("/?tag=concert")},
		{"/upcoming (user)", u.get("/upcoming")},
		{"/following (user)", u.get("/following")},
	}
	for _, pg := range pages {
		assertStatus(t, pg.r, 200)
		tags := formTags(pg.r.Body)
		if len(tags) == 0 {
			t.Errorf("%s has no <form>", pg.name)
			continue
		}
		for _, tag := range tags {
			lower := strings.ToLower(tag)
			if !strings.Contains(lower, `method="post"`) {
				t.Errorf("%s: form without method=\"post\": %s", pg.name, tag)
			}
			if !strings.Contains(lower, `action="/`) {
				t.Errorf("%s: form without a local action: %s", pg.name, tag)
			}
		}
	}
	// The secret link on /account is a plain anchor, not a form or script.
	r := u.get("/account")
	if !secretLinkHref.MatchString(r.Body) {
		t.Errorf("/account has no <a href=…/k/{key}> secret link")
	}
}
