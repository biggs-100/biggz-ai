# Design: CodeGraph Full Change-Intent Graph

## Technical Approach

Port gentle oracle to `openspec/changes/{change}/` in `internal/codegraph`: weighted `sdd` from `proposal.md` (required) + optional specs, Go scan via `go/packages` (fallback `parser`/`ast`), merged `{nodes,edges}` with BFS closure, dual JSON+MD via `cli_codegraph.go:reportRun`. Advisory hint only. Covers `codegraph-change-intent`×5, `cli`, `orchestrator`.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| Intent extraction | Regex vs NLP | NLP heavy | **Regex keyword+Symbol(`[A-Z][a-zA-Z0-9_]*`), w symbol 2>1, `sdd` reason, `proposal required` fail** |
| Go scan | `go/packages` vs stdlib vs hybrid | packages w/o `go list` fails; parse misses types | **Primary `go/packages` (cached, 30s timeout), fallback `parser`+`ast.Inspect`; Go-only** |
| Graph model | Flat vs closure | flat loses blast radius | **Unified `{nodes,edges}` `reason∈{sdd,import,call}`, BFS closure, isolated `sdd` kept** |
| Dual output | Stdout/file/both | stdout-only not persistent | **JSON stdout+file (default `codegraph.json`, `--json` override), MD `codegraph.md` (`--md` override), `MkdirAll`, no partial** |
| Orchestrator hook | Auto-scope vs hint | auto-scope violates advisory | **`LoadHint` in `hint.go`; `agentbuilder/sdd.go` surfaces hint, no auto-mutate** |

## Data Flow

```
proposal/spec/design/tasks ──parse keywords/symbols (weighted sdd)──┐
                                                                   ├─► graph.Build (merge sdd+import+call) ──► transitive closure (BFS) ──► Report{files[], graph{nodes,edges}}
Go sources ──go/packages (30s ctx, cached) / parser fallback──import+call edges──┘                                                          │
                                                                                                                                        ├─► JSON files+graph (stdout + --json path)
                                                                                                                                        └─► Markdown codegraph.md (--md path) ──► human + orchestrator hint
CLI: biggz codegraph report <change> [--cwd][--json][--md] ──switch router──► reportRun ──► codegraph.Report()
```

Abort on missing `proposal.md` (`proposal required`, exit 1) or 30s timeout (no partial).

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/codegraph/types.go` | Create | `Report/Files/Graph/Node/Edge`, `Reason` consts `sdd/import/call`, weights |
| `internal/codegraph/intent.go` | Create | `ExtractIntent(change,cwd)` — tokenize artifacts, weighted `sdd` map |
| `internal/codegraph/scan.go` | Create | `ScanGo(cwd,ctx)` — `packages.Load` + fallback parser, import/call edges |
| `internal/codegraph/graph.go` | Create | `BuildGraph` — merge + BFS closure, isolated `sdd` nodes, guard flat-list |
| `internal/codegraph/report.go` | Create | `Generate` (30s) + `Emit` (`MkdirAll`, no partial) |
| `internal/codegraph/render.go` | Create | `RenderMarkdown` — files table + graph summary |
| `internal/codegraph/hint.go` | Create | `LoadHint` — optional JSON read, nil if absent |
| `cmd/biggz/cli_codegraph.go` | Modify | `report` switch case, `--cwd/--json/--md` flags, `reportRun`, help |
| `cmd/biggz/cli_codegraph_test.go` | Modify | Help, missing change/proposal, custom paths, defaults |
| `internal/agentbuilder/sdd.go` | Modify | Pre-spec advisory surface, no auto-mutate |

## Interfaces / Contracts

```go
// internal/codegraph/types.go
type Reason string
const ReasonSDD Reason = "sdd"; const ReasonImport Reason = "import"; const ReasonCall Reason = "call"
type FileEntry struct { Path string `json:"path"`; Reasons []Reason `json:"reasons"` }
type Node struct { ID string `json:"id"`; Path string `json:"path"`; Reasons []Reason `json:"reasons"` }
type Edge struct { From string `json:"from"`; To string `json:"to"`; Reason Reason `json:"reason"` }
type Graph struct { Nodes []Node `json:"nodes"`; Edges []Edge `json:"edges"` }
type Report struct { Files []FileEntry `json:"files"`; Graph Graph `json:"graph"` }
func Generate(change, cwd string) (*Report, error) // 30s ctx, proposal required
func Emit(r *Report, jsonPath, mdPath string) error
func LoadHint(change, cwd string) (*Report, error) // advisory, nil if absent
```

```go
func reportRun(args []string) int // <change> --cwd --json --md → Generate+Emit
```
CLI `report <change> [--cwd .] [--json path] [--md path]`; help lists `report`. Invariant: `files` ⇒ `nodes/edges` non-empty; isolated `sdd` as node.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Weight `symbol>keyword`, `proposal required`, closure `A->B->C⇒A->C`, flat-list guard, `LoadHint` nil | `go test ./internal/codegraph` with temp fixtures |
| Integration | `go/packages`+fallback, 30s no-partial, `MkdirAll`, help | `t.TempDir` repos, `WithTimeout` injection |
| E2E | `report <change> --cwd .` dual emit, custom paths, orchestrator hint | Manual CLI + `go test -run TestCodeGraph_Report` |

## Threat Matrix

`go/packages` spawns `go list -json` with `context` timeout, cwd-scoped, no shell interpolation. No VCS/PR mutation.

| Boundary | Applicability | Design response | Planned RED tests |
|----------|---------------|-----------------|-------------------|
| Documentation-like paths | N/A — Go-only; docs never nodes | filter `*.go` | doc not in `files[]` |
| Git repository selection | N/A — no `git -C`; `--cwd` via `Abs`+`EvalSymlinks` | `resolveReportRoot` | traversal rejected |
| Commit state | N/A — read-only advisory | no write | — |
| Push state | N/A — no push | — | — |
| PR commands | N/A — no `gh pr` | — | — |

## Migration / Rollout

No migration. Additive; `init`/`guidance` unchanged. Rollback: revert `internal/codegraph/*`+`cli_codegraph.go`+`agentbuilder/sdd.go`; delete `codegraph.md` (safe, absence never blocks SDD).

## Open Questions

- [ ] JSON default: propose stdout always + file `codegraph.json` unless `--json` overrides (satisfies dual-output spec)
- [ ] Fallback `call` without types — emit `call` or `call:fallback`?
- [ ] Hint location `agentbuilder/sdd.go` vs `sdd/status.go` — prefer agentbuilder
```

