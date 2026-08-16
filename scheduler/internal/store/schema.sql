CREATE TABLE IF NOT EXISTS jobs (
  id               TEXT PRIMARY KEY,   -- source || '::' || job_id
  job_id           TEXT NOT NULL,
  source           TEXT NOT NULL,
  app_url          TEXT NOT NULL,
  secret           TEXT NOT NULL,
  route            TEXT NOT NULL,
  schedule         TEXT NOT NULL,
  enabled          INTEGER NOT NULL DEFAULT 1,
  timeout_seconds  INTEGER NOT NULL,
  max_attempts     INTEGER NOT NULL,
  description      TEXT,
  next_run_at      DATETIME NOT NULL,
  created_at       DATETIME NOT NULL,
  updated_at       DATETIME NOT NULL,
  UNIQUE(source, job_id)
);

CREATE INDEX IF NOT EXISTS idx_jobs_enabled_next_run ON jobs(enabled, next_run_at);

CREATE TABLE IF NOT EXISTS job_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT NOT NULL,
  status TEXT NOT NULL, -- in_progress | success | failed
  attempt INTEGER NOT NULL DEFAULT 1,
  started_at DATETIME NOT NULL,
  finished_at DATETIME,
  http_status INTEGER,
  error TEXT
);

CREATE INDEX IF NOT EXISTS idx_job_runs_job_id_status ON job_runs(job_id, status);
CREATE INDEX IF NOT EXISTS idx_job_runs_job_id_started ON job_runs(job_id, started_at DESC);
