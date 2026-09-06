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

// TestParseEventForm_MaxThreeLinks: the form has exactly three link slots. A
// posted link_url_4 is never read, so an event can never end up with more
// than maxLinks links, and the prefilled form never shows a fourth row.
func TestParseEventForm_MaxThreeLinks(t *testing.T) {
	if maxLinks != 3 {
		t.Fatalf("maxLinks = %d, want 3", maxLinks)
	}
	loc, err := time.LoadLocation("Europe/Athens")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{loc: loc}
	v := url.Values{
		"title": {"Concert"}, "date": {time.Now().AddDate(0, 0, 7).Format("2006-01-02")}, "time": {"21:00"},
		"venue": {"Gagarin"}, "tag": {"concert"},
		"link_label_1": {"One"}, "link_url_1": {"https://one.example/"},
		"link_label_2": {"Two"}, "link_url_2": {"https://two.example/"},
		"link_label_3": {"Three"}, "link_url_3": {"https://three.example/"},
		"link_label_4": {"Four"}, "link_url_4": {"https://four.example/"},
		"link_label_5": {"Five"}, "link_url_5": {"not a url"},
	}
	f, in := s.parseEventForm(postForm(t, v))
	if in == nil {
		t.Fatalf("valid form rejected: %+v", f.Errors)
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
	f, in = s.parseEventForm(postForm(t, v))
	if in != nil || f.Errors["link_url_3"] == "" {
		t.Fatalf("bad link_url_3 accepted: %+v", f.Errors)
	}
}
