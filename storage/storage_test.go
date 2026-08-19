package storage

import (
	"os"
	"path/filepath"
	"testing"

	"testforge/events"
)

// TestSaveRunRoundTrip writes a small run and reads it back, confirming both
// the run summary and its child events land in the database.
func TestSaveRunRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	evs := []events.TestEvent{
		events.NewTestEvent("a", events.TypeAPI, events.StatusPassed),
		events.NewTestEvent("b", events.TypeUI, events.StatusFailed),
	}
	evs[1].ErrorType = events.ErrSelectorNotFound
	evs[1].Selector = "#x"

	if err := s.SaveRun(evs); err != nil {
		t.Fatalf("save: %v", err)
	}

	rows, err := s.db.Query("SELECT total, passed, failed FROM runs")
	if err != nil {
		t.Fatalf("query runs: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one run row")
	}
	var total, passed, failed int
	if err := rows.Scan(&total, &passed, &failed); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if total != 2 || passed != 1 || failed != 1 {
		t.Fatalf("unexpected run totals: got %d/%d/%d", total, passed, failed)
	}

	var evCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM test_events").Scan(&evCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if evCount != 2 {
		t.Fatalf("expected 2 events, got %d", evCount)
	}
}

// TestOpenCreatesFile confirms a fresh database file is actually created.
func TestOpenCreatesFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("db should not exist yet")
	}
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s.Close()
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}
