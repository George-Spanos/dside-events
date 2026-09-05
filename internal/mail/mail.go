// Package mail delivers one-time login codes.
package mail

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"time"
)

// Sender delivers a login code to an address.
type Sender interface {
	SendCode(ctx context.Context, to, code string) error
}

// Dev logs the code and, when Path is set, appends
// "<RFC3339>\t<email>\t<code>\n" to that file. Used in development and tests.
type Dev struct {
	Path string
	Log  *slog.Logger
}

// SendCode implements Sender.
func (d Dev) SendCode(_ context.Context, to, code string) error {
	if d.Log != nil {
		d.Log.Info("login code", "email", to, "code", code)
	}
	if d.Path == "" {
		return nil
	}
	f, err := os.OpenFile(d.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\t%s\t%s\n", time.Now().UTC().Format(time.RFC3339), to, code)
	return err
}

// SMTP sends the code with net/smtp (STARTTLS negotiated by SendMail).
type SMTP struct {
	Addr string // host:port
	User string
	Pass string
	From string
}

// SendCode implements Sender.
func (s SMTP) SendCode(_ context.Context, to, code string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Your dside events code\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n"+
		"Your login code is %s\r\n\r\nIt works for a few minutes. If you didn't ask for it, ignore this email.\r\n",
		s.From, to, code)
	var auth smtp.Auth
	if s.User != "" {
		host := s.Addr
		for i := len(host) - 1; i >= 0; i-- {
			if host[i] == ':' {
				host = host[:i]
				break
			}
		}
		auth = smtp.PlainAuth("", s.User, s.Pass, host)
	}
	return smtp.SendMail(s.Addr, auth, s.From, []string{to}, []byte(msg))
}
