package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"dside.studio/events/internal/store"
)

// maxLinks is the number of link rows in the form; fields beyond it are
// never read, so an event carries at most this many links.
const maxLinks = 3

// maxSeriesDates caps the dates one repeating post may create (spec config
// max_series_occurrences).
const maxSeriesDates = 200

// linkField is one label/url pair of the form.
type linkField struct {
	Label string
	URL   string
}

// weekdayOption is one Days checkbox of the form.
type weekdayOption struct {
	Value, Label string
	Day          time.Weekday
}

// weekdays lists the Days checkboxes, Monday first.
var weekdays = []weekdayOption{
	{"mon", "Mon", time.Monday}, {"tue", "Tue", time.Tuesday}, {"wed", "Wed", time.Wednesday},
	{"thu", "Thu", time.Thursday}, {"fri", "Fri", time.Friday}, {"sat", "Sat", time.Saturday},
	{"sun", "Sun", time.Sunday},
}

// fieldError pairs an input id with its message, in display order.
type fieldError struct {
	Field string
	Msg   string
}

// eventForm holds the raw submitted values plus validation errors, so a 422
// re-render shows exactly what the poster typed.
type eventForm struct {
	Title       string
	Date        string
	Time        string
	Venue       string
	Price       string
	Description string
	Tags        []string
	TagSet      map[string]bool
	Links       [maxLinks]linkField

	Repeats  bool
	Weekdays [7]bool // indexed by time.Weekday
	Times    string  // raw, comma-separated clocks
	Until    string

	Errors map[string]string
	List   []fieldError
}

// fieldOrder is the order of the error summary.
var fieldOrder = []string{"title", "date", "time", "weekday", "times", "until", "venue", "price", "tags",
	"link_url_1", "link_url_2", "link_url_3", "description"}

func (f *eventForm) fail(field, msg string) {
	if f.Errors == nil {
		f.Errors = map[string]string{}
	}
	if _, dup := f.Errors[field]; !dup {
		f.Errors[field] = msg
	}
}

func (f *eventForm) finish() bool {
	f.List = f.List[:0]
	for _, k := range fieldOrder {
		if msg, ok := f.Errors[k]; ok {
			f.List = append(f.List, fieldError{k, msg})
		}
	}
	return len(f.Errors) == 0
}

// formFromEvent prefills the form from a stored event.
func (s *Server) formFromEvent(e *store.Event) *eventForm {
	start := e.StartsAt.In(s.loc)
	f := &eventForm{Title: e.Title, Date: start.Format("2006-01-02"), Time: start.Format("15:04"),
		Venue: e.Venue, Price: e.Price, Description: e.Description, Tags: e.Tags, TagSet: map[string]bool{}}
	for _, t := range e.Tags {
		f.TagSet[t] = true
	}
	for i, l := range e.Links {
		if i < maxLinks {
			f.Links[i] = linkField{Label: l.Label, URL: l.URL}
		}
	}
	return f
}

