# Proposal: Engram to BigMem Import

## Intent

One-way Engram -> BigMem migration without Pi coupling. `engram sync` produces `.engram/manifest.json` + `chunks/*.jsonl.gz`; new `biggz bigmem sync import --from-engram` reads them, maps `engram.Observation{id int64, sync_id string}` -> `bigmem.Observation{ID string}`, and inserts into `bigmem.db`. Removes dual-store drift; BigMem becomes sole authority.

## Scope

### In Scope
- `biggz bigmem sync import --from-engram [--project NAME] [--engram-dir PATH]`
- `EngramTransport` for `manifest.json` + `chunks/*.jsonl.gz`; `sync_id->ID` mapping
- `--project` filter, `--engram-dir` custom, idempotent `sync_chunks` dedup

### Out of Scope
- Cloud sync, reverse sync, Engram code changes, Pi/TUI changes, auto-migration

## Capabilities

### New Capabilities
- `bigmem-engram-import`: Engram chunk import (manifest/chunk read, ID mapping, filtering, dedup, bigmem.db insert)

### Modified Capabilities
- `cli`: adds `--from-engram`, `--engram-dir`, `--project` to `biggz bigmem sync import/sync`

## Approach

Reference Engram structs read-only (`herramientas/engram/internal/store`, `sync`). Add `EngramFileTransport` reusing `bigmem.FileTransport`/`gunzipData`. New `internal/bigmem/engram_import.go: ImportFromEngram(dir, project)` gunzips chunks, filters by project, maps `sync_id->ID`, inserts via `Store.Save`. CLI adds `--from-engram` switch. Dedup via `sync_chunks('engram:'+chunkID)`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/engram_import.go` | New | Import logic + EngramTransport |
| `internal/bigmem/sync.go` | Modified | Export `gunzipData`, share ChunkData types if needed |
| `cmd/biggz/cli_bigmem.go` | Modified | Parse `--from-engram`, `--engram-dir`, `--project` for sync import |
| `cmd/biggz/cli_bigmem_test.go` | Modified | Flag parsing + import path tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Engram chunk schema drift | Medium | Pin to `internal/store` structs; fail loud on unmarshal error |
| ID collision (int64 vs string) | Low | Always use `sync_id` as canonical; generate fallback `engram-<hash>` if empty |
| Large .engram dir blocks import | Low | Stream gunzip per chunk; report per-chunk counts |
| Pi coupling regression | Low | No Pi files touched; verify `pi` overlay unchanged |

## Rollback Plan

Revert commit; imported rows stay inert. Undo via `biggz bigmem delete project <name> --hard` or `bigmem.db` backup. `.engram` never mutated.

## Dependencies

- Engram repo `herramientas/engram` (reference only)
- `modernc.org/sqlite`, `bigmem.Store`

## Success Criteria

- [ ] `engram sync` then `biggz bigmem sync import --from-engram` imports all chunks correctly
- [ ] `--project biggz-ai` filters; `--engram-dir /tmp/.engram` overrides default
- [ ] `sync_id` preserved as `bigmem.ID`; int64 ignored; re-import is no-op
- [ ] Pi `sdd` flows unaffected; no `pi/` changes
- [ ] Missing manifest -> stderr + exit 1; corrupt gzip -> warn per chunk

## Proposal question round

Assumptions (interactive review, subagent cannot prompt):
1. `--from-engram` switches source exclusively; default path unchanged.
2. Orphan observations -> stub session `(recovered-missing-session)` like `SyncImportDependencySafe`.
3. `--project` filters per-observation after gunzip.
4. Read-only on `.engram`; tracking only in `bigmem.db`.
