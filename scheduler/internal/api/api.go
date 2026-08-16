// Package api implements the /api/v1/* admin HTTP API that cronify sync and the
// dashboard talk to, per CLAUDE.md's approved API contract.
package api

import (
	"context"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

// Runner is the subset of *scheduler.Scheduler the API needs, kept as a small
// interface so handler tests can use a stub instead of a real HTTP-firing scheduler.
type Runner interface {
	Claim(ctx context.Context, job model.Job) (claimed bool, runID int64, err error)
	RunAttempts(ctx context.Context, job model.Job, runID int64) error
}

type Handlers struct {
	Store      *store.Store
	Runner     Runner
	Next       store.ScheduleFunc
	AdminToken string
}
