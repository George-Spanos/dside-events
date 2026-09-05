package main

import (
	"net/http"

	"dside.studio/events/internal/store"
)

type eventFormPage struct {
	Base
	Edit   bool
	Slug   string
	Form   *eventForm
	Tags   []string
	Cancel string
}

func (s *Server) formPage(r *http.Request, f *eventForm, e *store.Event) eventFormPage {
	p := eventFormPage{Base: s.base(r), Form: f, Tags: tags, Cancel: "/"}
	if e != nil {
		p.Edit, p.Slug, p.Cancel = true, e.Slug, "/e/"+e.Slug
	}
	return p
}

// requirePoster returns the poster account, or writes the login redirect /
// returns 403. A nil account with a nil error means the redirect was sent.
func (s *Server) requirePoster(w http.ResponseWriter, r *http.Request) (*store.Account, error) {
	acct := accountFrom(r)
	if acct == nil {
		return nil, s.loginRedirect(w, r)
	}
	if !acct.IsPoster() {
		return nil, errForbidden
	}
	return acct, nil
}

func (s *Server) newForm(w http.ResponseWriter, r *http.Request) error {
	acct, err := s.requirePoster(w, r)
	if acct == nil {
		return err
	}
	return s.render(w, r, http.StatusOK, "eventform", s.formPage(r, &eventForm{TagSet: map[string]bool{}}, nil))
}

func (s *Server) newSubmit(w http.ResponseWriter, r *http.Request) error {
	acct, err := s.requirePoster(w, r)
	if acct == nil {
		return err
	}
	f, in := s.parseEventForm(r)
	if in == nil {
		return s.render(w, r, http.StatusUnprocessableEntity, "eventform", s.formPage(r, f, nil))
	}
	if dup, err := s.checkDuplicate(r, f, acct.ID, in, 0); err != nil {
		return err
	} else if dup {
		return s.render(w, r, http.StatusUnprocessableEntity, "eventform", s.formPage(r, f, nil))
	}
	slug, err := s.store.CreateEvent(r.Context(), acct.ID, eventSlug(in.Title, in.StartsAt, s.loc), *in)
	if err != nil {
		return err
	}
	return redirect(w, r, "/e/"+slug)
}

// ownedEvent loads the event and checks the viewer owns it.
func (s *Server) ownedEvent(w http.ResponseWriter, r *http.Request) (*store.Account, *store.Event, error) {
	acct := accountFrom(r)
	if acct == nil {
		return nil, nil, s.loginRedirect(w, r)
	}
	e, err := s.store.EventBySlug(r.Context(), r.PathValue("slug"), acct.ID)
	if err != nil {
		return nil, nil, err
	}
	if e.PosterID != acct.ID {
		return nil, nil, errForbidden
	}
	return acct, e, nil
}

func (s *Server) editForm(w http.ResponseWriter, r *http.Request) error {
	_, e, err := s.ownedEvent(w, r)
	if e == nil {
		return err
	}
	return s.render(w, r, http.StatusOK, "eventform", s.formPage(r, s.formFromEvent(e), e))
}

func (s *Server) editSubmit(w http.ResponseWriter, r *http.Request) error {
	acct, e, err := s.ownedEvent(w, r)
	if e == nil {
		return err
	}
	f, in := s.parseEventForm(r)
	if in == nil {
		return s.render(w, r, http.StatusUnprocessableEntity, "eventform", s.formPage(r, f, e))
	}
	if dup, err := s.checkDuplicate(r, f, acct.ID, in, e.ID); err != nil {
		return err
	} else if dup {
		return s.render(w, r, http.StatusUnprocessableEntity, "eventform", s.formPage(r, f, e))
	}
	if err := s.store.UpdateEvent(r.Context(), e.ID, *in); err != nil {
		return err
	}
	return redirect(w, r, "/e/"+e.Slug)
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request) error {
	_, e, err := s.ownedEvent(w, r)
	if e == nil {
		return err
	}
	if err := s.store.DeleteEvent(r.Context(), e.ID); err != nil {
		return err
	}
	return redirect(w, r, "/")
}

func (s *Server) interest(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.loginRedirect(w, r)
	}
	slug := r.PathValue("slug")
	state := r.PostFormValue("state")
	if state != store.Interested && state != store.NotInterested && state != "clear" {
		return errBadRequest
	}
	e, err := s.store.EventBySlug(r.Context(), slug, acct.ID)
	if err != nil {
		return err
	}
	if state == "clear" {
		err = s.store.ClearInterest(r.Context(), acct.ID, e.ID)
	} else {
		err = s.store.SetInterest(r.Context(), acct.ID, e.ID, state)
	}
	if err != nil {
		return err
	}
	back := "/e/" + slug
	if b := r.PostFormValue("back"); b != "" && safeNext(b) == b {
		back = b
	}
	return redirect(w, r, back)
}

func (s *Server) follow(w http.ResponseWriter, r *http.Request) error {
	acct := accountFrom(r)
	if acct == nil {
		return s.loginRedirect(w, r)
	}
	kind, key, on := r.PostFormValue("kind"), r.PostFormValue("key"), r.PostFormValue("on")
	if on != "1" && on != "0" {
		return errBadRequest
	}
	var err error
	switch kind {
	case "tag":
		if !validTag(key) {
			return errNotFound
		}
		if on == "1" {
			err = s.store.FollowTag(r.Context(), acct.ID, key)
		} else {
			err = s.store.UnfollowTag(r.Context(), acct.ID, key)
		}
	case "poster":
		p, perr := s.store.PosterBySlug(r.Context(), key)
		if perr != nil {
			return perr
		}
		if on == "1" {
			err = s.store.FollowPoster(r.Context(), acct.ID, p.ID)
		} else {
			err = s.store.UnfollowPoster(r.Context(), acct.ID, p.ID)
		}
	default:
		return errBadRequest
	}
	if err != nil {
		return err
	}
	back := "/"
	if b := r.PostFormValue("back"); b != "" && safeNext(b) == b {
		back = b
	}
	return redirect(w, r, back)
}
