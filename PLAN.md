# Project Plan: Self-Healing Test Dashboard (testforge)

## Phase 1: Foundation
> Goal: A Go test runner that can execute one API test and one UI test, with results printed as structured logs

- [x] Step 1.1: Project scaffold
- [x] Step 1.2: Event schema
- [x] Step 1.3: First API test + first UI test

## Phase 2: Core Test Engine
> Goal: A real test suite (API + UI) runs end-to-end and emits structured pass/fail/error events

- [x] Step 2.1: Test suite definition + sequential runner
- [x] Step 2.2: Retry + error classification
- [x] Step 2.3: SQLite persistence

## Phase 3: Live Dashboard
> Goal: One test renders front-and-center at a time, updating live, with a thin trail showing recent history

- [x] Step 3.1: WebSocket event server
- [x] Step 3.2: React shell — current-test view only
- [x] Step 3.3: History trail + running counts

## Phase 4: Self-Healing & Intelligence
> Goal: Broken UI selectors get auto-healed and failures get an AI-suggested fix, both shown in the same one-line detail slot

- [x] Step 4.1: DOM snapshot + candidate matching
- [x] Step 4.2: Healed state in the focused view
- [x] Step 4.3: AI-suggested fix on real failures

## Phase 5: DevOps & Polish
> Goal: The whole thing runs in CI, in Docker, with a README good enough to hand an interviewer

- [x] Step 5.1: Dockerize
- [x] Step 5.2: GitHub Actions CI
- [x] Step 5.3: README + demo recording
