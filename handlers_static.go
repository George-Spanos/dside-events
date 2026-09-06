package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"net/http"
	"strings"
	texttemplate "text/template"
)

//go:embed static
var staticFS embed.FS

// swTemplate renders static/sw.js with the build version baked in.
type swTemplate struct {
	tpl *texttemplate.Template
}

// initStatic parses the service worker template and draws the PNG icons.
func (s *Server) initStatic() error {
	raw, err := fs.ReadFile(staticFS, "static/sw.js")
	if err != nil {
		return fmt.Errorf("static/sw.js: %w", err)
	}
	tpl, err := texttemplate.New("sw.js").Parse(string(raw))
	if err != nil {
		return fmt.Errorf("static/sw.js: %w", err)
	}
	s.sw = &swTemplate{tpl: tpl}
	s.icons = map[string][]byte{}
	for _, size := range []int{180, 192, 512} {
		b, err := drawIcon(size)
		if err != nil {
			return err
		}
		s.icons[fmt.Sprintf("/icon-%d.png", size)] = b
	}
	return nil
}

// drawIcon renders the icon.svg design (64-unit plum square, white bar at
// y=40 h=5 x=12 w=40) as a PNG of the given size.
func drawIcon(size int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	plum := color.RGBA{0x6e, 0x2a, 0x5f, 0xff}
	draw.Draw(img, img.Bounds(), image.NewUniform(plum), image.Point{}, draw.Src)
	scale := func(u int) int { return u * size / 64 }
	bar := image.Rect(scale(12), scale(40), scale(52), scale(45))
	draw.Draw(img, bar, image.NewUniform(color.White), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// static serves /static/* from the embedded FS with a long immutable cache
// (URLs carry ?v=<version>).
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if info, err := fs.Stat(staticFS, name); err != nil || info.IsDir() {
		s.renderError(w, r, http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFileFS(w, r, staticFS, name)
}

func (s *Server) manifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, staticFS, "static/manifest.webmanifest")
}

func (s *Server) serviceWorker(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := s.sw.tpl.Execute(&buf, struct{ Version string }{s.version}); err != nil {
		s.renderError(w, r, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(buf.Bytes())
}

func (s *Server) iconSVG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFileFS(w, r, staticFS, "static/icon.svg")
}

func (s *Server) iconPNG(w http.ResponseWriter, r *http.Request) {
	b, ok := s.icons[r.URL.Path]
	if !ok {
		s.renderError(w, r, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(b)
}

func (s *Server) offline(w http.ResponseWriter, r *http.Request) error {
	return s.render(w, r, http.StatusOK, "offline", struct{ Base }{Base{Path: r.URL.Path, Version: s.version}})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.store.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, "db unavailable")
		return
	}
	fmt.Fprint(w, "ok")
}
