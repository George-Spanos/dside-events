package main

import (
	"strings"
	"testing"
	"time"
)

func testICSServer() *Server {
	loc, err := time.LoadLocation("Europe/Athens")
	if err != nil {
		panic(err)
	}
	return &Server{cfg: Config{BaseURL: "https://events.dside.studio"}, loc: loc}
}

// icsLines unfolds the file the way a calendar reads it: CRLF between
// content lines, a continuation marked by one leading space.
func icsLines(t *testing.T, body string) []string {
	t.Helper()
	if strings.Contains(strings.ReplaceAll(body, "\r\n", ""), "\n") {
		t.Fatal("bare LF in the calendar file; every line must end CRLF")
	}
	var out []string
	for _, l := range strings.Split(strings.TrimSuffix(body, "\r\n"), "\r\n") {
		if strings.HasPrefix(l, " ") && len(out) > 0 {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return out
}

func TestICSFor(t *testing.T) {
	s := testICSServer()
	start := time.Date(2026, 9, 20, 21, 30, 0, 0, s.loc) // 18:30 UTC
	body := s.icsFor(icsEvent{
		Slug:        "nadia-quartet-2026-09-20",
		Title:       "Nadia Quartet",
		StartsAt:    start,
		UpdatedAt:   time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC),
		Venue:       "Floyd, Gazi",
		Description: "Two sets, no support.",
	})
	got := icsLines(t, body)
	want := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//dside studio//dside events//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"BEGIN:VEVENT",
		"UID:nadia-quartet-2026-09-20@events.dside.studio",
		"DTSTAMP:20260907T080000Z",
		"DTSTART:20260920T183000Z",
		"SUMMARY:Nadia Quartet",
		`LOCATION:Floyd\, Gazi`,
		"DESCRIPTION:Two sets\\, no support.",
		"URL:https://events.dside.studio/e/nadia-quartet-2026-09-20",
		"END:VEVENT",
		"END:VCALENDAR",
	}
	if len(got) != len(want) {
		t.Fatalf("lines = %q\nwant %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	// An event has no end time here, so the file claims none.
	if strings.Contains(body, "DTEND") || strings.Contains(body, "DURATION") {
		t.Error("the file invents an end time")
	}
}

func TestICSForEmptyFields(t *testing.T) {
	s := testICSServer()
	body := s.icsFor(icsEvent{
		Slug:      "reading-2026-09-20",
		Title:     "Reading",
		StartsAt:  time.Date(2026, 9, 20, 19, 0, 0, 0, s.loc),
		UpdatedAt: time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC),
	})
	for _, name := range []string{"LOCATION", "DESCRIPTION"} {
		if strings.Contains(body, name) {
			t.Errorf("%s written for an event that has none", name)
		}
	}
}

func TestICSFoldLongLine(t *testing.T) {
	s := testICSServer()
	title := "Χειμωνιάτικο πρόγραμμα του κουαρτέτου με έργα του Σοστακόβιτς και άλλων"
	body := s.icsFor(icsEvent{
		Slug:      "long-2026-09-20",
		Title:     title,
		StartsAt:  time.Date(2026, 9, 20, 19, 0, 0, 0, s.loc),
		UpdatedAt: time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC),
	})
	for _, l := range strings.Split(strings.TrimSuffix(body, "\r\n"), "\r\n") {
		if len(l) > 75 {
			t.Errorf("line of %d octets exceeds 75: %q", len(l), l)
		}
	}
	var summary string
	for _, l := range icsLines(t, body) {
		if strings.HasPrefix(l, "SUMMARY:") {
			summary = strings.TrimPrefix(l, "SUMMARY:")
		}
	}
	if summary != title {
		t.Errorf("unfolded SUMMARY = %q, want %q", summary, title)
	}
}

func TestICSText(t *testing.T) {
	tests := map[string]string{
		"plain":                  "plain",
		"Floyd, Gazi":            `Floyd\, Gazi`,
		"a; b":                   `a\; b`,
		`back\slash`:             `back\\slash`,
		"two\r\nlines":           `two\nlines`,
		"three\nlines\rand more": `three\nlines\nand more`,
	}
	for in, want := range tests {
		if got := icsText(in); got != want {
			t.Errorf("icsText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestICSHost(t *testing.T) {
	tests := map[string]string{
		"https://events.dside.studio": "events.dside.studio",
		"http://localhost:8080":       "localhost:8080",
		"":                            "dside.studio",
	}
	for in, want := range tests {
		if got := icsHost(in); got != want {
			t.Errorf("icsHost(%q) = %q, want %q", in, got, want)
		}
	}
}
