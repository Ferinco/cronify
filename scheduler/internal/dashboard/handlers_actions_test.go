package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRunNowRedirectsAndTriggersRunner(t *testing.T) {
	h, runner := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")

	rec := doRequest(t, mux, http.MethodPost, "/jobs/app-a::job-a/run", true, url.Values{}, "http://example.com/jobs/app-a::job-a")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if loc.Path != "/jobs/app-a::job-a" {
		t.Fatalf("expected redirect back to job detail path, got %q", loc.Path)
	}
	if loc.Query().Get("flash") != "triggered" {
		t.Fatalf("expected flash=triggered, got %q", loc.RawQuery)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && runner.ranCount() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if runner.ranCount() != 1 {
		t.Fatalf("expected RunAttempts to have been invoked once, got %d", runner.ranCount())
	}
}

func TestRunNowLockHeldRedirectsWithFlash(t *testing.T) {
	h, runner := newTestHandlers(t)
	runner.claimFail = true
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")

	rec := doRequest(t, mux, http.MethodPost, "/jobs/app-a::job-a/run", true, url.Values{}, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected a redirect (not an error status) when the lock is held, got %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if loc.Query().Get("flash") != "lock_held" {
		t.Fatalf("expected flash=lock_held, got %q", loc.RawQuery)
	}
}

func TestRunNowRejectsForgedRefererHost(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")

	rec := doRequest(t, mux, http.MethodPost, "/jobs/app-a::job-a/run", true, url.Values{}, "https://evil.example.com/steal-me")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if strings.Contains(loc, "evil.example.com") || strings.HasPrefix(loc, "http") {
		t.Fatalf("expected redirect target to have no scheme/host, got %q", loc)
	}
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if parsed.Path != "/steal-me" {
		t.Fatalf("expected the (safe) path to still be honored, got %q", parsed.Path)
	}
}

func TestPauseResumeToggleEnabled(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")

	rec := doRequest(t, mux, http.MethodPost, "/jobs/app-a::job-a/pause", true, url.Values{}, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on pause, got %d", rec.Code)
	}
	job, err := h.Store.GetJob(context.Background(), "app-a::job-a")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Enabled {
		t.Fatalf("expected job disabled after pause")
	}

	rec = doRequest(t, mux, http.MethodPost, "/jobs/app-a::job-a/resume", true, url.Values{}, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on resume, got %d", rec.Code)
	}
	job, err = h.Store.GetJob(context.Background(), "app-a::job-a")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if !job.Enabled {
		t.Fatalf("expected job enabled after resume")
	}
}

func TestDeleteConfirmThenDelete(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)
	seedJob(t, h, "app-a", "job-a")

	rec := doRequest(t, mux, http.MethodGet, "/jobs/app-a::job-a/delete", true, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for confirm page, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `action="/jobs/app-a::job-a/delete"`) {
		t.Fatalf("expected confirm page to contain the delete form, got: %s", rec.Body.String())
	}

	rec = doRequest(t, mux, http.MethodPost, "/jobs/app-a::job-a/delete", true, url.Values{}, "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if loc.Path != "/" {
		t.Fatalf("expected redirect to home, got %q", loc.Path)
	}

	if _, err := h.Store.GetJob(context.Background(), "app-a::job-a"); err == nil {
		t.Fatalf("expected job to be gone after delete")
	}
}

func TestActionRoutes404ForUnknownJob(t *testing.T) {
	h, _ := newTestHandlers(t)
	mux := NewMux(h)

	for _, path := range []string{
		"/jobs/does-not-exist/run",
		"/jobs/does-not-exist/pause",
		"/jobs/does-not-exist/resume",
		"/jobs/does-not-exist/delete",
	} {
		rec := doRequest(t, mux, http.MethodPost, path, true, url.Values{}, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", path, rec.Code)
		}
	}

	rec := doRequest(t, mux, http.MethodGet, "/jobs/does-not-exist/delete", true, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete confirm: expected 404, got %d", rec.Code)
	}
}
