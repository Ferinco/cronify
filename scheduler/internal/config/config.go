// Package config loads scheduler configuration from environment variables, per CLAUDE.md's env var naming.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AdminToken             string
	Port                   int
	DBPath                 string
	TickInterval           time.Duration
	DefaultTimeoutSeconds  int
	DefaultMaxAttempts     int
	StaleLockTimeoutSecs   int
	WebhookURL             string // optional, stub-only for this pass
}

func Load() (Config, error) {
	token := os.Getenv("CRONIFY_ADMIN_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("CRONIFY_ADMIN_TOKEN must be set")
	}

	port, err := intEnv("CRONIFY_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	tickSeconds, err := intEnv("CRONIFY_TICK_INTERVAL_SECONDS", 60)
	if err != nil {
		return Config{}, err
	}
	defaultTimeout, err := intEnv("CRONIFY_DEFAULT_TIMEOUT_SECONDS", 30)
	if err != nil {
		return Config{}, err
	}
	defaultMaxAttempts, err := intEnv("CRONIFY_DEFAULT_MAX_ATTEMPTS", 3)
	if err != nil {
		return Config{}, err
	}
	staleLockTimeout, err := intEnv("CRONIFY_STALE_LOCK_TIMEOUT_SECONDS", 600)
	if err != nil {
		return Config{}, err
	}

	dbPath := os.Getenv("CRONIFY_DB_PATH")
	if dbPath == "" {
		dbPath = "./cronify.db"
	}

	return Config{
		AdminToken:            token,
		Port:                  port,
		DBPath:                dbPath,
		TickInterval:          time.Duration(tickSeconds) * time.Second,
		DefaultTimeoutSeconds: defaultTimeout,
		DefaultMaxAttempts:    defaultMaxAttempts,
		StaleLockTimeoutSecs:  staleLockTimeout,
		WebhookURL:            os.Getenv("CRONIFY_WEBHOOK_URL"),
	}, nil
}

func intEnv(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", name, v)
	}
	return n, nil
}
