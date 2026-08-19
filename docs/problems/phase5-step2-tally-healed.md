# Problem: `healed` tests were counted as failures in the run tally

**Phase:** 5 — DevOps & Polish
**Step:** 5.2 — GitHub Actions CI

## What happened

The run-end banner printed by `Suite.Run` used `if ev.Status == StatusPassed { passed++ } else { failed++ }`,
so a `healed` event (a UI test that recovered from a broken selector) was tallied as a **failure**.
`SaveRun`, by contrast, correctly counts `healed` as a pass. The two disagreed, which was invisible until
CI's pass-rate gate was added:

- `=== suite end: 3 passed, 1 failed ===` (banner, wrong)
- `Run persisted: 4 passed, 0 failed`  (SaveRun, correct)

Because the clean suite is 3 passed + 1 healed, the banner wrongly showed a failure, and the pass-rate
gate (which counts healed as pass → 100%) would have looked inconsistent with the banner. Worse, any
future code that trusts the banner's counts would misreport.

## Fix

Made `Suite.Run`'s tally match `SaveRun`: count both `StatusPassed` and `StatusHealed` as passes.
Verified the clean suite now reports `4 passed, 0 failed` consistently across three runs, and the
pass-rate gate reads 100% in CI mode.
