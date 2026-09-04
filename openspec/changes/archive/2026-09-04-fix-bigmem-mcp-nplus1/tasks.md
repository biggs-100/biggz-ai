# Tasks: Fix BigMem MCP N+1

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 320–380 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (Units 1→2→3 stack if split needed) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Store: `Offset` + `ListRelationsByIDs` + chunking | PR 1 | `go test ./internal/bigmem/ -run TestListRelationsByIDs -v` | N/A (unit-level, no live MCP needed) | `internal/bigmem/bigmem.go` additive only; revert leaves callers untouched |
| 2 | `mem_search`: ID-union rewrite + limit validation + stderr signal | PR 1 | `go test ./internal/bigmem/ -run TestMemSearchAnnotationBound -v` | `mem_search` with 50 results; verify ≤2 queries, zero Gets | `cmd/biggz-mcp/main.go` mem_search block only |
| 3 | Export paging + flags + round-trip + conflicts golden | PR 1 | `go test ./internal/bigmem/ -run TestExportPaging -v` | `biggz bigmem export out.json && biggz bigmem import out.json` | `cmd/biggz/cli_bigmem.go` export path only |

## Phase 1: Foundation — Store API (`internal/bigmem/bigmem.go`)

- [x] 1.1 Add `Offset int` to `SearchOptions`; append `OFFSET ?` only when >0 (verify old queries byte-identical)
- [x] 1.2 RED: add hostile-ID test to `internal/bigmem/batch_test.go` (e.g. `"x' OR '1'='1"`) expecting empty, no error
- [x] 1.3 Add `ListRelationsByIDs(ids)` with dedupe, empty→return empty without query, 400-ID chunks, no LIMIT, `ORDER BY created_at DESC`
- [x] 1.4 Unit test scoped lookup both endpoints + empty-input no-query in `batch_test.go`

## Phase 2: Core — `mem_search` (`cmd/biggz-mcp/main.go`)

- [x] 2.1 RED: write failing N+1 bound test (50 seeded results → ≤2 Store queries, zero `GetCtx`)
- [x] 2.2 Rewrite annotation: build ID union, single `ListRelationsByIDs`, in-memory title map with `"deleted"` fallback; drop per-rel `GetCtx`; errors never fail search
- [x] 2.3 Add limit validation: missing/non-numeric/≤0→20, >50→clamp 50 + stderr `limit clamped: requested=X effective=50`
- [x] 2.4 Test: cross-ID rel (A→Z, only A in results) annotates; missing title → `"deleted"`

## Phase 3: Integration — Export + CLI (`cmd/biggz/cli_bigmem.go`)

- [x] 3.1 Rewrite `export` to 50-page `Offset` loop until short page or `--limit` cap; add `--limit N` (0/omitted=uncapped, negative=error) + `--project P`
- [x] 3.2 Test: seed 120 obs → export returns 120; `--limit 60` → exactly 60; `--project P1` filters
- [x] 3.3 Round-trip test: paged `out.json` re-imports via `biggz bigmem import` with zero parse errors, shape unchanged
- [x] 3.4 Golden test: `conflicts list` output byte-identical before/after (keeps `ListRelations("")`)

## Phase 4: Verification

- [x] 4.1 Run `go test ./internal/bigmem/...` — all pass including `batch_test.go`
- [x] 4.2 Verify export >50 rows complete and `mem_search` ≤2 queries regardless of N
