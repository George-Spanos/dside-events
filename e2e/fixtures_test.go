package e2e

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
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

// uniqTitle is `<base> <id>`: plain ASCII letters and digits, unique per call.
func uniqTitle(t testing.TB, base string) string {
	t.Helper()
	return base + " " + strings.ToUpper(uid())
}

// uniqSlug is a lower-case poster slug unique per call (for `add-poster -slug`).
func uniqSlug(base string) string {
	return base + "-" + strings.ToLower(uid())
}

// daysFromNow is the Athens calendar day n days from today as 2006-01-02.
func daysFromNow(n int) string {
	return time.Now().In(athens).AddDate(0, 0, n).Format("2006-01-02")
}

func tomorrow() string  { return daysFromNow(1) }
func yesterday() string { return daysFromNow(-1) }

// ---- accounts ---------------------------------------------------------------

// secretLinkHref finds the account page's secret link: any href ending in
// /k/<43-char key>.
var secretLinkHref = regexp.MustCompile(`href="([^"]*/k/(` + keyPattern + `))"`)

// keyPath is the local path of a secret link (`/k/<key>`), so a link printed
// against any BASE_URL can be opened on the server under test.
func keyPath(link string) string {
	m := linkKey.FindStringSubmatch(link)
	if m == nil {
		return link
	}
	return "/k/" + m[1]
}

// openLink opens the secret link (or bare key path) on server s in a fresh
// client and asserts the contract: 303 /mine plus a session cookie.
func openLink(t testing.TB, s *server, link string) *client {
	t.Helper()
	c := newClient(t, s)
	r := c.get(keyPath(link))
	assertRedirect(t, r, "/mine")
	assertSessionCookieFlags(t, r)
	if c.cookie("session") == nil {
		t.Fatalf("open %s: no session cookie in jar after 303", keyPath(link))
	}
	return c
}

// asPoster returns a fresh client logged in as p on the shared server by
// opening p's secret link.
func asPoster(t testing.TB, p poster) *client {
	t.Helper()
	return openLink(t, shared, p.Link)
}

// newUser returns a fresh client on the shared server whose account was just
// created by its first action, an event follow, then cleared so the account
// follows nothing and has marked nothing.
func newUser(t testing.TB) *client {
	t.Helper()
	c := newClient(t, shared)
	slug := createEvent(t, asPoster(t, poster1), validEvent(t, tomorrow()))
	r := setEventFollow(c, slug, "follow", "/")
	assertRedirect(t, r, "/")
	if c.cookie("session") == nil {
		t.Fatalf("first POST /e/{slug}/follow did not start an account (no session cookie)")
	}
	assertRedirect(t, setEventFollow(c, slug, "clear", "/"), "/")
	return c
}

// secretLink reads the visitor's secret link from /account and returns the
// full href and its key.
func secretLink(t testing.TB, c *client) (link, key string) {
	t.Helper()
	r := c.get("/account")
	assertStatus(t, r, 200)
	m := secretLinkHref.FindStringSubmatch(r.Body)
	if m == nil {
		t.Fatalf("/account shows no secret link (href …/k/<43-char key>)\nbody: %s", snippet(r.Body))
	}
	return m[1], m[2]
}

// randomKey is a well-formed key that belongs to nobody.
func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---- events -----------------------------------------------------------------

// eventForm mirrors the /new and /edit form fields.
type eventForm struct {
	Title, Date, Time, Venue, Price, Description string
	Tags                                         []string
	LinkLabels, LinkURLs                         []string
	// The repeat controls of /new (the edit form has none). Emitted only when
	// set, so a single-event post is byte for byte what it always was.
	Repeats  bool
	Weekdays []string // mon..sun
	Times    string   // "18:00, 21:00"; empty means Time alone
	Until    string   // 2006-01-02
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
	if f.Repeats {
		v.Set("repeats", "1")
	}
	for _, d := range f.Weekdays {
		v.Add("weekday", d)
	}
	if f.Times != "" {
		v.Set("times", f.Times)
	}
	if f.Until != "" {
		v.Set("until", f.Until)
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

// ---- repeating events --------------------------------------------------------

// weekdayValue is the weekday form value (mon..sun) of a 2006-01-02 date.
func weekdayValue(date string) string {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return strings.ToLower(d.Weekday().String()[:3])
}

// validSeries is a complete, valid weekly repeating event: tomorrow's weekday
// only, from tomorrow until weeks-1 weeks later, so exactly weeks dates.
func validSeries(t testing.TB, weeks int) eventForm {
	t.Helper()
	f := validEvent(t, tomorrow())
	f.Title = uniqTitle(t, "Series")
	f.Repeats = true
	f.Weekdays = []string{weekdayValue(tomorrow())}
	f.Until = daysFromNow(1 + 7*(weeks-1))
	return f
}

// seriesSlugs returns, in list order, the slug of every /upcoming row titled
// title: the dates of a repeating event as the feed shows them to c.
func seriesSlugs(t testing.TB, c *client, title string) []string {
	t.Helper()
	r := c.get("/upcoming")
	assertStatus(t, r, 200)
	var out []string
	for _, row := range eventRows(r.Body) {
		if strings.Contains(row, ">"+title+"</a>") {
			out = append(out, slugOf(row))
		}
	}
	return out
}

// createSeries posts the repeating form f to /new as c. It returns the slug
// the 303 points at and every date's slug in feed order, and asserts the
// contract that the redirect names the earliest date.
func createSeries(t testing.TB, c *client, f eventForm) (first string, all []string) {
	t.Helper()
	first = createEvent(t, c, f)
	all = seriesSlugs(t, c, f.Title)
	if len(all) == 0 || all[0] != first {
		t.Fatalf("create series %q: 303 to /e/%s, but the /upcoming rows are %v (want the earliest date first)", f.Title, first, all)
	}
	return first, all
}

// ---- event follow ------------------------------------------------------------

var countRe = regexp.MustCompile(`(\d+) (?:following|followed)`)

// followerCount reads the public counter from an event page body:
// "N following" / "N followed" (past) → N. At zero the event page prints no
// counter at all (founder, 2026-09-06), so its absence is the zero.
func followerCount(t testing.TB, body string) int {
	t.Helper()
	m := countRe.FindStringSubmatch(body)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// setEventFollow posts state (follow, hide or clear) for slug as c with back
// and returns the response.
func setEventFollow(c *client, slug, state, back string) resp {
	c.t.Helper()
	form := url.Values{"state": {state}}
	if back != "" {
		form.Set("back", back)
	}
	return c.postForm("/e/"+slug+"/follow", form)
}
