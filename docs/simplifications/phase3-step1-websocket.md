# Phase 3 Step 3.1: WebSocket event server — Simplified

This step built the live pipe between the runner and the (not-yet-built) dashboard: a WebSocket server that streams every event to any connected client the instant it happens.

What was added (the `server` package):
- `Hub` — a minimal pub-sub broker. Dashboards connect over WebSocket; the runner calls `hub.Broadcast(event)` and the hub writes that event as JSON to every connected client. It keeps a locked map of live connections and drops any that error out.
- `ServeWS` — the `/ws` HTTP handler. It upgrades the connection, registers the client, logs `"dashboard client connected"`, and runs a read loop purely to detect disconnects (logging `"dashboard client disconnected"`). Dead connections are cleaned up so broadcasts don't pile up on zombies.
- `Start(addr)` — serves `/ws` on the given port (`:8080`).

How it's wired:
- `runner.Suite` gained an `Emit func(events.TestEvent)` callback. `RunWithRetry` now calls `emit` for *each* event as it's born — both the transient `retrying` events and the final outcome — not just at the end. This is what makes the feed truly live.
- `main.go` creates the hub, starts it on `:8080` in a goroutine, and sets `suite.Emit = func(e){ hub.Broadcast(e) }` before running the suite.

Verification: a server test spins up the hub on an `httptest` server, dials `/ws` with a real WebSocket client, confirms the client registers, broadcasts one event, and reads it back as JSON with the right name/status — proving events arrive live, one at a time, not batched at the end. All package tests pass. The visual "watch it update in the browser" confirmation happens in Step 3.2 once the React shell exists.

Note: `retrying` events are broadcast for the live view but are *not* persisted to SQLite — only final outcomes are, which is what history/trends should record.
