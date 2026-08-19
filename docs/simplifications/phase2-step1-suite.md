# Phase 2 Step 2.1: Test suite definition + sequential runner — Simplified

This step replaced the two hardcoded `RunAPITest`/`RunUITest` calls in `main.go` with a real, declarative test suite.

What was added:
- `TestCase` — a plain config struct describing one test: its `Name`, its `Type` (`api` or `ui`), and the details the matching runner needs. API tests carry `URL` + `ExpectedStatus`; UI tests carry `URL` + `Selector`. One struct for both keeps things simple instead of building a type hierarchy.
- `Suite` — an ordered list of `TestCase`s with a `Run()` method. It walks the cases in order, dispatches each to `RunAPITest` or `RunUITest` based on its type, collects every `TestEvent`, and returns them all.
- Suite-start / suite-end banners so the console shows overall progress (`=== suite start: 6 test(s) ===` … `=== suite end: 1 passed, 5 failed ===`) on top of the per-test lines.

`main.go` now builds a suite of 6 cases (4 API, 2 UI) — including a couple deliberately pointed at bad URLs/selectors so failures show up on demand — and calls `suite.Run()`.

Verification: running the suite printed all six events in order with a correct pass/fail tally, proving the orchestration layer works before retry, persistence, or dashboards are layered on.

### Follow-up: local mock API (added after this step)

A reliability problem surfaced immediately (see `docs/problems/phase2-step1-httpbin-flaky.md`): the public `httpbin.org` returned 503, making API results flaky and non-credible. Rather than accept that, a self-contained `mockapi` package was added that serves fixed status codes (`/status/200`, `/status/404`, `/status/500`, `/healthz`), plus a standalone `cmd/mockapi` binary and an in-process launch from `main.go` on `:8099`. The suite's API cases were repointed at it, giving a stable `3 passed, 3 failed` (the three failures being the intentional breakages). This keeps every later phase — retry, self-healing, CI — deterministic and offline-friendly.
