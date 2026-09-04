# Design: fix-bigmem-blob-docs

## Technical Approach

Centralize blob-failure visibility in `internal/bigmem/blobstore.go` (marker + resolve helper), rewire all 7 `err==nil` swallow sites to it. `Store.Save` stays non-failing (raw inline + log); edge callers add visible status. Ship `docs/bigmem-DOCS.md` as the single reference; comments become pointers. One-line `filepath.Join` in doctor. Maps to proposal steps 1–3 and spec's 2 MODIFIED + 2 ADDED.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Marker `[missing-blob blob:sha256:<hex>]` vs bare addr vs error return | Error return breaks `Get` signature consumers; bare addr is the bug | **Marker**: embeds addr (migration-safe), `grep missing-blob` finds all misses, anchored `IsBlobAddr` regex never matches it so no resolve loop |
| Shared `resolveBlobContent` helper vs per-site inline | 3 Search loops + GetCtx + MCP get = 5 read sites drifting | **One helper** `MissingBlobMarker(addr)+ResolveBlobOrMarker(content)` in blobstore.go; all read sites call it |
| `Store.Save` returns Put error vs log-only | Returning error breaks save flow (proposal risk table: Low) | **Log-only inside Save** (`log.Printf`), raw inline preserved; edge callers (MCP/CLI/guard) add visible status — Save is library, edges own UX |
| `docs/bigmem-DOCS.md` vs `openspec/specs/bigmem/` | openspec change-deltas archive away; operators need a stable path | **`docs/bigmem-DOCS.md`**: survives archive, success criteria name it; spec delta stays the change contract; comments point at DOCS, not deleted wholesale (minimal diff) |
| Marker persisted vs in-memory only | Persisting pollutes content-addressed rows | **In-memory only**: `Get`/`Search` mutate the returned struct, never `UPDATE`; DB keeps the addr so a restored blob file self-heals |

## Data Flow

```
Save: content ──ShouldExternalize?──→ PutBlob ──ok──→ store addr
                           │                       └──fail──→ keep RAW inline + log
                           │                                    └── DoctorFixBlobs migrates later
                           └──no──→ existing truncate/addr path (unchanged)

Read: row.content ──IsBlobAddr?──→ GetBlob ──hit──→ bytes (unchanged)
                        │                      └──miss──→ marker+addr, log, NO DB write
                        └──no──→ passthrough (unchanged)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/blobstore.go` | Modify | Add `MissingBlobMarker(addr)`, `IsMissingBlobMarker(s)`, `ResolveBlobOrMarker(content)` (marker + miss log, no DB touch) |
| `internal/bigmem/bigmem.go` | Modify | `Save` (~1500): log Put failure; `GetCtx` (~1768), `resolveBlobContent` (~1712), Search loops (~1877/~2005/~2057): use shared helper |
| `internal/bigmem/full.go` | Modify | `SyncImport` (~1358): already raw-inline — add miss log only; `SyncExport` miss error already visible, unchanged |
| `cmd/biggz-mcp/main.go` | Modify | mem_save (~646): `⚠️ blob externalize failed` note in result msg + stderr; mem_get (~807): shared helper + stderr |
| `cmd/biggz/cli_bigmem.go` | Modify | save (~162): keep exit 0, stderr line gains `bytes preserved inline, DoctorFixBlobs will migrate` |
| `internal/sdd/session_guard.go` | Modify | fallback (~197): comment-only — raw-inline already correct; no stderr in lib path |
| `internal/doctor/bigmem.go` | Modify | `:65` string concat → `filepath.Join(store.RootDir(), "bigmem.db")` |
| `docs/bigmem-DOCS.md` | Create | Schema + BlobRoot layout, 50k vs 100KB/`data:image/`, 300-char preview, 1MiB stdin scanner, lifecycle, DoctorFixBlobs note |
| 7 call-site comments | Modify | Replace parity prose with `// See docs/bigmem-DOCS.md` pointer |

## Interfaces / Contracts

```go
// blobstore.go — only new API
func MissingBlobMarker(addr string) string  // "[missing-blob blob:sha256:<hex>]"
func IsMissingBlobMarker(s string) bool     // prefix+addr-shape check
func ResolveBlobOrMarker(content string) string // hit→bytes, miss→marker+log, non-addr→passthrough
```

Consumers matching raw `blob:sha256:` output MUST handle marker-embedded addr (prefix check or substring scan). 300-char preview applies after resolution; markers are preview-safe.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Marker format/parse, miss-no-mutate, invalid-reject | `go test ./internal/bigmem/` — extend `blobstore_test.go` |
| Integration | Save-failure persists raw; Get-miss returns marker | Temp-HOME stores, `DoctorFixBlobs` re-migrates after restore |
| E2E | CLI/MCP visible status on forced Put failure | `go test ./internal/doctor/` + manual `HOME=""` run |

## Threat Matrix

No routing, shell, subprocess, VCS/PR, exec-classification, or process-integration boundary — this change only alters in-process error surfacing. Targeted rows per task scope:

| Row | Verdict | Reason / Safe behavior + RED test |
|-----|---------|-----------------------------------|
| Blob addr spoofing (traversal `../../`) | N/A — guarded, no change | `ValidateAddr` anchored regex + hex-only join rejects; existing `TestGetBlob_TraversalRejected` covers; marker never reaches filesystem |
| Marker injection (stored literal `[missing-blob …]`) | N/A — display-only | Marker never persisted, never fed back to `GetBlob`; helper checks `IsBlobAddr` first so literal text passes through untouched |
| State corruption (miss mutating DB) | Applicable | Safe: helpers take `string`, return `string`, no `*sql.DB` access. Fail: any `UPDATE` on miss path. RED: Get-miss then re-read raw row, addr intact |

## Migration / Rollout

No data migration. Past silent-failure rows already match `DoctorFixBlobs`'s query — next doctor run migrates them. Rollback: revert commit; marker is output-only. Exact-`IsBlobAddr` consumers need the marker-prefix branch.

## Open Questions

None — spec's SEVEN sites collapse to one helper + per-edge status mapping above.
