# Phase 5 Step 5.2: GitHub Actions CI — Simplified

This step put the framework in a real pipeline that runs on every push, so the "it works" claim is provable rather than asserted.

What was added: `.github/workflows/ci.yml` runs on push/pull_request and:
1. Sets up Go 1.25, Node 22, and **headless Chrome** via `browser-actions/setup-chrome` (chromedp needs a real browser; the path is exported to `CHROME_PATH` so the runner finds it).
2. `go vet ./...` and `go test ./...` — including the chromedp UI tests, which now have Chrome available.
3. `npm ci && npm run build` in `dashboard/` to prove the frontend compiles.
4. `go run ./cmd/testforge` to run the suite headlessly. A pass-rate gate (default 80% in CI) fails the job if too many tests fail; `healed` counts as a pass. `SHOWCASE_FAILURES` is left unset so CI runs the clean suite and stays green.
5. Uploads `testforge.db` as a build artifact (`actions/upload-artifact`) so the run history survives.

Supporting changes made this step possible:
- `main.go` gained a pass-rate gate + CI-aware exit: in CI it computes the rate, fails if below `PASS_THRESHOLD`, then exits (so the job finishes instead of blocking on the signal wait used locally). `buildSuite()` puts the *intentional* failures behind `SHOWCASE_FAILURES` so CI runs green while the demo still showcases failures/healing/AI.
- UI tests now hit the **local mock API** (`/` and `/shop`) instead of `example.com`, removing the last external-network dependency so CI is deterministic (this also retired the earlier httpbin-style flakiness risk).
- Two bugs surfaced and were fixed (see problem docs): `healed` was wrongly counted as a failure in the run tally, and the Go Dockerfile had a `.go` extension that broke `go build ./...`.

Verification (simulated locally, since there's no GitHub repo here yet):
- `go build ./...` + `go vet ./...` clean; `go test ./...` all pass.
- `CI=true PASS_THRESHOLD=80` (clean suite) → `pass rate 100% (threshold 80%)`, exit 0.
- `CI=true SHOWCASE_FAILURES=1 PASS_THRESHOLD=80` → `FAIL: pass rate 57% is below threshold 80%`, exit 1 — proving the "break a test → build fails" behavior the plan asked for.
- `docker compose up -d --build` with the new code still runs the full showcase suite (heal + failures) under in-container Chromium.

To actually enable it: push to GitHub; the Action runs automatically. Set `ANTHROPIC_API_KEY` or `OLLAMA_MODEL` as a repo secret if you want AI suggestions in CI (otherwise they're skipped).
