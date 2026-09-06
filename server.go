package main

import (
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dside.studio/events/internal/store"
)

// Server holds everything handlers need.
type Server struct {
	cfg     Config
	store   *store.Store
	loc     *time.Location
	version string
	pages   map[string]*template.Template
	sw      *swTemplate
	icons   map[string][]byte // "/icon-192.png" -> PNG bytes
}

func newServer(cfg Config, st *store.Store, loc *time.Location) (*Server, error) {
	s := &Server{cfg: cfg, store: st, loc: loc, version: time.Now().UTC().Format("20060102150405")}
	pages, err := parseTemplates(s.funcs())
	if err != nil {
		return nil, err
	}
	s.pages = pages
	if err := s.initStatic(); err != nil {
		return nil, err
	}
	return s, nil
}

// routes wires the mux and the middleware chain.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	h := s.handle

	mux.HandleFunc("GET /{$}", h(s.home))
	mux.HandleFunc("GET /upcoming", h(s.upcoming))
	mux.HandleFunc("GET /following", h(s.following))
	mux.HandleFunc("GET /mine", h(s.mine))
	mux.HandleFunc("GET /e/{slug}", h(s.event))
	mux.HandleFunc("GET /e/{slug}/edit", h(s.editForm))
	mux.HandleFunc("POST /e/{slug}/edit", h(s.editSubmit))
	mux.HandleFunc("POST /e/{slug}/delete", h(s.deleteEvent))
	mux.HandleFunc("POST /e/{slug}/interest", h(s.interest))
	mux.HandleFunc("GET /p/{slug}", h(s.poster))
	mux.HandleFunc("POST /follow", h(s.follow))
	mux.HandleFunc("GET /new", h(s.newForm))
	mux.HandleFunc("POST /new", h(s.newSubmit))

	mux.HandleFunc("GET /k/{key}", h(s.secretLink))
	mux.HandleFunc("GET /account", h(s.account))
	mux.HandleFunc("POST /account/key", h(s.accountKey))
	mux.HandleFunc("POST /account/delete", h(s.accountDelete))
	mux.HandleFunc("POST /forget", h(s.forget))

	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /static/", s.static)
	mux.HandleFunc("GET /manifest.webmanifest", s.manifest)
	mux.HandleFunc("GET /sw.js", s.serviceWorker)
	mux.HandleFunc("GET /icon.svg", s.iconSVG)
	mux.HandleFunc("GET /icon-180.png", s.iconPNG)
	mux.HandleFunc("GET /icon-192.png", s.iconPNG)
	mux.HandleFunc("GET /icon-512.png", s.iconPNG)
	mux.HandleFunc("GET /offline", h(s.offline))

	var handler http.Handler = s.notFound(mux)
	handler = s.withAccount(handler)
	handler = csrfGuard(handler)
	handler = logRequests(handler)
	handler = recoverPanics(handler)
	return handler
}

// notFound renders the custom 404 page for unmatched paths while preserving
// the mux's 405 behaviour for known paths with the wrong method.
func (s *Server) notFound(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern == "" {
			for _, m := range []string{http.MethodGet, http.MethodPost} {
				if m == r.Method {
					continue
				}
				probe := r.Clone(r.Context())
				probe.Method = m
				if _, p := mux.Handler(probe); p != "" {
					w.Header().Set("Allow", m)
					s.renderError(w, r, http.StatusMethodNotAllowed)
					return
				}
			}
			s.renderError(w, r, http.StatusNotFound)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
				slog.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", rec)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
			"ms", time.Since(start).Milliseconds())
	})
}

// csrfGuard rejects cross-site form posts. Browsers send Sec-Fetch-Site;
// older ones send Origin or Referer. Requests with none of them (curl, tests)
// are allowed.
func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if sfs := r.Header.Get("Sec-Fetch-Site"); sfs != "" {
				if sfs != "same-origin" && sfs != "none" {
					http.Error(w, "cross-site request rejected", http.StatusForbidden)
					return
				}
			} else if src := firstNonEmpty(r.Header.Get("Origin"), r.Header.Get("Referer")); src != "" && src != "null" {
				u, err := url.Parse(src)
				if err != nil || !sameHost(u.Host, r.Host) {
					http.Error(w, "cross-site request rejected", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// sameHost compares hosts, ignoring the port when one side has none.
func sameHost(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	ha, pa, err := net.SplitHostPort(a)
	if err != nil {
		ha = a
	}
	hb, pb, err := net.SplitHostPort(b)
	if err != nil {
		hb = b
	}
	return strings.EqualFold(ha, hb) && (pa == "" || pb == "")
}
