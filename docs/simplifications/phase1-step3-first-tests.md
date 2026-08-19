# Phase 1 Step 1.3: First API test + first UI test — Simplified

This step proved the runner can actually drive both kinds of tests, not just compile.

Two runners were added to the `runner` package:

- `RunAPITest(name, url, expectedStatus)` — does a real HTTP GET (with a 10s timeout) and compares the response's status code to what you expected. If they match → `passed`; otherwise → `failed` with a message like "expected status 200 but got 500". Network errors count as failures too.
- `RunUITest(name, url, selector)` — launches a real headless Chrome (the `chromedp` library drives Chrome), navigates to the URL, and checks whether the given CSS selector resolves to an element. Found → `passed`; not found → `failed` with "selector not found: …". Each test spins up its own browser instance so one crash can't poison the next.

Both return a `TestEvent`, the shared shape from Step 1.2, so they slot straight into the rest of the system.

`main.go` now wires up one of each — an API test against `httpbin.org` and a UI test against `example.com` checking for an `<h1>` — and prints each event with its `.String()` method.

Verification:
- Live run printed two passing events: `[✓] httpbin_status_200 (api)` and `[✓] example_h1_visible (ui)`.
- A small test additionally confirms the failure path: a bad expected status and a non-existent selector both correctly produce `failed` events with clear error messages.

Note: `chromedp` v0.16/0.15 require Go 1.26, but this environment has Go 1.25, so the dependency was pinned to v0.14.2. Chrome is present at `/usr/bin/google-chrome`, which the runner points at explicitly.
