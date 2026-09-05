package main

import (
	"crypto/subtle"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dside.studio/events/internal/store"
)

// Login copy. template.HTML so apostrophes reach the page verbatim; these
// are constants and never carry user input.
const (
	msgBadEmail    template.HTML = "Enter a valid email address."
	msgRateLimited template.HTML = "Too many codes requested for this email. Wait 15 minutes and try again."
	msgWrongCode   template.HTML = "That code didn't match. Check it and try again."
	msgExpiredCode template.HTML = "That code has expired. Request a new one."
	msgExhausted   template.HTML = "Too many attempts. Wait a few minutes and request a new code."
)

type loginPage struct {
	Base
	Email string
	Next  string
	Error template.HTML // constant copy only, never user input
	TTL   string
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) error {
	if accountFrom(r) != nil {
		return redirect(w, r, "/")
	}
	return s.render(w, r, http.StatusOK, "login", loginPage{Base: s.base(r), Next: safeNextOrEmpty(r.URL.Query().Get("next"))})
}

// safeNextOrEmpty keeps a local path or drops the value.
func safeNextOrEmpty(next string) string {
	if next == "" || safeNext(next) != next {
		return ""
	}
	return next
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) error {
	if accountFrom(r) != nil {
		return redirect(w, r, "/")
	}
	next := safeNextOrEmpty(r.PostFormValue("next"))
	raw := r.PostFormValue("email")
	page := loginPage{Base: s.base(r), Email: raw, Next: next}
	email, ok := normalizeEmail(raw)
	if !ok {
		page.Error = msgBadEmail
		return s.render(w, r, http.StatusUnprocessableEntity, "login", page)
	}
	n, err := s.store.CountRecentOTPs(r.Context(), email, time.Now().Add(-otpIssueWindow))
	if err != nil {
		return err
	}
	if n >= otpMaxIssued {
		page.Error = msgRateLimited
		return s.render(w, r, http.StatusUnprocessableEntity, "login", page)
	}
	code, err := newOTPCode()
	if err != nil {
		return err
	}
	if err := s.store.CreateOTP(r.Context(), email, hashCode(email, code), time.Now().Add(s.cfg.OTPTTL)); err != nil {
		return err
	}
	if err := s.mail.SendCode(r.Context(), email, code); err != nil {
		return err
	}
	s.setOTPCookie(w, email)
	to := "/login/code"
	if next != "" {
		to += "?next=" + url.QueryEscape(next)
	}
	return redirect(w, r, to)
}

func (s *Server) otpEmail(r *http.Request) string {
	c, err := r.Cookie(otpCookie)
	if err != nil {
		return ""
	}
	email, ok := normalizeEmail(c.Value)
	if !ok {
		return ""
	}
	return email
}

func (s *Server) codePage(r *http.Request, email, next string, msg template.HTML) loginPage {
	return loginPage{Base: s.base(r), Email: email, Next: next, Error: msg, TTL: humanDuration(s.cfg.OTPTTL)}
}

// humanDuration renders the OTP lifetime for the code page ("10 minutes").
func humanDuration(d time.Duration) string {
	switch m := int(d.Round(time.Minute).Minutes()); {
	case m <= 0:
		return d.String()
	case m == 1:
		return "1 minute"
	default:
		return strconv.Itoa(m) + " minutes"
	}
}

func (s *Server) codeForm(w http.ResponseWriter, r *http.Request) error {
	if accountFrom(r) != nil {
		return redirect(w, r, "/")
	}
	email := s.otpEmail(r)
	if email == "" {
		return redirect(w, r, "/login")
	}
	return s.render(w, r, http.StatusOK, "login_code", s.codePage(r, email, safeNextOrEmpty(r.URL.Query().Get("next")), ""))
}

func (s *Server) codeSubmit(w http.ResponseWriter, r *http.Request) error {
	if accountFrom(r) != nil {
		return redirect(w, r, "/")
	}
	email := s.otpEmail(r)
	if email == "" {
		return redirect(w, r, "/login")
	}
	next := safeNextOrEmpty(r.PostFormValue("next"))
	code := strings.TrimSpace(r.PostFormValue("code"))
	fail := func(msg template.HTML) error {
		return s.render(w, r, http.StatusUnprocessableEntity, "login_code", s.codePage(r, email, next, msg))
	}
	otp, err := s.store.LatestOTP(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		return fail(msgExpiredCode)
	}
	if err != nil {
		return err
	}
	if time.Now().After(otp.ExpiresAt) {
		return fail(msgExpiredCode)
	}
	if otp.Attempts >= s.cfg.OTPMaxAttempts {
		return fail(msgExhausted)
	}
	if subtle.ConstantTimeCompare([]byte(hashCode(email, code)), []byte(otp.CodeHash)) != 1 {
		attempts, err := s.store.BumpOTPAttempts(r.Context(), otp.ID)
		if err != nil {
			return err
		}
		if attempts >= s.cfg.OTPMaxAttempts {
			return fail(msgExhausted)
		}
		return fail(msgWrongCode)
	}
	if err := s.store.ConsumeOTP(r.Context(), otp.ID); err != nil {
		return err
	}
	acct, err := s.store.EnsureUser(r.Context(), email)
	if err != nil {
		return err
	}
	token, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.CreateSession(r.Context(), hashToken(token), acct.ID, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	s.setSessionCookie(w, token)
	s.clearOTPCookie(w)
	return redirect(w, r, safeNext(next))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if accountFrom(r) == nil {
		return s.loginRedirect(w, r)
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := s.store.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			return err
		}
	}
	s.clearSessionCookie(w)
	return redirect(w, r, "/")
}

type tagFollowRow struct {
	Tag       string
	Following bool
}

type accountPage struct {
	Base
	Email   string
	Role    string
	Poster  bool
	Slug    string
	Tags    []tagFollowRow
	Posters []store.Account
	Error   string
}

func (s *Server) accountData(r *http.Request, acct *store.Account) (accountPage, error) {
	follows, err := s.store.Follows(r.Context(), acct.ID)
	if err != nil {
		return accountPage{}, err
	}
	p := accountPage{Base: s.base(r), Email: acct.Email, Role: acct.Role, Poster: acct.IsPoster(), Slug: acct.Slug, Posters: follows.Posters}
	for _, t := range tags {
		p.Tags = append(p.Tags, tagFollowRow{Tag: t, Following: follows.HasTag(t)})
	}
	return p, nil
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.loginRedirect(w, r)
	}
	p, err := s.accountData(r, acct)
	if err != nil {
		return err
	}
	return s.render(w, r, http.StatusOK, "account", p)
}

func (s *Server) accountDelete(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.loginRedirect(w, r)
	}
	if acct.IsPoster() {
		return errCuratorAccount
	}
	if r.PostFormValue("confirm") != "1" {
		p, err := s.accountData(r, acct)
		if err != nil {
			return err
		}
		p.Error = "Tick the box to confirm you want to delete your account."
		return s.render(w, r, http.StatusUnprocessableEntity, "account", p)
	}
	if err := s.store.DeleteAccount(r.Context(), acct.ID); err != nil {
		return err
	}
	s.clearSessionCookie(w)
	return redirect(w, r, "/")
}
