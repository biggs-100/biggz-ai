# Design: Fix BigMem MCP N+1

## Technical Approach

Additive-only per proposal: one new scoped Store query plus call-site rewrites; `GetCtx`/`ListRelations` untouched. `mem_search` builds the ID union from results, issues a single `ListRelationsByIDs(ids)`, resolves titles from an in-memory map (`"deleted"` fallback) — zero hot-path `GetCtx`. Export pages `Search` at the legal 50-row cap with `OFFSET` until drained or the `--limit` cap hits. Covers `specs/bigmem/spec.md` (scoped lookup, ≤2-query bound, explicit limit signal) and `specs/cli/spec.md` (paged export, shape preservation, `conflicts` untouched).

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| `ListRelationsByIDs` vs `GetBatch`/`MGet` | `GetBatch` fixes hydration only; unscoped scan + LIMIT-50 truncation remain | `ListRelationsByIDs`: `WHERE source_id IN (…) OR target_id IN (…)` fixes discovery + hydration in one query |
| IN chunking: none vs chunked | Unchunked breaks past SQLite's 999-var limit; chunking adds a loop | Chunk at 400 IDs (800 vars); one chunk in practice (N≤50) |
| Scoped LIMIT: keep 50 vs none | LIMIT reintroduces the truncation being removed | No LIMIT, `ORDER BY created_at DESC`; set bounded by input IDs |
| Titles: lazy `GetCtx` vs map + `"deleted"` | Lazy Gets reintroduce N+1 | Map + `"deleted"`, zero hot-path Gets (spec-mandated) |
| Limit SIGNAL: response field vs stderr | Response field breaks the JSON-array contract; stderr has precedent (preview-truncation notice) | stderr (`limit clamped: requested=X effective=50`); array unchanged |
| Paging: `Offset` field vs cursor vs new method | Cursors duplicate rows on `updated_at` ties; new method duplicates SQL | Additive `SearchOptions.Offset`; `OFFSET ?` appended only when >0, so existing queries stay byte-identical |

## Data Flow

    SearchCtx ──→ results[N] ──→ ID union ──→ ListRelationsByIDs (1 query)
         │                                              │
         └──── titleByID map ──→ annotate in memory (no Gets)

    CLI ──→ Search("", {Limit: min(50, remaining), Offset, Project}) ──→ append
      │                         ▲                                       │
      └──── repeat until short page or cap ─────────────────────────────┘
                              │ write JSON array (shape unchanged)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modify | `ListRelationsByIDs` (dedupe, 400-chunk, empty→no query); `SearchOptions.Offset` honored only when >0 |
| `cmd/biggz-mcp/main.go` | Modify | `mem_search`: limit validate (missing/non-numeric/≤0→20, >50→clamp+stderr), ID-union + 1 scoped query, drop per-rel `GetCtx`; annotation errors never fail search |
| `cmd/biggz/cli_bigmem.go` | Modify | `export`: `--limit N` (0/omitted=uncapped, negative=error), `--project P`, 50-page `Offset` loop, same array shape |
| `internal/bigmem/batch_test.go` | Create | Scoped lookup, empty input, chunking, limit semantics, export paging, round-trip |

## Interfaces / Contracts

```go
func (s *Store) ListRelationsByIDs(ids []string) ([]Relation, error)

type SearchOptions struct {
    Offset int `json:"offset,omitempty"` // 0 = legacy behavior
}
```

```sql
-- per 400-ID chunk, placeholders only:
SELECT <same columns as ListRelations> FROM memory_relations
WHERE source_id IN (?, …) OR target_id IN (?, …)
ORDER BY created_at DESC
```

CLI: `biggz bigmem export [file] [--limit N] [--project P]`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Scoped both-endpoint lookup; empty→no query; hostile IDs inert | Table-driven test, seeded SQLite |
| Unit | Limits: 0/neg→20, >50→50+stderr, non-numeric→20 | Handler-level test |
| Integration | 50 results fully annotated; missing→`"deleted"`; 2 Store queries total | Seeded store; bound by construction (one scoped call, zero Gets) |
| Integration | Export 120→120; `--limit 60`→60; `--project` filters; import round-trips | Paging + CLI round-trip test |
| Regression | `conflicts list` output identical | Golden-output test |

## Threat Matrix

`references/threat-matrix.md` absent; assessed directly. R/O plus pre-existing export file write. No new attack surface.

| Boundary | Verdict | Behavior / RED test |
|----------|---------|---------------------|
| SQL IN-clause | Applicable | Bound placeholders only, chunked; hostile-ID test returns empty |
| Export file write | Applicable, pre-existing unchanged | Same `argv` path, `0644`, shape; round-trip test |
| Shell / subprocess | N/A | No `exec` added |
| MCP routing | N/A | No new/renamed tools; envelope unchanged |
| VCS / PR automation | N/A | Untouched |
| Executable classification | N/A | None |
| Process integration (stdio) | N/A | Signal via stderr only |

## Migration / Rollout

No migration required (additive API + call-site only; export shape unchanged). Rollback: revert commits.

## Open Questions

None blocking — SIGNAL (stderr), chunking (400), paging (`Offset`) all decided; ready for tasks.
