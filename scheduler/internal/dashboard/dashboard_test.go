package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

// stubRunner mirrors internal/api/api_test.go's stubRunner — duplicated rather
// than shared, matching this codebase's convention of each package owning its
// own test doubles.
type stubRunner struct {
	mu        sync.Mutex
	nextRunID int64
	claimed   []model.Job
	ran       []model.Job
	claimFail bool
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

func (s *stubRunner) ranCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ran)
}

const testToken = "test-token"

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
	h := &Handlers{Store: st, Runner: runner, AdminToken: testToken}
	return h, runner
}

var fixedNext = func(_ string, from time.Time) (time.Time, error) { return from.Add(time.Hour), nil }

func seedJob(t *testing.T, h *Handlers, source, jobID string) {
	t.Helper()
	payload := []model.JobPayload{
		{ID: jobID, Schedule: "0 9 * * *", Route: "/api/cron/" + jobID, Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3},
	}
	if _, err := h.Store.ReconcileSource(context.Background(), source, "https://"+source+".example.com", "secret", payload, time.Now(), fixedNext); err != nil {
		t.Fatalf("seed job: %v", err)
	}
}

// doRequest issues a request through mux. auth=true sends the correct
// credentials (any username, testToken as the password) — use doRequestAs
// directly for auth-failure cases that need specific/wrong credentials.
func doRequest(t *testing.T, mux http.Handler, method, path string, auth bool, form url.Values, referer string) *httptest.ResponseRecorder {
	t.Helper()
	if auth {
		return doRequestAs(t, mux, method, path, "admin", testToken, form, referer)
	}
	return doRequestAs(t, mux, method, path, "", "", form, referer)
}

func doRequestAs(t *testing.T, mux http.Handler, method, path, user, pass string, form url.Values, referer string) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
