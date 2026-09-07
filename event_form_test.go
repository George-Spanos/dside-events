package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func postForm(t *testing.T, v url.Values) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodPost, "/new", strings.NewReader(v.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func athens(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Athens")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// TestParseEventForm_MaxThreeLinks: the form has exactly three link slots. A
// posted link_url_4 is never read, so an event can never end up with more
// than maxLinks links, and the prefilled form never shows a fourth row.
func TestParseEventForm_MaxThreeLinks(t *testing.T) {
	if maxLinks != 3 {
		t.Fatalf("maxLinks = %d, want 3", maxLinks)
	}
	s := &Server{loc: athens(t)}
	v := url.Values{
		"title": {"Concert"}, "date": {time.Now().AddDate(0, 0, 7).Format(formDate)}, "time": {"21:00"},
		"venue": {"Gagarin"}, "tag": {"concert"},
		"link_label_1": {"One"}, "link_url_1": {"https://one.example/"},
		"link_label_2": {"Two"}, "link_url_2": {"https://two.example/"},
		"link_label_3": {"Three"}, "link_url_3": {"https://three.example/"},
		"link_label_4": {"Four"}, "link_url_4": {"https://four.example/"},
		"link_label_5": {"Five"}, "link_url_5": {"not a url"},
	}
	f, in, starts := s.parseEventForm(postForm(t, v), true)
	if in == nil {
		t.Fatalf("valid form rejected: %+v", f.Errors)
	}
	if len(starts) != 1 || !starts[0].Equal(in.StartsAt) {
		t.Fatalf("starts = %v, want just %v", starts, in.StartsAt)
	}
	if len(in.Links) != 3 || len(f.Links) != 3 {
		t.Fatalf("links stored = %d, form rows = %d, want 3 and 3", len(in.Links), len(f.Links))
	}
	for i, l := range in.Links {
		if l.Label != v.Get("link_label_"+string(rune('1'+i))) {
			t.Errorf("link %d label = %q", i+1, l.Label)
		}
	}
	if _, ok := f.Errors["link_url_4"]; ok {
		t.Error("link_url_4 was validated; it must be ignored")
	}
	for _, k := range fieldOrder {
		if k == "link_url_4" || k == "link_url_5" {
			t.Errorf("fieldOrder still lists %s", k)
		}
	}
	// A bad third link is still reported under its own field.
	v.Set("link_url_3", "ftp://nope")
	f, in, _ = s.parseEventForm(postForm(t, v), true)
	if in != nil || f.Errors["link_url_3"] == "" {
		t.Fatalf("bad link_url_3 accepted: %+v", f.Errors)
	}
}

// TestExpandSeries: weekdays × clocks over an inclusive day range, ascending,
// with the clock preserved across the DST change.
func TestExpandSeries(t *testing.T) {
	loc := athens(t)
	clock := func(h int) time.Time { return time.Date(0, 1, 1, h, 0, 0, 0, time.UTC) }
	var monWed [7]bool
	monWed[time.Monday], monWed[time.Wednesday] = true, true

	// Monday 2 March 2026 to Sunday 15 March: two Mondays and two Wednesdays.
	first := time.Date(2026, 3, 2, 0, 0, 0, 0, loc)
	got := expandSeries(first, first.AddDate(0, 0, 13), monWed, []time.Time{clock(18), clock(21)}, loc)
	if len(got) != 8 {
		t.Fatalf("got %d starts, want 8: %v", len(got), got)
	}
	for i, st := range got {
		if i > 0 && !got[i-1].Before(st) {
			t.Errorf("start %d (%v) not after %v", i, st, got[i-1])
		}
		if wd := st.Weekday(); wd != time.Monday && wd != time.Wednesday {
			t.Errorf("start %v falls on a %v", st, wd)
		}
		if h := st.Hour(); h != 18 && h != 21 {
			t.Errorf("start %v has hour %d", st, h)
		}
	}
	if got[0].Hour() != 18 || got[1].Hour() != 21 || !got[0].Equal(time.Date(2026, 3, 2, 18, 0, 0, 0, loc)) {
		t.Errorf("first day = %v, %v", got[0], got[1])
	}

	// The first date's weekday need not be ticked: a Tuesday start with
	// Mon+Wed yields Wednesday first.
	tue := first.AddDate(0, 0, 1)
	got = expandSeries(tue, tue.AddDate(0, 0, 6), monWed, []time.Time{clock(18)}, loc)
	if len(got) != 2 || got[0].Weekday() != time.Wednesday || got[1].Weekday() != time.Monday {
		t.Errorf("Tuesday start: got %v", got)
	}

	// Across Sunday 29 March 2026 (clocks go forward) the hour stays 18.
	var sat [7]bool
	sat[time.Saturday] = true
	got = expandSeries(time.Date(2026, 3, 21, 0, 0, 0, 0, loc), time.Date(2026, 4, 4, 0, 0, 0, 0, loc), sat, []time.Time{clock(18)}, loc)
	if len(got) != 3 {
		t.Fatalf("DST range: got %d starts, want 3", len(got))
	}
	for _, st := range got {
		if st.Hour() != 18 {
			t.Errorf("%v has hour %d, want 18", st, st.Hour())
		}
	}
	if got[2].Sub(got[1]) != 7*24*time.Hour-time.Hour {
		t.Errorf("DST week is %v long, want 167h", got[2].Sub(got[1]))
	}

	if got := expandSeries(first, first.AddDate(0, 0, 30), [7]bool{}, []time.Time{clock(18)}, loc); len(got) != 0 {
		t.Errorf("no weekdays: got %v", got)
	}
}

// TestParseEventForm_Repeats: the repeat fields expand to a series when
// valid, every failure lands on its own field, and they are ignored when the
// form is not repeatable (edit).
func TestParseEventForm_Repeats(t *testing.T) {
	loc := athens(t)
	s := &Server{loc: loc}
	now := time.Now().In(loc)
	first := now.AddDate(0, 0, 7)
	valid := func() url.Values {
		return url.Values{
			"title": {"Film"}, "date": {first.Format(formDate)}, "time": {"18:00"},
			"venue": {"Cinema"}, "tag": {"concert"},
			"repeats": {"1"}, "weekday": {"mon", "wed"}, "times": {"21:00, 18:00"},
			"until": {first.AddDate(0, 0, 13).Format(formDate)},
		}
	}

	f, in, starts := s.parseEventForm(postForm(t, valid()), true)
	if in == nil {
		t.Fatalf("valid series rejected: %+v", f.Errors)
	}
	if len(starts) != 8 || !in.StartsAt.Equal(starts[0]) {
		t.Fatalf("starts = %d (%v), in.StartsAt = %v", len(starts), starts, in.StartsAt)
	}
	if !f.Repeats || !f.Weekdays[time.Monday] || !f.Weekdays[time.Wednesday] || f.Weekdays[time.Friday] {
		t.Errorf("form not echoed: repeats=%v weekdays=%v", f.Repeats, f.Weekdays)
	}
	if starts[0].Hour() != 18 || starts[1].Hour() != 21 {
		t.Errorf("clocks not sorted: %v, %v", starts[0], starts[1])
	}

	// Times empty: every date starts at Start time.
	v := valid()
	v.Set("times", "")
	if _, in, starts := s.parseEventForm(postForm(t, v), true); in == nil || len(starts) != 4 || starts[0].Hour() != 18 {
		t.Errorf("empty times: in=%v starts=%v", in, starts)
	}

	// Not repeatable: the repeat fields are never read.
	f, in, starts = s.parseEventForm(postForm(t, valid()), false)
	if in == nil || len(starts) != 1 || f.Repeats || len(f.Errors) != 0 {
		t.Errorf("edit ignored repeats: in=%v starts=%v form=%+v", in, starts, f)
	}

	cases := []struct {
		name  string
		set   func(v url.Values)
		field string
		msg   string
	}{
		{"unknown weekday", func(v url.Values) { v["weekday"] = []string{"mon", "funday"} }, "weekday", "Pick weekdays from the list."},
		{"no weekday", func(v url.Values) { v.Del("weekday") }, "weekday", "Pick at least one weekday."},
		{"bad times", func(v url.Values) { v.Set("times", "18:00, noon") }, "times", "Pick valid start times, like 18:00, 21:00."},
		{"same times", func(v url.Values) { v.Set("times", "18:00, 21:00, 18:00") }, "times", "Each start time must be different."},
		{"until empty", func(v url.Values) { v.Set("until", "") }, "until", "Pick an until date."},
		{"until invalid", func(v url.Values) { v.Set("until", "soon") }, "until", "Pick a valid until date."},
		{"until before date", func(v url.Values) { v.Set("until", first.AddDate(0, 0, -1).Format(formDate)) }, "until", "Until must be on or after the date."},
		{"until too far", func(v url.Values) { v.Set("until", now.AddDate(3, 0, 7).Format(formDate)) }, "until", "Until must be within the next three years."},
		{"too few", func(v url.Values) {
			v.Set("until", first.Format(formDate))
			v.Set("times", "")
			v["weekday"] = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
		}, "until", "A repeating event needs at least two dates."},
		{"cap", func(v url.Values) {
			v.Set("until", first.AddDate(0, 0, 70).Format(formDate))
			v.Set("times", "10:00, 14:00, 18:00")
			v["weekday"] = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
		}, "until", "A repeating event can have at most 200 dates."},
	}
	for _, c := range cases {
		v := valid()
		c.set(v)
		f, in, starts := s.parseEventForm(postForm(t, v), true)
		if in != nil || starts != nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if got := f.Errors[c.field]; got != c.msg {
			t.Errorf("%s: %s error = %q, want %q (all: %v)", c.name, c.field, got, c.msg, f.Errors)
		}
		if len(f.Errors) != 1 {
			t.Errorf("%s: extra errors: %v", c.name, f.Errors)
		}
	}
}
