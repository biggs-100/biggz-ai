# Apply Progress: fix-bigmem-mcp-nplus1

**Mode**: Standard (Strict TDD false)
**Runner**: `go test -count=1 -timeout 180s`
**Delivery**: single PR, chain `stacked-to-main`
**Date**: 2026-09-04

## Skill resolution

- Read `internal/assets/biggz/biggz-orchestrator-workflow.md` before work: yes (SDD workflow, dispatcher, gates, ledger).
- Read `internal/assets/biggz/biggz-orchestrator-delegation.md` before work: yes (routing ladder, delegation rules, edit surfaces, lossless prompts).
- Review Workload Forecast in tasks.md: 320–380 lines, risk Medium, chained PRs No, decision needed No → proceeded as assigned single PR.

## Task status (14/14)

### Phase 1: Foundation — Store API

- [x] 1.1 `Offset int` added to `SearchOptions`; `OFFSET ?` appended only when >0 via `appendOffset` helper (topic-key, empty-query, FTS, LIKE fallback) — Offset=0 queries byte-identical (covered by `TestSearchOffsetByteIdentical`).
- [x] 1.2 Hostile-ID test added (`TestListRelationsByIDs_HostileIDs`): `x' OR '1'='1`, `DROP TABLE`, `%`, `" OR ""="` → empty, no error, table intact.
- [x] 1.3 `ListRelationsByIDs(ids)` added: dedupe, empty→nil without query, 400-ID chunks (800 vars < 999 limit), no LIMIT, `ORDER BY created_at DESC`, bound placeholders only.
- [x] 1.4 Scoped both-endpoint + empty-input tests pass (`TestListRelationsByIDs_ScopedBothEndpoints`, `TestListRelationsByIDs_EmptyInput`).

### Phase 2: Core — mem_search

- [x] 2.1 Bound test added (`TestMemSearchAnnotationBound` + `TestMemSearchCrossIDFallback`): 50 results → 1 Search + 1 scoped lookup, zero Gets by construction; cross-ID + `"deleted"` fallback covered.
- [x] 2.2 Annotation rewritten in `cmd/biggz-mcp/main.go`: ID union → single `ListRelationsByIDs`, in-memory title map + `"deleted"` fallback, all `GetCtx` call sites dropped from block; `ListRelations("")` unscoped scan removed; errors never fail search.
- [x] 2.3 Limit validation via `parseSearchLimit`: missing/non-numeric/≤0→20, >50→clamp 50 + stderr `limit clamped: requested=X effective=50`; tool description updated (default 20, max 50).
- [x] 2.4 Cross-ID + missing-title fallback covered at store level (FK=ON prevents truly dangling relations; MCP fallback is defensive and now the only resolution path).

### Phase 3: Integration — Export + CLI

- [x] 3.1 `export` rewritten: 50-page `Offset` loop until short page or cap; `--limit N` (0/omitted=uncapped, negative=error exit 1, non-integer=error); `--project P`; same JSON array shape (nil→`[]` guard).
- [x] 3.2 CLI tests: 70 rows → 70; `--limit 60` → 60; `--project exp2` → 15/15 filtered.
- [x] 3.3 Round-trip: paged `out.json` imports `70/70` into a fresh store, zero parse errors; store-level shape test also passes.
- [x] 3.4 `conflicts list` untouched (still `ListRelations("")`); stability test (two runs byte-identical, exit 0) passes.

### Phase 4: Verification

- [x] 4.1 `go test ./internal/bigmem/ -count=1` → ok (27.6s, full suite). `go test ./cmd/biggz-mcp/` → ok. `go test ./cmd/biggz/` → 1 pre-existing failure (`TestSDDStatusJSONEnvelopeDerivesStructuredFields`), reproduced on clean master via `git stash -u`, unrelated to this change.
- [x] 4.2 Export >50 complete (70/70 CLI test); mem_search ≤2 queries regardless of N by construction (single `ListRelationsByIDs` call site, zero `GetCtx` in block — verified with `rg`).

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test commands | `go test ./internal/bigmem/ -run 'TestListRelationsByIDs\|TestMemSearch\|TestSearchLimitSemantics\|TestExportPaging\|TestExportRoundTrip\|TestSearchOffsetByteIdentical' -count=1` → ok; `go test ./cmd/biggz-mcp/ -run TestParseSearchLimit` → ok (13/13); `go test ./cmd/biggz/ -run 'TestExport\|TestConflictsListStable'` → ok (5/5) |
| Full suites | `go test ./internal/bigmem/ -count=1 -timeout 180s` → ok; `go test ./cmd/biggz-mcp/ -count=1` → ok; `go test ./cmd/biggz/ -count=1` → 1 pre-existing FAIL (also fails on clean master) |
| Runtime harness | CLI: seed 70 via `save`, `export out.json` → 70 rows, `import` → 70/70 (in-process harness via new CLI tests). MCP `mem_search` live harness N/A (no live MCP server in scope); bound verified structurally (one scoped call site, `rg GetCtx` shows no hot-path Gets) + store-level bound test |
| Rollback boundary | `internal/bigmem/bigmem.go` (additive: Offset field, helper, new method — revert safe); `cmd/biggz-mcp/main.go` mem_search block only; `cmd/biggz/cli_bigmem.go` export case only; 3 new test files (delete to revert) |

## Test findings (fixed during apply)

1. `memory_relations` has FK to `observations(id)` with `PRAGMA foreign_keys=ON` — relation seeds must reference existing observations; chunk test uses a star pattern (hub + 410 leaves).
2. `rel-%d/UnixNano` IDs collide in tight seed loops (`ON CONFLICT DO NOTHING` + `JudgeRelation` "already judged") — test helper uses direct INSERT with deterministic IDs.
3. `Store.Save` dedupes on normalized hash — bulk seeds need unique content per row.
4. Cross-call seed ID collisions (`batch-obs-%04d` reused across projects upserts) — IDs now prefixed per project.
5. `GetCtx` does not filter soft-deleted rows, so the `"deleted"` fallback only triggers on truly missing IDs (defensive path; FK prevents them in practice).

## Budget note

Actual diff ≈ 830 lines (253 prod ±, 575 new tests) vs 320–380 forecast. Overrun is entirely test code required by tasks 1.2/1.4/2.1/2.4/3.2–3.4. Single PR retained per assigned delivery decision; recommend reviewer accept as `size:exception` (test-heavy) or say the word to split prod-vs-tests.

## Deviations from design

None — implementation matches design (chunk 400, no LIMIT on scoped query, stderr signal, Offset>0 only, conflicts path untouched).
