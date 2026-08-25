# Design: Engram to BigMem Import

## Technical Approach

Reuse `FileTransport`/`gunzipData`/`SyncTransport` from `sync.go` to add read-only `EngramFileTransport` reading `.engram/manifest.json` + `chunks/*.jsonl.gz`. `ImportFromEngram(dir, project)` implements manifest→chunk→gunzip→filter→map(`sync_id→ID`)→insert with `sync_chunks('engram:'+chunkID)` dedup. CLI adds `--from-engram` switch routing to Engram path; default unchanged. Covers REQ-1–8.

## Architecture Decisions

### Decision: EngramFileTransport vs Direct DB Copy

| Option | Tradeoff | Decision |
|---|---|---|
| Direct SQLite copy | Fast but couples to Engram schema, misses gzip contract | Rejected |
| EngramFileTransport reusing FileTransport+gunzipData | Reuses tested logic, read-only, drift fails loud | **Chosen** |

**Rationale**: Proposal mandates chunk/gzip contract; reusing `SyncTransport` keeps `.engram` read-only.

### Decision: sync_id→ID Mapping

| Option | Tradeoff | Decision |
|---|---|---|
| Use int64 `id` | Breaks string PK, cross-project collides | Rejected |
| `sync_id` canonical; `engram-<sha256[0:12]>` fallback | Deterministic, dedup-safe | **Chosen** |

**Rationale**: REQ-4 requires `sync_id`; hash fallback ensures idempotency.

### Decision: Dedup

| Option | Tradeoff | Decision |
|---|---|---|
| In-memory only | Duplicates after restart | Rejected |
| `sync_chunks('engram:'+chunkID)` + `ON CONFLICT DO NOTHING` | Persistent, matches `SyncImportDependencySafe` (REQ-5) | **Chosen** |

**Rationale**: Follows `LocalChunkTargetKey` pattern in `bigmem.go`.

## Data Flow

```
.engram/manifest.json → EngramFileTransport.ReadManifest() → ChunkEntry[]
        → ReadChunk(id) → gunzipData() → JSONL decode → filter project
        → map sync_id→ID (fallback hash) → INSERT ON CONFLICT → sync_chunks record
```

```
CLI[sync import --from-engram --engram-dir --project]
  ├─→ Engram: ImportFromEngram(dir, project) (*ImportResult)
  └─→ Default: SyncImportDependencySafe(FileTransport) → bigmem.db
```

Components: `cli_bigmem.go` → `engram_import.go` → `sync.go` → `bigmem.go`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/engram_import.go` | Create | `EngramFileTransport`, `EngramObservation` shim, `ImportFromEngram(dir, project) (*ImportResult, error)` |
| `internal/bigmem/sync.go` | Modify | Export `gunzipData` → `GunzipData` alias, share `ChunkData` types |
| `cmd/biggz/cli_bigmem.go` | Modify | Add `--from-engram`, `--engram-dir`, `--project` to `sync --import`; route + help |
| `cmd/biggz/cli_bigmem_test.go` | Modify | Flag parsing + routing tests |

## Interfaces / Contracts

```go
type EngramFileTransport struct{ dir string }
func NewEngramFileTransport(dir string) *EngramFileTransport
func (t *EngramFileTransport) ReadManifest() (*SyncManifest, error)
func (t *EngramFileTransport) ReadChunk(string) ([]byte, error)
func ResolveEngramDir(cliDir string) string

type EngramObservation struct {
    ID     int64  `json:"id"`
    SyncID string `json:"sync_id"`
    Observation
}
func (s *Store) ImportFromEngram(engramDir, project string) (*ImportResult, error)
type ImportResult struct{ ChunksImported, ChunksSkipped, SessionsImported, ObservationsImported, PromptsImported int }
```

Missing `manifest.json` → stderr + exit 1, zero mutations. Corrupt chunk → stderr warn `chunk <ID>`, skip, continue. Empty `sync_id` → `engram-<sha256(title+content)[0:6] hex>`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | REQ-4 mapping/fallback | Table-driven: `sync_id` preserved, empty→hash, int64 ignored |
| Unit | REQ-5 dedup | Re-import same chunk → `ChunksSkipped==1`, count unchanged |
| Unit | REQ-6/7 errors | No manifest → err `manifest.json`; corrupt gzip/JSON → warn, continue |
| Integration | REQ-1–3 dispatch/filter/dir | Temp `.engram` 2 projects; filter asserts only `biggz-ai` inserted; custom dir override |
| Integration | Large dir | 100 chunks streaming gunzip, per-chunk counts, no OOM |
| E2E (CLI) | REQ-CLI-1/2 routing/help | Parse flags; `--help` lists all three |
| Guard | REQ-8 isolation | `git diff -- pi/` empty; `.engram` mtime unchanged |

## Threat Matrix

| Threat | Applicability | Design Response | Planned RED Test |
|--------|---------------|-----------------|------------------|
| Corrupt chunk | Applicable | Per-chunk gunzip/JSON error → warn skip continue; exit 0 if ≥1 ok | `bad.jsonl.gz` invalid gzip + truncated JSONL; other chunk imports |
| Path traversal `--engram-dir` | Applicable | `filepath.Clean`, reject `..` escape, read-only | `../../etc/.engram` → error; `/tmp/.engram` → canonical |
| ID collision | Applicable | `sync_id` canonical; fallback `engram-<sha256[0:12]>`; `ON CONFLICT` | Same content empty `sync_id` → same hash no-op; `id=42` ignored |
| Large dir DoS | Applicable | Stream per chunk, sequential, per-chunk counts | 100×1MB chunks → RSS ok |
| Doc-like paths | N/A — no exec markdown | — | — |
| Git/commit/push/PR | N/A — no VCS write | — | — |

## Migration / Rollout

No migration. Additive, read-only on `.engram`. Rollback: `biggz bigmem delete project <name> --hard` or `bigmem.db` restore. Re-import idempotent. Behind `--from-engram` switch.

## Open Questions

- [ ] Confirm Engram JSON field `sync_id` vs `syncId` when vendored; shim with alias?
- [ ] Default `ResolveEngramDir`: project root vs cwd — align with `cli_bigmem.go` git-root detection?
