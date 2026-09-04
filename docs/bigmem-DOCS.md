# BigMem Storage Reference

Single authoritative reference for BigMem persistence: schema, blob
lifecycle, thresholds, protocol limits, and the `DoctorFixBlobs` migration
note. Code comments point here; scattered prose elsewhere is not the source
of truth.

## Schema

Observations live in SQLite (`bigmem.db`), table `observations`, one row per
entry. Key columns: `id` (primary key, `obs-<nanotime>-<seq>`), `title`,
`type`, `content`, `session_id`, `tool_name`, `topic_key`, `project`,
`scope` (`project` default, `personal` cross-project), `normalized_hash`
(SHA-256 of stored content, drives dedup), `revision_count`,
`duplicate_count`, `pinned`, `created_at` / `updated_at` (RFC3339),
`deleted_at` (soft delete), `review_after`, `expires_at`. Full-text search
uses an FTS5 virtual table (`observations_fts`: title, content, topic_key,
tool_name, type, project) with BM25 ranking and a LIKE fallback when FTS
returns no rows. Relations (`memory_relations`), lifecycle reviews
(`reviews`), prompts (`prompts`), sessions (`sessions`), and sync tracking
(`sync_chunks`) are sibling tables — see `internal/bigmem/` for DDL.

## BlobRoot layout

Large payloads are content-addressed to disk, never duplicated in the DB:

- DB: `<home>/.biggz/bigmem/bigmem.db` (`defaultBigmemRoot`)
- Blobs: `<home>/.biggz/blobs/<sha256-hex>` (`BlobRoot`, a sibling of the
  DB directory, isolated from `~/.omp`)
- Address form: `blob:sha256:<64 lower-hex>` (`BlobPrefix`, anchored regex
  `^blob:sha256:[0-9a-f]{64}$`)
- Writes are atomic (temp file in the same directory + rename) with
  write-if-not-exists dedup; concurrent `PutBlob` of identical bytes yields
  one file.
- Empty `$HOME` returns `""` with no `XDG_RUNTIME_DIR` fallback: `PutBlob`
  fails visibly and callers keep bytes raw inline until storage is
  available again.

## Thresholds

| Knob | Value | Where | Meaning |
|------|-------|-------|---------|
| `maxStoredBytes` | 50,000 bytes | `internal/bigmem/bigmem.go` | Inline content cap. The 50–100 KiB window is externalized via `PutBlob` instead of truncated; legacy `truncateIfNeeded` (50k + `" ... [truncated]"`) only applies below the externalize line. |
| `ShouldExternalize` | `len > 100,000` OR contains `data:image/` | `internal/bigmem/blobstore.go` via `ShouldExternalize` | Content above 100 KB or any inline image payload goes to blob storage (`PutBlob` → addr). Small images still externalize. |
| Search preview | 300 chars | `cmd/biggz-mcp/main.go` (`truncate(r.Content, 300)`) | `mem_search` returns previews; call `mem_get_observation` for full content. Previews apply after blob resolution and are marker-safe. |
| Stdin scanner | 1 MiB line cap | `cmd/biggz-mcp/main.go` (`scanner.Buffer(1 MiB, 1 MiB)`) | Single JSON-RPC lines over stdin must fit in 1 MiB. |

50k truncate vs 100 KB externalize: anything that would be cut at 50k but
qualifies for externalize (>100 KB or `data:image/`) is stored as a blob
address instead — no silent truncation. The 50–100 KiB plain-text window is
also routed through `PutBlob` on save so no bytes are lost.

## Blob lifecycle

```text
Save: content ──ShouldExternalize?──→ PutBlob ──ok──→ store addr
                           │                       └──fail──→ keep RAW inline + visible status
                           │                                    └── DoctorFixBlobs migrates later
                           └──no──→ inline row (unchanged)

Read: row.content ──IsBlobAddr?──→ GetBlob ──hit──→ bytes (unchanged)
                        │                      └──miss──→ "[missing-blob blob:sha256:<hex>]" + log, NO DB write
                        └──no──→ passthrough (unchanged)
```

- `Save` (`Store.SaveCtx`) never fails on blob errors: log-only
  (`log.Printf`), raw inline preserved. Edge callers own UX: MCP `mem_save`
  appends a `⚠️ blob externalize failed` note to the result message plus a
  stderr line; CLI save prints a stderr line and keeps exit 0; the
  `session_guard` fallback is comment-only (library path, no stderr).
- `Get`/`Search` resolve through one shared helper
  (`MissingBlobMarker` + `IsMissingBlobMarker` + `ResolveBlobOrMarker` in
  `internal/bigmem/blobstore.go`). A miss returns the in-memory marker
  `[missing-blob blob:sha256:<hex>]` (embeds the addr, `grep missing-blob`
  finds all misses) plus a log line. The DB row is never mutated on read,
  so restoring the blob file self-heals on the next read.
- The anchored `IsBlobAddr` regex never matches a marker, so markers cannot
  loop back into `GetBlob`. Consumers that matched raw `blob:sha256:`
  output must handle the marker-embedded addr (marker-prefix check or
  substring scan). Stored literal marker text passes through untouched and
  is never fed to the filesystem; traversal-shaped input
  (`../../`, non-hex) is rejected by `ValidateAddr` (`ErrInvalidAddr`).
- `SyncExport` bundles blob bytes into the export and fails visibly if a
  blob is missing; `SyncImport` skips orphan blob refs with a miss log
  instead of inserting dangling addresses, and re-externalizes large
  inline payloads (raw inline survives `PutBlob` failure).

## DoctorFixBlobs migration note

`Store.DoctorFixBlobs` migrates legacy large inline rows
(`length(content) > 100000 OR content LIKE 'data:image/%'`, not already an
addr) to blob storage: `PutBlob` per row, `UPDATE` to the addr on success,
`Migrated`/`Skipped`/`Errors` counts out. It is idempotent (re-run migrates
0) and never runs inside plain `DoctorFix` — large rows stay inline until
an explicit `DoctorFixBlobs` run. Any row that survived a past silent
`PutBlob` failure already matches the migration query, so the next doctor
run picks it up with no manual backfill. Rollback is a plain revert:
markers are output-only and blobs are content-addressed/immutable, so no
data migration needs undoing. Prune advisory: operators may run
`find ~/.biggz/blobs -type f -mtime +30` to review stale blobs; nothing is
auto-deleted (GC test pins this).
