package main

// The event as a calendar file. A visitor presses "Add to calendar" and
// hands the download to Google Calendar, Apple Calendar or anything else
// that reads RFC 5545. The file says what the page says — title, start,
// venue, description and the event's own URL — and nothing more. There is
// no end time in it because an event has no end time here: a calendar that
// invents one would be lying about the evening.

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// icsStamp is an RFC 5545 UTC date-time: 20060102T150405Z.
const icsStamp = "20060102T150405Z"

// eventICS serves one event as text/calendar.
func (s *Server) eventICS(w http.ResponseWriter, r *http.Request) error {
	// The file is the same for everyone: no viewer, no follow state in it.
	e, err := s.store.EventBySlug(r.Context(), r.PathValue("slug"), 0)
	if err != nil {
		return err
	}
	body := s.icsFor(icsEvent{
		Slug:        e.Slug,
		Title:       e.Title,
		StartsAt:    e.StartsAt,
		UpdatedAt:   e.UpdatedAt,
		Venue:       e.Venue,
		Description: e.Description,
	})
	h := w.Header()
	h.Set("Content-Type", "text/calendar; charset=utf-8")
	h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", e.Slug+".ics"))
	h.Set("Cache-Control", "no-store")
	_, err = w.Write([]byte(body))
	return err
}

// icsEvent is the part of an event a calendar entry is made of.
type icsEvent struct {
	Slug        string
	Title       string
	StartsAt    time.Time
	UpdatedAt   time.Time
	Venue       string
	Description string
}

// icsFor renders one VEVENT wrapped in a VCALENDAR. Times go out in UTC, so
// the file needs no VTIMEZONE to be read the same everywhere.
func (s *Server) icsFor(e icsEvent) string {
	var b strings.Builder
	line := func(name, value string) { b.WriteString(icsFold(name + ":" + value)) }

	b.WriteString(icsFold("BEGIN:VCALENDAR"))
	line("VERSION", "2.0")
	line("PRODID", "-//dside studio//dside events//EN")
	line("CALSCALE", "GREGORIAN")
	line("METHOD", "PUBLISH")
	b.WriteString(icsFold("BEGIN:VEVENT"))
	line("UID", e.Slug+"@"+icsHost(s.cfg.BaseURL))
	// The stamp is the last edit, not the moment of the download: two
	// downloads of an unchanged event are the same file.
	line("DTSTAMP", e.UpdatedAt.UTC().Format(icsStamp))
	line("DTSTART", e.StartsAt.UTC().Format(icsStamp))
	line("SUMMARY", icsText(e.Title))
	if e.Venue != "" {
		line("LOCATION", icsText(e.Venue))
	}
	if e.Description != "" {
		line("DESCRIPTION", icsText(e.Description))
	}
	line("URL", s.abs("/e/"+e.Slug))
	b.WriteString(icsFold("END:VEVENT"))
	b.WriteString(icsFold("END:VCALENDAR"))
	return b.String()
}

// icsHost is the host part of the base URL, for the UID's right-hand side.
func icsHost(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return "dside.studio"
}

// icsText escapes a value for a TEXT property: backslash, semicolon and
// comma are literal, and a line break is the two characters \n.
func icsText(v string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`;`, `\;`,
		`,`, `\,`,
		"\r\n", `\n`,
		"\n", `\n`,
		"\r", `\n`,
	)
	return r.Replace(v)
}

// icsFold breaks a content line into the 75 octets RFC 5545 allows, each
// continuation starting with one space, and terminates it with CRLF. A
// multi-byte character is never split.
func icsFold(line string) string {
	const limit = 75
	var b strings.Builder
	n := 0 // octets written on the current line
	for _, ru := range line {
		w := len(string(ru))
		if n+w > limit {
			b.WriteString("\r\n ")
			n = 1 // the leading space counts
		}
		b.WriteRune(ru)
		n += w
	}
	b.WriteString("\r\n")
	return b.String()
}
