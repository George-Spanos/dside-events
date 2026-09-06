package main

// Everything search engines read: page titles and descriptions, canonical
// links, robots.txt, sitemap.xml and the schema.org Event that lets Google
// list an event in its own right.
//
// Two rules run through this file. Only describe what the page actually
// shows — a claim made to win a rich result is a lie whether or not it
// works. And say which URL should rank: the site renders the same events on
// several paths, so every page names its canonical one.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const brand = "dside events"

// siteDesc is the fallback description: what the whole site is.
const siteDesc = "Concerts, theater, films and exhibitions in Athens, posted by the people who put them on. Dates, venues, prices and ticket links — no ads, no sign-up."

// tagCopy is how each tag reads to somebody looking at a search result. The
// tag list itself stays in tags.go; this only adds the words, so a missing
// entry falls back to the tag.
var tagCopy = map[string]struct{ Title, Desc string }{
	"concert":    {"Concerts in Athens", "Upcoming concerts in Athens with dates, venues, prices and ticket links. Jazz, rock, classical and everything else our curators are going to."},
	"theater":    {"Theater in Athens", "Upcoming theater in Athens with dates, venues, prices and ticket links, posted by the people staging the work."},
	"film":       {"Films in Athens", "What is screening in Athens: open-air cinemas, repertory programmes and premieres, with times, venues and ticket links."},
	"exhibition": {"Exhibitions in Athens", "Exhibitions open in Athens now and opening soon, with venues, dates and entry prices."},
}

// tagTitle and tagDesc name a tag page for search. An unknown tag cannot
// reach here — tagFilter 404s first — but both degrade rather than panic.
func tagTitle(tag string) string {
	if c, ok := tagCopy[tag]; ok {
		return c.Title
	}
	return tag + " in Athens"
}

func tagDesc(tag string) string {
	if c, ok := tagCopy[tag]; ok {
		return c.Desc
	}
	return "Upcoming " + tag + " events in Athens."
}

// title suffixes the brand, unless the page is the brand.
func title(s string) string {
	if s == "" || s == brand {
		return brand
	}
	return s + " · " + brand
}

// abs turns a rooted path into the absolute URL search engines must be
// given. Canonical links and sitemaps are worthless as relative URLs.
func (s *Server) abs(path string) string { return s.cfg.BaseURL + path }

// seo fills in what search engines read: the page title, the description
// under it, and the canonical path — the URL that should rank when several
// show the same events.
func (s *Server) seo(b Base, pageTitle, desc, canonical string) Base {
	b.Title, b.Description, b.Canonical = title(pageTitle), desc, s.abs(canonical)
	return b
}

// hidden marks a page that has no business in search results: a private
// list, a form, an error, the offline fallback. It carries a title for the
// browser tab and a noindex for everyone else.
func hidden(b Base, pageTitle string) Base {
	b.Title, b.NoIndex = title(pageTitle), true
	return b
}

// tagPath is the canonical path of a tag view: always the full list, never
// the home page's first ten. Home with a tag shows a truncated version of
// the same thing, so it points here.
func tagPath(base, tag string) string {
	if tag == "" {
		return base
	}
	return base + "?tag=" + url.QueryEscape(tag)
}

// descMax is where a description is cut. Google picks its own snippet length
// per query and device, so this is a tidy limit, not a rule.
const descMax = 160

// clamp shortens text to at most descMax characters, breaking on a word so
// the result reads as a sentence rather than a truncation.
func clamp(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= descMax {
		return text
	}
	cut := text[:descMax]
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > descMax/2 {
		cut = cut[:i]
	}
	return strings.TrimRightFunc(cut, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",;:·-–—", r)
	}) + "…"
}

// eventDesc leads with the facts somebody scanning a result wants — when,
// where, how much — and then as much of the curator's description as fits.
func (s *Server) eventDesc(e eventView) string {
	facts := []string{fmt.Sprintf("%s, %s", e.Start.In(s.loc).Format("Monday 2 January"), e.Start.In(s.loc).Format("15:04"))}
	if e.Venue != "" {
		facts = append(facts, e.Venue)
	}
	if e.Price != "" {
		facts = append(facts, e.Price)
	}
	return clamp(strings.Join(facts, ". ") + ". " + e.Description)
}

// eventTitle is the title, the venue and the date — the three things people
// actually type. The venue is cut at its first comma, which drops the
// neighbourhood and keeps the name.
func (s *Server) eventTitle(e eventView) string {
	parts := []string{e.Title}
	if v := venueName(e.Venue); v != "" {
		parts = append(parts, v)
	}
	parts = append(parts, e.Start.In(s.loc).Format("2 Jan 2006"))
	return title(strings.Join(parts, " · "))
}

// venueName is the venue up to its first comma: "Floyd, Gazi" → "Floyd".
func venueName(venue string) string {
	if i := strings.IndexByte(venue, ','); i >= 0 {
		return strings.TrimSpace(venue[:i])
	}
	return strings.TrimSpace(venue)
}

// eventCanonical is the URL that should rank for an event. A single event
// stands for itself; one date of a series hands its claim to the date that
// represents the run, so nine identical pages become one.
func (s *Server) eventCanonical(ctx context.Context, e eventView) string {
	if e.SeriesCount > 1 {
		if slug, err := s.store.SeriesCanonicalSlug(ctx, e.SeriesID, time.Now()); err == nil {
			return s.abs("/e/" + slug)
		}
		// A failed lookup is not worth a 500: fall back to self-canonical,
		// which is what the page would have said without a series.
	}
	return s.abs("/e/" + e.Slug)
}

