package main

import (
	"html/template"
	"net/http"
	"net/url"
	"time"

	"dside.studio/events/internal/store"
)

// InterestView drives the "interest" partial (buttons + counter).
type InterestView struct {
	Action   string
	Back     string
	State    string // "", interested, not_interested
	Count    int
	Past     bool
	LoggedIn bool
	Compact  bool // /mine rows: only the Interested button, no counter
}

// LoginHref is the login link for anonymous visitors, returning to Back.
// Typed as template.URL so the path stays readable (no %2f escaping).
func (v InterestView) LoginHref() template.URL { return template.URL("/login?next=" + v.Back) }

// EventRow is one line of the programme list.
type EventRow struct {
	Slug       string
	Title      string
	Start      time.Time
	Venue      string
	Price      string
	Interested int
	Interest   *InterestView
}

// DayGroup is the rows of one Athens day.
type DayGroup struct {
	Date   time.Time
	Label  string // Today / Tomorrow / ""
	Events []EventRow
}

// DayList is what the "daylist" partial renders.
type DayList struct {
	Days []DayGroup
	Sub  bool // day headings are h3, nested under a section h2 (/mine)
}

// Filter is one plain-text link of the filter row.
type Filter struct {
	Label  string
	Href   string
	Active bool
}

type tagFollowView struct {
	Tag       string
	Back      string
	Following bool
}

type feedPage struct {
	Base
	Filters   []Filter
	TagFollow *tagFollowView
	List      DayList
	Empty     string
}

func row(e store.Event) EventRow {
	return EventRow{Slug: e.Slug, Title: e.Title, Start: e.StartsAt, Venue: e.Venue, Price: e.Price, Interested: e.Interested}
}

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

func (s *Server) filters(acct *store.Account, path, tag string) []Filter {
	fs := []Filter{{Label: "all", Href: "/", Active: path == "/" && tag == ""}}
	if acct != nil {
		fs = append(fs, Filter{Label: "following", Href: "/following", Active: path == "/following"})
	}
	for _, t := range tags {
		fs = append(fs, Filter{Label: t, Href: "/?tag=" + url.QueryEscape(t), Active: path == "/" && tag == t})
	}
	return fs
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	tag := r.URL.Query().Get("tag")
	if tag != "" && !validTag(tag) {
		return errNotFound
	}
	events, err := s.store.Feed(r.Context(), store.FeedOpts{From: s.midnight(time.Now()), Tag: tag, ViewerID: viewerID(acct)})
	if err != nil {
		return err
	}
	page := feedPage{Base: s.base(r), Filters: s.filters(acct, "/", tag), Empty: "No upcoming events yet. Check back soon."}
	if tag != "" {
		page.Empty = "No upcoming events tagged " + tag + "."
		if acct != nil {
			follows, err := s.store.Follows(r.Context(), acct.ID)
			if err != nil {
				return err
			}
			page.TagFollow = &tagFollowView{Tag: tag, Back: r.URL.RequestURI(), Following: follows.HasTag(tag)}
		}
	}
	page.List = s.groupByDay(rows(events))
	return s.render(w, r, http.StatusOK, "feed", page)
}

func rows(events []store.Event) []EventRow {
	out := make([]EventRow, 0, len(events))
	for _, e := range events {
		out = append(out, row(e))
	}
	return out
}

func (s *Server) following(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.loginRedirect(w, r)
	}
	follows, err := s.store.Follows(r.Context(), acct.ID)
	if err != nil {
		return err
	}
	page := feedPage{Base: s.base(r), Filters: s.filters(acct, "/following", "")}
	if !follows.Any() {
		page.Empty = "You're not following anything yet. Pick a tag above and press Follow, or follow a curator from an event page."
	} else {
		page.Empty = "Nothing upcoming from the tags and curators you follow."
		events, err := s.store.Feed(r.Context(), store.FeedOpts{From: s.midnight(time.Now()), ViewerID: acct.ID, FollowingOnly: true})
		if err != nil {
			return err
		}
		page.List = s.groupByDay(rows(events))
	}
	return s.render(w, r, http.StatusOK, "feed", page)
}

type minePage struct {
	Base
	Upcoming DayList
	Past     []EventRow
	Hidden   []EventRow
	Empty    bool // no marks at all, neither interested nor hidden
}

func (s *Server) mine(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.loginRedirect(w, r)
	}
	interested, err := s.store.InterestedEvents(r.Context(), acct.ID)
	if err != nil {
		return err
	}
	hidden, err := s.store.HiddenEvents(r.Context(), acct.ID)
	if err != nil {
		return err
	}
	upcoming, past := s.splitPast(interested)
	var up []EventRow
	for _, e := range upcoming {
		rw := row(e)
		rw.Interest = &InterestView{Action: "/e/" + e.Slug + "/interest", Back: "/mine", State: store.Interested,
			Count: e.Interested, LoggedIn: true, Compact: true}
		up = append(up, rw)
	}
	page := minePage{Base: s.base(r), Upcoming: s.groupByDay(up), Past: rows(past), Hidden: rows(hidden),
		Empty: len(interested) == 0 && len(hidden) == 0}
	page.Upcoming.Sub = true
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
}

type eventPage struct {
	Base
	Event    eventView
	Interest InterestView
	CanEdit  bool
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
			Description: e.Description, Tags: e.Tags, Links: e.Links, Past: past, Poster: posterRef{e.PosterName, e.PosterSlug}},
		Interest: InterestView{Action: "/e/" + e.Slug + "/interest", Back: "/e/" + e.Slug, State: e.ViewerState,
			Count: e.Interested, Past: past, LoggedIn: acct != nil},
		CanEdit: acct != nil && acct.ID == e.PosterID,
	}
	return s.render(w, r, http.StatusOK, "event", page)
}

type followView struct {
	Kind      string
	Key       string
	Back      string
	Following bool
	Label     string
}

type posterPage struct {
	Base
	Poster   posterRef
	Follow   *followView // nil for anonymous visitors and the poster themself
	Anon     bool
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
	page := posterPage{Base: s.base(r), Poster: posterRef{p.Name, p.Slug}, Anon: acct == nil,
		Upcoming: s.groupByDay(rows(upcoming)), Past: rows(past)}
	if acct != nil && acct.ID != p.ID {
		follows, err := s.store.Follows(r.Context(), acct.ID)
		if err != nil {
			return err
		}
		page.Follow = &followView{Kind: "poster", Key: p.Slug, Back: r.URL.Path, Following: follows.HasPoster(p.ID), Label: p.Name}
	}
	return s.render(w, r, http.StatusOK, "poster", page)
}
