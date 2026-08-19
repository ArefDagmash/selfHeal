# Phase 3 Step 3.2: React shell — current-test view only — Simplified

This step built the dashboard's heart: a React page that shows exactly one test at a time, front and center, updating live. No list, no table, no feed.

What was created in `dashboard/`:
- A Vite + React 18 app (`package.json`, `vite.config.js` on port 5173, `index.html`, `src/main.jsx`).
- `src/App.jsx` — the whole UI. On mount it opens a WebSocket to `ws://localhost:8080/ws` (with a 1.5s auto-reconnect) and stores **only the most recent** `TestEvent` in state — a single object, never an array. Every incoming frame just replaces it.
- The focal card renders three things, centered: a 56px circular icon (color + glyph by status), the test name (16px medium), and one muted detail line (13px). Status colors: passed = green check, failed = red cross, healed/running = blue refresh (running spins), retrying = amber clock.
- The detail line is intentionally minimal: for a `healed` event it shows `old_selector → new_selector` from the event's metadata; for `failed` it shows the error message; otherwise it's blank.
- `src/index.css` — dark stage, centered layout, a tiny live/connecting indicator in the corner. Icons are inline SVGs so there are no extra UI dependencies.

Why this shape: the plan's whole design thesis is "one test at a time." Keeping state to a single object physically prevents the app from ever growing a scrollable feed — the discipline is enforced in the data model, not just the layout.

Verification: `npm install` + `npm run build` succeed and produce a production bundle (no missing imports, JSX valid). The live data contract is already proven server-side (Step 3.1's WS test), and the event JSON fields the component reads (`test_name`, `status`, `error_message`, `metadata`) match the Go `TestEvent` exactly. The full visual "watch it flip as tests run" check happens when you run `npm run dev` next to the Go server (Step 3.3 adds the trail/counts, after which a screen recording becomes meaningful).

Note: the 2 `npm audit` warnings are in Vite/esbuild *dev* dependencies and don't affect the shipped bundle; they're deferred to the Phase 5 cleanup pass.
