# Apply Progress: Organic Routing Parity

## Status: COMPLETE

## Changes Applied

### Task 1: Work Routing Ladder Rewrite
**File:** `internal/assets/biggz/biggz-orchestrator-delegation.md`
- Replaced 3-tier ladder (Inline Direct / Simple Delegation / SDD) with 3-route table (Direct inline / Delegated direct / Optional SDD)
- Added explicit rules: SIZE NEVER SELECTS SDD, Per-action delegation does not change route, Direct/delegated create zero SDD artifacts
- Added route selection thresholds: 1-3 files → direct, 4+ → delegated, ambiguous + request → SDD
- **Lines changed:** ~30 (replaced old ladder text)

### Task 2: Public States
**File:** `internal/assets/biggz/biggz-orchestrator-workflow.md`
- Added `## Public Implementation States` section with Working/Checking/Ready/Needs your decision table
- Added state transition diagram
- Documented that public states replace old synthesis markers
- **Lines added:** ~20

### Task 3: Route Field in ChangeStatus
**File:** `internal/sdd/status.go`
- Added `Route string` field to `ChangeStatus` struct (omitempty, backward compatible)
- Added `deriveRoute(cs *ChangeStatus) string` function: returns "sdd" if any SDD artifact exists, "organic" otherwise, "" if archived
- Wired `cs.Route = deriveRoute(cs)` in both `deriveChangeStatusWithForcedStoreCtx` and `deriveChangeStatusCtx`
- **Lines added:** ~20 (1 struct field + 1 function + 2 wiring calls)

### Task 4: Route Context in sdd-continue
**File:** `cmd/biggz/cli_sdd.go`
- Added route context output after "Next phase" in `runSddContinue`
- Shows "Route: organic (direct inline or delegated direct)" for organic work
- Shows "Route: sdd" for SDD work
- **Lines added:** ~15

### Task 5: Build and Smoke Test
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `sdd-status --json` — active change shows `"route":"sdd"`, archived show empty
- `sdd-continue` — shows route context correctly

## Test Evidence

- [x] `go build ./...` exits 0
- [x] `go vet ./...` exits 0
- [x] `sdd-status --json` output includes `"route"` field
- [x] `sdd-continue` shows route context
- [x] No regressions in existing SDD status output
- [x] Orchestrator guidance files contain routing thresholds and public states

## Files Changed

| File | Lines Added | Lines Removed | Net |
|------|-------------|---------------|-----|
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | ~30 | ~15 | +15 |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | ~20 | 0 | +20 |
| `internal/sdd/status.go` | ~20 | 0 | +20 |
| `cmd/biggz/cli_sdd.go` | ~15 | 0 | +15 |
| **Total** | ~85 | ~15 | **+70** |

## Risks

- **Low:** Route field is additive (omitempty), backward compatible
- **Low:** Prompt-only routing is reversible by reverting .md files
- **Low:** Public states replace synthesis markers, but synthesis gate is already passthrough since 2026-09-04

## Next: Verify
