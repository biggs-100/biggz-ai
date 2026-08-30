# Tasks: CodeGraph Full Change-Intent Graph

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~560 (types 40 + intent 80 + scan 100 + graph 80 + report 60 + render 40 + hint 20 + cli 40 + tests 100) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: core engine (types/intent/scan/graph/report/render/hint + unit) → PR 2: CLI + orchestrator hint + e2e |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Core engine `internal/codegraph/*` (types/intent/scan/graph/report/render/hint) + unit tests | PR 1 → main | `go test ./internal/codegraph -run TestExtractIntent\|TestGraph -count=1` | `go test ./internal/codegraph -count=1 -timeout 30s` | `internal/codegraph/*` only; revert without CLI impact |
| 2 | CLI `report` verb + orchestrator hint + integration/e2e | PR 2 → main | `go test ./cmd/biggz -run TestCodeGraph -count=1` | `biggz codegraph report <change> --cwd . --json /tmp/cg.json --md /tmp/cg.md` | `cmd/biggz/cli_codegraph.go`, `cmd/biggz/cli_codegraph_test.go`, `internal/agentbuilder/sdd.go` revert independently |

## Phase 1: Foundation

- [x] 1.1 Create `internal/codegraph/types.go` with `Report/FileEntry/Graph/Node/Edge`, `Reason` consts `sdd/import/call`, weights symbol 2 > keyword 1
- [x] 1.2 Create `internal/codegraph/intent.go` `ExtractIntent(change,cwd)` regex keyword+`[A-Z][a-zA-Z0-9_]*` symbol, weighted `sdd` map, fail `proposal required` if missing
- [x] 1.3 Create `internal/codegraph/scan.go` `ScanGo(cwd,ctx)` primary `go/packages` cached 30s timeout, fallback `parser`+`ast.Inspect`, Go-only `*.go` filter

## Phase 2: Core Graph

- [x] 2.1 Create `internal/codegraph/graph.go` `BuildGraph` merge sdd+import+call into `{nodes,edges}`, BFS transitive closure, keep isolated sdd nodes, guard flat-list
- [x] 2.2 Create `internal/codegraph/report.go` `Generate(change,cwd)` 30s ctx `proposal required` abort, `Emit(r,jsonPath,mdPath)` `MkdirAll` no partial on error/timeout
- [x] 2.3 Create `internal/codegraph/render.go` `RenderMarkdown` files table with reasons + graph summary
- [x] 2.4 Create `internal/codegraph/hint.go` `LoadHint(change,cwd)` nil if absent, no block, advisory only

## Phase 3: Integration / Wiring

- [ ] 3.1 Modify `cmd/biggz/cli_codegraph.go` add `report` case to router, flags `--cwd/--json/--md`, `reportRun` → `Generate`+`Emit`, help lists `report`, `resolveReportRoot` via `Abs`+`EvalSymlinks`
- [ ] 3.2 Modify `internal/agentbuilder/sdd.go` pre-spec advisory `LoadHint` surface `files` with reasons, no auto-mutate/block when absent

## Phase 4: Testing

- [x] 4.1 Unit `internal/codegraph` temp fixtures: proposal-only extraction, `proposal required` fail, symbol>keyword weight, closure `A->B->C⇒A->C`, flat-list guard, `LoadHint` nil
- [x] 4.2 Integration `internal/codegraph` `t.TempDir` repos: `go/packages`+fallback import/call edges, 30s timeout no-partial, `MkdirAll` for nested custom paths
- [ ] 4.3 CLI `cmd/biggz/cli_codegraph_test.go` help documents `report`, missing `<change>`/`proposal required` non-zero, custom paths exact, defaults `codegraph.json`+`codegraph.md`, `init`/`guidance` preserved
- [ ] 4.4 Orchestrator hint tests: report present surfaces files, absent continues, advisory does not auto-apply; manual `biggz codegraph report` dual emit verify
- [ ] 4.5 Run `go test ./... -count=1 -timeout 180s` + `go vet ./...` pass, no `init` regression

## Phase 5: Cleanup

- [ ] 5.1 Verify `codegraph.md` deletable safe, JSON stdout+file dual output, no partial on timeout
- [ ] 5.2 Remove temp fixtures, confirm `openspec/changes/{change}/codegraph.md` git-ignored-safe
