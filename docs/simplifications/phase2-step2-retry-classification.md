# Phase 2 Step 2.2: Retry + error classification — Simplified

This step made failures *useful* instead of a flat "failed": it now knows *why* a test failed and gives transient failures a second chance.

What changed:
- `TestEvent` gained an `ErrorType` field, one of: `none` (passing), `timeout`, `assertion` (wrong status/value), `network` (can't reach the host), `selector_not_found` (UI element missing). The dashboard and later self-healing logic will branch on this.
- Both runners now classify their failures. The API runner calls a `timeout`-vs-`network` check on the error (a client timeout is `timeout`; DNS/refused/TLS is `network`) and labels a status mismatch as `assertion`. The UI runner labels unreachable pages as `timeout`/`network` and a missing element as `selector_not_found`.
- The log line now shows the class, e.g. `[✗] example_missing_element (ui) — failed — selector_not_found — selector not found: …`.
- `RunWithRetry(tc, maxRetries)` wraps a single case: run it, and if it fails, print a `[RETRYING]` line with the error class and a `retrying` event, then run it again — up to `maxRetries` times — returning the final outcome. `Suite.Run` now calls `RunWithRetry(tc, 1)`, so each failing test is automatically retried once.

Verification:
- Live run shows each intentional failure emit a retry line + `retrying` event, then a final `failed` event carrying the right class (`assertion`, `network`, `selector_not_found`).
- Tests now assert `ErrorType` for each failure path (assertion via a local `httptest` server, network via a refused port, selector_not_found via a missing element) and that `RunWithRetry` retries once and still fails. All pass.

Note: the `retrying` event is currently only printed (for the log); in Phase 3 it will also be broadcast live over the WebSocket so the dashboard can show the spinner in real time.
