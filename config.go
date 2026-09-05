package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config is read from the environment; every field has a default.
type Config struct {
	Addr          string
	DBPath        string
	BaseURL       string
	SecureCookies bool
	LogLevel      slog.Level
}

func loadConfig() (Config, error) {
	c := Config{
		Addr:     env("ADDR", ":8080"),
		DBPath:   env("DB_PATH", "events.db"),
		BaseURL:  strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		LogLevel: slog.LevelInfo,
	}
	var err error
	if v := os.Getenv("SECURE_COOKIES"); v != "" {
		if c.SecureCookies, err = strconv.ParseBool(v); err != nil {
			return c, fmt.Errorf("SECURE_COOKIES: %w", err)
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		if err := c.LogLevel.UnmarshalText([]byte(strings.ToUpper(v))); err != nil {
			return c, fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}
	return c, nil
}

// secretLink is the URL that opens the account with the given key.
func (c Config) secretLink(key string) string { return c.BaseURL + "/k/" + key }

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
