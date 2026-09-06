package e2e

import (
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// client is one persona: its own cookie jar, no automatic redirects, no
// JavaScript-ish headers. Every request is what a plain browser form would send.
type client struct {
	t    testing.TB
	base *url.URL
	hc   *http.Client
	jar  *cookiejar.Jar
	s    *server
}

// resp is the part of an HTTP response the tests look at.
type resp struct {
	Status   int
	Header   http.Header
	Body     string
	Location string
}

func newClient(t testing.TB, s *server) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := newClientWithJar(t, s, jar)
	if s == shared {
		t.Cleanup(func() {
			if t.Failed() {
				shared.dumpNewLogs(t)
			}
		})
	}
	return c
}

func newClientWithJar(t testing.TB, s *server, jar *cookiejar.Jar) *client {
	t.Helper()
	base, err := url.Parse(s.url)
	if err != nil {
		t.Fatal(err)
	}
	return &client{
		t:    t,
		base: base,
		jar:  jar,
		s:    s,
		hc: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// anon is a fresh visitor of the shared server: no cookie, no account.
func anon(t testing.TB) *client {
	t.Helper()
	return newClient(t, shared)
}

// abs resolves path against the client's server. An absolute URL is taken
// as is.
func (c *client) abs(path string) string {
	u, err := url.Parse(path)
	if err != nil {
		c.t.Fatalf("bad path %q: %v", path, err)
	}
	return c.base.ResolveReference(u).String()
}

func (c *client) do(req *http.Request) resp {
	c.t.Helper()
	res, err := c.hc.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		c.t.Fatalf("%s %s: read body: %v", req.Method, req.URL, err)
	}
	return resp{Status: res.StatusCode, Header: res.Header, Body: string(b), Location: res.Header.Get("Location")}
}

func (c *client) get(path string) resp {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.abs(path), nil)
	if err != nil {
		c.t.Fatal(err)
	}
	return c.do(req)
}

// postForm submits application/x-www-form-urlencoded, like a plain <form>.
func (c *client) postForm(path string, form url.Values) resp {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodPost, c.abs(path), strings.NewReader(form.Encode()))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

// follow performs the GET a browser would do after r's redirect.
func (c *client) follow(r resp) resp {
	c.t.Helper()
	if r.Location == "" {
		c.t.Fatalf("follow: response %d has no Location", r.Status)
	}
	return c.get(r.Location)
}

// cookie returns the named cookie as the jar would send it for "/", or nil.
func (c *client) cookie(name string) *http.Cookie {
	u, _ := url.Parse(c.abs("/"))
	for _, ck := range c.jar.Cookies(u) {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

// setRawCookie plants a cookie in the jar (e.g. a replayed session token).
func (c *client) setRawCookie(name, value, path string) {
	u, _ := url.Parse(c.abs(path))
	c.jar.SetCookies(u, []*http.Cookie{{Name: name, Value: value, Path: path}})
}

// sessionSetCookie returns the Set-Cookie header for the session cookie, or "".
func sessionSetCookie(r resp) string {
	for _, sc := range r.Header.Values("Set-Cookie") {
		if strings.HasPrefix(sc, "session=") {
			return sc
		}
	}
	return ""
}

// ---- assertions -----------------------------------------------------------

var spaces = regexp.MustCompile(`\s+`)

// snippet condenses a body for failure messages.
func snippet(body string) string {
	s := spaces.ReplaceAllString(body, " ")
	if len(s) > 700 {
		s = s[:700] + "…"
	}
	return s
}

func assertStatus(t testing.TB, r resp, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status = %d, want %d (Location=%q)\nbody: %s", r.Status, want, r.Location, snippet(r.Body))
	}
}

// assertRedirect wants a 303 to exactly wantLocation.
func assertRedirect(t testing.TB, r resp, wantLocation string) {
	t.Helper()
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 → %s\nbody: %s", r.Status, wantLocation, snippet(r.Body))
	}
	if r.Location != wantLocation {
		t.Fatalf("Location = %q, want %q", r.Location, wantLocation)
	}
}

// assertRedirectPrefix wants a 303 whose Location starts with prefix.
func assertRedirectPrefix(t testing.TB, r resp, prefix string) {
	t.Helper()
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 → %s…\nbody: %s", r.Status, prefix, snippet(r.Body))
	}
	if !strings.HasPrefix(r.Location, prefix) {
		t.Fatalf("Location = %q, want prefix %q", r.Location, prefix)
	}
}

