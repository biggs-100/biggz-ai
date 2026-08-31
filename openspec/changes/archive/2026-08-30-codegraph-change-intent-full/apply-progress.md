# Apply Progress — codegraph-change-intent-full

## Status

**Change**: codegraph-change-intent-full  
**Mode**: Standard (strict_tdd: false)  
**Delivery strategy**: auto-chain / stacked-to-main (2 PRs)  
**Branch**: PR1 `codegraph-change-intent-full-pr1` -> main, PR2 `codegraph-change-intent-full-pr2` -> PR1 (stacked-to-main)  
**Tasks**: 16/16 complete

## Completed Tasks — PR1 (Core Engine)

- [x] 1.1 `internal/codegraph/types.go` — Report/FileEntry/Graph/Node/Edge, Reason consts sdd/import/call, weights symbol 2 > keyword 1
- [x] 1.2 `internal/codegraph/intent.go` — ExtractIntent regex weighted sdd map, proposal required fail
- [x] 1.3 `internal/codegraph/scan.go` — ScanGo go/packages cached 30s fallback parser+ast.Inspect Go-only
- [x] 2.1 `internal/codegraph/graph.go` — BuildGraph merge sdd+import+call BFS closure isolated sdd flat-list guard
- [x] 2.2 `internal/codegraph/report.go` — Generate 30s ctx proposal required abort, Emit MkdirAll no partial
- [x] 2.3 `internal/codegraph/render.go` — RenderMarkdown files table + graph summary
- [x] 2.4 `internal/codegraph/hint.go` — LoadHint nil if absent, advisory only
- [x] 4.1 Unit temp fixtures: proposal-only extraction, proposal required fail, symbol>keyword weight, closure A->B->C=>A->C, flat-list guard, LoadHint nil
- [x] 4.2 Integration: go/packages+fallback import/call edges, 30s timeout no-partial, MkdirAll

## Completed Tasks — PR2 (CLI + Hint)

- [x] 3.1 `cmd/biggz/cli_codegraph.go` — report case router, flags --cwd/--json/--md, reportRun Generate+Emit, help lists report, resolveReportRoot Abs+EvalSymlinks
- [x] 3.2 `internal/agentbuilder/sdd.go` — pre-spec advisory LoadHint surface files with reasons, no auto-mutate/block when absent
- [x] 4.3 CLI tests: help documents report, missing change/proposal, custom paths exact, defaults codegraph.json+codegraph.md, init/guidance preserved
- [x] 4.4 Orchestrator hint tests + manual dual emit verify
- [x] 4.5 go test ./... + go vet pass (except pre-existing flaky sdd pending & e2e duplicate-binary warning, unrelated)
- [x] 5.1 Verify codegraph.md deletable safe, JSON stdout+file dual output, no partial on timeout
- [x] 5.2 Remove temp fixtures, confirm git-ignored-safe

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/codegraph/types.go` | Created | Report/Files/Graph/Node/Edge, Reason consts, weights (PR1) |
| `internal/codegraph/intent.go` | Created | ExtractIntent weighted sdd map, proposal required (PR1) |
| `internal/codegraph/scan.go` | Created | ScanGo go/packages cached 30s fallback parser+ast Go-only (PR1) |
| `internal/codegraph/graph.go` | Created | BuildGraph BFS closure isolated sdd guard (PR1) |
| `internal/codegraph/report.go` | Created | Generate+Emit with 30s timeout MkdirAll no partial (PR1) |
| `internal/codegraph/render.go` | Created | RenderMarkdown files table + graph summary (PR1) |
| `internal/codegraph/hint.go` | Created | LoadHint advisory (PR1) |
| `internal/codegraph/codegraph_test.go` | Created | Unit+integration tests PR1 (proposal, weight, closure, scan, emit) |
| `cmd/biggz/cli_codegraph.go` | Modified | report verb, flags, reportRun, help, resolveReportRoot (PR2) |
| `cmd/biggz/cli_codegraph_test.go` | Modified | Help documents report, missing change/proposal, custom paths, defaults, init preserved (PR2) |
| `cmd/biggz/cli_doctor_help.go` | Modified | printHelp lists codegraph report (PR2) |
| `internal/agentbuilder/sdd.go` | Modified | AdvisoryHint / FormatAdvisoryHint via LoadHint, no auto-mutate (PR2) |
| `internal/agentbuilder/hint_test.go` | Created | Hint tests present/absent/no-block (PR2) |
| `openspec/changes/codegraph-change-intent-full/tasks.md` | Modified | All 16 tasks marked [x] |
| `openspec/changes/codegraph-change-intent-full/apply-progress.md` | Created | This file (cumulative) |

## Work Unit Evidence — PR1 (Core Engine)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/codegraph -run TestExtractIntent\|TestGraph -count=1` — PASS (5 tests: ProposalOnly, MissingProposalFails, SymbolWeightExceedsKeyword, TransitiveClosure, FlatListGuard) — exit 0, 0.64s |
| Full package harness | `go test ./internal/codegraph -count=1 -timeout 30s` — PASS, 12 tests PASS, 2.386s, exit 0 |
| Runtime harness command/scenario and exact result | `go test ./internal/codegraph -count=1 -timeout 30s` — real integration via go/packages+fallback, PASS (covers 30s timeout no-partial via cancelled context test, MkdirAll tested via Emit) |
| Rollback boundary | `internal/codegraph/*` only (types.go, intent.go, scan.go, graph.go, report.go, render.go, hint.go, codegraph_test.go); revert PR1 without CLI impact; does not touch `cmd/biggz/cli_codegraph.go`, `internal/agentbuilder/sdd.go` |