// parseEventForm reads the posted fields and validates them. It returns the
// form (for re-rendering) and, when valid, the store input plus the start
// instants in ascending order: exactly one for a single event, two or more
// when repeatable and the poster ticked Repeats. in.StartsAt is always
// starts[0]. When repeatable is false the repeat fields are never read.
func (s *Server) parseEventForm(r *http.Request, repeatable bool) (*eventForm, *store.EventInput, []time.Time) {
	f := &eventForm{
		Title: strings.TrimSpace(r.PostFormValue("title")), Date: strings.TrimSpace(r.PostFormValue("date")),
		Time: strings.TrimSpace(r.PostFormValue("time")), Venue: strings.TrimSpace(r.PostFormValue("venue")),
		Price: strings.TrimSpace(r.PostFormValue("price")), Description: strings.TrimSpace(r.PostFormValue("description")),
		TagSet: map[string]bool{},
	}
	for _, t := range r.PostForm["tag"] {
		if !f.TagSet[t] {
			f.TagSet[t] = true
			f.Tags = append(f.Tags, t)
		}
	}
	for i := range f.Links {
		f.Links[i] = linkField{
			Label: strings.TrimSpace(r.PostFormValue(fmt.Sprintf("link_label_%d", i+1))),
			URL:   strings.TrimSpace(r.PostFormValue(fmt.Sprintf("link_url_%d", i+1))),
		}
	}
	if repeatable {
		f.Repeats = r.PostFormValue("repeats") != ""
		f.Times = strings.TrimSpace(r.PostFormValue("times"))
		f.Until = strings.TrimSpace(r.PostFormValue("until"))
		for _, v := range r.PostForm["weekday"] {
			known := false
			for _, o := range weekdays {
				if o.Value == v {
					f.Weekdays[o.Day], known = true, true
				}
			}
			if !known && f.Repeats {
				f.fail("weekday", "Pick weekdays from the list.")
			}
		}
	}

	in := &store.EventInput{Title: f.Title, Venue: f.Venue, Price: f.Price, Description: f.Description}
	switch n := utf8.RuneCountInString(f.Title); {
	case n == 0:
		f.fail("title", "Title is required.")
	case n > 120:
		f.fail("title", "Title must be 120 characters or fewer.")
	}
	if f.Date == "" {
		f.fail("date", "Pick a date.")
	}
	if f.Time == "" {
		f.fail("time", "Pick a start time.")
	}
	var starts []time.Time
	if f.Date != "" && f.Time != "" {
		start, err := time.ParseInLocation("2006-01-02 15:04", f.Date+" "+f.Time, s.loc)
		if err != nil {
			if _, derr := time.Parse("2006-01-02", f.Date); derr != nil {
				f.fail("date", "Pick a valid date.")
			} else {
				f.fail("time", "Pick a valid start time.")
			}
		} else {
			now := time.Now()
			if start.Before(now.AddDate(-1, 0, 0)) || start.After(now.AddDate(3, 0, 0)) {
				f.fail("date", "Date must be within the past year or the next three years.")
			}
			in.StartsAt = start
			starts = []time.Time{start}
			if f.Repeats {
				if starts = s.seriesStarts(f, start); len(starts) > 0 {
					in.StartsAt = starts[0]
				}
			}
		}
	}
	switch n := utf8.RuneCountInString(f.Venue); {
	case n == 0:
		f.fail("venue", "Venue is required.")
	case n > 120:
		f.fail("venue", "Venue must be 120 characters or fewer.")
	}
	if utf8.RuneCountInString(f.Price) > 60 {
		f.fail("price", "Price must be 60 characters or fewer.")
	}
	if utf8.RuneCountInString(f.Description) > 4000 {
		f.fail("description", "Description must be 4000 characters or fewer.")
	}
	for _, t := range f.Tags {
		if !validTag(t) {
			f.fail("tags", "Pick tags from the list.")
		}
	}
	switch {
	case len(f.Tags) == 0:
		f.fail("tags", "Pick at least one tag.")
	case len(f.Tags) > 4:
		f.fail("tags", "Pick at most 4 tags.")
	}
	in.Tags = f.Tags
	for i, l := range f.Links {
		if l.Label == "" && l.URL == "" {
			continue
		}
		field := fmt.Sprintf("link_url_%d", i+1)
		u, err := url.Parse(l.URL)
		if l.URL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(l.URL) > 500 {
			f.fail(field, fmt.Sprintf("Link %d needs a full address starting with https://.", i+1))
			continue
		}
		label := l.Label
		if label == "" {
			label = u.Host
		}
		if utf8.RuneCountInString(label) > 80 {
			f.fail(field, fmt.Sprintf("Link %d label must be 80 characters or fewer.", i+1))
			continue
		}
		in.Links = append(in.Links, store.Link{Label: label, URL: l.URL})
	}
	if !f.finish() {
		return f, nil, nil
	}
	return f, in, starts
}