// assertRedirectPath wants a 303 whose Location path (ignoring the query)
// is wantPath.
func assertRedirectPath(t testing.TB, r resp, wantPath string) {
	t.Helper()
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 → %s\nbody: %s", r.Status, wantPath, snippet(r.Body))
	}
	u, err := url.Parse(r.Location)
	if err != nil || u.Path != wantPath {
		t.Fatalf("Location = %q, want path %q", r.Location, wantPath)
	}
}

func assertContains(t testing.TB, r resp, want string) {
	t.Helper()
	if !strings.Contains(r.Body, want) {
		t.Errorf("body lacks %q\nbody: %s", want, snippet(r.Body))
	}
}

func assertNotContains(t testing.TB, r resp, unwanted string) {
	t.Helper()
	if strings.Contains(r.Body, unwanted) {
		t.Errorf("body must not contain %q\nbody: %s", unwanted, snippet(r.Body))
	}
}

// plainText undoes HTML escaping and typographic apostrophes so copy with
// "don't" / "You're" matches however the template emitted it.
func plainText(body string) string {
	s := html.UnescapeString(body)
	s = strings.NewReplacer("’", "'", "‘", "'").Replace(s)
	return spaces.ReplaceAllString(s, " ")
}

// assertCopy wants the sentence want (from the contract's copy) in the body,
// ignoring HTML escaping, curly apostrophes and whitespace runs.
func assertCopy(t testing.TB, r resp, want string) {
	t.Helper()
	if !strings.Contains(plainText(r.Body), plainText(want)) {
		t.Errorf("body lacks copy %q\nbody: %s", want, snippet(r.Body))
	}
}

// assertNoCopy is the negation of assertCopy.
func assertNoCopy(t testing.TB, r resp, unwanted string) {
	t.Helper()
	if strings.Contains(plainText(r.Body), plainText(unwanted)) {
		t.Errorf("body must not contain copy %q\nbody: %s", unwanted, snippet(r.Body))
	}
}

// assertHeader wants header key to start with wantPrefix.
func assertHeader(t testing.TB, r resp, key, wantPrefix string) {
	t.Helper()
	if got := r.Header.Get(key); !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("header %s = %q, want prefix %q", key, got, wantPrefix)
	}
}

// assertHeaderContains wants header key to contain want.
func assertHeaderContains(t testing.TB, r resp, key, want string) {
	t.Helper()
	if got := r.Header.Get(key); !strings.Contains(got, want) {
		t.Errorf("header %s = %q, want it to contain %q", key, got, want)
	}
}

// assertBefore wants a to appear in the body before b.
func assertBefore(t testing.TB, r resp, a, b string) {
	t.Helper()
	ia, ib := strings.Index(r.Body, a), strings.Index(r.Body, b)
	if ia < 0 || ib < 0 {
		t.Errorf("body lacks %q (at %d) or %q (at %d)\nbody: %s", a, ia, b, ib, snippet(r.Body))
		return
	}
	if ia > ib {
		t.Errorf("%q (at %d) should come before %q (at %d)", a, ia, b, ib)
	}
}

// assertSessionCookieFlags checks the Set-Cookie header that starts a
// session: HttpOnly, SameSite=Lax, Path=/ and a 365-day lifetime.
func assertSessionCookieFlags(t testing.TB, r resp) {
	t.Helper()
	sc := sessionSetCookie(r)
	if sc == "" {
		t.Fatalf("no Set-Cookie for session; headers: %v", r.Header)
	}
	for _, want := range []string{"HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(sc, want) {
			t.Errorf("Set-Cookie %q lacks %s", sc, want)
		}
	}
	if !strings.Contains(sc, "Max-Age=31536000") && !strings.Contains(sc, "Expires=") {
		t.Errorf("Set-Cookie %q is not persistent (want Max-Age=31536000 or Expires)", sc)
	}
}

// ---- HTML helpers -----------------------------------------------------------

// forms splits body into its <form …>…</form> chunks.
func forms(body string) []string {
	var out []string
	rest := body
	for {
		i := strings.Index(rest, "<form")
		if i < 0 {
			return out
		}
		rest = rest[i:]
		end := strings.Index(rest, "</form>")
		if end < 0 {
			return append(out, rest)
		}
		out = append(out, rest[:end+len("</form>")])
		rest = rest[end+len("</form>"):]
	}
}

