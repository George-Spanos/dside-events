package main

import (
	"errors"
	"net/http"

	"dside.studio/events/internal/store"
)

// secretLink opens the account behind /k/{key}: a new session for it, cookie
// set, on to /mine. Posters log in this way too. An unknown key is a 404 with
// its own copy; a session cookie already present is simply replaced.
func (s *Server) secretLink(w http.ResponseWriter, r *http.Request) error {
	acct, err := s.store.AccountByKey(r.Context(), r.PathValue("key"))
	if errors.Is(err, store.ErrNotFound) {
		return errBadLink
	}
	if err != nil {
		return err
	}
	if err := s.startSession(w, r, acct.ID); err != nil {
		return err
	}
	return redirect(w, r, "/mine")
}

type accountPage struct {
	Base
	Link   string // the secret link; "" without a session
	Poster bool
	Slug   string
	Error  string
}

func (s *Server) accountData(r *http.Request, acct *store.Account) (accountPage, error) {
	p := accountPage{Base: s.base(r)}
	if acct == nil {
		return p, nil
	}
	key, err := s.store.AccountKey(r.Context(), acct.ID)
	if err != nil {
		return p, err
	}
	p.Link, p.Poster, p.Slug = s.cfg.secretLink(key), acct.IsPoster(), acct.Slug
	return p, nil
}

// account renders one of two pages: the empty state for a device without a
// session, or the account with its secret link, follows and device controls.
func (s *Server) account(w http.ResponseWriter, r *http.Request) error {
	p, err := s.accountData(r, accountFrom(r))
	if err != nil {
		return err
	}
	return s.render(w, r, http.StatusOK, "account", p)
}

// accountKey rotates the secret link. The old one stops working.
func (s *Server) accountKey(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return redirect(w, r, "/account")
	}
	if _, err := s.store.RotateKey(r.Context(), acct.ID); err != nil {
		return err
	}
	return redirect(w, r, "/account")
}

// forget ends this device's session. The account and its data stay; the
// secret link brings them back.
func (s *Server) forget(w http.ResponseWriter, r *http.Request) error {
	if accountFrom(r) == nil {
		return redirect(w, r, "/account")
	}
	if err := s.endSession(w, r); err != nil {
		return err
	}
	return redirect(w, r, "/")
}

func (s *Server) accountDelete(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return redirect(w, r, "/account")
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
