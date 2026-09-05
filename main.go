// Command dside-events is the event discovery site for Athens: a Go binary,
// a SQLite file and server-rendered HTML.
//
//	dside-events serve
//	dside-events add-poster -email x -name y [-slug z]
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

	"dside.studio/events/internal/mail"
	"dside.studio/events/internal/store"
)

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
		return errors.New("usage: dside-events serve | add-poster -email x -name y [-slug z]")
	}
	switch args[0] {
	case "serve":
		return serve(cfg)
	case "add-poster":
		return addPoster(cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q (want serve or add-poster)", args[0])
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

	var sender mail.Sender
	if cfg.SMTPHost != "" {
		sender = mail.SMTP{Addr: net.JoinHostPort(cfg.SMTPHost, cfg.SMTPPort), User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom}
	} else {
		sender = mail.Dev{Path: cfg.DevOTPFile, Log: slog.Default()}
	}
	srv, err := newServer(cfg, st, sender, loc)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.Addr)
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

func addPoster(cfg Config, args []string) error {
	fs := flag.NewFlagSet("add-poster", flag.ContinueOnError)
	email := fs.String("email", "", "poster email (required)")
	name := fs.String("name", "", "public curator name (required)")
	slug := fs.String("slug", "", "public slug (default: derived from name)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	e, ok := normalizeEmail(*email)
	if !ok || *name == "" {
		return errors.New("add-poster: -email and -name are required")
	}
	explicit := *slug != ""
	base := slugBase(*slug)
	if !explicit {
		base = slugBase(*name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()
	acct, err := st.UpsertPoster(ctx, e, *name, base)
	if err != nil {
		return err
	}
	fmt.Printf("poster %s %s\n", acct.Slug, acct.Email)
	return nil
}
