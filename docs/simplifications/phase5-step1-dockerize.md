# Phase 5 Step 5.1: Dockerize — Simplified

This step made the whole project run from a single command, fully containerized, so anyone can try it without installing Go/Node/Chrome.

What was added:
- `Dockerfile.go` — multi-stage: builds the static Go binary (`CGO_ENABLED=0`, so the pure-Go SQLite driver needs no C library) on `golang:1.25`, then copies it onto `alpine:3.20` with `chromium` installed via apk. Sets `CHROME_PATH=/usr/bin/chromium` so `chromedp` finds the browser in-container. The service writes its SQLite DB into `/data` (a `VOLUME`), listens on `:8080`.
- `Dockerfile.dashboard` — multi-stage: installs deps and builds the React app on `node:22-slim`, then serves `dist/` with `nginx:alpine` on `:5173` (custom `dashboard/nginx.conf` listens on 5173 and SPA-falls back to index.html).
- `docker-compose.yml` — brings up both services. The Go service publishes `18080:8080` (host 8080 was taken by a pre-existing Spark container) with a `./data:/data` volume for SQLite persistence; the dashboard publishes `5173:5173` and depends on it. Optional AI env vars (`OLLAMA_MODEL`/`OLLAMA_BASE_URL`/`ANTHROPIC_API_KEY`) are passed through. The dashboard's WS URL is configurable via `VITE_WS_URL` (default `ws://localhost:18080/ws`).
- `.dockerignore` and a `data/` git-ignore entry.

Two supporting code changes were needed for containers:
- `runner/ui_test_runner.go` now reads `CHROME_PATH` (default `/usr/bin/google-chrome`) instead of a hardcoded path, and adds `--disable-gpu` / `--disable-dev-shm-usage` so headless Chrome runs reliably in a container.
- `cmd/testforge/main.go` now blocks on `SIGINT`/`SIGTERM` after the run, keeping the WebSocket server (and thus the dashboard connection) alive as a long-running service instead of exiting immediately.

Verification: `docker compose config` validates; `docker compose build` produced both images; `docker compose up -d` brought up both, the Go container ran the full 7-test suite (UI tests + self-heal worked under in-container Chromium), persisted `testforge.db` to the `./data` volume, and logged "testforge idle". `curl localhost:5173` served the dashboard HTML, and a WebSocket client connected successfully through the published `18080` port (server logged connect/disconnect).

Note: host port 8080 was already occupied by another container, so the Go service is published on **18080** — open the dashboard at `http://localhost:5173` and it points at `ws://localhost:18080/ws`.
