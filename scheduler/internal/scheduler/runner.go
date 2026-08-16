package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
)

// TriggerRun claims the job's lock, then fires attempts with backoff between them
// until one succeeds or maxAttempts is exhausted. Used by the tick loop, where the
// caller doesn't need the claim and execution phases split apart.
//
// A run that never got claimed (lock already held by a fresh in-progress run)
// returns runID 0 and a nil error: from the caller's perspective this is a normal,
// silent skip, not a failure.
func (s *Scheduler) TriggerRun(ctx context.Context, job model.Job) (runID int64, err error) {
	claimed, runID, err := s.Claim(ctx, job)
	if err != nil || !claimed {
		return runID, err
	}
	return runID, s.RunAttempts(ctx, job, runID)
}

// Claim acquires the job's lock (CLAUDE.md's stale-lock-aware algorithm, via
// store.ClaimRun) without running any attempts. Split out from TriggerRun so a
// manual "run now" HTTP handler can claim synchronously — and so return the run id
// in its response immediately — while running attempts in the background via
// RunAttempts.
func (s *Scheduler) Claim(ctx context.Context, job model.Job) (claimed bool, runID int64, err error) {
	staleAfter := time.Duration(max(int(s.StaleLockTimeout/time.Second), job.TimeoutSeconds)) * time.Second

	claimed, runID, err = s.Store.ClaimRun(ctx, job.ID, staleAfter, time.Now())
	if err != nil {
		return false, 0, fmt.Errorf("claim run: %w", err)
	}
	if !claimed {
		slog.Info("cronify: skipping run, lock held", "job", job.ID)
	}
	return claimed, runID, nil
}

// RunAttempts fires attempts with backoff between them, all against the single
// job_runs row runID (already inserted in_progress by Claim), until one succeeds
// or maxAttempts is exhausted. The row is only finalized (FinishRun, to success
// or failed) once — on the success, or on the last attempt's failure — so it
// stays in_progress, and the job's lock stays held, for the run's entire
// multi-attempt/backoff duration. See AdvanceAttempt's comment for why an
// earlier version that finalized every attempt individually was a locking bug.
func (s *Scheduler) RunAttempts(ctx context.Context, job model.Job, runID int64) error {
	var lastErr error
	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		if attempt > 1 {
			if err := s.Store.AdvanceAttempt(ctx, runID, attempt); err != nil {
				return fmt.Errorf("advance to attempt %d: %w", attempt, err)
			}
		}

		success, httpStatus, attemptErr := s.fireOnce(ctx, job)
		finishedAt := time.Now()

		if success {
			if err := s.Store.FinishRun(ctx, runID, model.StatusSuccess, httpStatus, nil, finishedAt); err != nil {
				return fmt.Errorf("finish run: %w", err)
			}
			return nil
		}

		lastErr = attemptErr

		if attempt == job.MaxAttempts {
			errMsg := attemptErr.Error()
			if err := s.Store.FinishRun(ctx, runID, model.StatusFailed, httpStatus, &errMsg, finishedAt); err != nil {
				return fmt.Errorf("finish run: %w", err)
			}
			break
		}

		slog.Warn("cronify: attempt failed, retrying", "job", job.ID, "attempt", attempt, "error", attemptErr)
		select {
		case <-time.After(s.Backoff.Delay(attempt)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("exhausted %d attempt(s): %w", job.MaxAttempts, lastErr)
}

// fireOnce makes a single HTTP call to the job's route. Failure is non-2xx,
// timeout, or a transport/connection error — status/timeout only, never response
// body, since a lock-skip and a real success both come back 200 {"success":true}
// and are indistinguishable by body (see packages/cronify/src/server.ts).
func (s *Scheduler) fireOnce(ctx context.Context, job model.Job) (success bool, httpStatus *int, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(job.TimeoutSeconds)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, job.AppURL+job.Route, nil)
	if err != nil {
		return false, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("authorization", "Bearer "+job.Secret)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return false, nil, fmt.Errorf("timed out after %ds", job.TimeoutSeconds)
		}
		return false, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 300 {
		return false, &status, fmt.Errorf("unexpected status %d", status)
	}
	return true, &status, nil
}
