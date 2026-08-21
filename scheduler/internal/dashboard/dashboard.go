// Package dashboard is the bundled, server-rendered HTML admin UI: job list,
// job detail + run history, and run/pause/resume/delete actions via plain
// HTML forms. Served from the same process/port as internal/api's JSON API,
// per SPEC.md's "one binary, one Docker image, one deploy."
package dashboard

import (
	"context"

	"github.com/Ferinco/cronify/scheduler/internal/model"
	"github.com/Ferinco/cronify/scheduler/internal/store"
)

// Runner is the subset of *scheduler.Scheduler the dashboard needs. Defined
// locally (same precedent as api.Runner) so tests can stub it without this
// package depending on the sibling api package.
type Runner interface {
	Claim(ctx context.Context, job model.Job) (claimed bool, runID int64, err error)
	RunAttempts(ctx context.Context, job model.Job, runID int64) error
}

type Handlers struct {
	Store      *store.Store
	Runner     Runner
	AdminToken string

	// loginThrottle backs withBasicAuth's failed-attempt throttling (see
	// middleware.go). Left nil in struct literals — throttle() lazily
	// allocates it on first use.
	loginThrottle *loginThrottle
}