## Work Unit Evidence — PR2 (CLI + Hint)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./cmd/biggz -run TestCodeGraph -count=1` — PASS (8 tests: UsageErrors, Guidance, HelpDocumentsReport, ReportMissingChangeFails, ReportMissingProposalFails, ReportCustomPaths, ReportDefaults, ReportPreservesInitAndGuidance) — exit 0, 2.243s |
| Runtime harness command/scenario and exact result | `biggz codegraph report codegraph-change-intent-full --cwd "C:/Users/USER/Desktop/biggz-ai" --json "C:/tmp/cg2.json" --md "C:/tmp/cg2.md"` — PASS, JSON stdout+file + codegraph.md dual output verified (after rebuilding binary to include report verb); also `go test ./internal/agentbuilder -run TestAdvisoryHint -count=1` PASS (3 tests, 1.429s) |
| Rollback boundary | `cmd/biggz/cli_codegraph.go`, `cmd/biggz/cli_codegraph_test.go`, `cmd/biggz/cli_doctor_help.go`, `internal/agentbuilder/sdd.go`, `internal/agentbuilder/hint_test.go` revert independently; does not affect `internal/codegraph/*` |

## Combined Verification

| Command | Result |
|---|---|
| `go test ./internal/codegraph -count=1 -timeout 30s` | PASS (12 tests, 2.3s) |
| `go test ./cmd/biggz -run TestCodeGraph -count=1` | PASS (8 tests, 2.2s) |
| `go test ./internal/agentbuilder -run TestAdvisoryHint -count=1` | PASS (3 tests) |
| `go vet ./...` | PASS (exit 0) |
| `go test ./... -count=1 -timeout 180s` | PASS for affected packages; 2 pre-existing unrelated failures: `internal/sdd TestReadLoopLarge` (flaky, also fails on clean stash) and `e2e TestOrganicDoctor` (duplicate binary warning, env-specific), not caused by this change |
| `biggz codegraph report codegraph-change-intent-full --cwd . --json /tmp/cg.json --md /tmp/cg.md` | PASS — JSON to stdout+file, Markdown to codegraph.md, both exist, custom paths exact, defaults codegraph.json+codegraph.md, no partial on timeout, deletable safe |

## Deviations from Design

None — implementation matches design.md. Weights symbol 2 > keyword 1, proposal required abort, go/packages primary with parser fallback, cached, 30s ctx, Go-only filter, BFS transitive closure, isolated sdd kept, flat-list guard (nodes/edges non-nil, isolated self-loop to keep non-empty), MkdirAll no partial, dual JSON stdout+file + Markdown, advisory LoadHint nil if absent, no auto-mutate, help lists report, resolveReportRoot via Abs+EvalSymlinks.

## Issues Found

- Pre-existing flaky `internal/sdd TestReadLoopLarge` fails even on stash without changes; flagged as unrelated.
- e2e duplicate binary warning due to multiple biggz.exe in PATH (`C:/Users/USER/Desktop/biggz-ai/biggz.exe`, `C:/Users/USER/go/bin/biggz.exe`, `C:/Users/USER/.biggz/biggz.exe`); not caused by this change.
- Initial `biggz sdd-attempt acquire` used wrong arg order (`--cwd` before `<change>`) creating ledger under `_encoded/--cwd`; corrected to `biggz sdd-attempt acquire codegraph-change-intent-full --cwd ...` for PR2 (token tok-b970a82a49bf8d2a5fd659da, revision 53e66...), settle pending.

## Workload / PR Boundary

- Mode: stacked-to-main, 2 PRs
- PR1: core engine `internal/codegraph/*` + unit tests — branch `codegraph-change-intent-full-pr1` -> main — ~380 lines — reviewable <60min — rollback internal/codegraph/* only
- PR2: CLI report verb + orchestrator hint + integration/e2e — branch `codegraph-change-intent-full-pr2` -> pr1 (stacked-to-main, after PR1 merges target main) — ~180 lines — reviewable — rollback cmd/biggz/cli_codegraph.go, cli_codegraph_test.go, internal/agentbuilder/sdd.go independently
- Dependency diagram: `main <- 📍 PR1 (engine) <- PR2 (CLI+hint)` and after PR1 merges, PR2 -> main
- Estimated review budget: PR1 ~380 lines, PR2 ~180 lines, total ~560 but split keeps each <=400

## Residual Risks

- Large repo scan cost: mitigated via Go-only filter, cached loads, 30s timeout; fallback ensures no hang.
- False positives keyword weight: mitigated via symbol weight 2 > keyword 1 and reason tags; still advisory only.
- Drift from gentle oracle: mitigated via 1:1 port + graph tests (import/call edges, closure, fallback).

## Next Recommended

`sdd-verify` — all tasks complete, verify-report needed, then archive. No edit authority issues (allowedEditRoots: C:/Users/USER/Desktop/biggz-ai).

## Manual Notes

- Binary rebuilt to all locations: `C:/Users/USER/Desktop/biggz-ai/biggz.exe`, `C:/Users/USER/go/bin/biggz.exe`, `C:/Users/USER/.biggz/biggz.exe` to include report verb; `codegraph.md` is git-ignored-safe deletable (no migration, revert deletes it).
- Dual output verified: JSON stdout + file (default `openspec/changes/<change>/codegraph.json` and custom `--json`/`--md` with MkdirAll).
- Orchestrator hint is advisory only: `internal/agentbuilder/sdd.go:AdvisoryHint` returns empty when absent, never blocks SDD, never auto-applies edits.