// seriesStarts validates the repeat fields against the parsed first start
// and returns every start of the run, ascending; nil when a field failed.
func (s *Server) seriesStarts(f *eventForm, start time.Time) []time.Time {
	if f.Weekdays == [7]bool{} {
		f.fail("weekday", "Pick at least one weekday.")
	}
	var clocks []time.Time
	for _, tok := range strings.Split(f.Times, ",") {
		if tok = strings.TrimSpace(tok); tok == "" {
			continue
		}
		c, err := time.Parse("15:04", tok)
		if err != nil {
			f.fail("times", "Pick valid start times, like 18:00, 21:00.")
			break
		}
		clocks = append(clocks, c)
	}
	if len(clocks) == 0 {
		// Times left empty: every date starts at Start time.
		clocks = []time.Time{time.Date(0, time.January, 1, start.Hour(), start.Minute(), 0, 0, time.UTC)}
	}
	sort.Slice(clocks, func(i, j int) bool { return clocks[i].Before(clocks[j]) })
	for i := 1; i < len(clocks); i++ {
		if clocks[i].Equal(clocks[i-1]) {
			f.fail("times", "Each start time must be different.")
			break
		}
	}
	first := s.midnight(start)
	until, err := time.ParseInLocation("2006-01-02", f.Until, s.loc)
	switch {
	case f.Until == "":
		f.fail("until", "Pick an until date.")
	case err != nil:
		f.fail("until", "Pick a valid until date.")
	case until.Before(first):
		f.fail("until", "Until must be on or after the date.")
	case until.After(time.Now().AddDate(3, 0, 0)):
		f.fail("until", "Until must be within the next three years.")
	}
	for _, k := range []string{"weekday", "times", "until"} {
		if _, bad := f.Errors[k]; bad {
			return nil
		}
	}
	starts := expandSeries(first, until, f.Weekdays, clocks, s.loc)
	switch {
	case len(starts) < 2:
		f.fail("until", "A repeating event needs at least two dates.")
		return nil
	case len(starts) > maxSeriesDates:
		f.fail("until", fmt.Sprintf("A repeating event can have at most %d dates.", maxSeriesDates))
		return nil
	}
	return starts
}

// expandSeries lists every start of a repeating event: each day from first
// to until (inclusive, both local midnights) whose weekday is ticked, at
// each clock, ascending. Days are walked with AddDate and every start is
// built with time.Date in loc, so a DST change never shifts a start by an
// hour. The walk stops once the list exceeds maxSeriesDates; the caller
// refuses the length.
func expandSeries(first, until time.Time, weekdays [7]bool, clocks []time.Time, loc *time.Location) []time.Time {
	var starts []time.Time
	for d := first; !d.After(until) && len(starts) <= maxSeriesDates; d = d.AddDate(0, 0, 1) {
		if !weekdays[d.Weekday()] {
			continue
		}
		for _, c := range clocks {
			starts = append(starts, time.Date(d.Year(), d.Month(), d.Day(), c.Hour(), c.Minute(), 0, 0, loc))
		}
	}
	return starts
}

// checkDuplicate enforces "one post per event": same poster, same normalised
// title, same Athens day as any of starts (ascending). One query covers the
// whole range. excludeID skips the event being edited.
func (s *Server) checkDuplicate(r *http.Request, f *eventForm, posterID int64, title string, starts []time.Time, excludeID int64) (bool, error) {
	days := make(map[int64]bool, len(starts))
	for _, t := range starts {
		days[s.midnight(t).Unix()] = true
	}
	from, to := s.midnight(starts[0]), s.midnight(starts[len(starts)-1]).AddDate(0, 0, 1)
	others, err := s.store.EventsByPosterBetween(r.Context(), posterID, from, to)
	if err != nil {
		return false, err
	}
	want := normalizeTitle(title)
	for _, o := range others {
		day := s.midnight(o.StartsAt)
		if o.ID != excludeID && normalizeTitle(o.Title) == want && days[day.Unix()] {
			f.fail("title", fmt.Sprintf("You already posted this event on %s.", day.Format("2 January 2006")))
			f.finish()
			return true, nil
		}
	}
	return false, nil
}

func normalizeTitle(t string) string { return strings.ToLower(strings.Join(strings.Fields(t), " ")) }
