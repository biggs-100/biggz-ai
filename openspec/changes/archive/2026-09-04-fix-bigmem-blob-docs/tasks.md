# Tasks: fix-bigmem-blob-docs

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 300–380 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (Units 1→2 merge in order) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Helper + read wiring | PR 1 | `go test ./internal/bigmem/ -run 'TestMissingBlob\|TestResolveBlob' -count=1` | N/A (unit-covered) | `internal/bigmem/blobstore.go`, `internal/bigmem/bigmem.go` |
| 2 | Save status + DOCS + Join | PR 1 | `go test ./internal/bigmem/ ./internal/doctor/ -count=1` | `HOME="" go run ./cmd/biggz bigmem save -m t` | `cmd/biggz-mcp/main.go`, `cmd/biggz/cli_bigmem.go`, `internal/sdd/session_guard.go`, `internal/doctor/bigmem.go`, `docs/bigmem-DOCS.md` |

## Phase 1: Foundation — blob helper (RED first)

- [x] 1.1 RED: add `TestMissingBlobMarker_Format` + `TestIsMissingBlobMarker_AnchoredRegex` to `internal/bigmem/blobstore_test.go` (anchored regex never matches marker). Verify: `go test ./internal/bigmem/ -run TestMissingBlob` fails.
- [x] 1.2 GREEN: implement `MissingBlobMarker`, `IsMissingBlobMarker`, `ResolveBlobOrMarker` in `internal/bigmem/blobstore.go` (miss→marker+log, no DB). Verify: same command passes.
- [x] 1.3 RED: add `TestResolveBlobOrMarker_NoMutate` (miss keeps raw addr) + traversal-reject conform in `blobstore_test.go`. Verify: fails pre-wiring, passes after.

## Phase 2: Core — read-path wiring

- [x] 2.1 Rewire `GetCtx` (~1768) + `resolveBlobContent` (~1712) in `internal/bigmem/bigmem.go` to helper. Verify: `go test ./internal/bigmem/ -run TestGet` passes.
- [x] 2.2 Collapse 3 Search loops (~1877/~2005/~2057) in `bigmem.go` to helper. Verify: `go test ./internal/bigmem/ -run TestSearch` passes.
- [x] 2.3 Rewire MCP `mem_get` (~807) in `cmd/biggz-mcp/main.go` to helper + stderr miss line. Verify: `go vet ./cmd/biggz-mcp/` passes.
- [x] 2.4 Add miss-only log in `SyncImport` (~1358) `internal/bigmem/full.go`; keep `SyncExport` untouched. Verify: `go test ./internal/bigmem/ -run TestSync` passes.

## Phase 3: Save-path visibility + integration

- [x] 3.1 Keep `Store.Save` (~1500) log-only on `PutBlob` fail (raw inline, no return) in `bigmem.go`. Verify: `go test ./internal/bigmem/ -run TestSave` passes.
- [x] 3.2 Add `⚠️ blob externalize failed` note to MCP `mem_save` (~646) + extend CLI save (~162) stderr (`DoctorFixBlobs will migrate`, exit 0). Verify: `go vet ./cmd/biggz-mcp/ ./cmd/biggz/` passes.
- [x] 3.3 Comment-only guard fallback (~197) in `internal/sdd/session_guard.go` (no stderr in lib). Verify: `go vet ./internal/sdd/` passes.
- [x] 3.4 Add `TestSaveFailure_PersistsRawInline_And_DoctorFixBlobsRemigrates` (temp-HOME fail Put, assert raw, restore, migrate). Verify: `go test ./internal/bigmem/ -run TestSaveFailure` passes.

## Phase 4: DOCS, doctor fix, e2e

- [x] 4.1 Create `docs/bigmem-DOCS.md` (schema, BlobRoot, 50k vs 100KB/`data:image/`, 300-char preview, 1MiB scanner, lifecycle, migration note). Verify: all six items present.
- [x] 4.2 Point 7 call-site comments at `docs/bigmem-DOCS.md` (pointer only). Verify: `rg -n "bigmem-DOCS" --glob '*.go'` hits 7.
- [x] 4.3 Replace concat with `filepath.Join(store.RootDir(), "bigmem.db")` in `internal/doctor/bigmem.go:65`. Verify: `go test ./internal/doctor/` passes.
- [x] 4.4 E2E: `HOME=""` save shows visible status, never silent bare addr. Verify: `HOME="" go run ./cmd/biggz bigmem save -m e2e-probe` passes.
