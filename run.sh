#!/usr/bin/env bash
# One command to run the whole stack locally:
#   Go backend (mock API + WebSocket)  -> :8080  (ws://localhost:8080/ws)
#   React dashboard (Vite dev server)  -> http://localhost:5173
# Press Ctrl-C to stop both.
#
# Env: SHOWCASE_FAILURES (default 1), OLLAMA_MODEL, ANTHROPIC_API_KEY, CHROME_PATH
# are passed through to the backend.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

# Defaults so the demo showcases failures / healing / AI out of the box.
export SHOWCASE_FAILURES="${SHOWCASE_FAILURES:-1}"

cleanup() {
  echo ""
  echo "shutting down..."
  [[ -n "${GO_PID:-}" ]] && kill "$GO_PID" 2>/dev/null
  [[ -n "${FE_PID:-}" ]] && kill "$FE_PID" 2>/dev/null
  wait 2>/dev/null
}
trap cleanup SIGINT SIGTERM EXIT

echo ">> starting backend (go run ./cmd/testforge) — first run compiles, be patient..."
go run ./cmd/testforge &
GO_PID=$!

# Wait until the backend's mock API is actually serving (i.e. go finished
# compiling and the server is up) before launching the dashboard. This avoids a
# long "connecting…" spinner and means the dashboard usually connects mid-run.
echo ">> waiting for backend to be ready..."
HEALTHY=0
for i in $(seq 1 120); do
  if curl -s -o /dev/null "http://localhost:8099/healthz" 2>/dev/null; then
    HEALTHY=1
    break
  fi
  # If the backend process died, don't hang.
  if ! kill -0 "$GO_PID" 2>/dev/null; then
    echo "backend process exited before becoming ready — check output above."
    break
  fi
  sleep 1
done
if [ "$HEALTHY" -ne 1 ]; then
  echo "backend did not become ready. Aborting frontend start."
  exit 1
fi
echo ">> backend ready."

echo ">> starting frontend (vite dev)..."
cd "$ROOT/dashboard"
if [ ! -d node_modules ]; then
  echo ">> installing dashboard deps (first run)..."
  npm install
fi
npm run dev &
FE_PID=$!

echo ""
echo "Backend:  ws://localhost:8080/ws"
echo "Frontend: http://localhost:5173"
echo "Press Ctrl-C to stop."
wait
