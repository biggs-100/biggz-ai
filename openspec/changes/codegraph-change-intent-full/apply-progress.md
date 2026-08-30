# Apply Progress — codegraph-change-intent-full

## Status

**Change**: codegraph-change-intent-full  
**Mode**: Standard (strict_tdd: false)  
**Delivery strategy**: auto-chain / stacked-to-main  
**Current work unit**: PR1 Core Engine (work unit 1)  
**Branch**: codegraph-change-intent-full-pr1 -> main

## Completed Tasks

- [x] 1.1 `internal/codegraph/types.go` — Report/FileEntry/Graph/Node/Edge, Reason consts sdd/import/call, weights symbol 2 > keyword 1
- [x] 1.2 `internal/codegraph/intent.go` — ExtractIntent with regex symbol `[A-Z][a-zA-Z0-9_]*`, weighted sdd map, proposal required fail
- [x] 1.3 `internal/codegraph/scan.go` — ScanGo primary go/packages cached 30s timeout, fallback parser+ast.Inspect, Go-only filter
- [x] 2.1 `internal/codegraph/graph.go` — BuildGraph merge sdd+import+call, BFS transitive closure, isolated sdd nodes, flat-list guard
- [x] 2.2 `internal/codegraph/report.go` — Generate 30s ctx proposal required abort, Emit MkdirAll no partial
- [x] 2.3 `internal/codegraph/render.go` — RenderMarkdown files table + graph summary
- [x] 2.4 `internal/codegraph/hint.go` — LoadHint nil if absent, advisory only
- [x] 4.1 Unit temp fixtures: proposal-only extraction, proposal required fail, symbol>keyword weight, closure A->B->C=>A->C, flat-list guard, LoadHint nil
- [x] 4.2 Integration: go/packages+fallback import/call edges, 30s timeout no-partial, MkdirAll for nested custom paths

### Remaining Tasks (deferred to PR2)

- [ ] 3.1 cli_codegraph.go report verb + flags + help + resolveReportRoot
- [ ] 3.2 agentbuilder/sdd.go advisory LoadHint surface
- [ ] 4.3 CLI tests: help documents report, missing change/proposal, custom paths, defaults, init/guidance preserved
- [ ] 4.4 Orchestrator hint tests + manual dual emit verify
- [ ] 4.5 go test ./... + go vet pass
- [ ] 5.1 Verify codegraph.md deletable safe, JSON stdout+file dual output, no partial on timeout
- [ ] 5.2 Remove temp fixtures, confirm git-ignored-safe

## Files Changed (PR1)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/codegraph/types.go` | Created | Report/Files/Graph/Node/Edge, Reason consts, weights |
| `internal/codegraph/intent.go` | Created | ExtractIntent regex weighted sdd map, proposal required |
| `internal/codegraph/scan.go` | Created | ScanGo go/packages cached 30s + parser fallback, Go-only, dedup, cache |
| `internal/codegraph/graph.go` | Created | BuildGraph merge + BFS closure, isolated sdd kept, flat-list guard |
| `internal/codegraph/report.go` | Created | Generate 30s ctx, Emit MkdirAll no partial, token->files mapping |
| `internal/codegraph/render.go` | Created | RenderMarkdown files table + graph summary |
| `internal/codegraph/hint.go` | Created | LoadHint nil if absent, FormatHint |
| `internal/codegraph/codegraph_test.go` | Created | Unit+integration tests for PR1 (proposal, weight, closure, scan, emit, render) |
| `openspec/changes/codegraph-change-intent-full/tasks.md` | Modified | Marked Phase 1, Phase2, 4.1-4.2 complete |

## Work Unit Evidence — PR1 (Core Engine)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/codegraph -run TestExtractIntent\|TestGraph -count=1` — PASS (TestExtractIntent_ProposalOnly PASS, TestExtractIntent_MissingProposalFails PASS, TestExtractIntent_SymbolWeightExceedsKeyword PASS, TestGraph_TransitiveClosure PASS, TestGraph_FlatListGuard PASS) — 5 tests PASS, exit 0 |
| Focused test command (full package) | `go test ./internal/codegraph -count=1 -timeout 30s` — PASS, 12 tests PASS, exit 0, 2.386s |
| Runtime harness command/scenario and exact result | `go test ./internal/codegraph -count=1 -timeout 30s` — real integration harness via go/packages + fallback, PASS (12 tests, 2.386s) — also covers 30s timeout no-partial via cancelled context test |
| Rollback boundary | `internal/codegraph/*` only (types.go, intent.go, scan.go, graph.go, report.go, render.go, hint.go, codegraph_test.go); revert without CLI impact; does not touch `cmd/biggz/cli_codegraph.go`, `internal/agentbuilder/sdd.go` |

## Deviations from Design

None — implementation matches design.md. Weights, reasons, 30s timeout, fallback, Go-only filter, BFS closure, isolated sdd, flat-list guard, MkdirAll no partial all implemented as designed.

## Issues Found

None.

## Workload / PR Boundary

- Mode: stacked PR slice (PR1 of 2)
- Current work unit: Core engine `internal/codegraph/*` + unit tests
- Boundary: Starts at types.go, ends at hint.go + codegraph_test.go; verification included via focused + harness; rollback is `internal/codegraph/*` revert
- Estimated review budget impact: ~380 lines (PR1), within 400 budget; PR2 will be separate slice

## Testing Capabilities

- strict_tdd: false
- test_command: go test ./... -count=1 -timeout 180s
- layers: unit true, integration true, e2e true, coverage true, linter true, type_checker true, formatter true
