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

// requirePoster returns the poster account, or 403 for everyone else
// (anonymous visitors included: there is no login page to send them to).
func (s *Server) requirePoster(r *http.Request) (*store.Account, error) {
	acct := accountFrom(r)
	if !acct.IsPoster() {
		return nil, errForbidden
	}
	return acct, nil
}

func (s *Server) newForm(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.requirePoster(r); err != nil {
		return err
	}
	return s.render(w, r, http.StatusOK, "eventform", s.formPage(r, &eventForm{TagSet: map[string]bool{}}, nil))
}

func (s *Server) newSubmit(w http.ResponseWriter, r *http.Request) error {
	acct, err := s.requirePoster(r)
	if err != nil {
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

// ownedEvent loads the event and checks the viewer is the poster who owns it.
func (s *Server) ownedEvent(r *http.Request) (*store.Account, *store.Event, error) {
	acct, err := s.requirePoster(r)
	if err != nil {
		return nil, nil, err
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
	_, e, err := s.ownedEvent(r)
	if err != nil {
		return err
	}
	return s.render(w, r, http.StatusOK, "eventform", s.formPage(r, s.formFromEvent(e), e))
}

func (s *Server) editSubmit(w http.ResponseWriter, r *http.Request) error {
	acct, e, err := s.ownedEvent(r)
	if err != nil {
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
	_, e, err := s.ownedEvent(r)
	if err != nil {
		return err
	}
	if err := s.store.DeleteEvent(r.Context(), e.ID); err != nil {
		return err
	}
	return redirect(w, r, "/")
}

// followEvent records the visitor's state for an event: state=follow,
// state=hide or state=clear. The first press from a device without a session
// starts its anonymous account (after the event and the state have been
// validated, so a 404 or 400 creates nothing).
func (s *Server) followEvent(w http.ResponseWriter, r *http.Request) error {
	slug := r.PathValue("slug")
	var state string
	switch r.PostFormValue("state") {
	case "follow":
		state = store.Following
	case "hide":
		state = store.Hidden
	case "clear":
		state = "clear"
	default:
		return errBadRequest
	}
	e, err := s.store.EventBySlug(r.Context(), slug, viewerID(accountFrom(r)))
	if err != nil {
		return err
	}
	if accountFrom(r) == nil && state == "clear" {
		// Nothing to clear for a device without an account; don't start one.
		return redirect(w, r, backOr(r, "/e/"+slug))
	}
	acct, err := s.ensureAccount(w, r)
	if err != nil {
		return err
	}
	if state == "clear" {
		err = s.store.ClearEventFollow(r.Context(), acct.ID, e.ID)
	} else {
		err = s.store.SetEventFollow(r.Context(), acct.ID, e.ID, state)
	}
	if err != nil {
		return err
	}
	return redirect(w, r, backOr(r, "/e/"+slug))
}
