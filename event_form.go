package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"dside.studio/events/internal/store"
)

// maxLinks is the number of link rows in the form; fields beyond it are
// never read, so an event carries at most this many links.
const maxLinks = 3

// linkField is one label/url pair of the form.
type linkField struct {
	Label string
	URL   string
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

	Errors map[string]string
	List   []fieldError
}

// fieldOrder is the order of the error summary.
var fieldOrder = []string{"title", "date", "time", "venue", "price", "tags",
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
// form (for re-rendering) and, when valid, the store input.
func (s *Server) parseEventForm(r *http.Request) (*eventForm, *store.EventInput) {
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
		return f, nil
	}
	return f, in
}

// checkDuplicate enforces "one post per event": same poster, same normalised
// title, same Athens day. excludeID skips the event being edited.
func (s *Server) checkDuplicate(r *http.Request, f *eventForm, posterID int64, in *store.EventInput, excludeID int64) (bool, error) {
	day := s.midnight(in.StartsAt)
	others, err := s.store.EventsByPosterBetween(r.Context(), posterID, day, day.AddDate(0, 0, 1))
	if err != nil {
		return false, err
	}
	want := normalizeTitle(in.Title)
	for _, o := range others {
		if o.ID != excludeID && normalizeTitle(o.Title) == want {
			f.fail("title", "You already posted this event on that day.")
			f.finish()
			return true, nil
		}
	}
	return false, nil
}

func normalizeTitle(t string) string { return strings.ToLower(strings.Join(strings.Fields(t), " ")) }
