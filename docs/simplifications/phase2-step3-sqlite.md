# Phase 2 Step 2.3: SQLite persistence — Simplified

This step gave the framework a memory: every run and every event it produces now lives in a local SQLite file instead of vanishing when the process exits.

What was added (the `storage` package):
- `Open(path)` connects to a SQLite database and creates the schema on first use. It uses the pure-Go `modernc.org/sqlite` driver (no CGO), which keeps Docker/CI builds simple across platforms.
- Two tables: `runs` (one row per suite execution — id, started/finished time, and the total/passed/failed counts) and `test_events` (one row per event — test name, type, status, error type, error message, duration, timestamp, all linked to their run by `run_id`). An index on `run_id` keeps lookups fast.
- `SaveRun(events)` writes one run plus all its child events inside a single transaction, so a crash mid-write can't leave a half-saved run. Totals are computed from the events (passed or healed count as green; everything else as failed). It prints `Run persisted: N passed, M failed`.
- `main.go` now opens `testforge.db` after the suite finishes and calls `SaveRun` on the collected events.

Why it matters: this is the historical record the dashboard's trends and the AI layer will read later — without it, a restart wipes all context.

Verification:
- Running the suite twice produced two rows in `runs` (6 tests each, 3 passed / 3 failed) and 12 rows in `test_events`, each with the correct type and error class.
- A storage test (using a temp DB) confirms the run/event round-trip and that a fresh file is created.

The `*.db` file is git-ignored, so the real database never gets committed.
