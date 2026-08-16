package dashboard

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
)

func TestBasicAuthMatrix(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/", false, nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: expected 401, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected WWW-Authenticate header on 401")
	}

	rec = doRequestAs(t, mux, http.MethodGet, "/", "admin", "wrong-token", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: expected 401, got %d", rec.Code)
	}

	rec = doRequestAs(t, mux, http.MethodGet, "/", "any-username-works", testToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("correct token: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHomeListsJobsWithLastRunStatus(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")
	seedJob(t, h, "app-b", "job-a") // same job id, different source — must not collide

	ctx := context.Background()
	_, runID, err := h.Store.ClaimRun(ctx, "app-a::job-a", 10*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("claim run: %v", err)
	}
	if err := h.Store.FinishRun(ctx, runID, model.StatusSuccess, intPtr(200), nil, time.Now()); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	rec := doRequest(t, mux, http.MethodGet, "/", true, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "job-a") {
		t.Fatalf("expected job-a in body")
	}
	if !strings.Contains(body, "app-a") || !strings.Contains(body, "app-b") {
		t.Fatalf("expected both sources in body")
	}
	if !strings.Contains(body, "badge-success") {
		t.Fatalf("expected a success badge for app-a::job-a's last run, got: %s", body)
	}
	if !strings.Contains(body, "badge-none") {
		t.Fatalf("expected a never-run badge for app-b::job-a, got: %s", body)
	}
}

func TestHomeEmptyState(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/", true, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No jobs registered") {
		t.Fatalf("expected empty-state message, got: %s", rec.Body.String())
	}
}

func TestJobDetailShowsConfigAndRunHistory(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")

	ctx := context.Background()
	_, runID, err := h.Store.ClaimRun(ctx, "app-a::job-a", 10*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("claim run: %v", err)
	}
	errMsg := "unexpected status 500"
	if err := h.Store.FinishRun(ctx, runID, model.StatusFailed, intPtr(500), &errMsg, time.Now()); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	rec := doRequest(t, mux, http.MethodGet, "/jobs/app-a::job-a", true, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"job-a", "0 9 * * *", "/api/cron/job-a", "badge-failed", "unexpected status 500"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got: %s", want, body)
		}
	}
}

func TestJobDetail404ForUnknownJob(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	rec := doRequest(t, mux, http.MethodGet, "/jobs/does-not-exist", true, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHTMLEscapesUserSuppliedStrings(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	desc := "<script>alert(1)</script>"
	payload := []model.JobPayload{
		{ID: "job-a", Schedule: "0 9 * * *", Route: "/api/cron/job-a", Enabled: true, TimeoutSeconds: 30, MaxAttempts: 3, Description: &desc},
	}
	if _, err := h.Store.ReconcileSource(context.Background(), "app-a", "https://app-a.example.com", "secret", payload, time.Now(), fixedNext); err != nil {
		t.Fatalf("seed job: %v", err)
	}

	rec := doRequest(t, mux, http.MethodGet, "/jobs/app-a::job-a", true, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("expected description to be HTML-escaped, found raw script tag in body")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("expected escaped description in body, got: %s", body)
	}
}

func intPtr(n int) *int { return &n }