// hasForm reports whether some form in body contains every fragment.
func hasForm(body string, fragments ...string) bool {
	for _, f := range forms(body) {
		ok := true
		for _, frag := range fragments {
			if !strings.Contains(f, frag) {
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

func assertForm(t testing.TB, r resp, fragments ...string) {
	t.Helper()
	if !hasForm(r.Body, fragments...) {
		t.Errorf("no <form> containing all of %q\nbody: %s", fragments, snippet(r.Body))
	}
}

func assertNoForm(t testing.TB, r resp, fragments ...string) {
	t.Helper()
	if hasForm(r.Body, fragments...) {
		t.Errorf("unexpected <form> containing all of %q\nbody: %s", fragments, snippet(r.Body))
	}
}

var hrefAttr = regexp.MustCompile(`href="([^"]*)"`)

// hasLinkToPath reports whether body links to a URL whose path is wantPath.
func hasLinkToPath(body, wantPath string) bool {
	for _, m := range hrefAttr.FindAllStringSubmatch(body, -1) {
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err == nil && u.Path == wantPath {
			return true
		}
	}
	return false
}

var formTag = regexp.MustCompile(`(?is)<form\b[^>]*>`)

// formTags returns every opening <form …> tag in body.
func formTags(body string) []string {
	return formTag.FindAllString(body, -1)
}

// containsFold is a case-insensitive contains.
func containsFold(body, want string) bool {
	return strings.Contains(strings.ToLower(body), strings.ToLower(want))
}

// ---- lists and rows ---------------------------------------------------------

// assertListed wants the event titled title on the list page at path: an
// upcoming list such as /upcoming, /upcoming?tag=x or /following. "/" is not
// a list page for this purpose: it shows only the next ten events and the
// shared server accumulates events across tests, so a title created here
// may legitimately be beyond the tenth.
func assertListed(t testing.TB, c *client, path, title string) {
	t.Helper()
	r := c.get(path)
	assertStatus(t, r, 200)
	if !strings.Contains(r.Body, title) {
		t.Errorf("%s does not list %q\nbody: %s", path, title, snippet(r.Body))
	}
}

// assertNotListed is the negation of assertListed.
func assertNotListed(t testing.TB, c *client, path, title string) {
	t.Helper()
	r := c.get(path)
	assertStatus(t, r, 200)
	if strings.Contains(r.Body, title) {
		t.Errorf("%s lists %q, must not\nbody: %s", path, title, snippet(r.Body))
	}
}

var (
	listItem   = regexp.MustCompile(`(?s)<li\b[^>]*>.*?</li>`)
	eventsList = regexp.MustCompile(`(?s)<ul class="events">.*?</ul>`)
	rowSlug    = regexp.MustCompile(`href="/e/([a-z0-9-]+)"`)
	sectionTag = regexp.MustCompile(`(?s)<section class="(mine|upcoming)">.*?</section>`)
)

// eventRows returns every <li>…</li> inside a <ul class="events"> list: the
// upcoming rows of a page, never the past ones (<ul class="past">).
func eventRows(body string) []string {
	var out []string
	for _, ul := range eventsList.FindAllString(body, -1) {
		out = append(out, listItem.FindAllString(ul, -1)...)
	}
	return out
}

// rowsFor returns every <li> in body (any list) linking to /e/slug.
func rowsFor(body, slug string) []string {
	var out []string
	for _, li := range listItem.FindAllString(body, -1) {
		if strings.Contains(li, `href="/e/`+slug+`"`) {
			out = append(out, li)
		}
	}
	return out
}

// rowFor returns the first <li> in body linking to /e/slug, or "".
func rowFor(body, slug string) string {
	if rs := rowsFor(body, slug); len(rs) > 0 {
		return rs[0]
	}
	return ""
}

// slugOf returns the event slug a row links to, or "".
func slugOf(row string) string {
	if m := rowSlug.FindStringSubmatch(row); m != nil {
		return m[1]
	}
	return ""
}

// section returns the home page's <section class="name">…</section>, or "".
func section(body, name string) string {
	for _, s := range sectionTag.FindAllString(body, -1) {
		if strings.HasPrefix(s, `<section class="`+name+`">`) {
			return s
		}
	}
	return ""
}
