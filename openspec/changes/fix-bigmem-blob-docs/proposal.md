# Proposal: fix-bigmem-blob-docs

## Intent

Blob failures are silent today: `GetBlob` miss returns raw `blob:sha256:` addr as if content, `PutBlob` errors vanish into stderr-only. Close audit P2: make every blob failure visible and ship one authoritative BigMem DOCS reference.

## Scope

### In Scope
- Visible blob errors: `Get` miss → explicit missing-blob marker + log; `PutBlob` failure → wrapped error surfaced, never silent addr passthrough or stderr-only
- Single DOCS reference (`docs/bigmem-DOCS.md`): schema, limits, protocol, blob lifecycle, `DoctorFixBlobs` migration note; obsoletes scattered comments as source of truth
- Doctor fix: `internal/doctor/bigmem.go:65` string concat → `filepath.Join`

### Out of Scope
- Cloud sync, package split, MCP N+1, ctx handling, save bypass (covered by other SDDs)

## Capabilities

### New Capabilities
- None (DOCS is reference prose, not a behavioral capability)

### Modified Capabilities
- `bigmem-blobstore`: `Get` on missing blob MUST return explicit marker + log (replaces silent raw-addr fallback); `Save`/`PutBlob` failure MUST surface wrapped error (replaces stderr-only/err==nil guard)

## Approach

1. Replace `err == nil` guards in `cmd/biggz-mcp/main.go` (~646, ~807), `cmd/biggz/cli_bigmem.go` (~162), `internal/sdd/session_guard.go` (~197) with log + wrapped-error/marker paths; keep DB unmutated on read.
2. Write `docs/bigmem-DOCS.md` (schema, 50k `maxStoredBytes`, 100k/`data:image/` `ShouldExternalize`, 300-char search preview, blob lifecycle, DoctorFixBlobs); point code comments at it.
3. One-line `filepath.Join(store.RootDir(), "bigmem.db")` in doctor check.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/blobstore.go` | Modified | Error/marker helpers (no threshold change) |
| `cmd/biggz-mcp/main.go` | Modified | Save/Get visible-error paths |
| `cmd/biggz/cli_bigmem.go` | Modified | Save visible-error path |
| `internal/sdd/session_guard.go` | Modified | Save fallback visible-error path |
| `internal/doctor/bigmem.go` | Modified | `filepath.Join` fix |
| `docs/bigmem-DOCS.md` | New | Single BigMem reference |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Marker breaks consumers expecting raw addr | Med | Marker keeps addr embedded + grep-able prefix |
| Error surfacing breaks save flow | Low | Save still persists; error is additive log/wrap |

## Rollback Plan

Revert commit; blobs are content-addressed/immutable so no data migration to undo; DOCS file deletion is safe.

## Dependencies

- None (pure local change, no schema migration)

## Success Criteria

- [ ] Missing blob never returns bare addr silently (marker + log present)
- [ ] `PutBlob` failure surfaces beyond stderr (wrapped error / explicit status)
- [ ] `docs/bigmem-DOCS.md` covers schema, limits, protocol, lifecycle, migration note
- [ ] Doctor uses `filepath.Join`; `go test ./internal/bigmem/ ./internal/doctor/` passes
