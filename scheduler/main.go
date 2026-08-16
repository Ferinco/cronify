// Command cronify-scheduler is the self-hosted tick loop + admin API + dashboard
// described in CLAUDE.md: SQLite-backed job storage, retries with backoff,
// overlap-protection locking, the /api/v1/* API that `cronify sync` talks to, and
// the bundled HTML dashboard — all served from one process on one port.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/api"
	"github.com/Ferinco/cronify/scheduler/internal/config"
	"github.com/Ferinco/cronify/scheduler/internal/dashboard"
	"github.com/Ferinco/cronify/scheduler/internal/scheduler"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("cronify: fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := st.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	sched := scheduler.New(st, cfg.TickInterval, time.Duration(cfg.StaleLockTimeoutSecs)*time.Second)
	go sched.Run(ctx)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, &api.Handlers{
		Store:      st,
		Runner:     sched,
		Next:       scheduler.Next,
		AdminToken: cfg.AdminToken,
	})
	dashboard.RegisterRoutes(mux, &dashboard.Handlers{
		Store:      st,
		Runner:     sched,
		AdminToken: cfg.AdminToken,
	})
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("cronify: scheduler listening", "port", cfg.Port, "db", cfg.DBPath, "tickInterval", cfg.TickInterval)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("cronify: shutting down")
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
