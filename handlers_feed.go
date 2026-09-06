package main

import (
	"net/http"
	"net/url"
	"time"

	"dside.studio/events/internal/store"
)

const homeListSize = 10 // founder decision, deliberately not configurable

// FollowView drives the "eventfollow" partial on the event page (both
// buttons + counter).
type FollowView struct {
	Action      string
	Back        string
	State       string // "", following, hidden
	Count       int
	Past        bool
	SeriesCount int // dates in the run the toggle acts on; 0 or 1 for a single event
}

// EventRow is one line of the programme list. Back is non-empty for upcoming
// rows, which carry the Follow toggle posting back to that URL.
type EventRow struct {
	Slug        string
	Title       string
	Start       time.Time
	Venue       string
	Price       string
	Followers   int
	Tags        []string
	PosterName  string
	PosterSlug  string
	SeriesID    int64 // shared by every date of a repeating event; 0 when standalone
	SeriesCount int
	Following   bool   // the viewer follows the event
	Back        string // current path incl. query; "" for past rows (no toggle)
}

// DayGroup is the rows of one Athens day.
type DayGroup struct {
	Date   time.Time
	Label  string // Today / Tomorrow / ""
	Events []EventRow
}

// DayList is what the "daylist" partial renders.
type DayList struct {
	Days  []DayGroup
	Morph bool // primary list of the page: titles carry a view-transition-name
}

// Filter is one plain-text link of the filter row.
type Filter struct {
	Label  string
	Href   string
	Active bool
}

// feedPage renders /upcoming: one full list.
type feedPage struct {
	Base
	Filters []Filter
	List    DayList
	Empty   string
}

// homePage renders /: Mine and Upcoming side by side.
type homePage struct {
	Base
	Filters  []Filter
	Mine     DayList // empty without a session or without upcoming followed events
	Upcoming DayList
	Empty    string
	AllHref  string // /upcoming, with the tag filter when set
}

func row(e store.Event) EventRow {
	return EventRow{Slug: e.Slug, Title: e.Title, Start: e.StartsAt, Venue: e.Venue, Price: e.Price, Followers: e.Followers,
		Tags: e.Tags, PosterName: e.PosterName, PosterSlug: e.PosterSlug, SeriesID: e.SeriesID, SeriesCount: e.SeriesCount}
}

// rows converts past events: no toggle.
func rows(events []store.Event) []EventRow {
	out := make([]EventRow, 0, len(events))
	for _, e := range events {
		out = append(out, row(e))
	}
	return out
}

// toggleRows converts upcoming events; every row carries the Follow
// toggle, which returns the visitor to back.
func toggleRows(events []store.Event, back string) []EventRow {
	out := make([]EventRow, 0, len(events))
	for _, e := range events {
		rw := row(e)
		rw.Following, rw.Back = e.ViewerState == store.Following, back
		out = append(out, rw)
	}
	return out
}

// morph marks a list as the page's primary one, so its titles carry a
// view-transition-name and travel to the event heading on navigation.
func (s *Server) morph(l DayList) DayList { l.Morph = true; return l }

// groupByDay splits chronologically sorted rows into Athens days.
func (s *Server) groupByDay(rows []EventRow) DayList {
	var list DayList
	now := time.Now()
	for _, r := range rows {
		d := s.midnight(r.Start)
		if n := len(list.Days); n == 0 || !list.Days[n-1].Date.Equal(d) {
			list.Days = append(list.Days, DayGroup{Date: d, Label: s.dayLabel(d, now)})
		}
		list.Days[len(list.Days)-1].Events = append(list.Days[len(list.Days)-1].Events, r)
	}
	return list
}

// splitPast partitions events into upcoming (asc) and past (desc), where
// "past" means before Athens midnight today.
func (s *Server) splitPast(events []store.Event) (upcoming, past []store.Event) {
	today := s.midnight(time.Now())
	for _, e := range events {
		if e.StartsAt.Before(today) {
			past = append(past, e)
		} else {
			upcoming = append(upcoming, e)
		}
	}
	for i, j := 0, len(past)-1; i < j; i, j = i+1, j-1 {
		past[i], past[j] = past[j], past[i]
	}
	return upcoming, past
}

func viewerID(acct *store.Account) int64 {
	if acct == nil {
		return 0
	}
	return acct.ID
}

// filters builds the filter row. On /upcoming the tag links stay on the
// full list; everywhere else they lead home.
func (s *Server) filters(path, tag string) []Filter {
	base := "/"
	if path == "/upcoming" {
		base = "/upcoming"
	}
	fs := []Filter{{Label: "all", Href: base, Active: path == base && tag == ""}}
	for _, t := range tags {
		fs = append(fs, Filter{Label: t, Href: base + "?tag=" + url.QueryEscape(t), Active: path == base && tag == t})
	}
	return fs
}

// tagFilter validates ?tag= and returns the empty copy for that tag. Unknown
// tags are a 404.
func (s *Server) tagFilter(r *http.Request) (tag, empty string, err error) {
	tag = r.URL.Query().Get("tag")
	if tag == "" {
		return "", "No upcoming events yet. Check back soon.", nil
	}
	if !validTag(tag) {
		return "", "", errNotFound
	}
	return tag, "No upcoming events tagged " + tag + ".", nil
}

