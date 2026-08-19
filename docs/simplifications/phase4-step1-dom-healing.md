# Phase 4 Step 4.1: DOM snapshot + candidate matching — Simplified

This step is the project's headline feature: when a UI test can't find its selector, the runner inspects the live page, finds the most likely replacement element, and "heals" the test instead of failing it.

How it works:
- `runner/healing.go` holds the logic, completely separate from the runner plumbing.
  - `parseSelector` breaks the original CSS selector into tag / id / class tokens.
  - After a UI test fails to find its selector, `RunUITest` grabs the page's full DOM via `chromedp.OuterHTML("html", …)` and parses it with `golang.org/x/net/html`.
  - `scoreCandidates` walks every element and scores it with a simple heuristic: +2 for a matching tag, +3 for a shared class substring, +5 if the element's visible text contains a class token, +3 for a shared id substring, +5 if the text contains the id token. `candidateSelector` turns the winner into a real CSS selector (preferring the renamed class when the original named classes, else id, else first class, else tag).
  - If the best score clears `healThreshold` (3) **and** the candidate selector actually resolves to an element on the page, the test is re-checked with the new selector and returns a `healed` event carrying `old_selector` → `new_selector` (plus the score) in `Metadata`.
- `RunUITest` now logs every heal attempt explicitly (`[HEAL] … best='…' score=N`) and a success line (`[HEALED] '.submit' -> '.submit-btn' (score: 3)`) so bad heals are debuggable.

Demo wiring:
- The mock API gained a `/shop` page whose "buy" button was *renamed* from class `submit` to `submit-btn` — a realistic drift.
- The suite gained a `shop_submit_healed` UI test pointing at the old `.submit`; it now heals to `.submit-btn` instead of failing.

Verification:
- Live run shows `[✚] shop_submit_healed (ui) — healed` with metadata `old_selector=.submit`, `new_selector=.submit-btn`.
- Tests prove the heal (`.submit` → `.submit-btn`) and prove a selector with *no* plausible match stays a permanent `selector_not_found` failure. Full `go test ./...` is green.
