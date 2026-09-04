# Archive Report: Organic Routing Parity

## Summary

Ported gentle-ai organic routing into biggz-ai orchestrator discipline. Added three implementation routes (Direct inline / Delegated direct / Optional SDD), public states (Working/Checking/Ready/Needs your decision), and route field in status/continue output.

## Artifacts Synced

| Artifact | Source | Destination | Status |
|---|---|---|---|
| Spec delta | `changes/organic-routing-parity/specs/orchestrator/spec.md` | `openspec/specs/orchestrator/spec.md` (merged) | ✅ Synced |
| Proposal | `changes/organic-routing-parity/proposal.md` | Archived with change | ✅ |
| Design | `changes/organic-routing-parity/design.md` | Archived with change | ✅ |
| Tasks | `changes/organic-routing-parity/tasks.md` | Archived with change | ✅ |
| Apply Progress | `changes/organic-routing-parity/apply-progress.md` | Archived with change | ✅ |
| Verify Report | `changes/organic-routing-parity/verify-report.md` | Archived with change | ✅ |

## Changes Summary

| File | Change | Lines |
|---|---|---|
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Routing ladder rewrite (1-3/4+/SDD) | +15 net |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Public states section | +20 |
| `internal/sdd/status.go` | Route field + deriveRoute() | +20 |
| `cmd/biggz/cli_sdd.go` | Route context in sdd-continue | +15 |
| `openspec/specs/orchestrator/spec.md` | REQ-OR-001 through REQ-OR-006 merged | +180 |
| **Total** | | **+250** |

## Verification

- [x] `go build ./...` PASS
- [x] `go vet ./...` PASS
- [x] `go test ./internal/sdd/...` PASS
- [x] `sdd-status --json` shows route field
- [x] `sdd-continue` shows route context
- [x] Spec synced to canonical location
- [x] Pre-existing test failure (RDD receipt) unrelated

## Risk

Low. Additive fields, prompt-only routing, fully reversible.

## Commits

To be committed as:
```
feat(sdd): organic routing parity with direct/delegated/optional SDD

- Add 3-route routing ladder (Direct inline / Delegated direct / Optional SDD)
- Add public states (Working/Checking/Ready/Needs your decision)
- Add Route field to ChangeStatus and sdd-continue output
- SIZE NEVER SELECTS SDD enforced
- REQ-OR-001 through REQ-OR-006 in orchestrator spec
```
