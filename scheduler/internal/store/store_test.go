package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// fixedNext advances by exactly one hour each call — deterministic, no cron parsing needed.
func fixedNext(_ string, from time.Time) (time.Time, error) {
	return from.Add(time.Hour), nil
}

func strPtr(s string) *string { return &s }

func TestMigrateIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestReconcileSourceCreateUpdateDeleteUnchanged(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

	payload := []model.JobPayload{
		{ID: "daily-report", Schedule: "0 9 * * *", Route: "/api/cron/daily-report", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3, Description: nil},
	}
	res, err := s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret-a", payload, now, fixedNext)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Created != 1 || res.Updated != 0 || res.Deleted != 0 || res.Unchanged != 0 {
		t.Fatalf("unexpected result on create: %+v", res)
	}

	// Identical re-sync -> unchanged.
	res, err = s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret-a", payload, now, fixedNext)
	if err != nil {
		t.Fatalf("reconcile unchanged: %v", err)
	}
	if res.Unchanged != 1 || res.Created != 0 || res.Updated != 0 || res.Deleted != 0 {
		t.Fatalf("unexpected result on unchanged: %+v", res)
	}

	// Change a field -> updated.
	payload[0].MaxAttempts = 5
	res, err = s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret-a", payload, now, fixedNext)
	if err != nil {
		t.Fatalf("reconcile updated: %v", err)
	}
	if res.Updated != 1 {
		t.Fatalf("unexpected result on update: %+v", res)
	}
	job, err := s.GetJob(ctx, "app-a::daily-report")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.MaxAttempts != 5 {
		t.Fatalf("expected max attempts 5, got %d", job.MaxAttempts)
	}

	// Empty jobs list -> delete.
	res, err = s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret-a", nil, now, fixedNext)
	if err != nil {
		t.Fatalf("reconcile delete: %v", err)
	}
	if res.Deleted != 1 {
		t.Fatalf("unexpected result on delete: %+v", res)
	}
	if _, err := s.GetJob(ctx, "app-a::daily-report"); err != ErrNotFound {
		t.Fatalf("expected job gone, got err=%v", err)
	}
}

func TestReconcileSourceNoCrossSourceCollision(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	payload := []model.JobPayload{
		{ID: "daily-report", Schedule: "0 9 * * *", Route: "/api/cron/daily-report", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3},
	}
	if _, err := s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret-a", payload, now, fixedNext); err != nil {
		t.Fatalf("reconcile app-a: %v", err)
	}
	if _, err := s.ReconcileSource(ctx, "app-b", "https://b.example.com", "secret-b", payload, now, fixedNext); err != nil {
		t.Fatalf("reconcile app-b: %v", err)
	}

	jobs, err := s.ListJobs(ctx)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs (one per source), got %d", len(jobs))
	}

	jobA, err := s.GetJob(ctx, "app-a::daily-report")
	if err != nil {
		t.Fatalf("get app-a job: %v", err)
	}
	jobB, err := s.GetJob(ctx, "app-b::daily-report")
	if err != nil {
		t.Fatalf("get app-b job: %v", err)
	}
	if jobA.AppURL == jobB.AppURL {
		t.Fatalf("expected distinct app URLs, both were %q", jobA.AppURL)
	}

	// Deleting app-a's jobs must not touch app-b's job with the same job_id.
	if _, err := s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret-a", nil, now, fixedNext); err != nil {
		t.Fatalf("reconcile app-a delete: %v", err)
	}
	if _, err := s.GetJob(ctx, "app-b::daily-report"); err != nil {
		t.Fatalf("expected app-b job to survive app-a's deletion, got err=%v", err)
	}
}

func TestClaimRunFreshHeldStaleReclaim(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	staleAfter := 10 * time.Minute

	claimed, runID1, err := s.ClaimRun(ctx, "job-1", staleAfter, now)
	if err != nil {
		t.Fatalf("claim 1: %v", err)
	}
	if !claimed || runID1 == 0 {
		t.Fatalf("expected first claim to succeed, got claimed=%v runID=%d", claimed, runID1)
	}

	// Still in progress and fresh -> second claim attempt must not succeed.
	claimed, _, err = s.ClaimRun(ctx, "job-1", staleAfter, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("claim 2: %v", err)
	}
	if claimed {
		t.Fatalf("expected fresh in-progress lock to block claim")
	}

	// Same lock, now stale -> must be reclaimed with a new run row.
	claimed, runID2, err := s.ClaimRun(ctx, "job-1", staleAfter, now.Add(20*time.Minute))
	if err != nil {
		t.Fatalf("claim 3: %v", err)
	}
	if !claimed {
		t.Fatalf("expected stale lock to be reclaimed")
	}
	if runID2 == runID1 {
		t.Fatalf("expected a new run row on reclaim")
	}

	runs, err := s.ListRuns(ctx, "job-1", 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 run rows, got %d", len(runs))
	}
	var sawFailed bool
	for _, r := range runs {
		if r.ID == runID1 {
			if r.Status != model.StatusFailed || r.Error == nil || *r.Error != "stale lock" {
				t.Fatalf("expected original run marked failed/stale lock, got %+v", r)
			}
			sawFailed = true
		}
	}
	if !sawFailed {
		t.Fatalf("did not find original run row among results")
	}
}

func TestDueJobsAndAdvanceNextRun(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)

	payload := []model.JobPayload{
		{ID: "job-a", Schedule: "irrelevant", Route: "/api/cron/job-a", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3},
	}
	if _, err := s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret", payload, now.Add(-2*time.Hour), fixedNext); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	// fixedNext put next_run_at at now-1h, so it's due at `now`.
	due, err := s.DueJobs(ctx, now)
	if err != nil {
		t.Fatalf("due jobs: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due job, got %d", len(due))
	}

	if err := s.AdvanceNextRun(ctx, due[0].ID, due[0].Schedule, now, fixedNext); err != nil {
		t.Fatalf("advance: %v", err)
	}
	due, err = s.DueJobs(ctx, now)
	if err != nil {
		t.Fatalf("due jobs after advance: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("expected 0 due jobs after advance, got %d", len(due))
	}
}

func TestSetEnabledAndDeleteJob(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	payload := []model.JobPayload{
		{ID: "job-a", Schedule: "0 9 * * *", Route: "/api/cron/job-a", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3, Description: strPtr("desc")},
	}
	if _, err := s.ReconcileSource(ctx, "app-a", "https://a.example.com", "secret", payload, now, fixedNext); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if err := s.SetEnabled(ctx, "app-a::job-a", false); err != nil {
		t.Fatalf("set enabled false: %v", err)
	}
	job, err := s.GetJob(ctx, "app-a::job-a")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Enabled {
		t.Fatalf("expected job disabled")
	}

	if err := s.SetEnabled(ctx, "does-not-exist", false); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := s.DeleteJob(ctx, "app-a::job-a"); err != nil {
		t.Fatalf("delete job: %v", err)
	}
	if _, err := s.GetJob(ctx, "app-a::job-a"); err != ErrNotFound {
		t.Fatalf("expected job gone, got %v", err)
	}
	if err := s.DeleteJob(ctx, "app-a::job-a"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on double delete, got %v", err)
	}
}
