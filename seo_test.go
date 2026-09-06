package main

import (
	"strings"
	"testing"
)

func TestPriceAmount(t *testing.T) {
	tests := []struct {
		price string
		want  string
		ok    bool
	}{
		{"18€", "18", true},
		{"12 EUR", "12", true},
		{"from 15€", "15", true},
		{"7.50€", "7.50", true},
		{"7,50€", "7.50", true},
		{"Free", "0", true},
		{"free", "0", true},
		{"Δωρεάν", "0", true},
		// No single number to read: the event ships without an offer rather
		// than with a figure the page never claimed.
		{"", "", false},
		{"8–12€", "", false},
		{"8-12 EUR", "", false},
		{"ask at the door", "", false},
		{"15€, 12€ reduced", "", false},
	}
	for _, tt := range tests {
		got, ok := priceAmount(tt.price)
		if ok != tt.ok || got != tt.want {
			t.Errorf("priceAmount(%q) = %q, %v; want %q, %v", tt.price, got, ok, tt.want, tt.ok)
		}
	}
}

func TestVenueName(t *testing.T) {
	tests := map[string]string{
		"Floyd, Gazi": "Floyd",
		"Gazarte":     "Gazarte",
		"Stoa Culture – Aithrio, Athens centre": "Stoa Culture – Aithrio",
		"": "",
	}
	for venue, want := range tests {
		if got := venueName(venue); got != want {
			t.Errorf("venueName(%q) = %q, want %q", venue, got, want)
		}
	}
}

func TestClamp_KeepsShortTextAndBreaksOnAWord(t *testing.T) {
	short := "Doors open at eight."
	if got := clamp(short); got != short {
		t.Errorf("clamp(%q) = %q, want it unchanged", short, got)
	}

	long := strings.Repeat("alpha beta ", 40)
	got := clamp(long)
	if len(got) > descMax+len("…") {
		t.Errorf("clamp returned %d characters, want at most %d", len(got), descMax+len("…"))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("clamp(%q…) = %q, want it to end in an ellipsis", long[:20], got)
	}
	// The cut lands on a word boundary, so the result reads as prose.
	if strings.HasSuffix(strings.TrimSuffix(got, "…"), "alph") {
		t.Errorf("clamp cut mid-word: %q", got)
	}
}

func TestClamp_CollapsesWhitespace(t *testing.T) {
	if got, want := clamp("two\n\nparagraphs  here"), "two paragraphs here"; got != want {
		t.Errorf("clamp = %q, want %q", got, want)
	}
}

func TestTitle_SuffixesTheBrandOnce(t *testing.T) {
	if got, want := title("Concerts in Athens"), "Concerts in Athens · dside events"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	for _, in := range []string{"", brand} {
		if got := title(in); got != brand {
			t.Errorf("title(%q) = %q, want %q", in, got, brand)
		}
	}
}

// Every tag in the fixed list needs copy that reads as a search result, or
// the tag pages fall back to a machine-shaped title.
func TestTagCopy_CoversEveryTag(t *testing.T) {
	for _, tag := range tags {
		c, ok := tagCopy[tag]
		if !ok {
			t.Errorf("tag %q has no title and description for search", tag)
			continue
		}
		if !strings.Contains(c.Title, "Athens") {
			t.Errorf("tag %q title %q does not say Athens", tag, c.Title)
		}
		if len(c.Desc) > descMax {
			t.Errorf("tag %q description is %d characters, want at most %d", tag, len(c.Desc), descMax)
		}
	}
}

func TestTagPath(t *testing.T) {
	tests := map[string]string{
		"":        "/upcoming",
		"concert": "/upcoming?tag=concert",
	}
	for tag, want := range tests {
		if got := tagPath("/upcoming", tag); got != want {
			t.Errorf("tagPath(%q) = %q, want %q", tag, got, want)
		}
	}
}
