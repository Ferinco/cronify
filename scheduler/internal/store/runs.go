package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/model"
)

// ClaimRun implements CLAUDE.md's locking algorithm: inside a BEGIN IMMEDIATE
// transaction, look for an in_progress row for jobID. No row -> claim (insert a
// fresh in_progress row). Fresh in_progress row (younger than staleAfter) -> don't
// claim, caller should skip. Stale in_progress row -> mark it failed ("stale lock"),
// reclaim, and start a new run.
//
// BEGIN IMMEDIATE (set via the store's DSN _txlock=immediate) is what makes this
// check-then-act atomic against a concurrent manual "run now" call for the same job.
func (s *Store) ClaimRun(ctx context.Context, jobID string, staleAfter time.Duration, now time.Time) (claimed bool, runID int64, err error) {
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	var startedAt time.Time
	scanErr := tx.QueryRowContext(ctx,
		`SELECT id, started_at FROM job_runs WHERE job_id = ? AND status = ? ORDER BY id DESC LIMIT 1`,
		jobID, model.StatusInProgress).Scan(&existingID, &startedAt)

	switch {
	case scanErr == sql.ErrNoRows:
		runID, err = insertInProgress(ctx, tx, jobID, 1, now)
		if err != nil {
			return false, 0, err
		}
		claimed = true

	case scanErr != nil:
		return false, 0, fmt.Errorf("query in-progress: %w", scanErr)

	default:
		if now.Sub(startedAt) < staleAfter {
			claimed = false
		} else {
			staleErr := "stale lock"
			if _, err := tx.ExecContext(ctx,
				`UPDATE job_runs SET status = ?, finished_at = ?, error = ? WHERE id = ?`,
				model.StatusFailed, now, staleErr, existingID); err != nil {
				return false, 0, fmt.Errorf("reclaim stale lock: %w", err)
			}
			runID, err = insertInProgress(ctx, tx, jobID, 1, now)
			if err != nil {
				return false, 0, err
			}
			claimed = true
		}
	}

	if err := tx.Commit(); err != nil {
		return false, 0, fmt.Errorf("commit: %w", err)
	}
	return claimed, runID, nil
}

func insertInProgress(ctx context.Context, tx *sql.Tx, jobID string, attempt int, now time.Time) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO job_runs (job_id, status, attempt, started_at) VALUES (?, ?, ?, ?)`,
		jobID, model.StatusInProgress, attempt, now)
	if err != nil {
		return 0, fmt.Errorf("insert run: %w", err)
	}
	return res.LastInsertId()
}

// StartAttempt records a new attempt (2nd, 3rd, ...) within a run this goroutine
// already owns from ClaimRun — it does not re-scan for a held lock.
func (s *Store) StartAttempt(ctx context.Context, jobID string, attempt int, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO job_runs (job_id, status, attempt, started_at) VALUES (?, ?, ?, ?)`,
		jobID, model.StatusInProgress, attempt, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("start attempt: %w", err)
	}
	return res.LastInsertId()
}

// FinishRun marks a run row success or failed.
func (s *Store) FinishRun(ctx context.Context, runID int64, status string, httpStatus *int, runErr *string, finishedAt time.Time) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE job_runs SET status = ?, finished_at = ?, http_status = ?, error = ? WHERE id = ?`,
		status, finishedAt.UTC(), httpStatus, runErr, runID); err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return nil
}

func (s *Store) ListRuns(ctx context.Context, jobID string, limit int) ([]model.JobRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, job_id, status, attempt, started_at, finished_at, http_status, error
		 FROM job_runs WHERE job_id = ? ORDER BY started_at DESC, id DESC LIMIT ?`, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var out []model.JobRun
	for rows.Next() {
		var r model.JobRun
		var finishedAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.JobID, &r.Status, &r.Attempt, &r.StartedAt, &finishedAt, &r.HTTPStatus, &r.Error); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			r.FinishedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
