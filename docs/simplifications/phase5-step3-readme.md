# Phase 5 Step 5.3: README + demo recording — Simplified

The last step makes the project explainable to an interviewer: a README that
states what it is, how it's wired, how to run it, and how the self-healing
actually works — plus a cleanup pass so the code is presentable.

README (`README.md`) contains:
- A one-paragraph pitch (self-healing selectors + AI fix suggestions + a minimal
  live dashboard) framed for a QA/automation role.
- An architecture diagram in both Mermaid and plain ASCII: Go runner → WebSocket
  hub → React dashboard, runner → SQLite, runner ↔ mock API, runner → LLM.
- A **Docker quick start** (`docker compose up -d`, open :5173) and a **local dev**
  path (`go run ./cmd/testforge` + `npm run dev`), with a table of every env var
  (`SHOWCASE_FAILURES`, `OLLAMA_MODEL`, `OLLAMA_BASE_URL`, `ANTHROPIC_API_KEY`,
  `CHROME_PATH`, `PASS_THRESHOLD`, `VITE_WS_URL`) and its default.
- A "How self-healing works" section in plain language: grab DOM → score every
  element by tag/class/text/id → pick the best → re-check → report `healed` with
  `old → new`. Includes the concrete `submit` → `submit-btn` example.
- AI suggestions (provider choice), CI summary, a **demo GIF placeholder**
  (`docs/demo.gif` with a TODO to record it), and a project-layout map.

Cleanup pass:
- `gofmt -w` on the four files that weren't formatted; `go vet ./...` and
  `go build ./...` are clean (Go already errors on unused imports, so none remain).
- Confirmed the important values are env-configurable; only the service ports
  (`:8099` mock API, `:8080` WS) stay as constants, which is reasonable.
- No dead code: every runner/`healing`/`ai` function is reachable from `main`.

Status: the demo GIF is intentionally a placeholder to be recorded with a screen
capture of the dashboard healing a broken selector live. Everything else in the
README has been verified by running the exact commands it describes (Docker build
+ `docker compose up`, local `go run`, Ollama suggestions, CI gate).

This completes all five phases of the plan.
