# Proposal: bigmem-blobstore (D1: BlobStore externalization)

## Intent

BigMem SQLite stores large payloads inline (>500K, `data:image/`). WAL reaches 4.4 MB + split-brain with images. Apply oh-my-pi `BlobStore` (`blob:sha256:<hash>`): payloads over threshold go to content-addressed filesystem, SQLite keeps only addr. Transparent read, bounded WAL, no branching.

## Scope

### In Scope
- `internal/bigmem/blobstore.go`: `PutBlob([]byte) (string, error)`, `GetBlob(string) ([]byte, error)`
- `biggz-mcp` externalization: if `len(content)>100KB` or `data:image/` → `PutBlob` → store `blob:sha256:<hex>` in `observations.content`
- `Get` transparent resolve: `blob:sha256:` → `~/.biggz/blobs/<sha256>` → bytes, fallback to addr on miss
- `biggz bigmem doctor --fix-blobs`: scan + migrate existing large rows, idempotent
- GC docs: `find ~/.biggz/blobs -type f -mtime +30` (manual only)

### Out of Scope
- D2 `leafId`/branching, merge, history rewrite
- `sdd-apply` branch strategy
- Auto GC, cloud sync, encryption, compression

## Capabilities

### New Capabilities
- `bigmem-blobstore`: content-addressed blob storage (`PutBlob`/`GetBlob`, `~/.biggz/blobs/<sha256>`, 100KB/image threshold, doctor migration)

### Modified Capabilities
- `bigmem`: `Get`/`Search` resolve `blob:sha256:` transparently
- `system-diagnostics`: `doctor --fix-blobs` remedy

## Approach

Filesystem sibling to DB: `~/.biggz/blobs/<sha256>` (mirrors `~/.omp/blobs/<sha256>`). `PutBlob` = `sha256` hex, `mkdir -p`, write-if-not-exists (dedup), return `blob:sha256:<hex>`. SQLite stores 71-char addr only. `Get` checks prefix then `os.ReadFile`. MCP `Save` intercepts before `Store.Save`. `doctor --fix-blobs` migrates `WHERE length(content)>100000 OR content LIKE 'data:image/%'` and not blob addr. No schema change. GC manual.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/blobstore.go` | New | PutBlob/GetBlob, addr validation |
| `internal/bigmem/bigmem.go` | Modified | Get transparent resolve |
| `cmd/biggz/cli_bigmem.go` | Modified | `--fix-blobs` flag |
| `internal/bigmem/doctor.go` | Modified | Migration remedy |
| `biggz-mcp` handler | Modified | Save externalization |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Orphan blobs | Medium | Immutable + dedup; manual GC only |
| Missing blob file | Low | Fallback returns addr; doctor reports; re-Put on save |
| Concurrent PutBlob | Low | Write-if-not-exists, atomic |
| Migration partial | Low | Idempotent per-row, re-runnable |

## Rollback Plan

`git revert`. Blobs inert under `~/.biggz/blobs/` (no DB harm). Optional pre-revert inline via `doctor --fix-blobs` reverse. No destructive migration → `rm -rf ~/.biggz/blobs` if desired.

## Dependencies

- `internal/bigmem.Store`, `modernc.org/sqlite`, `crypto/sha256`, `biggz-mcp`

## Success Criteria

- [ ] `PutBlob`/`GetBlob` round-trip >100KB and `data:image/`; addr `blob:sha256:<64hex>`
- [ ] MCP save stores addr; `Get` resolves transparently
- [ ] `doctor --fix-blobs` migrates idempotently
- [ ] WAL bounded under image workload; no split-brain
- [ ] No `leafId`/branching or `sdd-apply` branch

## Proposal question round

Assumptions (interactive/openspec/auto-chain/800):
1. 100KB + any `data:image/` triggers — confirm.
2. Root `~/.biggz/blobs/` not `~/.omp/blobs/` — confirm.
3. GC advisory `find -mtime` only — confirm.