// robots lets everything through except the secret links. The pages that
// must not be indexed carry a noindex meta tag instead of a Disallow here,
// because a crawler has to fetch a page to read a noindex — disallowing and
// noindexing the same URL cancels out.
func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /k/\n\nSitemap: %s\n", s.abs("/sitemap.xml"))
}

// urlEntry is one <url> of the sitemap.
type urlEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	NS      string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

// sitemap lists the canonical URLs only: the two lists, one landing page per
// tag, every curator, and one page per event — a series contributing the one
// date the others point at.
func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) error {
	set := urlSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	add := func(path, lastmod string) {
		set.URLs = append(set.URLs, urlEntry{Loc: s.abs(path), LastMod: lastmod})
	}
	add("/", "")
	add("/upcoming", "")
	for _, tag := range tags {
		add(tagPath("/upcoming", tag), "")
	}
	events, err := s.store.IndexableEvents(r.Context(), time.Now())
	if err != nil {
		return err
	}
	for _, e := range events {
		add("/e/"+e.Slug, e.UpdatedAt.In(s.loc).Format(time.RFC3339))
	}
	posters, err := s.store.PosterSlugs(r.Context())
	if err != nil {
		return err
	}
	for _, slug := range posters {
		add("/p/"+slug, "")
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(set)
}

// The schema.org subset Google reads for an event. Only what the page shows
// is filled in: there is no organizer or performer here, because a curator
// is the person who posted the listing, not the person putting the event on,
// and claiming otherwise would be a lie in machine-readable form.
type ldEvent struct {
	Context             string    `json:"@context"`
	Type                string    `json:"@type"`
	Name                string    `json:"name"`
	StartDate           string    `json:"startDate"`
	EventStatus         string    `json:"eventStatus"`
	EventAttendanceMode string    `json:"eventAttendanceMode"`
	Location            ldPlace   `json:"location"`
	Description         string    `json:"description,omitempty"`
	URL                 string    `json:"url"`
	Offers              *ldOffers `json:"offers,omitempty"`
}

type ldPlace struct {
	Type    string    `json:"@type"`
	Name    string    `json:"name"`
	Address ldAddress `json:"address"`
}

type ldAddress struct {
	Type            string `json:"@type"`
	AddressLocality string `json:"addressLocality"`
	AddressCountry  string `json:"addressCountry"`
}

type ldOffers struct {
	Type          string `json:"@type"`
	Price         string `json:"price"`
	PriceCurrency string `json:"priceCurrency"`
	URL           string `json:"url"`
}

// priceAmount reads the figure out of a free-text price such as "18€",
// "from 15€" or "Free". It gives up when the text carries no single
// unambiguous number ("8–12€", "ask at the door"), and an event whose price
// it cannot read ships without an offer rather than with a guess.
func priceAmount(text string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return "", false
	}
	if t == "free" || t == "δωρεάν" {
		return "0", true
	}
	var nums []string
	var cur strings.Builder
	for _, r := range t {
		switch {
		case unicode.IsDigit(r):
			cur.WriteRune(r)
		case (r == '.' || r == ',') && cur.Len() > 0:
			cur.WriteRune('.')
		default:
			if cur.Len() > 0 {
				nums = append(nums, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		nums = append(nums, cur.String())
	}
	if len(nums) != 1 {
		return "", false
	}
	return strings.Trim(nums[0], "."), true
}

// eventJSONLD is the structured data for an event page, or "" when the page
// is one date of a series pointing at another: the canonical page is the one
// that describes the run, and repeating the claim here would compete with it.
func (s *Server) eventJSONLD(e eventView, canonical string) template.JS {
	if canonical != s.abs("/e/"+e.Slug) {
		return ""
	}
	ld := ldEvent{
		Context:             "https://schema.org",
		Type:                "Event",
		Name:                e.Title,
		StartDate:           e.Start.In(s.loc).Format(time.RFC3339),
		EventStatus:         "https://schema.org/EventScheduled",
		EventAttendanceMode: "https://schema.org/OfflineEventAttendanceMode",
		Location: ldPlace{
			Type: "Place",
			Name: e.Venue,
			Address: ldAddress{
				Type:            "PostalAddress",
				AddressLocality: "Athens",
				AddressCountry:  "GR",
			},
		},
		Description: e.Description,
		URL:         canonical,
	}
	if amount, ok := priceAmount(e.Price); ok {
		ld.Offers = &ldOffers{Type: "Offer", Price: amount, PriceCurrency: "EUR", URL: canonical}
	}
	// Marshal escapes <, > and &, so a description containing "</script>"
	// cannot break out of the surrounding tag.
	b, err := json.Marshal(ld)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

// siteJSONLD names the site itself on the home page.
func (s *Server) siteJSONLD() template.JS {
	b, err := json.Marshal(struct {
		Context     string `json:"@context"`
		Type        string `json:"@type"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
		AreaServed  string `json:"areaServed"`
	}{"https://schema.org", "Organization", brand, s.cfg.BaseURL, siteDesc, "Athens, Greece"})
	if err != nil {
		return ""
	}
	return template.JS(b)
}
