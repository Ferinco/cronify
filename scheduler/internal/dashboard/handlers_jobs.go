package dashboard

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

// jobRow is the home page's per-job view model: the job plus its last-run
// status, resolved to plain fields so the template never has to deal with
// pointers or a possibly-empty run history.
type jobRow struct {
	Job        model.Job
	HasRun     bool
	LastStatus string
	LastRunAt  time.Time
}

type homeData struct {
	page
	Jobs []jobRow
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.Store.ListJobs(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}

	rows := make([]jobRow, 0, len(jobs))
	for _, job := range jobs {
		row := jobRow{Job: job}
		runs, err := h.Store.ListRuns(r.Context(), job.ID, 1)
		if err != nil {
			serverError(w, err)
			return
		}
		if len(runs) > 0 {
			row.HasRun = true
			row.LastStatus = runs[0].Status
			row.LastRunAt = runs[0].StartedAt
		}
		rows = append(rows, row)
	}

	render(w, http.StatusOK, homeTmpl, homeData{
		page: page{Title: "Jobs", Flash: readFlash(r)},
		Jobs: rows,
	})
}

// runRow is the job-detail run-history view model: JobRun's nullable fields
// (FinishedAt, HTTPStatus, Error) resolved to plain strings/bools so the
// template doesn't need to dereference pointers.
type runRow struct {
	Attempt     int
	Status      string
	StartedAt   time.Time
	HasFinished bool
	FinishedAt  time.Time
	HTTPStatus  string
	Error       string
}

const defaultRunsLimit = 50

type jobDetailData struct {
	page
	Job       model.Job
	Runs      []runRow
	ShowMore  bool
	MoreLimit int
}

func (h *Handlers) JobDetail(w http.ResponseWriter, r *http.Request) {
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

	limit := defaultRunsLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	runs, err := h.Store.ListRuns(r.Context(), id, limit)
	if err != nil {
		serverError(w, err)
		return
	}

	rows := make([]runRow, 0, len(runs))
	for _, run := range runs {
		row := runRow{
			Attempt:   run.Attempt,
			Status:    run.Status,
			StartedAt: run.StartedAt,
		}
		if run.FinishedAt != nil {
			row.HasFinished = true
			row.FinishedAt = *run.FinishedAt
		}
		if run.HTTPStatus != nil {
			row.HTTPStatus = strconv.Itoa(*run.HTTPStatus)
		}
		if run.Error != nil {
			row.Error = *run.Error
		}
		rows = append(rows, row)
	}

	render(w, http.StatusOK, jobDetailTmpl, jobDetailData{
		page:      page{Title: job.JobID, Flash: readFlash(r)},
		Job:       job,
		Runs:      rows,
		ShowMore:  len(runs) == limit,
		MoreLimit: limit * 4,
	})
}
