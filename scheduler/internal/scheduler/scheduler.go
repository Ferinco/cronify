package scheduler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

type Scheduler struct {
	Store            *store.Store
	HTTPClient       *http.Client
	Backoff          BackoffConfig
	StaleLockTimeout time.Duration // default; effective-per-job is max(this, job.TimeoutSeconds)
	TickInterval     time.Duration
	MaxInFlight      int
}

func New(st *store.Store, tickInterval, staleLockTimeout time.Duration) *Scheduler {
	return &Scheduler{
		Store:            st,
		HTTPClient:       &http.Client{},
		Backoff:          DefaultBackoff(),
		StaleLockTimeout: staleLockTimeout,
		TickInterval:     tickInterval,
		MaxInFlight:      20,
	}
}

// Run drives the tick loop until ctx is canceled.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.tick(ctx, now)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	due, err := s.Store.DueJobs(ctx, now)
	if err != nil {
		slog.Error("cronify: due-jobs query failed", "error", err)
		return
	}

	sem := make(chan struct{}, max(s.MaxInFlight, 1))
	for _, job := range due {
		// Advance next_run_at before firing: a retry sequence spanning multiple tick
		// intervals must never cause the same occurrence to be dispatched twice.
		if err := s.Store.AdvanceNextRun(ctx, job.ID, job.Schedule, now, Next); err != nil {
			slog.Error("cronify: failed to advance next_run_at", "job", job.ID, "error", err)
			continue
		}

		sem <- struct{}{}
		go func(job model.Job) {
			defer func() { <-sem }()
			if _, err := s.TriggerRun(context.Background(), job); err != nil {
				slog.Warn("cronify: run did not complete successfully", "job", job.ID, "error", err)
			}
		}(job)
	}
}
