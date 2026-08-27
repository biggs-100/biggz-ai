# Tasks: bigmem-blobstore — BlobStore externalization

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 550–700 (prod 300–350 + tests 250–350) |
| 400-line budget risk | High (>400) |
| 800-line budget risk | Low (within 800) |
| Chained PRs recommended | No (single PR fits 800) |
| Suggested split | Single PR; optional PR1 core → PR2 migration/MCP |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Core BlobStore + transparent Get/Search | PR 1 | `go test ./internal/bigmem -run TestBlob -count=1` | `go run ./cmd/biggz-mcp` mem_save 150KB→mem_get resolves bytes | `internal/bigmem/blobstore.go`, `internal/bigmem/bigmem.go` |
| 2 | Doctor migration + MCP/CLI wiring | PR 1 (or PR2 if split) | `go test ./internal/bigmem -run TestDoctorFixBlobs -count=1` | `biggz bigmem doctor --fix-blobs` on temp DB + `go vet ./...` | `internal/bigmem/full.go`, `cmd/biggz/cli_bigmem.go`, `cmd/biggz-mcp/main.go` |

## Phase 1: Foundation — BlobStore Primitive

- [x] 1.1 RED: `internal/bigmem/blobstore_test.go` — `TestGetBlob_TraversalRejected` (`blob:sha256:../../etc/passwd`,`zzzz`,`<hex>/../`) and `TestGetBlob_InvalidRejected` → `ErrInvalidAddr`, no FS outside `BlobRoot()`
- [x] 1.2 Create `internal/bigmem/blobstore.go` — `BlobPrefix`, `blobAddrRe`, `BlobRoot()` (`filepath.Dir(defaultBigmemRoot())/blobs`, 0755), `IsBlobAddr`, `ValidateAddr` (regex `^blob:sha256:[0-9a-f]{64}$`, reject `..`/`/`), `PutBlob` (sha256, temp+rename, write-if-not-exists, dedup), `GetBlob` (`ErrInvalidAddr`|`ErrBlobNotFound`); `go vet ./internal/bigmem`
- [x] 1.3 RED+GREEN: `TestPutBlob_RoundTrip150KB` + `TestPutBlob_DedupNoOverwrite` (same bytes→same addr, mtime unchanged, 71-char addr) + `TestGetBlob_ValidResolves`
- [x] 1.4 GREEN: `TestGetBlob_MissingNotFound` (valid addr no file→`ErrBlobNotFound`) + `TestBlob_ConcurrentSameBytes` (2×`PutBlob(same 200KB)`→same addr, uncorrupted; `go test -race`)

## Phase 2: Core — Threshold and Transparent Resolve

- [x] 2.1 `internal/bigmem/bigmem.go` add `ShouldExternalize(c string) bool` (`len>100000` OR `data:image/`) — table `TestShouldExternalize` (10KB inline, 150KB addr, 5KB `data:image/png`→addr)
- [x] 2.2 `Store.Get`/`Search` in `internal/bigmem/bigmem.go` — if `blob:sha256:` then `GetBlob`; success→bytes, miss→raw addr (no error), else passthrough; no DB mutate; `TestGet_BlobResolved`/`TestGet_MissingFallback`/`TestSearch_BlobPassthrough`
- [x] 2.3 Verify `BlobRoot()` resolves via `defaultBigmemRoot()` sibling, rejects traversal before `filepath.Join`; storage `~/.biggz/blobs` not `~/.omp/blobs`

## Phase 3: Integration — MCP and Doctor

- [x] 3.1 `cmd/biggz-mcp/main.go` `handleToolCall` — `mem_save` intercept before `Store.Save`: `ShouldExternalize`→`PutBlob`→addr else inline; `mem_get_observation` resolve fallback; tests `TestMCP_MemSaveExternalized`/`SmallInline`/`SmallImageExternalized`
- [x] 3.2 `internal/bigmem/full.go` add `DoctorFixBlobs() (*FixResult{Migrated,Skipped,Errors})` — scan `WHERE (length(content)>100000 OR content LIKE 'data:image/%') AND NOT LIKE 'blob:sha256:%'`, per-row `PutBlob`+`UPDATE`, idempotent; tests `Migrates2Skips1`, `IdempotentReRun`, `NoFlagUntouched`
- [x] 3.3 `cmd/biggz/cli_bigmem.go` — `doctor --fix-blobs` flag→`DoctorFixBlobs()`, print `migrated/skipped/errors` + hint `find ~/.biggz/blobs -type f -mtime +30`; `TestDoctorFixBlobs_AdvisoryHint`

## Phase 4: Verification — GC and Gates

- [x] 4.1 `TestGC_NoAutoDeletion` — Saves/Gets/`doctor` never delete under `BlobRoot()`; blob count non-decreasing; orphans tolerated
- [x] 4.2 E2E: 20×150KB saves → `os.Stat` blobs exist, DB `content` len≤71, WAL bounded; verify no `leafId`/branching, no schema change
- [x] 4.3 Gates: `go vet ./...` and `go test ./... -count=1 -timeout 180s` — all 17 scenarios pass, no auto-GC path
