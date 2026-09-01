# Design: fix-bigmem-recall-recency

## Technical Approach

A+C: keep FTS `ORDER BY rank` (:1844). Add `Recent(opts)` wrapper → `Search("", opts)` → `:1801` `ORDER BY o.updated_at DESC`. CLI `biggz recall` (primary) + alias `biggz bigmem recent` share handler with `--type/--project/--scope/--limit/--json` cap 50. Harden Session Boot Recall gate + guardrail literal via `bigmem-protocol.md`+`install.go`. No `--order`/SQL/schema. Covers REQ-RR1/RR2, RR1-CLI, RR3/RR4, RR5.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| **Helper** A wrapper vs B new SQL vs C mutate FTS | B drift risk; C breaks relevance | **A** `internal/bigmem/recall.go: Recent` → `Search("",opts)`, preserves 1801/1844 |
| **CLI** A `recall`+`recent` vs B only `recent` | B hides intent; A covers recall + grouped bigmem | **A** primary `recall` in `main.go`, alias `recent` in `cli_bigmem.go`, shared `recallRun` |
| **Guardrail** A `bigmem-protocol.md`+`install.go` vs B workflow-only | B bypassable; A reaches all agents via marker | **A** single source `<!-- biggz:bigmem-protocol -->` |

## Data Flow

```
recall/recent --flags--> recallRun --Search("",opts)--> bigmem.go:1801 ORDER BY updated_at DESC --> cap50+filter --> JSON/human
search --query "session" --------------------------------> bigmem.go:1844 ORDER BY rank (BM25) ---------------^
Session Boot Recall: biggz_mem_context(5) -> Recent(5) -> fallback git log -15 + sdd-status --json
```

```mermaid
flowchart TD
  Q{query?} -->|""| R[ORDER BY updated_at DESC @1801]
  Q -->|"session"| K[ORDER BY rank @1844]
  R --> C[cap 50 + filters]
  K --> C
  C --> J{--json?}
  J -->|yes| JSON[JSON]
  J -->|no| TBL[lines]
```

FTS/LIKE fallback untouched.

## File Changes

| File | Action | Description |
|---|---|---|
| `cmd/biggz/main.go` | Modify | Add `case "recall"` → `recallRun()` + help |
| `cmd/biggz/cli_bigmem.go` | Modify | Add `case "recent"` + shared handler; flag parity with `search`; cap 50; help recency note |
| `internal/bigmem/recall.go` | Create | `Recent(opts) ([]*Observation, error) { return s.Search("", opts) }` — no SQL |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | Gate: mandate `mem_context(5)`/`Recent`/`Search("",…)` for latest; ban FTS for latest; fallback `git log --oneline -15` + `sdd-status --json` |
| `internal/assets/biggz/bigmem-protocol.md` | Modify | WHEN TO SEARCH guardrail literal: `For recency use bigmem search --query "" ORDER BY updated_at DESC or biggz recall; never use FTS term search for 'latest'.` |
| `internal/install/install.go` | Modify | `DeployBigMemProtocol` injects literal via `<!-- biggz:bigmem-protocol -->`, preserved on reinstall |
| `docs/architecture.md` | Modify | Add Rank vs Recency table (query / ORDER BY / when / example) |

## Interfaces / Contracts

```go
// internal/bigmem/recall.go
func (s *Store) Recent(opts SearchOptions) ([]*Observation, error)
// SearchOptions unchanged: Project, Type, Scope, Limit, MatchMode, AllProjects, BM25Floor
```

CLI: `biggz recall [--type T] [--project P] [--scope S] [--limit N] [--json] [--all]` alias `biggz bigmem recent` same flags. `--limit` clamped 50. `--json` → `[]Observation`; human → `id [type] title (updated_at)`. `--help` contains `recency uses empty query ordered by updated_at DESC`. Invariant: 1801 `ORDER BY o.updated_at DESC`, 1844 `ORDER BY rank`.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Recent 2026-09-01 before 2026-08-27; cap 50; forwarding | Temp DB two obs assert order; `--limit 100`→50; `go test ./internal/bigmem -run Recent` |
| Integration | Both entrypoints; flag parity; help | `cli_bigmem_test.go` `TestRecall*` recall vs recent; help asserts |
| E2E | Gate not stale; fallback; literal after install | Seed obs gate sim; empty → git log/sdd-status; `grep -F "For recency use"` |

## Threat Matrix

Custom:

| Threat | Applicability | Mitigation | RED test |
|---|---|---|---|
| Prompt bypass (uses FTS for latest) | Applicable | Hard gate: Recent/Search("") first; fallback git log + sdd-status | Seed stale+fresh, assert fresh first |
| Limit bypass (--limit 100) | Applicable | Cap 50 in Search + handler | `--limit 100` → len ≤50 |

Generic `references/threat-matrix.md`:

| Boundary | Applicable | Reason | RED test |
|---|---|---|---|
| Documentation-like paths | N/A | No executable doc classification | — |
| Git repository selection | Applicable | Fallback git log must respect cwd/commonDir | Worktree vs commonDir lineage |
| Commit state | N/A | No commit automation | — |
| Push state | N/A | No push | — |
| PR commands | N/A | No PR automation | — |

No new shell injection; unknown flags error, `--query` sentinel for dash-prefixed queries.

## Migration / Rollout

No migration/flag/DB. Rollout: merge → `biggz install`. Rollback: `git revert` + `install` + `go test ./...`. Success: recall fresh first; guardrail present; `go test ./internal/bigmem` + `assets/biggz` pass; `search "session"` still rank.

## Open Questions

- [ ] Primary `recall` vs `recent` swap trivial if UX prefers inverse.
- [ ] Fallback count 15 per spec.
