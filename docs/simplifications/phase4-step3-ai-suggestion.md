# Phase 4 Step 4.3: AI-suggested fix on real failures — Simplified

This step added the second intelligent behavior: for failures that *can't* be auto-healed (API errors, assertion failures, network/timeout), the framework asks a local LLM for a one-line suggested root cause + fix and shows it in the same detail slot used for heal diffs.

Provider choice (updated from the original plan): the user opted to use a **local Ollama** model instead of the Anthropic API, keeping the project fully self-contained/offline — consistent with the local mock API decision. The `ai` package therefore supports two providers selected from the environment:
- `ANTHROPIC_API_KEY` set → Anthropic Messages API (model `AI_MODEL`, default `claude-sonnet-4-5`).
- `OLLAMA_MODEL` (or `OLLAMA_BASE_URL`) set → local Ollama via its OpenAI-compatible `/v1/chat/completions` endpoint (default model `qwen2.5-coder:7b`, default base `http://localhost:11434/v1`).
- Neither set → client disabled; `main.go` silently skips AI (no crash, no hang).

What was added:
- `ai` package (`ai/ai.go`) — a `Client` with `New()`, `Enabled()`, `Provider()`, and `Suggest(ctx, FailureContext)`. `FailureContext` carries the failed test's name, type, error type, error message, URL, expected status, and selector. `buildPrompt` asks for exactly one short sentence; `postJSON` is shared by both providers.
- `main.go` — after the suite finishes, every `failed` (non-healed) event gets an AI suggestion via `aiClient.Suggest`, attached to `Metadata["ai_suggestion"]`, logged with latency (`AI suggestion (ollama) generated for '…' in Nms`), and re-broadcast so the dashboard's current-test detail refreshes. Only then is the run persisted, so the suggestion is saved too.
- Dashboard (`App.jsx` + `index.css`) — for a `failed` event, if `metadata.ai_suggestion` exists it's shown in the detail line (single line, ellipsized with `…` if too long); otherwise the plain error message shows.

Schema follow-up: the `test_events` table gained a `metadata` TEXT column so heal diffs and AI suggestions survive in SQLite, not just in the live broadcast. `SaveRun` now marshals `event.Metadata` to JSON.

Verification:
- `ai` unit tests pass (disabled-without-any-provider fails fast; an `httptest` mock proves the Ollama chat/completions parsing; `trimOneLine` and `buildPrompt` checked). `go build ./...` and `npm run build` succeed.
- **Live test gate actually run against local Ollama** (`OLLAMA_MODEL=qwen2.5-coder:7b`): three failed tests each produced a real one-line suggestion (e.g. *"API endpoint is incorrectly returning a 200 instead of the expected 500. Fix by updating the API logic…"*), logged with latency and stored in `test_events.metadata` — confirming the plan's gate without needing any external API key.
