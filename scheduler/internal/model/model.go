// Package model holds the domain types shared across storage, scheduling, and the HTTP API.
package model

import "time"

const (
	StatusInProgress = "in_progress"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
)

// Job is a registered cron job, synced from one source app.
type Job struct {
	ID             string    `json:"id"` // composite: Source + "::" + JobID
	JobID          string    `json:"jobId"`
	Source         string    `json:"source"`
	AppURL         string    `json:"appUrl"`
	Secret         string    `json:"-"` // CRON_SECRET for AppURL, from the sync payload; never serialized
	Route          string    `json:"route"`
	Schedule       string    `json:"schedule"`
	Enabled        bool      `json:"enabled"`
	TimeoutSeconds int       `json:"timeoutSeconds"`
	MaxAttempts    int       `json:"maxAttempts"`
	Description    *string   `json:"description"`
	NextRunAt      time.Time `json:"nextRunAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// JobRun is one attempt (or lock-claim) row for a job, per CLAUDE.md's job_runs schema.
type JobRun struct {
	ID         int64      `json:"id"`
	JobID      string     `json:"jobId"` // Job.ID (composite)
	Status     string     `json:"status"` // in_progress | success | failed
	Attempt    int        `json:"attempt"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	HTTPStatus *int       `json:"httpStatus"`
	Error      *string    `json:"error"`
}

// JobPayload is one entry of a sync request's jobs array — mirrors packages/cronify's ManifestJobEntry.
type JobPayload struct {
	ID             string  `json:"id"`
	Schedule       string  `json:"schedule"`
	Route          string  `json:"route"`
	Enabled        bool    `json:"enabled"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
	MaxAttempts    int     `json:"maxAttempts"`
	Description    *string `json:"description"`
}

// SyncRequest is the exact body cronify sync posts to /api/v1/sync.
type SyncRequest struct {
	Source     string       `json:"source"`
	AppURL     string       `json:"appUrl"`
	CronSecret string       `json:"cronSecret"`
	Jobs       []JobPayload `json:"jobs"`
}

// SyncResult is the response body for a successful /api/v1/sync call.
type SyncResult struct {
	Created   int `json:"created"`
	Updated   int `json:"updated"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}
