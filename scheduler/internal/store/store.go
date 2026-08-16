// Package store is the SQLite-backed persistence layer: job configs and run history.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DATE/DATETIME/TIMESTAMP-declared columns get modernc.org/sqlite's built-in
// time.Time <-> TEXT round-trip (see its rows.go/conn.go): binding a time.Time
// writes it via t.String(), and scanning such a column back yields a time.Time
// automatically. All datetime columns in schema.sql rely on this — always bind
// and scan them as time.Time (normalized to UTC), never hand-format to string.

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path, configured for a
// single-writer, non-cgo setup: one pooled connection, BEGIN IMMEDIATE by default
// (required by ClaimRun's check-then-act atomicity), and a busy_timeout as a backstop.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_txlock=immediate&_busy_timeout=5000", url.PathEscape(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate applies schema.sql. Idempotent: every statement is CREATE ... IF NOT EXISTS.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
