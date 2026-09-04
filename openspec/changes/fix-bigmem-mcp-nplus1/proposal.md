# Proposal: Fix BigMem MCP N+1

## Intent

`mem_search` annotates relations via unscoped `ListRelations("")` + `GetCtx` per endpoint (O(R×N) loop with N+1 Gets). CLI `export` requests `Limit:100000` but `SearchCtx` silently clamps to 50, truncating output. Fix latency blowup and export correctness.

## Scope

### In Scope
- Batch title hydration for `mem_search` (one scoped query limited to result IDs)
- Additive Store API: `ListRelationsByIDs(ids)` or `GetBatch/MGet` (one chosen in spec, D2 convention)
- Export bound: paginated/streaming export with explicit cap; fix silent 50-clamp truncation
- Validate/clamp `mem_search` limit input

### Out of Scope
- Blob/DOCS (SDD4), raw-SQL rework (SDD2 done), Store ctx threading (SDD1 done)
- Ranking/BM25 changes, new MCP tools

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `bigmem`: relation-hydration query bounds, batched Get, search/export limit semantics
- `cli`: `biggz bigmem export` pagination/cap behavior

## Approach

Additive Store method only (existing `GetCtx`/`ListRelations` untouched). `mem_search` builds ID set from results, runs one scoped query, resolves titles from in-memory map — zero per-rel Gets on hot path. Export pages `Search` in chunks to a file stream with `--limit`/`--project` flags and explicit row cap; JSON array shape unchanged. `conflicts` CLI keeps `ListRelations("")`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/biggz-mcp/main.go` | Modified | `mem_search` hydration (~667-795) |
| `internal/bigmem/` | Modified | Additive batch/scoped API + paging helper |
| `cmd/biggz/cli_bigmem.go` | Modified | Export path (~558-577) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Batch API alters read semantics | Low | Additive-only; existing paths untouched + tests |
| Paged export breaks import round-trip | Low | Keep JSON shape; round-trip test |
| Scoped query misses cross-ID rels | Med | Scope = union of result IDs on both endpoints |

## Rollback Plan

Revert change commits. No migration (additive API + call-site only); export shape unchanged so prior CLI stays compatible.

## Dependencies

- None (SQLite Store only)

## Success Criteria

- [ ] `mem_search` issues ≤2 Store queries for relation annotation regardless of result count
- [ ] Export of >50 rows returns complete set (paged) honoring explicit cap flag
- [ ] `go test ./internal/bigmem/...` passes, including new N+1/export-bound tests
- [ ] `conflicts` CLI output unchanged