// home is the front page: the visitor's next followed events (when any) next
// to the next homeListSize upcoming events.
func (s *Server) home(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	tag, empty, err := s.tagFilter(r)
	if err != nil {
		return err
	}
	back := r.URL.RequestURI()
	events, err := s.store.Feed(r.Context(), store.FeedOpts{From: s.midnight(time.Now()), Tag: tag,
		ViewerID: viewerID(acct), Limit: homeListSize})
	if err != nil {
		return err
	}
	page := homePage{Base: s.base(r), Filters: s.filters("/", tag), Empty: empty, AllHref: "/upcoming"}
	if tag != "" {
		page.AllHref = "/upcoming?tag=" + url.QueryEscape(tag)
	}
	page.Upcoming = s.morph(s.groupByDay(toggleRows(events, back)))
	if acct != nil {
		followed, err := s.store.FollowedEvents(r.Context(), acct.ID, tag)
		if err != nil {
			return err
		}
		mine, _ := s.splitPast(followed)
		if len(mine) > homeListSize {
			mine = mine[:homeListSize]
		}
		page.Mine = s.groupByDay(toggleRows(mine, back))
	}
	return s.render(w, r, http.StatusOK, "home", page)
}

// upcoming lists all upcoming events, optionally narrowed to one tag.
func (s *Server) upcoming(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	tag, empty, err := s.tagFilter(r)
	if err != nil {
		return err
	}
	events, err := s.store.Feed(r.Context(), store.FeedOpts{From: s.midnight(time.Now()), Tag: tag, ViewerID: viewerID(acct)})
	if err != nil {
		return err
	}
	page := feedPage{Base: s.base(r), Filters: s.filters("/upcoming", tag), Empty: empty,
		List: s.morph(s.groupByDay(toggleRows(events, r.URL.RequestURI())))}
	return s.render(w, r, http.StatusOK, "feed", page)
}

type minePage struct {
	Base
	Upcoming DayList
	Past     []EventRow
	Hidden   []EventRow
	Empty    bool // nothing at all, neither followed nor hidden
}

// mine lists the visitor's followed and hidden events; a device without a
// session sees the empty state.
func (s *Server) mine(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.render(w, r, http.StatusOK, "mine", minePage{Base: s.base(r), Empty: true})
	}
	followed, err := s.store.FollowedEvents(r.Context(), acct.ID, "")
	if err != nil {
		return err
	}
	hidden, err := s.store.HiddenEvents(r.Context(), acct.ID)
	if err != nil {
		return err
	}
	upcoming, past := s.splitPast(followed)
	page := minePage{Base: s.base(r), Upcoming: s.morph(s.groupByDay(toggleRows(upcoming, r.URL.RequestURI()))),
		Past: rows(past), Hidden: rows(hidden), Empty: len(followed) == 0 && len(hidden) == 0}
	return s.render(w, r, http.StatusOK, "mine", page)
}

type posterRef struct {
	Name string
	Slug string
}

type eventView struct {
	Slug        string
	Title       string
	Start       time.Time
	Venue       string
	Price       string
	Description string
	Tags        []string
	Links       []store.Link
	Past        bool
	Poster      posterRef
	SeriesCount int
}

type eventPage struct {
	Base
	Event   eventView
	Follow  FollowView
	CanEdit bool
}

func (s *Server) event(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	e, err := s.store.EventBySlug(r.Context(), r.PathValue("slug"), viewerID(acct))
	if err != nil {
		return err
	}
	past := e.StartsAt.Before(s.midnight(time.Now()))
	page := eventPage{
		Base: s.base(r),
		Event: eventView{Slug: e.Slug, Title: e.Title, Start: e.StartsAt, Venue: e.Venue, Price: e.Price,
			Description: e.Description, Tags: e.Tags, Links: e.Links, Past: past, Poster: posterRef{e.PosterName, e.PosterSlug},
			SeriesCount: e.SeriesCount},
		Follow: FollowView{Action: "/e/" + e.Slug + "/follow", Back: "/e/" + e.Slug, State: e.ViewerState,
			Count: e.Followers, Past: past, SeriesCount: e.SeriesCount},
		CanEdit: acct != nil && acct.ID == e.PosterID,
	}
	return s.render(w, r, http.StatusOK, "event", page)
}

type posterPage struct {
	Base
	Poster   posterRef
	Upcoming DayList
	Past     []EventRow
}

func (s *Server) poster(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	p, err := s.store.PosterBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		return err
	}
	events, err := s.store.EventsByPoster(r.Context(), p.ID, viewerID(acct))
	if err != nil {
		return err
	}
	upcoming, past := s.splitPast(events)
	page := posterPage{Base: s.base(r), Poster: posterRef{p.Name, p.Slug},
		Upcoming: s.morph(s.groupByDay(toggleRows(upcoming, r.URL.RequestURI()))), Past: rows(past)}
	return s.render(w, r, http.StatusOK, "poster", page)
}
