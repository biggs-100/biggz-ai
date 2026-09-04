# Apply progress: fix-bigmem-blob-docs

**Change**: fix-bigmem-blob-docs
**Mode**: Standard (strict_tdd false)
**Runner**: `go test -count=1 -timeout 180s`
**Skill resolution**: `internal/assets/biggz/biggz-orchestrator-workflow.md` +
  `internal/assets/biggz/biggz-orchestrator-delegation.md` read before work
  (workflow graph/dispatcher/gates/ledger + routing ladder/delegation rules).
**Delivery**: single PR, stacked-to-main, forecast 300–380 lines (Medium),
  no exception needed.

## Completed tasks

- [x] 1.1 RED tests: `TestMissingBlobMarker_Format`,
  `TestIsMissingBlobMarker_AnchoredRegex` (extend `blobstore_test.go`)
- [x] 1.2 GREEN: `MissingBlobMarker`, `IsMissingBlobMarker`,
  `ResolveBlobOrMarker` in `blobstore.go` (miss→marker+log, no DB touch)
- [x] 1.3 RED: `TestResolveBlobOrMarker_NoMutate` (marker+addr, DB keeps raw
  addr, traversal-reject, literal-marker passthrough)
- [x] 2.1 `GetCtx` + `resolveBlobContent` rewired to shared helper
- [x] 2.2 Three `SearchCtx` loops collapsed to shared helper
  (direct-results, FTS, LIKE fallback)
- [x] 2.3 MCP `mem_get` rewired to helper + stderr miss line
- [x] 2.4 `SyncImport` miss-only log (`full.go`); `SyncExport` untouched
- [x] 3.1 `Store.SaveCtx` `PutBlob`-fail path is log-only, raw inline, no return
- [x] 3.2 MCP `mem_save` result-message + stderr note; CLI save stderr gains
  `bytes preserved inline, DoctorFixBlobs will migrate` (exit 0 kept)
- [x] 3.3 `session_guard.go` fallback: comment-only (no stderr in lib path)
- [x] 3.4 `TestSaveFailure_PersistsRawInline_And_DoctorFixBlobsRemigrates`
  (forced `PutBlob` failure → raw inline → restore → `DoctorFixBlobs` migrates)
- [x] 4.1 `docs/bigmem-DOCS.md` created (schema, BlobRoot, 50k vs 100KB /
  `data:image/`, 300-char preview, 1MiB scanner, lifecycle, migration note)
- [x] 4.2 Seven call-site comments point at `docs/bigmem-DOCS.md`
  (`rg -n "bigmem-DOCS" --glob '*.go'` → 7 hits)
- [x] 4.3 `internal/doctor/bigmem.go`: concat → `filepath.Join`
- [x] 4.4 E2E: normal save prints visible `Saved:` status, exit 0, no bare
  addr; `HOME="" USERPROFILE=""` fails loudly at store open
  (`error: open bigmem: home dir: not found`), never silent

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test (Unit 1) | `go test ./internal/bigmem/ -run 'TestMissingBlob\|TestResolveBlob' -count=1` → PASS (3/3) |
| Focused test (Unit 2) | `go test ./internal/bigmem/ ./internal/doctor/ -count=1` → PASS (both packages `ok`) |
| Read-path slice | `go test ./internal/bigmem/ -run 'TestGet\|TestSearch\|TestSave\|TestSync' -count=1` → PASS |
| Vet | `go vet ./internal/bigmem/ ./internal/doctor/ ./internal/sdd/ ./cmd/biggz/ ./cmd/biggz-mcp/` → clean |
| Build | `go build ./...` → exit 0 |
| Runtime harness | `go run ./cmd/biggz bigmem save -m e2e-probe-blob-docs` → `Saved: obs-…`, exit 0, no bare addr; `HOME="" USERPROFILE="" … save -m …` → visible `error: open bigmem: home dir: not found` |
| Rollback boundary | Revert commit; blobs content-addressed/immutable, DOCS prose-only, marker output-only. Files: `internal/bigmem/blobstore.go`, `blobstore_test.go`, `bigmem.go`, `full.go`, `cmd/biggz-mcp/main.go`, `cmd/biggz/cli_bigmem.go`, `internal/sdd/session_guard.go`, `internal/doctor/bigmem.go`, `docs/bigmem-DOCS.md` |

## Deviations from design

- `TestGet_MissingFallback` (pre-existing) asserted the old silent
  raw-addr fallback the spec explicitly replaces, so its assertion was
  updated to the new contract (marker + embedded addr, DB keeps raw addr).
  `blobstore_test.go` was otherwise extended only, per the allowed surface.
- None other — helper-first rollout, log-only `Save`, in-memory markers,
  `IsBlobAddr` regex untouched (never matches marker), no
  threshold/schema changes.

## Issues found

- None blocking. `gofmt -l` flags several `internal/bigmem/` files, but it
  also flags untouched files (pre-existing CRLF/line-ending state); the
  `git diff` for this change is minimal and no reformat was applied.

## RDD correction (review CRITICALs R1-2/R4-3, shared root cause)
- GetBlob now rejects empty BlobRoot deterministically (`home dir: not found — blob unavailable`) instead of joining a cwd-relative path. Mirrors PutBlob guard. Pre-existing hole, 5-line fix + TestGetBlob_EmptyRootDeterministicError.
- Evidence: `go test ./internal/bigmem/ -run TestGetBlob -count=1` PASS (5/5); full bigmem suite green.
- Out of correction scope (accepted): session-guard comment-only (approved design tradeoff), marker-spoof indistinguishability (display-only by design), inline-bloat window (DoctorFixBlobs migration path), image case-sensitivity (pre-existing), DoctorFixBlobs lock/ctx (pre-existing), Remedy scope (pre-existing).
