# Tasks: Engram to BigMem Import

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 480–580 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 Core engine → PR2 CLI + hardening |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | EngramFileTransport + ImportFromEngram + ID map + filter | PR1 base:main | `go test ./internal/bigmem -run TestEngram -count=1` | `biggz bigmem sync import --from-engram --engram-dir /tmp/.engram --project biggz-ai` on temp .engram (2 projects) | Revert `internal/bigmem/engram_import.go` + `sync.go` alias |
| 2 | CLI flags + dedup + error handling + Pi guard | PR2 base:PR1 | `go test ./cmd/biggz -run TestBigmemSyncImport -count=1` | Missing manifest exit1; bad gzip warn-continue; `git diff -- pi/` empty | Revert `cmd/biggz/cli_bigmem.go` flag block |

## Phase 1: Foundation

- [x] 1.1 Export `gunzipData→GunzipData` alias in `internal/bigmem/sync.go` (REQ-1)
- [x] 1.2 Create `internal/bigmem/engram_import.go`: `EngramFileTransport`, `EngramObservation{id,sync_id}`, `ResolveEngramDir`, `NewEngramFileTransport`
- [x] 1.3 Implement `ReadManifest()` + `ReadChunk(id)` via `filepath.Clean` + `ReadFile(manifest.json|chunks/*.jsonl.gz)` read-only (REQ-2, REQ-8)

## Phase 2: Core Import (REQ-3..5 + Threats)

- [x] 2.1 RED: `engram_import_test.go` `sync_id="obs-abc123",id=42 → ID="obs-abc123"`; empty → `engram-<sha256[0:12]>` deterministic (REQ-4, ID collision)
- [x] 2.2 Implement `syncIDToID` fallback `engram-<sha256(title+content)[0:6]>` ignoring int64 (REQ-4)
- [x] 2.3 RED: project filter — temp .engram 2 projects, `--project biggz-ai` only that inserted (REQ-3)
- [x] 2.4 Implement `ImportFromEngram(dir,project) (*ImportResult,error)`: per-chunk `GunzipData`→JSONL→filter→map→`INSERT ON CONFLICT DO NOTHING` + stub `(recovered-missing-session)` (REQ-1,3)
- [x] 2.5 Dedup: `sync_chunks('engram:'+chunkID)` skip known, sequential streaming, `ImportResult{Imported,Skipped}` (REQ-5, Large DoS)
- [x] 2.6 RED: corrupt chunk — `bad.jsonl.gz` invalid gzip/truncated JSONL must warn `chunk <ID>` and import other chunk

## Phase 3: Error + CLI (REQ-6..8, CLI-1/2)

- [x] 3.1 Missing manifest: `IsNotExist` → stderr `manifest.json` exit1, zero mutations; corrupt per-chunk warn skip continue exit0 if ≥1 ok (REQ-6,7)
- [x] 3.2 RED: path traversal — `ResolveEngramDir("../../etc/.engram")` error; `/tmp/.engram` canonical ok
- [x] 3.3 Modify `cmd/biggz/cli_bigmem.go` sync import: add `--from-engram`, `--engram-dir`, `--project`; route `--from-engram`→`ImportFromEngram` else default; help lists three (REQ-1,2,CLI-1/2)
- [x] 3.4 Update `cmd/biggz/cli_bigmem_test.go`: flag parsing + routing + help-contains checks

## Phase 4: Testing & Verification

- [x] 4.1 Dedup re-import: same chunk → `ChunksSkipped==1` no duplicates (REQ-5)
- [x] 4.2 Missing/corrupt integration: 1 good+1 bad chunk → warn+continue; empty dir → exit1 (REQ-6,7)
- [x] 4.3 Pi guard: `git diff -- pi/` empty; `.engram` mtime unchanged (REQ-8)
- [x] 4.4 `go vet ./internal/bigmem ./cmd/biggz && go test ./... -count=1` green

## Phase 5: Cleanup

- [x] 5.1 Remove fixtures, verify no `pi/` imports, update sync help docs
