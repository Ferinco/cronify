package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Ferinco/cronify/scheduler/internal/httpjson"
	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.Store.ListJobs(r.Context())
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if jobs == nil {
		jobs = []model.Job{}
	}
	httpjson.Write(w, http.StatusOK, jobs)
}

func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	job, err := h.Store.GetJob(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpjson.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpjson.Write(w, http.StatusOK, job)
}

func (h *Handlers) ListRuns(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.Store.GetJob(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		httpjson.Error(w, http.StatusNotFound, "job not found")
		return
	} else if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	runs, err := h.Store.ListRuns(r.Context(), id, limit)
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []model.JobRun{}
	}
	httpjson.Write(w, http.StatusOK, runs)
}

// RunNow handles POST /api/v1/jobs/{id}/run. It claims the job's lock synchronously
// (so the response can include the resulting run id), then runs the attempt/backoff
// sequence in the background — a full retry sequence can take 1.5+ minutes, so this
// does not block the HTTP request. Callers poll GET /jobs/{id}/runs for the outcome.
func (h *Handlers) RunNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.Store.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpjson.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	claimed, runID, err := h.Runner.Claim(r.Context(), job)
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !claimed {
		httpjson.Write(w, http.StatusConflict, map[string]string{"status": "lock_held"})
		return
	}

	go func() {
		if err := h.Runner.RunAttempts(context.Background(), job, runID); err != nil {
			slog.Warn("cronify: manual run did not complete successfully", "job", job.ID, "error", err)
		}
	}()

	httpjson.Write(w, http.StatusAccepted, map[string]any{"status": "triggered", "runId": runID})
}

func (h *Handlers) Pause(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *Handlers) Resume(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

func (h *Handlers) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	err := h.Store.SetEnabled(r.Context(), r.PathValue("id"), enabled)
	if errors.Is(err, store.ErrNotFound) {
		httpjson.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	err := h.Store.DeleteJob(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpjson.Error(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
