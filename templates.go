package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

// pageNames are the templates that define "title" and "content" blocks.
var pageNames = []string{"feed", "event", "poster", "mine", "account", "eventform", "error", "offline"}

// Base is embedded in every page's data.
type Base struct {
	Account *AccountView
	Path    string
	Version string
}

// AccountView is what templates know about the viewer.
type AccountView struct {
	Name   string
	Slug   string
	Poster bool
}

func (s *Server) base(r *http.Request) Base {
	b := Base{Path: r.URL.Path, Version: s.version}
	if a := accountFrom(r); a != nil {
		b.Account = &AccountView{Name: a.Name, Slug: a.Slug, Poster: a.IsPoster()}
	}
	return b
}

// parseTemplates builds one template set per page: layout + partials + page.
func parseTemplates(funcs template.FuncMap) (map[string]*template.Template, error) {
	base, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/_*.html")
	if err != nil {
		return nil, err
	}
	pages := make(map[string]*template.Template, len(pageNames))
	for _, name := range pageNames {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := t.ParseFS(templateFS, "templates/"+name+".html"); err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		pages[name] = t
	}
	return pages, nil
}

func (s *Server) funcs() template.FuncMap {
	return template.FuncMap{
		"day":        func(t time.Time) string { return t.In(s.loc).Format("Monday 2 January") },
		"clock":      func(t time.Time) string { return t.In(s.loc).Format("15:04") },
		"iso":        func(t time.Time) string { return t.In(s.loc).Format(time.RFC3339) },
		"dayLabel":   func(t time.Time) string { return s.dayLabel(t, time.Now()) },
		"paragraphs": paragraphs,
		"tagChecked": func(set map[string]bool, tag string) bool { return set[tag] },
		"inc":        func(i int) int { return i + 1 },
	}
}

// dayLabel returns "Today", "Tomorrow" or "" for t relative to now (Athens).
func (s *Server) dayLabel(t, now time.Time) string {
	today := s.midnight(now)
	d := s.midnight(t)
	switch {
	case d.Equal(today):
		return "Today"
	case d.Equal(today.AddDate(0, 0, 1)):
		return "Tomorrow"
	}
	return ""
}

// midnight returns the start of t's day in Athens.
func (s *Server) midnight(t time.Time) time.Time {
	t = t.In(s.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, s.loc)
}

// paragraphs splits plain text on blank lines.
func paragraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// render executes a page into a buffer, then writes it with the given status.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page string, data any) error {
	t, ok := s.pages[page]
	if !ok {
		return fmt.Errorf("unknown template %q", page)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return fmt.Errorf("render %s: %w", page, err)
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "same-origin")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

// redirect is a 303 helper for handlers that return error.
func redirect(w http.ResponseWriter, r *http.Request, to string) error {
	http.Redirect(w, r, to, http.StatusSeeOther)
	return nil
}
