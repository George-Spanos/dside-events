package main

import (
	"net/http"
	"time"
)

const themeCookie = "theme"

// themeFrom returns the visitor's chosen theme, "light" or "dark", or "" when
// the page should follow the operating system.
func themeFrom(r *http.Request) string {
	c, err := r.Cookie(themeCookie)
	if err != nil {
		return ""
	}
	if c.Value == "light" || c.Value == "dark" {
		return c.Value
	}
	return ""
}

// theme stores the light/dark choice in a one-year cookie. It is a plain form
// post so it works without JavaScript; "auto" removes the choice.
func (s *Server) theme(w http.ResponseWriter, r *http.Request) error {
	choice := r.PostFormValue("theme")
	switch choice {
	case "light", "dark":
		http.SetCookie(w, &http.Cookie{Name: themeCookie, Value: choice, Path: "/", MaxAge: int((365 * 24 * time.Hour).Seconds()), SameSite: http.SameSiteLaxMode, Secure: s.cfg.SecureCookies})
	case "auto":
		http.SetCookie(w, &http.Cookie{Name: themeCookie, Value: "", Path: "/", MaxAge: -1, SameSite: http.SameSiteLaxMode, Secure: s.cfg.SecureCookies})
	default:
		return errBadRequest
	}
	return redirect(w, r, backOr(r, "/"))
}
