package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

// stubRunner is a fake Runner: Claim always succeeds (using a fake, always-past-lock
// store call under the hood is unnecessary — it just hands back an incrementing id),
// and RunAttempts records that it was invoked. Lets API tests exercise routing, auth,
// and response shapes without depending on scheduler's real HTTP-firing/retry logic.
type stubRunner struct {
	mu        sync.Mutex
	nextRunID int64
	claimed   []model.Job
	ran       []model.Job
	claimFail bool
	runFail   bool
}

func (s *stubRunner) Claim(ctx context.Context, job model.Job) (bool, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimFail {
		return false, 0, nil
	}
	s.nextRunID++
	s.claimed = append(s.claimed, job)
	return true, s.nextRunID, nil
}

func (s *stubRunner) RunAttempts(ctx context.Context, job model.Job, runID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ran = append(s.ran, job)
	return nil
}

func newTestHandlers(t *testing.T) (*Handlers, *stubRunner) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	runner := &stubRunner{}
	h := &Handlers{
		Store:      st,
		Runner:     runner,
		Next:       func(_ string, from time.Time) (time.Time, error) { return from.Add(time.Hour), nil },
		AdminToken: "test-token",
	}
	return h, runner
}

func doRequest(t *testing.T, mux http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAuthMatrix(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	cases := []struct {
		name  string
		token string
	}{
		{"missing token", ""},
		{"wrong token", "wrong"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, mux, http.MethodGet, "/api/v1/jobs", tc.token, nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", rec.Code)
			}
		})
	}

	rec := doRequest(t, mux, http.MethodGet, "/api/v1/jobs", "test-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHealthzNeedsNoAuth(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected healthz body: %v", body)
	}
}

func TestSyncCreatesJobsAndReturnsShape(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	payload := model.SyncRequest{
		Source:     "app-a",
		AppURL:     "https://a.example.com",
		CronSecret: "s3cret",
		Jobs: []model.JobPayload{
			{ID: "daily-report", Schedule: "0 9 * * *", Route: "/api/cron/daily-report", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3},
		},
	}
	rec := doRequest(t, mux, http.MethodPost, "/api/v1/sync", "test-token", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result model.SyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode sync result: %v", err)
	}
	if result.Created != 1 {
		t.Fatalf("expected created=1, got %+v", result)
	}

	// camelCase wire-format check: raw JSON must use "createdAt"/"nextRunAt", not Go's default PascalCase.
	rec = doRequest(t, mux, http.MethodGet, "/api/v1/jobs/app-a::daily-report", "test-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	for _, field := range []string{"id", "jobId", "appUrl", "nextRunAt", "createdAt", "timeoutSeconds", "maxAttempts"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected camelCase field %q in job JSON, got keys %v", field, keys(raw))
		}
	}
	if _, ok := raw["Secret"]; ok {
		t.Errorf("job JSON must never include the secret")
	}
}

func TestSyncRejectsMissingFields(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/sync", "test-token", map[string]any{"appUrl": "https://a.example.com"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing source, got %d", rec.Code)
	}
}

func TestRunNowTriggersRunnerAndReturns202(t *testing.T) {
	h, runner := newTestHandlers(t)
	mux := NewMux(h)

	seedJob(t, h)

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/jobs/app-a::job-a/run", "test-token", nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "triggered" {
		t.Fatalf("unexpected body: %v", body)
	}
	if _, ok := body["runId"]; !ok {
		t.Fatalf("expected runId in response body: %v", body)
	}

	// RunAttempts runs in a goroutine; give it a moment.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		runner.mu.Lock()
		ran := len(runner.ran)
		runner.mu.Unlock()
		if ran == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if len(runner.claimed) != 1 || len(runner.ran) != 1 {
		t.Fatalf("expected claim+run once each, got claimed=%d ran=%d", len(runner.claimed), len(runner.ran))
	}
}

func TestRunNowReturns404ForUnknownJob(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/jobs/does-not-exist/run", "test-token", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestPauseResumeDelete(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h)

	rec := doRequest(t, mux, http.MethodPost, "/api/v1/jobs/app-a::job-a/pause", "test-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on pause, got %d", rec.Code)
	}
	job, err := h.Store.GetJob(context.Background(), "app-a::job-a")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Enabled {
		t.Fatalf("expected job disabled after pause")
	}

	rec = doRequest(t, mux, http.MethodPost, "/api/v1/jobs/app-a::job-a/resume", "test-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on resume, got %d", rec.Code)
	}
	job, err = h.Store.GetJob(context.Background(), "app-a::job-a")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !job.Enabled {
		t.Fatalf("expected job enabled after resume")
	}

	rec = doRequest(t, mux, http.MethodDelete, "/api/v1/jobs/app-a::job-a", "test-token", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", rec.Code)
	}
	if _, err := h.Store.GetJob(context.Background(), "app-a::job-a"); err != store.ErrNotFound {
		t.Fatalf("expected job gone, got %v", err)
	}
}

func seedJob(t *testing.T, h *Handlers) {
	t.Helper()
	payload := []model.JobPayload{
		{ID: "job-a", Schedule: "0 9 * * *", Route: "/api/cron/job-a", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3},
	}
	if _, err := h.Store.ReconcileSource(context.Background(), "app-a", "https://a.example.com", "secret", payload, time.Now(), h.Next); err != nil {
		t.Fatalf("seed job: %v", err)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
