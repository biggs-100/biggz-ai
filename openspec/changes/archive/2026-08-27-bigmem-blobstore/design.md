# Design: bigmem-blobstore

## Technical Approach

Content-addressed FS sibling to SQLite: `~/.biggz/blobs/<sha256>` mirrors oh-my-pi BlobStore (500K, `blob:sha256:<hex>`) but at 100KB + any `data:image/` to bound WAL (4.4 MB split-brain). `PutBlob` SHA-256 + atomic write-if-not-exists → 71-char addr stored in `observations.content` (no schema change). `Get`/`Search` resolve transparently; `doctor --fix-blobs` migrates legacy rows idempotently. 

## Architecture Decisions

### Content-addressed file vs DB column

| Option | Tradeoffs | Decision |
|---|---|---|
| `blob BLOB` column | Atomic but WAL still blooms | Rejected |
| **External file `~/.biggz/blobs/<sha256>` + `TEXT` addr** | Bounded WAL, dedup, zero migration, oh-my-pi parity | **Chosen** |
| Compress inline (`zlib`) | -30% size, FTS still indexes large rows | Rejected |

### Threshold 100KB vs 500K

| Option | Tradeoffs | Decision |
|---|---|---|
| 500K (oh-my-pi default) | Fewer files, 100-500K images still bloat WAL | Considered |
| **100KB OR `data:image/`** | Catches 5KB base64 images; addr=71 chars | **Chosen** |
| 10KB aggressive | Over-externalizes notes, FS churn | Rejected |

### Storage root `~/.biggz/blobs` vs table

| Option | Tradeoffs | Decision |
|---|---|---|
| `blobs` SQLite table | Transactional, reintroduces WAL pressure | Rejected |
| `~/.omp/blobs` reuse | Collision with Pi | Rejected |
| **`~/.biggz/blobs/<sha256>` sibling to `bigmem.db`** via `defaultBigmemRoot` | Isolated, survives `VACUUM`, manual `find -mtime +30` | **Chosen** |

### GC manual vs auto

| Option | Tradeoffs | Decision |
|---|---|---|
| Auto GC / sweep | Needs ref-count, risk data loss | Rejected |
| **Manual only: `find ~/.biggz/blobs -type f -mtime +30` hint** | Immutable+dedup tolerates orphans; no delete path | **Chosen** |

## Data Flow

```
mcp mem_save(content)
  ├─ ShouldExternalize? len>100000 OR contains "data:image/" → YES → PutBlob → sha256→~/.biggz/blobs/<hex> (temp+rename, dedup) → blob:sha256:<hex> → Save(addr)
  │                                                               NO → Save(verbatim)
mcp/CLI Get(id) / Search
  ├─ content hasPrefix blob:sha256: ? → GetBlob(validate→ReadFile) → bytes | miss→addr fallback ; else passthrough (no DB mutate)
doctor --fix-blobs → scan WHERE (length>100000 OR LIKE 'data:image/%') AND NOT LIKE 'blob:sha256:%' → per-row PutBlob → UPDATE → {migrated,skipped,errors} idempotent
```

Immutable; atomic dedup.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/blobstore.go` | Create | `PutBlob`/`GetBlob`/`IsBlobAddr`/`BlobRoot`/`ValidateAddr`, atomic write-if-not-exists |
| `internal/bigmem/bigmem.go` | Modify | `Get`/`Search` transparent resolve; `ShouldExternalize` |
| `cmd/biggz-mcp/main.go` | Modify | `mem_save` intercept + `mem_get_observation` resolve fallback |
| `cmd/biggz/cli_bigmem.go` | Modify | `doctor --fix-blobs` flag → `DoctorFixBlobs()` + GC hint |
| `internal/bigmem/full.go` | Modify | `DoctorFixBlobs() (*FixResult)` idempotent migration |

No schema change; `0755`.

## Interfaces / Contracts

```go
const BlobPrefix = "blob:sha256:"
var blobAddrRe = regexp.MustCompile(`^blob:sha256:[0-9a-f]{64}$`)
func BlobRoot() string
func IsBlobAddr(s string) bool
func ValidateAddr(a string) (hex string, err error) // strict hex, rejects "..", "/"
func PutBlob(b []byte) (addr string, err error)     // sha256, temp+rename, dedup
func GetBlob(addr string) ([]byte, error)           // ErrInvalidAddr | ErrBlobNotFound
func ShouldExternalize(c string) bool
type FixResult struct{ Migrated, Skipped, Errors int }
func (s *Store) DoctorFixBlobs() (*FixResult, error)
```

`ValidateAddr` before `Join`; hex-only.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | PutBlob 150KB round-trip, dedup, concurrent 2×200KB, invalid/missing/`..` | `go test ./internal/bigmem -run Blob -count=1` temp dir |
| Unit | ShouldExternalize (10KB inline, 150KB addr, 5KB data:image→addr) | Table-driven |
| Unit | Get resolve + missing fallback (no DB mutate) | Save addr row, delete file, Get returns addr |
| Integration | mcp mem_save→addr + mem_get→bytes | `handleToolCall` harness + temp store |
| Integration | doctor --fix-blobs: 2 migrated/1 skipped, re-run 0, no-flag 0 | Prep rows, call twice |
| E2E | WAL bounded: 20×150KB saved, blobs on FS vs DB addr size | `os.Stat` blobs + `doctor` |

## Threat Matrix

| Boundary | Cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, `README.sh` | N/A: no MDX exec | — | — |
| Git repository selection | `git -C`, relative/absolute | N/A: no git routing | — | — |
| Commit state | staged, `commit -a`, empty | N/A: no commit | — | — |
| Push state | tracking, first push, refspec | N/A: no push | — | — |
| PR commands | `--head`, env prefix, composed | N/A: no PR | — | — |
| **Blob addr traversal** | `blob:sha256:../../etc/passwd`, `zzzz`, `<hex>/../`, `/tmp` | **Applicable** | Regex `^blob:sha256:[0-9a-f]{64}$` before `Join`; hex-only; error without FS outside `BlobRoot()` | `TestGetBlob_TraversalRejected`, `TestGetBlob_InvalidRejected` |

Applicable rows → `tasks.md`; RED before prod.

## Migration / Rollout

No migration. Ship `blobstore.go`, MCP externalizes, `doctor --fix-blobs` opt-in. `git revert`; `rm -rf ~/.biggz/blobs` optional. Re-runnable.

## Open Questions

- [ ] `BlobRoot` as `filepath.Join(filepath.Dir(defaultBigmemRoot()), "blobs")` vs literal `~/.biggz/blobs` for custom `BIGGZ_HOME`?
- [ ] `Search` preview: resolve blobs or return addr to avoid large payloads?

