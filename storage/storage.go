package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"testforge/events"
)

// Store wraps a SQLite database that records test runs and their events so the
// data survives process restarts (needed for history/trends later).
type Store struct {
	db *sql.DB
}

// Open connects to the SQLite file at path and ensures the schema exists,
// creating the tables on first run.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// migrate creates the runs and test_events tables if they don't exist yet.
func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id         TEXT PRIMARY KEY,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			total      INTEGER NOT NULL,
			passed     INTEGER NOT NULL,
			failed     INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS test_events (
			id            TEXT PRIMARY KEY,
			run_id        TEXT NOT NULL,
			test_name     TEXT NOT NULL,
			type          TEXT NOT NULL,
			status        TEXT NOT NULL,
			error_type    TEXT,
			error_message TEXT,
			metadata      TEXT,
			duration_ms   INTEGER,
			timestamp     TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_events_run ON test_events(run_id);
	`)
	return err
}

// SaveRun inserts one run row plus a child row per event, inside a single
// transaction so a crash mid-save can't leave a half-written run.
func (s *Store) SaveRun(evs []events.TestEvent) error {
	if len(evs) == 0 {
		return nil
	}

	runID := evs[0].ID // reuse first event's uuid as the run id is fine; but
	// better a dedicated run id. We generate one from the first event's
	// timestamp to keep it stable and unique per run.
	runID = fmt.Sprintf("run-%d", evs[0].Timestamp.UnixNano())

	started := evs[0].Timestamp
	finished := evs[len(evs)-1].Timestamp
	total, passed, failed := 0, 0, 0
	for _, e := range evs {
		total++
		switch e.Status {
		case events.StatusPassed, events.StatusHealed:
			passed++
		default:
			failed++
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO runs (id, started_at, finished_at, total, passed, failed)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		runID, started.Format(time.RFC3339Nano), finished.Format(time.RFC3339Nano),
		total, passed, failed,
	); err != nil {
		return fmt.Errorf("insert run: %w", err)
	}

	for _, e := range evs {
		meta, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata for %s: %w", e.ID, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO test_events
			   (id, run_id, test_name, type, status, error_type, error_message, metadata, duration_ms, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, runID, e.TestName, string(e.Type), string(e.Status),
			string(e.ErrorType), e.ErrorMessage, string(meta), e.DurationMs,
			e.Timestamp.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("insert event %s: %w", e.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Printf("Run persisted: %d passed, %d failed\n", passed, failed)
	return nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	return s.db.Close()
}
