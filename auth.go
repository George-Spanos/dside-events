package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"dside.studio/events/internal/store"
)

const (
	sessionCookie = "session"
	sessionTTL    = 365 * 24 * time.Hour
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

// accountFrom returns the request's account or nil.
func accountFrom(r *http.Request) *store.Account {
	a, _ := r.Context().Value(ctxKey{}).(*store.Account)
	return a
}

// ensureAccount returns the request's account, creating an anonymous user
// account plus a session (and setting the cookie) when there is none. Callers
// must validate their target first so a 404 never leaves an orphan account.
func (s *Server) ensureAccount(w http.ResponseWriter, r *http.Request) (*store.Account, error) {
	if a := accountFrom(r); a != nil {
		return a, nil
	}
	a, _, err := s.store.CreateUser(r.Context())
	if err != nil {
		return nil, err
	}
	if err := s.startSession(w, r, a.ID); err != nil {
		return nil, err
	}
	return a, nil
}

// startSession creates a session for the account and sets the cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, accountID int64) error {
	token, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.CreateSession(r.Context(), hashToken(token), accountID, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	s.setSessionCookie(w, token)
	return nil
}

// endSession deletes the request's session row (if any) and clears the cookie.
func (s *Server) endSession(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := s.store.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			return err
		}
	}
	s.clearSessionCookie(w)
	return nil
}

// safeNext returns next when it is a local absolute path, otherwise "/".
func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.HasPrefix(next, "/\\") {
		return next
	}
	return "/"
}

// backOr returns the form's `back` field when it is a local path, else def.
func backOr(r *http.Request, def string) string {
	if b := r.PostFormValue("back"); b != "" && safeNext(b) == b {
		return b
	}
	return def
}

// newToken returns 32 random bytes as base64url (43 characters).
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
