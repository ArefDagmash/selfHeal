# Phase 4 Step 4.2: Healed state in the focused view — Simplified

This step taught the dashboard to *show* a heal — it just means the detail line does its job for the `healed` status. No new component, no badge, no expandable panel, because there's only ever one test on screen.

What changed in `dashboard/src/App.jsx` + `index.css`:
- The `Detail` component now special-cases `healed`: it renders three spans — the old selector (muted + struck through), a muted `→` arrow, and the new selector in the accent color (`--accent`, the same blue as the healed icon). For `failed` it still shows the error message; everything else stays blank.
- Added `--accent: #3b82f6` to the theme and `.old-sel` / `.arrow` / `.new-sel` styles. It fits on one line at 13px for realistic selector names; there's no card or collapse around it.

Why nothing more: the entire UI premise is single-focus. "Showing the heal" is just the detail line fulfilling its contract for one more status — adding a dedicated widget would have broken the minimalism the design was built on.

Verification: `npm run build` succeeds. The component reads `event.metadata.old_selector` / `new_selector`, which exactly match the keys `RunUITest` writes in Step 4.1, so a healed event surfaces as `~~.submit~~ → .submit-btn` in the accent color. The full visual confirmation is the live run you'll do with `npm run dev` next to the Go server (where the `shop_submit_healed` test heals).

Note: this is purely a frontend presentation change; the Go side is untouched and all Go tests remain green.
