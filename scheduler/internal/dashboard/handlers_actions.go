package dashboard

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

// RunNow claims the job's lock synchronously (same claim-then-async pattern
// as api.Handlers.RunNow) and runs the attempt/backoff sequence in the
// background, then redirects back to whichever page the form was on. A run
// that couldn't be claimed (a run is already in progress) is a normal
// outcome, not an error — it redirects with a flash message, same as it
// would surface as a 409 from the JSON API.
func (h *Handlers) RunNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.Store.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	claimed, runID, err := h.Runner.Claim(r.Context(), job)
	if err != nil {
		serverError(w, err)
		return
	}
	if !claimed {
		http.Redirect(w, r, redirectTarget(r, "/", "lock_held"), http.StatusSeeOther)
		return
	}

	go func() {
		if err := h.Runner.RunAttempts(context.Background(), job, runID); err != nil {
			slog.Warn("cronify: dashboard-triggered run did not complete successfully", "job", job.ID, "error", err)
		}
	}()

	http.Redirect(w, r, redirectTarget(r, "/", "triggered"), http.StatusSeeOther)
}

func (h *Handlers) Pause(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false, "paused")
}

func (h *Handlers) Resume(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true, "resumed")
}

func (h *Handlers) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool, flash string) {
	id := r.PathValue("id")
	err := h.Store.SetEnabled(r.Context(), id, enabled)
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, redirectTarget(r, "/", flash), http.StatusSeeOther)
}

// DeleteConfirm is a read-only confirmation page — the only place a delete
// can actually be triggered from is its form, which POSTs to Delete.
func (h *Handlers) DeleteConfirm(w http.ResponseWriter, r *http.Request) {
	job, err := h.Store.GetJob(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	render(w, http.StatusOK, deleteConfirmTmpl, deleteConfirmData{
		page: page{Title: "Delete " + job.JobID},
		Job:  job,
	})
}

type deleteConfirmData struct {
	page
	Job model.Job
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	err := h.Store.DeleteJob(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	// The referring page (the job's own detail page) no longer exists.
	u := "/?flash=deleted"
	http.Redirect(w, r, u, http.StatusSeeOther)
}
