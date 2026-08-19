# Problem: httpbin.org is unreliable from this environment

**Phase:** 2 — Core Test Engine
**Step:** 2.1 — Test suite definition + sequential runner

## What happened

The hardcoded suite used `https://httpbin.org` as the API-under-test target
(the plan's suggested public API). When the suite was run, *every* httpbin
request returned HTTP 503 (service unavailable) instead of the expected 200/404:

```
[✗] httpbin_status_200 (api) — failed — expected status 200 but got 503
[✗] httpbin_status_404 (api) — failed — expected status 404 but got 503
```

The runner behaved correctly (it honestly reported the mismatch), but the
demo now shows API tests failing for an external, non-reproducible reason
rather than for real assertion/network bugs. That undermines the portfolio
story: a reviewer can't tell whether the framework or the free service is at
fault, and the failure is flaky (httpbin sometimes works, sometimes 503s).

## Fix

Resolved by **option 1**: a local mock API was added and the suite repointed at
it.

- New `mockapi` package (`mockapi/mockapi.go`) serves `/status/200`,
  `/status/404`, `/status/500`, and `/healthz` with the exact status codes.
- New standalone binary `cmd/mockapi` so it can also run as its own Docker
  service ("the system under test").
- `cmd/testforge/main.go` starts the mock in-process on `:8099` and waits for
  `/healthz` before running the suite, so `go run ./cmd/testforge` is fully
  self-contained, deterministic, and offline-friendly.
- The suite's API cases now hit `http://localhost:8099/...`.

Result after the fix: the suite yields a stable `3 passed, 3 failed` — the
failures are the two intentional breakages (wrong expected status, dead host)
plus the intentionally missing UI selector. No more flaky external 503s.
