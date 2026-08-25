# Apply Progress: engram-bigmem-import — PR1+PR2

## Summary

PR1 (base main → feature/engram-bigmem-import): Exported `GunzipData` alias, created `EngramFileTransport` with `ResolveEngramDir`/`ReadManifest`/`ReadChunk` Clean guards, implemented `syncIDToID` fallback hash, and `ImportFromEngram` with project filtering, dedup via `sync_chunks('engram')`, corrupt-chunk warn-continue, and stub session recovery. Added `engram_import_test.go` covering ID mapping, project filter, dedup, corrupt chunk, stub session, and missing manifest.

PR2 (stacked-to-main, base PR1 → continued): Added CLI hardening — `biggz bigmem sync [--import|import] --from-engram [--engram-dir PATH] [--project NAME]` routing to `ImportFromEngram` with missing-manifest exit1 and per-chunk corrupt warn-continue, path-traversal guard via `ResolveEngramDir`, help listing all three flags, and Pi isolation guard. Verified dedup re-import (`ChunksSkipped==1`), corrupt/missing integration, `go vet`, and `git diff pi/` empty.

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/bigmem/sync.go` | Modified | Exported `GunzipData` alias wrapping `gunzipData` (REQ-1) — PR1 |
| `internal/bigmem/engram_import.go` | Created | `EngramFileTransport`, `EngramObservation` shim, `ResolveEngramDir`, `NewEngramFileTransport`, `ReadManifest`, `ReadChunk`, `syncIDToID`, `ImportFromEngram` with filter/dedup/stub/corrupt handling — PR1 |
| `internal/bigmem/engram_import_test.go` | Created | Table-driven tests: sync_id preserved, fallback hash, project filter, dedup, corrupt warn, stub session, missing manifest, GunzipData — PR1 |
| `cmd/biggz/cli_bigmem.go` | Modified | Extended `sync` handler: added `--from-engram`, `--engram-dir` (incl. `--engram-dir=PATH`), `--project`/`--project=` parsing, `import` positional alias, `ResolveEngramDir` validation, `ImportFromEngram` vs `SyncImportDependencySafe(FileTransport)` dispatch, help lists all three flags; fast-path `sync --help` without DB open; `defer store.Close()` — PR2 |
| `cmd/biggz/cli_bigmem_test.go` | Created | Tests: help contains `--from-engram`/`--engram-dir`/`--project`, missing manifest exit1 with `manifest.json`, empty manifest via `--from-engram --engram-dir` and `--engram-dir=` forms — PR2 |

## Test Results

```
# PR1 unit suite
go vet ./internal/bigmem — exit 0
go test ./internal/bigmem -run TestSyncIDToID_TableDriven -count=1 — PASS (4 cases)
go test ./internal/bigmem -run TestEngram -count=1 — PASS
go test ./internal/bigmem -run TestResolveEngramDir -count=1 — PASS
go test ./internal/bigmem -run TestImportFromEngram_ProjectFilter — PASS
go test ./internal/bigmem -run TestImportFromEngram_DedupAndFallback — PASS (including re-import Skipped==1)
go test ./internal/bigmem -run TestImportFromEngram_CorruptChunkWarnContinue — PASS (warn chunk bad5678, other chunk imported)
go test ./internal/bigmem -run TestImportFromEngram_StubSession — PASS (stub (recovered-missing-session))
go test ./internal/bigmem -run TestImportFromEngram_MissingManifest — PASS (error mentions manifest.json)
go test ./internal/bigmem -count=1 — PASS (all 2.12s)

# PR2 CLI + hardening
go vet ./internal/bigmem ./cmd/biggz — exit 0
go test ./internal/bigmem -run "TestSyncID|TestEngram|TestResolve|TestImport|TestGunzip" -count=1 — PASS
go test ./cmd/biggz -run TestBigmemSync -count=1 -v — PASS (5 tests: HelpContainsFlags, HelpListsFromEngram, MissingManifestExit1, FromEngramFlagParsing, EngramDirEqualsForm)
go test ./cmd/biggz -run TestSync -count=1 -v — PASS (DryRun, SelectiveFlags, Help, UnknownFlag)
git diff -- pi/ — empty (REQ-8 verified)
# Full package vet + engram suite green; cmd/biggz has one pre-existing FAIL unrelated to this change (TestPostUpdateReconcile_Success) — not introduced by this change
```

## Deviations from Design

- `ResolveEngramDir` returns `(string, error)` instead of `string` to support path-traversal error signaling required by task 3.2 (PR2 consumes it). Compatible with design intent.
- `sync_chunks` dedup uses both `engram` and `engram:<id>` target_keys for robustness; either lookup satisfies `sync_chunks('engram:'+chunkID)` expectation.
- Chunk decoding uses JSON unmarshaling of `engramChunkData` (full JSON payload) rather than pure JSONL line iteration; preserves contract and handles both forms.
- PR2 CLI supports both `biggz bigmem sync --import` and `biggz bigmem sync import` positional form for spec compatibility (`biggz bigmem sync import --from-engram`).
- PR2 help fast-path for `sync --help` avoids opening DB, improving test isolation.
- `defer store.Close()` added to `bigmemRun()` to release SQLite lock for in-process tests (no behavior change for production).

## Work Unit Evidence

### PR1 Core Engine

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/bigmem -run "TestSyncID\|TestEngram\|TestResolve\|TestImport\|TestGunzip" -count=1` — exit 0, 9 tests PASS |
| Runtime harness command/scenario and exact result | `go test ./internal/bigmem -run TestImportFromEngram_ProjectFilter` — temp .engram with 2 projects, `--project biggz-ai` filters correctly (2 of 3 obs imported, other excluded); corrupt chunk warn-continue verified |
| Rollback boundary | Revert `internal/bigmem/engram_import.go` + `internal/bigmem/engram_import_test.go` + `sync.go` GunzipData alias |

### PR2 CLI + Hardening (stacked-to-main, base PR1)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./cmd/biggz -run TestBigmemSync -count=1 -v` — exit 0, 5 PASS (HelpContainsFlags, HelpListsFromEngram, MissingManifestExit1, FromEngramFlagParsing, EngramDirEqualsForm); `go test ./internal/bigmem -run TestImportFromEngram_DedupAndFallback` — PASS with `ChunksSkipped==1` on re-import |
| Runtime harness command/scenario and exact result | Missing manifest: `biggz bigmem sync --import --from-engram --engram-dir /tmp/empty` → stderr `manifest.json` exit 1; corrupt mix: `biggz bigmem sync --import --from-engram` with 1 good + 1 bad gzip chunk → stderr `warning: chunk bad5678` exit 0 with other chunk imported; `git diff -- pi/` → empty (REQ-8); `biggz bigmem sync --help` contains `--from-engram --engram-dir --project` |
| Rollback boundary | Revert `cmd/biggz/cli_bigmem.go` flag block + `cmd/biggz/cli_bigmem_test.go` — PR1 `internal/bigmem/engram_import.go` remains intact |

## Status

PR1 complete (tasks 1.1-1.3, 2.1-2.6). PR2 complete (tasks 3.1-3.4, 4.1-4.4). Remaining: 5.1 cleanup (fixtures removal, docs) — optional polish, not blocking verify.

## Remaining Tasks

- [ ] 5.1 Remove fixtures, verify no `pi/` imports, update sync help docs
