# Phase 1 Step 1.2: Event schema — Simplified

This step defined the single data shape every other part of the system speaks: the `TestEvent`.

Why it matters: the runner, the database, and the dashboard all need to agree on what a "test result" looks like. Defining it once up front, as a shared struct in the `events` package, means nobody refactors later because the shape drifted between components.

What's inside `TestEvent`:
- `ID` — a unique identifier (a UUID, like a serial number that's never reused) so each event can be stored and looked up individually.
- `TestName` — the human label, e.g. `checkout_button_visible`.
- `Type` — `api` or `ui`, so the system knows which runner produced it.
- `Status` — one of `running`, `passed`, `failed`, `healed`, `retrying`. "healed" and "retrying" are the interesting ones that later phases (self-healing) depend on.
- `Timestamp` + `DurationMs` — when it happened and how long the test took, in milliseconds.
- `ErrorMessage` — only filled when something failed, so the dashboard has something to show.
- `Selector` — only for UI tests, the CSS selector that was checked.
- `Metadata` — a flexible string map for extras (e.g. the old→new selector when a test self-heals).

Two helpers come with it: `NewTestEvent(...)` stamps a fresh UUID and timestamp so callers don't forget, and `String()` produces a one-line log line like `[✓] api_ok (api) — passed in 42ms` — the only visibility the system has before the dashboard exists.

A small test confirms one event of every status renders without panicking and reads clearly. It passes.
