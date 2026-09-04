# Archive report: 2026-09-04-verify-complexity-gate

Implemented and verified 2026-09-04. Diff-aware complexity gate blocks `verify` on NEW 15/20 offenders in critical packages; grandfathering is structural (hunk-bounded measurement).

## Evidence
- Lens export: `OffendersForFileDiffs` + `SplitFileDiffs` (`internal/review/lens/readability/complexity.go`), 5 new tests.
- Gate: `internal/sdd/complexity_gate.go` + 10 tests (diff fixtures, temp git repos: uncommitted/untracked/clean/non-repo).
- Gatekeeper: `complexity_gate` check (verify-only, skips non-git) + 4 tests; surfaces via existing `sdd-gatekeeper` CLI.
- Prevention: 15/20 budget line in `sdd-apply.md`.
- Live dogfood: clean change → `complexity_gate passed:true`; scratch violator (cyclo=21) → `passed:false` with offender reason; scratch removed.
- Suites: `sdd`, `readability`, `assets`, `doctor` green; `go vet` clean.

## Out of scope (kept)
Committed work units mid-change (R2/CI cover), `_test.go` blocking, VerifyReport schema change, git hooks.
