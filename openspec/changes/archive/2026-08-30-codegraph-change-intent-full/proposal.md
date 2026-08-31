# Proposal: CodeGraph Full Change-Intent Graph

## Intent

Port gentle's CodeGraph change-intent oracle. Today `biggz codegraph init --cwd` only validates roots; biggz-ai cannot infer which files a change touches or its blast-radius. Add full graph (dependency + call-graph) from SDD artifacts + repo scan for human + orchestrator SDD scoping.

## Scope

### In Scope
- `biggz codegraph report <change> [--cwd] [--json <path>] [--md <path>]`
- Inference: `openspec/changes/{change}/` (proposal/spec/design/tasks) + repo scan (Go imports, symbols, call edges) — gentle parity
- Full graph: dependency + call-graph with transitive closure, not flat list
- Dual output: JSON `{files:[{path,reasons}], graph:{nodes,edges}}` + `openspec/changes/{change}/codegraph.md`
- Advisory consumption by human and orchestrator (visible, not CI-only)

### Out of Scope
- Upstream `codegraph` queries (`explore/query/impact`) — stays upstream CLI
- `.codegraph/` watcher/index persistence — unchanged
- Auto-scoping without human approval

## Capabilities

### New Capabilities
- `codegraph-change-intent`: SDD+code full graph inference with JSON + Markdown, consumed by human + orchestrator.

### Modified Capabilities
- `cli`: Add `codegraph report` verb, flags, help.
- `orchestrator`: Read report JSON to hint SDD scope (advisory).

## Approach

Extend `internal/codegraph` with intent engine: extract keywords/symbols from SDD artifacts, scan Go imports + call edges (`go/packages` or stdlib parse), build dependency + call graph, expand closure, emit JSON/MD. Wire `cmd/biggz/cli_codegraph.go:reportRun`. Adapt gentle oracle to `openspec/changes/{change}/` path. Orchestrator: optional pre-spec read if report exists.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/codegraph/` | Modified | Intent engine, graph builders, renderers |
| `cmd/biggz/cli_codegraph.go` | Modified | `report` verb + tests |
| `openspec/changes/{change}/codegraph.md` | New | Generated Markdown report |
| `internal/sdd/` / `internal/orchestrator/` | Modified | Optional scope hint consumer |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Scan cost large repo | Med | Go-only, cache, 30s timeout |
| False positives (keyword) | Med | Weight symbols > keywords; reason tags |
| Drift from gentle | Low | 1:1 port + graph tests |

## Rollback Plan

Revert `internal/codegraph` + `cli_codegraph.go` commit(s); delete generated `codegraph.md`. No migration. `init`/`guidance` unaffected.

## Dependencies

- `go/packages` or stdlib parser
- SDD artifacts present (`proposal.md` required)
- Gentle reference for parity (no runtime dep)

## Success Criteria

- [ ] `biggz codegraph report <change> --cwd .` emits JSON (`files[]`+`graph`) and `codegraph.md`
- [ ] Graph contains dependency + call edges with `reasons` (sdd/import/call)
- [ ] `biggz codegraph --help` documents `report`
- [ ] Orchestrator can read JSON for scope hint (manual verify)
- [ ] `go test ./...` + `go vet` pass; `init` unchanged
