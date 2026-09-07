package e2e

import (
	"strings"
	"testing"
	"time"
)

// unfold joins the continuation lines of an .ics file so a test can read a
// property on one line, the way a calendar does.
func unfold(t testing.TB, body string) string {
	t.Helper()
	if !strings.HasSuffix(body, "\r\n") {
		t.Fatalf("calendar file does not end with CRLF: %q", snippet(body))
	}
	return strings.ReplaceAll(body, "\r\n ", "")
}

// spec: EventDetail, Event
func TestEventCalendar_Download(t *testing.T) {
	p := asPoster(t, poster1)
	f := validEvent(t, tomorrow())
	f.Venue = "Floyd, Gazi"
	slug := createEvent(t, p, f)

	// The event page offers it, to everyone, without an account.
	a := anon(t)
	r := a.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertContains(t, r, `href="/e/`+slug+`/calendar.ics"`)

	ics := a.get("/e/" + slug + "/calendar.ics")
	assertStatus(t, ics, 200)
	assertHeader(t, ics, "Content-Type", "text/calendar")
	assertHeaderContains(t, ics, "Content-Disposition", slug+".ics")

	body := unfold(t, ics.Body)
	start, err := time.ParseInLocation("2006-01-02 15:04", f.Date+" "+f.Time, athens)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"BEGIN:VEVENT",
		"UID:" + slug + "@",
		"DTSTART:" + start.UTC().Format("20060102T150405Z"),
		"SUMMARY:" + f.Title,
		`LOCATION:Floyd\, Gazi`,
		"DESCRIPTION:" + f.Description,
		"/e/" + slug,
		"END:VEVENT",
		"END:VCALENDAR",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("calendar file lacks %q\n%s", want, snippet(ics.Body))
		}
	}
	// Nothing here knows when an event ends, so the file says nothing.
	if strings.Contains(body, "DTEND") || strings.Contains(body, "DURATION") {
		t.Errorf("calendar file invents an end time\n%s", snippet(ics.Body))
	}
}

// spec: EventDetail
func TestEventCalendar_PastAndUnknown(t *testing.T) {
	p := asPoster(t, poster1)
	slug := createEvent(t, p, validEvent(t, yesterday()))

	// A past event keeps its page, and its page drops the offer.
	a := anon(t)
	r := a.get("/e/" + slug)
	assertStatus(t, r, 200)
	assertNotContains(t, r, "/calendar.ics")

	assertStatus(t, a.get("/e/no-such-event-2026-01-01/calendar.ics"), 404)
}
