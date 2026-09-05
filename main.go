// Command dside-events is the event discovery site for Athens: a Go binary,
// a SQLite file and server-rendered HTML.
//
//	dside-events serve
//	dside-events add-poster -name "Maria P." [-slug maria]
//	dside-events poster-link -slug maria
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"dside.studio/events/internal/store"
)

const usage = "usage: dside-events serve | add-poster -name x [-slug y] | poster-link -slug y"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dside-events:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "serve":
		return serve(cfg)
	case "add-poster":
		return addPoster(cfg, args[1:])
	case "poster-link":
		return posterLink(cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func serve(cfg Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loc, err := time.LoadLocation("Europe/Athens")
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	if os.Getenv("BASE_URL") == "" {
		// No public URL configured: secret links point at the listener itself.
		cfg.BaseURL = listenBaseURL(ln.Addr())
	}
	srv, err := newServer(cfg, st, loc)
	if err != nil {
		return err
	}
	// The one and only line on stdout; the e2e harness waits for it.
	fmt.Printf("listening on http://%s\n", ln.Addr().String())

	hs := &http.Server{
		Handler:           srv.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- hs.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return hs.Shutdown(shutdownCtx)
}

// listenBaseURL is the URL of a listener: http://host:port, with an
// unspecified host ("" / 0.0.0.0 / ::) shown as localhost.
func listenBaseURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// openStore opens the database for a CLI command.
func openStore(cfg Config) (*store.Store, context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	return st, ctx, cancel, nil
}

// addPoster creates a curator account and prints its slug and secret link.
// Re-running for an existing slug changes nothing and prints no link.
func addPoster(cfg Config, args []string) error {
	fs := flag.NewFlagSet("add-poster", flag.ContinueOnError)
	name := fs.String("name", "", "public curator name (required)")
	slug := fs.String("slug", "", "public slug (default: derived from name)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return errors.New("add-poster: -name is required")
	}
	base := slugBase(*name)
	if *slug != "" {
		base = slugBase(*slug)
	}
	st, ctx, cancel, err := openStore(cfg)
	if err != nil {
		return err
	}
	defer cancel()
	defer st.Close()
	acct, key, err := st.CreatePoster(ctx, *name, base)
	if err != nil {
		return err
	}
	fmt.Printf("poster %s\n", acct.Slug)
	if key == "" {
		fmt.Println("link (unchanged, run poster-link to get a new one)")
		return nil
	}
	fmt.Printf("link %s\n", cfg.secretLink(key))
	return nil
}

// posterLink rotates a curator's secret link and prints the new one.
func posterLink(cfg Config, args []string) error {
	fs := flag.NewFlagSet("poster-link", flag.ContinueOnError)
	slug := fs.String("slug", "", "public slug of the curator (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *slug == "" {
		return errors.New("poster-link: -slug is required")
	}
	st, ctx, cancel, err := openStore(cfg)
	if err != nil {
		return err
	}
	defer cancel()
	defer st.Close()
	acct, err := st.PosterBySlug(ctx, *slug)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("poster-link: no curator with slug %q", *slug)
	}
	if err != nil {
		return err
	}
	key, err := st.RotateKey(ctx, acct.ID)
	if err != nil {
		return err
	}
	fmt.Printf("link %s\n", cfg.secretLink(key))
	return nil
}
