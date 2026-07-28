# Archive Report: core-protocol-and-model

**Archived**: 2026-07-27
**Mode**: Standard (strict_tdd: false)
**Artifact Store**: openspec

## Summary

Implemented the foundational core protocol and data model for biggz-ai. Delivered 3 new capabilities (core-review, plugin-system, cli) across 5 phases with 15 tasks, all completed:

- **Phase 1 — Foundation**: `go.mod`, `model/review.go`, `model/fsm.go`, `model/hash.go` — ReviewState with UUIDv7, 5-state FSM with pure transition validation, evidence chain with linked PrevHash and SHA-256 MerkleRoot
- **Phase 2 — Plugin Interfaces**: `plugin/interfaces.go`, `policy/evaluator.go`, `registry/registry.go` — LensPlugin, ProviderPlugin, Evaluator interfaces, build-time thread-safe Registry
- **Phase 3 — Pipeline + Orchestrator**: `pipeline/pipeline.go`, `orchestrator/orchestrator.go` — Stage/Pipeline with sequential execution and reverse-order rollback (including failed stage), Orchestrator wrapping pipeline and FSM
- **Phase 4 — CLI + Wiring**: `cmd/biggz/main.go`, `cmd/biggz/dummylens.go`, `cmd/biggz/mockprovider.go` — stdin JSON → orchestrator → stdout JSON
- **Phase 5 — Tests**: 27 tests across model, registry, and pipeline packages; all passing; CLI runtime harness confirmed with end-to-end integration test

## Spec Compliance

**Verdict**: PASS WITH WARNINGS
- 0 CRITICAL findings
- 13/30 scenarios compliant (1 partial)
- 16 untested scenarios across Schema Versioning, PolicyEvaluator, LensPlugin, ProviderPlugin, Orchestrator failure, and CLI error paths
- Build: clean (exit 0)
- Tests: all passing (exit 0)
- CLI runtime: valid JSON output with all expected fields (exit 0)

This is acceptable in Standard mode (strict_tdd: false). Test coverage gaps are documented and tracked for future work.

## Spec Sync

No delta specs existed in the change folder — all 3 specs were written directly to `openspec/specs/` as new capability specs (not deltas against existing specs). No spec sync was needed.

- `openspec/specs/core-review/spec.md` ✅
- `openspec/specs/plugin-system/spec.md` ✅
- `openspec/specs/cli/spec.md` ✅

## Archived Artifacts

All 6 SDD artifacts are preserved in `openspec/changes/archive/2026-07-27-core-protocol-and-model/`:

| Artifact | Path | Status |
|----------|------|--------|
| Proposal | `proposal.md` | ✅ |
| Exploration | `exploration.md` | ✅ |
| Design | `design.md` | ✅ |
| Tasks | `tasks.md` | ✅ (15/15 complete) |
| Apply Progress | `apply-progress.md` | ✅ |
| Verify Report | `verify-report.md` | ✅ |
| Archive Report | `archive-report.md` | ✅ (this file) |

## Task Completion Gate

All 15 implementation tasks are marked [x] in `tasks.md`. No stale unchecked tasks. ✓

## Deviations from Design

2 intentional deviations documented in apply-progress.md:
1. **Pipeline rolls back failed stage** — Implementation exceeds design by rolling back the failed stage's `RoleRun()` call before prior completed stages (matches the spec requirement).
2. **Orchestrator uses pure Transition** — Design implied mutation function; actual orchestrator uses pure `Transition()` validation + explicit status assignment (semantically equivalent).

## Next Recommendation

**Option A (preferred)**: New SDD change — **test-coverage-improvement** — to address the 16 untested scenarios across Schema Versioning, PolicyEvaluator, LensPlugin, ProviderPlugin, Orchestrator failure path, and CLI error paths. This would add:
- Unit tests for `orchestrator/`, `plugin/`, `policy/`, and `cmd/biggz/` packages
- Schema version comparison tests
- CLI invalid-input and error-exit tests
- Orchestrator pipeline-failure path test

**Option B**: New SDD change for the next feature slice — e.g., persistent storage, review result reporting, or webhook integration.

## Audit Trail

The SDD cycle for `core-protocol-and-model` is fully complete: proposed → explored → designed → specified → tasks → applied → verified → archived.
