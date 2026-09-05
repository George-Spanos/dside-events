package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"dside.studio/events/internal/store"
)

// httpError carries a status code out of a handler.
type httpError struct {
	status  int
	message string // optional copy shown instead of the generic text
}

func (e *httpError) Error() string {
	return fmt.Sprintf("http %d %s", e.status, http.StatusText(e.status))
}

var (
	errBadRequest     = &httpError{status: http.StatusBadRequest}
	errForbidden      = &httpError{status: http.StatusForbidden}
	errNotFound       = &httpError{status: http.StatusNotFound}
	errCuratorAccount = &httpError{status: http.StatusForbidden, message: "Curator accounts are removed by hand. Ask us."}
	errBadLink        = &httpError{status: http.StatusNotFound, message: "This link doesn't work. It may have been replaced with a new one."}
)

// handlerFunc is an http.HandlerFunc that may return an error instead of
// writing a response.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

// handle adapts a handlerFunc, mapping errors to error pages.
func (s *Server) handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err == nil {
			return
		}
		var he *httpError
		switch {
		case errors.As(err, &he):
			s.renderErrorMessage(w, r, he.status, he.message)
		case errors.Is(err, store.ErrNotFound):
			s.renderError(w, r, http.StatusNotFound)
		default:
			slog.Error("handler failed", "method", r.Method, "path", r.URL.Path, "err", err)
			s.renderError(w, r, http.StatusInternalServerError)
		}
	}
}

// errorPage is the data for error.html.
type errorPage struct {
	Base
	Status  int
	Message string // "" → the template's generic copy for Status
}

// renderError writes the custom error page (plain text for 405).
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int) {
	s.renderErrorMessage(w, r, status, "")
}

// renderErrorMessage is renderError with specific copy replacing the generic text.
func (s *Server) renderErrorMessage(w http.ResponseWriter, r *http.Request, status int, message string) {
	if status == http.StatusMethodNotAllowed {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		fmt.Fprintln(w, http.StatusText(status))
		return
	}
	if err := s.render(w, r, status, "error", errorPage{Base: s.base(r), Status: status, Message: message}); err != nil {
		slog.Error("render error page", "err", err)
		http.Error(w, http.StatusText(status), status)
	}
}
