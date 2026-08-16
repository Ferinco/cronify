package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
)

var ErrNotFound = errors.New("not found")

// ScheduleFunc computes a job's next run time from its cron schedule string.
// Injected by the caller so this package doesn't need to depend on the cron parser.
type ScheduleFunc func(schedule string, from time.Time) (time.Time, error)

func compositeID(source, jobID string) string {
	return source + "::" + jobID
}

type jobRow struct {
	appURL, secret, route, schedule string
	enabled                         bool
	timeoutSeconds, maxAttempts     int
	description                     *string
}

func (r jobRow) equalsPayload(appURL, secret string, jp model.JobPayload) bool {
	return r.appURL == appURL &&
		r.secret == secret &&
		r.route == jp.Route &&
		r.schedule == jp.Schedule &&
		r.enabled == jp.Enabled &&
		r.timeoutSeconds == jp.TimeoutSeconds &&
		r.maxAttempts == jp.MaxAttempts &&
		strPtrEqual(r.description, jp.Description)
}

func strPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// ReconcileSource performs full reconciliation for one source: upserts jobs present
// in the payload, deletes jobs previously registered for this source that are no
// longer present. job_runs history for deleted jobs is left untouched.
func (s *Store) ReconcileSource(ctx context.Context, source, appURL, secret string, jobs []model.JobPayload, now time.Time, next ScheduleFunc) (model.SyncResult, error) {
	now = now.UTC()
	var result model.SyncResult

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT job_id, app_url, secret, route, schedule, enabled, timeout_seconds, max_attempts, description
		 FROM jobs WHERE source = ?`, source)
	if err != nil {
		return result, fmt.Errorf("query existing: %w", err)
	}
	existing := map[string]jobRow{}
	for rows.Next() {
		var jobID string
		var r jobRow
		var enabledInt int
		if err := rows.Scan(&jobID, &r.appURL, &r.secret, &r.route, &r.schedule, &enabledInt, &r.timeoutSeconds, &r.maxAttempts, &r.description); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan existing: %w", err)
		}
		r.enabled = enabledInt != 0
		existing[jobID] = r
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, fmt.Errorf("iterate existing: %w", err)
	}
	rows.Close()

	incoming := make(map[string]bool, len(jobs))

	for _, jp := range jobs {
		incoming[jp.ID] = true
		id := compositeID(source, jp.ID)

		if row, ok := existing[jp.ID]; ok {
			if row.equalsPayload(appURL, secret, jp) {
				result.Unchanged++
				continue
			}
			if row.schedule == jp.Schedule {
				// Schedule unchanged: keep the existing next_run_at rather than resetting it.
				if _, execErr := tx.ExecContext(ctx,
					`UPDATE jobs SET app_url=?, secret=?, route=?, schedule=?, enabled=?, timeout_seconds=?, max_attempts=?, description=?, updated_at=?
					 WHERE id=?`,
					appURL, secret, jp.Route, jp.Schedule, jp.Enabled, jp.TimeoutSeconds, jp.MaxAttempts, jp.Description, now, id); execErr != nil {
					return result, fmt.Errorf("update job %q: %w", jp.ID, execErr)
				}
			} else {
				nr, err := next(jp.Schedule, now)
				if err != nil {
					return result, fmt.Errorf("parse schedule for job %q: %w", jp.ID, err)
				}
				if _, execErr := tx.ExecContext(ctx,
					`UPDATE jobs SET app_url=?, secret=?, route=?, schedule=?, enabled=?, timeout_seconds=?, max_attempts=?, description=?, next_run_at=?, updated_at=?
					 WHERE id=?`,
					appURL, secret, jp.Route, jp.Schedule, jp.Enabled, jp.TimeoutSeconds, jp.MaxAttempts, jp.Description, nr.UTC(), now, id); execErr != nil {
					return result, fmt.Errorf("update job %q: %w", jp.ID, execErr)
				}
			}
			result.Updated++
			continue
		}

		nr, err := next(jp.Schedule, now)
		if err != nil {
			return result, fmt.Errorf("parse schedule for job %q: %w", jp.ID, err)
		}
		if _, execErr := tx.ExecContext(ctx,
			`INSERT INTO jobs (id, job_id, source, app_url, secret, route, schedule, enabled, timeout_seconds, max_attempts, description, next_run_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, jp.ID, source, appURL, secret, jp.Route, jp.Schedule, jp.Enabled, jp.TimeoutSeconds, jp.MaxAttempts, jp.Description, nr.UTC(), now, now); execErr != nil {
			return result, fmt.Errorf("insert job %q: %w", jp.ID, execErr)
		}
		result.Created++
	}

	for jobID := range existing {
		if incoming[jobID] {
			continue
		}
		if _, execErr := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, compositeID(source, jobID)); execErr != nil {
			return result, fmt.Errorf("delete job %q: %w", jobID, execErr)
		}
		result.Deleted++
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit: %w", err)
	}
	return result, nil
}

const jobColumns = `id, job_id, source, app_url, secret, route, schedule, enabled, timeout_seconds, max_attempts, description, next_run_at, created_at, updated_at`

func scanJob(row interface{ Scan(...any) error }) (model.Job, error) {
	var j model.Job
	var enabledInt int
	if err := row.Scan(&j.ID, &j.JobID, &j.Source, &j.AppURL, &j.Secret, &j.Route, &j.Schedule, &enabledInt, &j.TimeoutSeconds, &j.MaxAttempts, &j.Description, &j.NextRunAt, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return model.Job{}, err
	}
	j.Enabled = enabledInt != 0
	return j, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (model.Job, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Job{}, ErrNotFound
	}
	if err != nil {
		return model.Job{}, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

func (s *Store) ListJobs(ctx context.Context) ([]model.Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM jobs ORDER BY source, job_id`)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var out []model.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// DueJobs returns enabled jobs whose next_run_at is at or before now.
func (s *Store) DueJobs(ctx context.Context, now time.Time) ([]model.Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE enabled = 1 AND next_run_at <= ? ORDER BY next_run_at`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("due jobs: %w", err)
	}
	defer rows.Close()

	var out []model.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// AdvanceNextRun sets a job's next_run_at to its next future occurrence after now.
// Called before firing so a slow run's retries can never cause the same occurrence
// to be picked up twice by a later tick.
func (s *Store) AdvanceNextRun(ctx context.Context, id, schedule string, now time.Time, next ScheduleFunc) error {
	nr, err := next(schedule, now)
	if err != nil {
		return fmt.Errorf("parse schedule: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET next_run_at = ? WHERE id = ?`, nr.UTC(), id); err != nil {
		return fmt.Errorf("advance next run: %w", err)
	}
	return nil
}

func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE jobs SET enabled = ?, updated_at = ? WHERE id = ?`, enabled, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set enabled rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteJob(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete job rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
