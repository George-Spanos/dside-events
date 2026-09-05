package e2e

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	athens = mustLocation("Europe/Athens")
	seq    atomic.Int64
	epoch  = time.Now().UnixNano() % 1_000_000_000
)

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// uid is short and unique within one run (and very likely across runs).
func uid() string {
	return strconv.FormatInt(epoch, 36) + strconv.FormatInt(seq.Add(1), 36)
}

// uniqEmail is `prefix-<test>-<id>@example.test`, lower-case and unique.
func uniqEmail(t testing.TB, prefix string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(t.Name()) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 24 {
		name = name[len(name)-24:]
	}
	return fmt.Sprintf("%s-%s-%s@example.test", prefix, name, uid())
}

// uniqTitle is `<base> <id>`: plain ASCII letters and digits, unique per call.
func uniqTitle(t testing.TB, base string) string {
	t.Helper()
	return base + " " + strings.ToUpper(uid())
}

// daysFromNow is the Athens calendar day n days from today as 2006-01-02.
func daysFromNow(n int) string {
	return time.Now().In(athens).AddDate(0, 0, n).Format("2006-01-02")
}

func tomorrow() string  { return daysFromNow(1) }
func yesterday() string { return daysFromNow(-1) }

// ---- login ------------------------------------------------------------------

// normalise mirrors the server's email normalisation for locating OTP lines.
func normalise(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// requestCode posts the first login step.
func requestCode(c *client, email, next string) resp {
	c.t.Helper()
	form := url.Values{"email": {email}}
	if next != "" {
		form.Set("next", next)
	}
	return c.postForm("/login", form)
}

// verifyCode posts the second login step.
func verifyCode(c *client, code, next string) resp {
	c.t.Helper()
	form := url.Values{"code": {code}}
	if next != "" {
		form.Set("next", next)
	}
	return c.postForm("/login/code", form)
}

// loginOn runs the whole OTP flow for email against s and returns a
// logged-in client.
func loginOn(t testing.TB, s *server, email string) *client {
	t.Helper()
	c := newClient(t, s)
	before := otpLineCount(s, normalise(email))
	r := requestCode(c, email, "")
	assertRedirectPath(t, r, "/login/code")
	code := waitOTP(t, s, normalise(email), before+1)
	r = verifyCode(c, code, "")
	assertStatus(t, r, 303)
	if c.cookie("session") == nil {
		t.Fatalf("login as %s: no session cookie after 303", email)
	}
	return c
}

// loginAs logs email in on the shared server.
func loginAs(t testing.TB, email string) *client {
	t.Helper()
	return loginOn(t, shared, email)
}

var (
	posterJarsMu sync.Mutex
	posterJars   = map[string]*client{}
)

// asPoster returns a client logged in as p on the shared server. The session
// is created once per run and reused so the OTP rate limit never bites.
func asPoster(t testing.TB, p poster) *client {
	t.Helper()
	posterJarsMu.Lock()
	defer posterJarsMu.Unlock()
	if c, ok := posterJars[p.Email]; ok {
		return newClientWithJar(t, shared, c.jar)
	}
	c := loginAs(t, p.Email)
	posterJars[p.Email] = c
	return newClientWithJar(t, shared, c.jar)
}

// ---- events -----------------------------------------------------------------

// eventForm mirrors the /new and /edit form fields.
type eventForm struct {
	Title, Date, Time, Venue, Price, Description string
	Tags                                         []string
	LinkLabels, LinkURLs                         []string
}

func (f eventForm) values() url.Values {
	v := url.Values{}
	v.Set("title", f.Title)
	v.Set("date", f.Date)
	v.Set("time", f.Time)
	v.Set("venue", f.Venue)
	v.Set("price", f.Price)
	v.Set("description", f.Description)
	for _, tg := range f.Tags {
		v.Add("tag", tg)
	}
	n := len(f.LinkLabels)
	if len(f.LinkURLs) > n {
		n = len(f.LinkURLs)
	}
	for i := 0; i < n; i++ {
		if i < len(f.LinkLabels) {
			v.Set(fmt.Sprintf("link_label_%d", i+1), f.LinkLabels[i])
		}
		if i < len(f.LinkURLs) {
			v.Set(fmt.Sprintf("link_url_%d", i+1), f.LinkURLs[i])
		}
	}
	return v
}

// validEvent is a complete, valid event on date with a unique title.
func validEvent(t testing.TB, date string) eventForm {
	t.Helper()
	id := uid()
	return eventForm{
		Title:       uniqTitle(t, "Concert"),
		Date:        date,
		Time:        "20:30",
		Venue:       "Gazarte",
		Price:       "10 EUR",
		Description: "Doors open at eight. Bring a friend.",
		Tags:        []string{"concert"},
		LinkLabels:  []string{"Listen"},
		LinkURLs:    []string{"https://example.test/listen/" + id},
	}
}

var eventLocation = regexp.MustCompile(`^/e/([a-z0-9-]+)$`)

// createEvent posts f to /new as c and returns the new slug.
func createEvent(t testing.TB, c *client, f eventForm) string {
	t.Helper()
	r := c.postForm("/new", f.values())
	if r.Status != 303 {
		t.Fatalf("create %q: status %d, want 303\nbody: %s", f.Title, r.Status, snippet(r.Body))
	}
	m := eventLocation.FindStringSubmatch(r.Location)
	if m == nil {
		t.Fatalf("create %q: Location %q does not match /e/{slug}", f.Title, r.Location)
	}
	return m[1]
}

// ---- interest ---------------------------------------------------------------

var countRe = regexp.MustCompile(`(\d+) (?:were |are )?interested`)

// interestedCount reads the public counter from an event page body:
// "Nobody yet interested" → 0, "N interested" / "N were interested" → N.
func interestedCount(t testing.TB, body string) int {
	t.Helper()
	if strings.Contains(body, "Nobody yet interested") {
		return 0
	}
	m := countRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no interested counter in body: %s", snippet(body))
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// setInterest posts state for slug as c with back and returns the response.
func setInterest(c *client, slug, state, back string) resp {
	c.t.Helper()
	form := url.Values{"state": {state}}
	if back != "" {
		form.Set("back", back)
	}
	return c.postForm("/e/"+slug+"/interest", form)
}

// follow posts a follow/unfollow toggle.
func follow(c *client, kind, key, on, back string) resp {
	c.t.Helper()
	form := url.Values{"kind": {kind}, "key": {key}, "on": {on}}
	if back != "" {
		form.Set("back", back)
	}
	return c.postForm("/follow", form)
}
