package main

import (
	"strings"
	"testing"
	"time"
)

func TestSlugBase(t *testing.T) {
	cases := map[string]string{
		"Συναυλία στο Gagarin":     "synaylia-sto-gagarin",
		"Θέατρο: Ψυχή & Χρόνος":    "theatro-psychi-chronos",
		"  Hello,  World!  ":       "hello-world",
		"Café Übermensch":          "cafe-ubermensch",
		"!!!":                      "event",
		"":                         "event",
		"ΜΕΓΑΛΑ ΓΡΑΜΜΑΤΑ":          "megala-grammata",
		"Live 21:30 @ Six D.O.G.S": "live-21-30-six-d-o-g-s",
	}
	for in, want := range cases {
		if got := slugBase(in); got != want {
			t.Errorf("slugBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugCap(t *testing.T) {
	long := strings.Repeat("word ", 30)
	got := slugBase(long)
	if len(got) > slugMax || strings.HasSuffix(got, "-") || strings.Contains(got, "--") {
		t.Fatalf("cap: %q (%d)", got, len(got))
	}
	if !strings.HasPrefix(got, "word-word") {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestEventSlug(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Athens")
	// 23:30 UTC on the 6th is 02:30 on the 7th in Athens (EEST).
	start := time.Date(2026, 9, 6, 23, 30, 0, 0, time.UTC)
	if got := eventSlug("Συναυλία", start, loc); got != "synaylia-2026-09-07" {
		t.Fatalf("eventSlug = %q", got)
	}
}
