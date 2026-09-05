package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dside.studio/events/internal/store"
)

const (
	sessionCookie  = "session"
	otpCookie      = "otp_email"
	sessionTTL     = 90 * 24 * time.Hour
	otpCookieTTL   = 10 * time.Minute
	otpMaxIssued   = 3
	otpIssueWindow = 15 * time.Minute
)

type ctxKey struct{}

// withAccount resolves the session cookie to an account for the request.
func (s *Server) withAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			acct, err := s.store.AccountBySession(r.Context(), hashToken(c.Value))
			if err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, acct))
			} else if !errors.Is(err, store.ErrNotFound) {
				s.renderError(w, r, http.StatusInternalServerError)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// accountFrom returns the logged-in account or nil.
func accountFrom(r *http.Request) *store.Account {
	a, _ := r.Context().Value(ctxKey{}).(*store.Account)
	return a
}

// loginRedirect sends an anonymous visitor to /login, remembering where to
// come back to: the request URI for GET/HEAD, the form's `back` field for a
// POST. A POST path itself is never used as next (it is not revisitable), so
// without a usable back field the redirect carries no next at all.
func (s *Server) loginRedirect(w http.ResponseWriter, r *http.Request) error {
	next := ""
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		next = r.URL.RequestURI()
	default:
		if back := safeNext(r.PostFormValue("back")); back != "/" {
			next = back
		}
	}
	to := "/login"
	if next != "" {
		to += "?next=" + url.QueryEscape(next)
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
	return nil
}

// safeNext returns next when it is a local absolute path, otherwise "/".
func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.HasPrefix(next, "/\\") {
		return next
	}
	return "/"
}

func newOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashCode(email, code string) string {
	sum := sha256.Sum256([]byte(email + ":" + code))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

// normalizeEmail trims, lowercases and loosely validates an address.
func normalizeEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	at := strings.LastIndexByte(s, '@')
	if len(s) > 254 || at < 1 || at == len(s)-1 || strings.ContainsAny(s, " \t\r\n") {
		return "", false
	}
	domain := s[at+1:]
	if dot := strings.IndexByte(domain, '.'); dot < 1 || dot == len(domain)-1 {
		return "", false
	}
	return s, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) setOTPCookie(w http.ResponseWriter, email string) {
	http.SetCookie(w, &http.Cookie{
		Name: otpCookie, Value: email, Path: "/login", MaxAge: int(otpCookieTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearOTPCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: otpCookie, Value: "", Path: "/login", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}
