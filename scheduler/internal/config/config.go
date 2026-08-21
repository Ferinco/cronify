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

// weakAdminTokenLength is the threshold below which AdminToken is treated as
// a brute-force risk once this deployment is reachable from anywhere but
// localhost. A real random token (e.g. `openssl rand -hex 32`, 64 chars) is
// nowhere near this; short, memorable values like "dev" are.
const weakAdminTokenLength = 20

// WeakAdminToken reports whether AdminToken is short enough to be worth
// warning about. Deliberately a soft signal the caller logs, not a load
// failure — scheduler/README.md's own local-run quickstart recommends
// CRONIFY_ADMIN_TOKEN=dev, and that must keep working unmodified.
func (c Config) WeakAdminToken() bool {
	return len(c.AdminToken) < weakAdminTokenLength
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
