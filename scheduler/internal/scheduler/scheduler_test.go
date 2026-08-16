package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func testScheduler(st *store.Store) *Scheduler {
	return &Scheduler{
		Store:            st,
		HTTPClient:       &http.Client{},
		Backoff:          BackoffConfig{Base: 5 * time.Millisecond, Multiplier: 2, JitterPct: 0, Cap: time.Second},
		StaleLockTimeout: 10 * time.Minute,
		MaxInFlight:      20,
	}
}

func testJob(id, appURL string) model.Job {
	return model.Job{
		ID:             id,
		JobID:          id,
		Source:         "test-app",
		AppURL:         appURL,
		Secret:         "shh",
		Route:          "/api/cron/" + id,
		Schedule:       "0 9 * * *",
		Enabled:        true,
		TimeoutSeconds: 5,
		MaxAttempts:    3,
	}
}

func TestTriggerRunImmediateSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("authorization"); got != "Bearer shh" {
			t.Errorf("unexpected authorization header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := newTestStore(t)
	s := testScheduler(st)
	job := testJob("job-success", srv.URL)

	runID, err := s.TriggerRun(context.Background(), job)
	if err != nil {
		t.Fatalf("TriggerRun: %v", err)
	}
	if runID == 0 {
		t.Fatalf("expected a run id")
	}

	runs, err := st.ListRuns(context.Background(), job.ID, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != model.StatusSuccess {
		t.Fatalf("expected 1 successful run, got %+v", runs)
	}
}

func TestTriggerRunTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := newTestStore(t)
	s := testScheduler(st)
	job := testJob("job-timeout", srv.URL)
	job.TimeoutSeconds = 1
	job.MaxAttempts = 1

	_, err := s.TriggerRun(context.Background(), job)
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	runs, err := st.ListRuns(context.Background(), job.ID, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != model.StatusFailed {
		t.Fatalf("expected 1 failed run, got %+v", runs)
	}
}

func TestTriggerRunRetryThenSucceed(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := newTestStore(t)
	s := testScheduler(st)
	job := testJob("job-retry", srv.URL)

	runID, err := s.TriggerRun(context.Background(), job)
	if err != nil {
		t.Fatalf("TriggerRun: %v", err)
	}
	if runID == 0 {
		t.Fatalf("expected a run id")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls (1 failure + 1 success), got %d", got)
	}

	// One row for the whole run, not one per attempt: the row must stay
	// in_progress across the retry/backoff gap (see AdvanceAttempt's doc
	// comment) — a second row here would mean that gap still exists.
	runs, err := st.ListRuns(context.Background(), job.ID, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run row covering both attempts, got %d", len(runs))
	}
	if runs[0].Status != model.StatusSuccess || runs[0].Attempt != 2 {
		t.Fatalf("expected a successful run row with attempt=2, got %+v", runs[0])
	}
}

// TestTriggerRunLockHeldDuringBackoffGap is a regression test for a real bug:
// an earlier version finalized each failed attempt's row to "failed" and only
// inserted the next attempt's in_progress row when that next attempt actually
// started firing, leaving a window *during the backoff sleep* where no
// in_progress row existed for the job at all. A concurrent manual trigger
// landing in that window sailed past ClaimRun's lock check and started a
// second, fully independent, overlapping run — exactly what locking exists to
// prevent. This asserts the lock is held continuously across that gap.
func TestTriggerRunLockHeldDuringBackoffGap(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	st := newTestStore(t)
	s := testScheduler(st)
	s.Backoff = BackoffConfig{Base: 300 * time.Millisecond, Multiplier: 2, JitterPct: 0, Cap: time.Second}
	job := testJob("job-backoff-gap", srv.URL)
	job.MaxAttempts = 3

	done := make(chan struct{})
	go func() {
		s.TriggerRun(context.Background(), job)
		close(done)
	}()

	// Wait for attempt 1 to fail and land inside its backoff sleep (attempt 1
	// fails near-instantly against the fake server; the 300ms backoff gives a
	// wide, non-flaky window to land a concurrent claim inside).
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		select {
		case <-deadline:
			t.Fatalf("attempt 1 never reached the fake server")
		case <-time.After(2 * time.Millisecond):
		}
	}
	time.Sleep(50 * time.Millisecond) // now inside the backoff gap

	runID, err := s.TriggerRun(context.Background(), job)
	if err != nil {
		t.Fatalf("concurrent TriggerRun during backoff gap: %v", err)
	}
	if runID != 0 {
		t.Fatalf("expected the concurrent trigger to be skipped (lock still held), got runID %d", runID)
	}

	<-done
	runs, err := st.ListRuns(context.Background(), job.ID, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected exactly 1 run row for the whole job (no overlapping second run got created), got %d", len(runs))
	}
}

func TestTriggerRunExhaustsMaxAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	st := newTestStore(t)
	s := testScheduler(st)
	job := testJob("job-exhausted", srv.URL)
	job.MaxAttempts = 3

	_, err := s.TriggerRun(context.Background(), job)
	if err == nil {
		t.Fatalf("expected an error after exhausting attempts")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls, got %d", got)
	}

	runs, err := st.ListRuns(context.Background(), job.ID, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run row covering all 3 attempts, got %d", len(runs))
	}
	if runs[0].Status != model.StatusFailed || runs[0].Attempt != 3 {
		t.Fatalf("expected a failed run row with attempt=3 (the last one tried), got %+v", runs[0])
	}
}

func TestTriggerRunLockContention(t *testing.T) {
	var reached int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reached, 1)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := newTestStore(t)
	s := testScheduler(st)
	job := testJob("job-contended", srv.URL)
	job.TimeoutSeconds = 5

	done := make(chan struct{}, 2)
	go func() {
		s.TriggerRun(context.Background(), job)
		done <- struct{}{}
	}()

	// Give the first call time to claim the lock and reach the fake handler.
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&reached) == 0 {
		select {
		case <-deadline:
			t.Fatalf("first request never reached the fake handler")
		case <-time.After(5 * time.Millisecond):
		}
	}

	runID, err := s.TriggerRun(context.Background(), job)
	if err != nil {
		t.Fatalf("second TriggerRun: %v", err)
	}
	if runID != 0 {
		t.Fatalf("expected second call to be skipped (runID 0), got %d", runID)
	}

	close(release)
	<-done

	if got := atomic.LoadInt32(&reached); got != 1 {
		t.Fatalf("expected exactly 1 request to reach the fake app, got %d", got)
	}
}
