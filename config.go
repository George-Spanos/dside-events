package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is read from the environment; every field has a default.
type Config struct {
	Addr          string
	DBPath        string
	BaseURL       string
	SecureCookies bool

	SMTPHost, SMTPPort, SMTPUser, SMTPPass, SMTPFrom string
	DevOTPFile                                       string

	OTPTTL         time.Duration
	OTPMaxAttempts int
	LogLevel       slog.Level
}

func loadConfig() (Config, error) {
	c := Config{
		Addr:           env("ADDR", ":8080"),
		DBPath:         env("DB_PATH", "events.db"),
		BaseURL:        env("BASE_URL", "http://localhost:8080"),
		SMTPHost:       os.Getenv("SMTP_HOST"),
		SMTPPort:       env("SMTP_PORT", "587"),
		SMTPUser:       os.Getenv("SMTP_USER"),
		SMTPPass:       os.Getenv("SMTP_PASS"),
		SMTPFrom:       env("SMTP_FROM", "events@dside.studio"),
		DevOTPFile:     os.Getenv("DEV_OTP_FILE"),
		OTPTTL:         10 * time.Minute,
		OTPMaxAttempts: 5,
		LogLevel:       slog.LevelInfo,
	}
	var err error
	if v := os.Getenv("SECURE_COOKIES"); v != "" {
		if c.SecureCookies, err = strconv.ParseBool(v); err != nil {
			return c, fmt.Errorf("SECURE_COOKIES: %w", err)
		}
	}
	if v := os.Getenv("OTP_TTL"); v != "" {
		if c.OTPTTL, err = time.ParseDuration(v); err != nil {
			return c, fmt.Errorf("OTP_TTL: %w", err)
		}
	}
	if v := os.Getenv("OTP_MAX_ATTEMPTS"); v != "" {
		if c.OTPMaxAttempts, err = strconv.Atoi(v); err != nil || c.OTPMaxAttempts < 1 {
			return c, fmt.Errorf("OTP_MAX_ATTEMPTS: must be a positive integer")
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		if err := c.LogLevel.UnmarshalText([]byte(strings.ToUpper(v))); err != nil {
			return c, fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
