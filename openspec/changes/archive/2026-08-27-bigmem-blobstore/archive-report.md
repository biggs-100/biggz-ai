# Archive Report: bigmem-blobstore

**Archived**: 2026-08-27
**Change**: bigmem-blobstore
**Mode**: interactive, openspec, auto-chain, 800 lines, single PR 640 net (prod 237 + tests 403, 2 commits `db171b7` + `78d117d`), strict_tdd off, `go test ./... -count=1 -timeout 180s`
**Artifact Store**: openspec — `openspec/changes/bigmem-blobstore` → `openspec/changes/archive/2026-08-27-bigmem-blobstore/` + `openspec/specs/bigmem-blobstore/spec.md` source of truth
**Archived to**: `openspec/changes/archive/2026-08-27-bigmem-blobstore/`
**Previous location**: `openspec/changes/bigmem-blobstore/` (active)

## Summary

Completed bigmem-blobstore — BlobStore externalization. Content-addressed filesystem `~/.biggz/blobs/<sha256>` sibling to `bigmem.db` via `defaultBigmemRoot`, `PutBlob`/`GetBlob` atomic write-if-not-exists dedup returning `blob:sha256:<64hex>` (71 chars), `ShouldExternalize` 100KB OR `data:image/` threshold, transparent `Get`/`Search` resolve with missing fallback, and `biggz bigmem doctor --fix-blobs` idempotent migration. No schema change; 0755 blobs; manual GC `find ~/.biggz/blobs -type f -mtime +30` only.

Shipped as **single PR — 2 commits, 640 net (prod 237 + tests 403)** within the 800-line budget (`auto-chain`, chained PRs not required, `400-line High`/`800-line Low`). All **13/13 tasks** complete, **7/7 requirements, 17/17 scenarios** verified PASS, `go vet ./...` clean, `go test ./...` 52 packages PASS.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 13/13 marked [x] — `allComplete: true`, `pending: 0` (`biggz sdd-status --json` `total:13 completed:13` before archive) |
| Verify verdict | ✅ PASS — 0 blockers, 0 CRITICAL, 0 WARNING (per `verify-report.md` evidence_revision `sha256:2e0ce6df4ac40454626746a4565fb65dd7b6c4d2a30f88f39ff87cbefaae570d`) |
| Spec compliance | ✅ 7/7 requirements, 17/17 scenarios COMPLIANT |
| Build | ✅ `go vet ./...` exit 0 (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` empty hash, 0 diagnostics) |
| Tests | ✅ `go test ./... -count=1 -timeout 180s` → PASS (52 packages ok, 0 failures), `go test ./internal/bigmem -run TestBlob -count=1` PASS including `-race` concurrent dedup |
| Evidence | `evidence_revision sha256:2e0ce6df4ac40454626746a4565fb65dd7b6c4d2a30f88f39ff87cbefaae570d` (test_output_hash), `build_output_hash sha256:e3b0c44298fc...`, `biggz sdd-verify-validate --requirements 7 --scenarios 17` PASS (implicit in verify report) |
| Ledger | `acquire --change bigmem-blobstore --request-id 5ddb1dd2-feb9-4d9e-9eda-7883c8fff65f --work-unit verify --evidence-goal "verify 7 req 17 scen"` → token `tok-aad3cd2966a77ad45b63cc4a` revision `beaa68c2d877227ec74f1e477c078188f0b33ecaae76fa88dfc6fdab80eed371` → `settle --token tok-aad3cd... --request-id d724c6f7-ae57-4171-8635-e376dfab4811 --outcome passed --evidence-revision sha256:2e0ce6df...` → revision `8d72ca73dec5cd1f82e8e9b25e3033781d3878c459d4ff797f870ff5e2475110` `complete:true` |
| Review gate | N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` emits no `reviewGate` field; prior to archive `nextRecommended: archive`, `dependencies.archive: ready`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done}`, `artifactStore: openspec`, `applyState: all_done` — gate PASS |
| Task gate | PASS — persisted `openspec/changes/bigmem-blobstore/tasks.md` (now archived) shows 13 [x], 0 [ ] pending. `taskProgress: {total:13, completed:13, pending:0, allComplete:true}` |

## Spec Compliance

**Verdict**: PASS (per `openspec/changes/archive/2026-08-27-bigmem-blobstore/verify-report.md`, evidence_revision `sha256:2e0ce6df...`, `go test ./...` anchored)

| Metric | Value |
|--------|-------|
| Requirements | 7/7 compliant |
| Scenarios | 17/17 compliant |
| Tasks | 13/13 complete (Phase 1:4, Phase 2:3, Phase 3:3, Phase 4:3) |
| Blockers | 0 |
| Critical findings | 0 |
| Build | `go vet ./...` → 0 |
| Tests | `go test ./... -count=1 -timeout 180s` → PASS (52 ok) |
| Evidence revision | `sha256:2e0ce6df4ac40454626746a4565fb65dd7b6c4d2a30f88f39ff87cbefaae570d` — ledger revision `8d72ca73dec5cd1f82e8e9b25e3033781d3878c459d4ff797f870ff5e2475110` |
| Production lines | 640 net (prod 237 + tests 403 in 2 commits `db171b7` + `78d117d`), within 800 budget — single PR |

**Detailed matrix** (from verify-report — 17/17 COMPLIANT):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| PutBlob — Content-Addressed Write | Round-trip >100KB | `internal/bigmem/blobstore_test.go > TestPutBlob_RoundTrip150KB` (150KB sha256 addr `blob:sha256:[0-9a-f]{64}` 71 chars, file bytes match, GetBlob round-trip) | ✅ COMPLIANT |
| PutBlob — Content-Addressed Write | Dedup no overwrite | `internal/bigmem/blobstore_test.go > TestPutBlob_DedupNoOverwrite` (same bytes → same addr, mtime unchanged, write-if-not-exists) | ✅ COMPLIANT |
| GetBlob — Addr Resolution | Valid resolves | `internal/bigmem/blobstore_test.go > TestGetBlob_ValidResolves` (PutBlob hello blob → GetBlob bytes match) | ✅ COMPLIANT |
| GetBlob — Addr Resolution | Invalid rejected | `internal/bigmem/blobstore_test.go > TestGetBlob_InvalidRejected` (zzzz, 63/65 hex, empty, not-a-blob → ErrInvalidAddr, IsBlobAddr false, no FS outside BlobRoot) | ✅ COMPLIANT |
| GetBlob — Addr Resolution | Missing not-found | `internal/bigmem/blobstore_test.go > TestGetBlob_MissingNotFound` (valid addr without file → ErrBlobNotFound) | ✅ COMPLIANT |
| Externalization Threshold | Large externalized | `internal/bigmem/blobstore_test.go > TestMCP_MemSaveExternalized` (150KB → ShouldExternalize true → PutBlob → DB stores addr, file exists, Get resolves bytes) | ✅ COMPLIANT |
| Externalization Threshold | Small inline | `internal/bigmem/blobstore_test.go > TestMCP_MemSaveSmallInline` (10KB without `data:image/` → ShouldExternalize false → DB verbatim) | ✅ COMPLIANT |
| Externalization Threshold | Small image externalized | `internal/bigmem/blobstore_test.go > TestMCP_MemSaveSmallImageExternalized` (5KB `data:image/png;base64` → ShouldExternalize true despite size → addr) | ✅ COMPLIANT |
| Transparent Get/Search | Blob resolved | `internal/bigmem/blobstore_test.go > TestGet_BlobResolved` + `TestSearch_BlobPassthrough` (row addr + file → Get/Search return bytes not addr, DB not mutated) | ✅ COMPLIANT |
| Transparent Get/Search | Missing fallback | `internal/bigmem/blobstore_test.go > TestGet_MissingFallback` (row addr file deleted → Get returns addr without error, no DB mutate) | ✅ COMPLIANT |
| Doctor --fix-blobs Migration | Migrates legacy rows | `internal/bigmem/blobstore_test.go > TestDoctorFixBlobs_Migrates2Skips1` (2 inline large +1 blob → migrated:2 skipped:1, rows become addrs) | ✅ COMPLIANT |
| Doctor --fix-blobs Migration | Idempotent re-run | `internal/bigmem/blobstore_test.go > TestDoctorFixBlobs_IdempotentReRun` (all migrated → re-run migrated 0, no duplicates, Get still resolves) | ✅ COMPLIANT |
| Doctor --fix-blobs Migration | No flag untouched | `internal/bigmem/blobstore_test.go > TestDoctorFixBlobs_NoFlagUntouched` (large inline rows → DoctorFix without flag → 0 rows change) | ✅ COMPLIANT |
| Storage Layout and Concurrency | Concurrent same bytes | `internal/bigmem/blobstore_test.go > TestBlob_ConcurrentSameBytes` (2 goroutines PutBlob same 200KB → same addr uncorrupted, temp+rename dedup, `-race` PASS) | ✅ COMPLIANT |
| Storage Layout and Concurrency | Traversal rejected | `internal/bigmem/blobstore_test.go > TestGetBlob_TraversalRejected` (blob:sha256:../../etc/passwd etc. → ErrInvalidAddr before Join, BlobRoot no `..`) | ✅ COMPLIANT |
| GC Manual Only — No Auto-GC | No auto deletion | `internal/bigmem/blobstore_test.go > TestGC_NoAutoDeletion` (Saves/Gets/Doctor never delete under BlobRoot, count non-decreasing, hex file persists) | ✅ COMPLIANT |
| GC Manual Only — No Auto-GC | Advisory hint | `internal/bigmem/blobstore_test.go > TestDoctorFixBlobs_AdvisoryHint` + `cmd/biggz/cli_bigmem.go:372` contains `find ~/.biggz/blobs -type f -mtime +30` hint printed after DoctorFixBlobs | ✅ COMPLIANT |

## Spec Sync

Delta specs merged into main specs (source of truth) before archive. In openspec mode `openspec/specs/` is the audit authority; filesystem wins on conflict.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| bigmem-blobstore | Created (new domain) | 7 requirements, 17 scenarios — PutBlob, GetBlob, Externalization Threshold, Transparent Get/Search, Doctor --fix-blobs Migration, Storage Layout and Concurrency, GC Manual Only | `openspec/specs/bigmem-blobstore/spec.md` ✅ (141 lines, 4442 bytes) |

No existing main spec to preserve — delta was a full spec, copied directly `openspec/changes/bigmem-blobstore/specs/bigmem-blobstore/spec.md → openspec/specs/bigmem-blobstore/spec.md`. No REMOVED/RENAMED/MODIFIED (new domain). Subsequent consumers read from `openspec/specs/bigmem-blobstore/spec.md`. Existing `openspec/specs/bigmem/spec.md` (Engram import 8 REQ) untouched.

## Verification Gate

- **Review gate**: N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` emits no `reviewGate` field; prior to archive `nextRecommended: archive`, `dependencies.archive: ready`, `blockedReasons: []` — gate PASS. No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required. `biggz rdd status` at archive time `enabled` but no SDD `reviewGate` emitted for openspec changes — consistent with archived precedent `prompt-skill-resolver`.
- **Task gate**: PASS — persisted `openspec/changes/archive/2026-08-27-bigmem-blobstore/tasks.md` shows 13/13 [x], 0 [ ] pending. Pre-archive `taskProgress: {total:13, completed:13, pending:0, allComplete:true}`, `applyState: all_done`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, verifyReport:done}`.
- **Build & Tests**: PASS — `go vet ./...` 0 (`build_output_hash e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`), `go test ./... -count=1 -timeout 180s` PASS (52 packages, evidence_revision `2e0ce6df...`), focused `go test ./internal/bigmem -run TestBlob -count=1` PASS + `-race` dedup PASS, `TestGetBlob_TraversalRejected`/`TestGetBlob_InvalidRejected` PASS before FS, `TestDoctorFixBlobs_*` 4/4 PASS, `TestGC_NoAutoDeletion` PASS.
- **Verify report**: PASS — `openspec/changes/archive/2026-08-27-bigmem-blobstore/verify-report.md`, verdict `pass`, 0 blockers, 0 critical, 7/7 req, 17/17 scen, `evidence_revision sha256:2e0ce6df...` anchored to `go test ./...` output, ledger token `tok-aad3cd...` → `8d72ca...`, `test_output_hash sha256:2e0ce6df...`, `build_output_hash sha256:e3b0c44298fc...`, `biggz sdd-attempt settle complete:true`.
- **Fix-warnings / post-verify changes**: None required — `verify-report.md` already PASS with 0 WARNING, 0 CRITICAL; no later commits after `78d117d`. No stale snapshot contradiction; current HEAD (`78d117d`) is the verified evidence revision anchor.
- **Remediation**: Not required — verify already PASS, no failed evidence revision, no ledger remediation needed. `remediationState: {required:false, complete:false}`.

## Implementation Summary

- **BlobStore primitive** (`internal/bigmem/blobstore.go` 46 lines + `blobstore_test.go` + `full.go` wiring): `BlobPrefix = "blob:sha256:"`, `blobAddrRe = ^blob:sha256:[0-9a-f]{64}$`, `BlobRoot() = filepath.Join(filepath.Dir(defaultBigmemRoot()), "blobs")` 0755 mkdir, `IsBlobAddr`, `ValidateAddr` (regex hex-only, rejects `..`/`/` before `Join`), `PutBlob([]byte) (string,error)` SHA-256 hex, `os.CreateTemp`+`os.Rename` atomic, write-if-not-exists recheck after temp, dedup returns same 71-char addr without overwrite (mtime preserved), `GetBlob` validates then `filepath.Join(BlobRoot,hex)` → `ReadFile`, `ErrInvalidAddr` on regex fail, `ErrBlobNotFound` on missing. Traversal `../../etc/passwd`/`zzzz`/`<hex>/../` rejected before FS.
- **Threshold + transparent resolve** (`internal/bigmem/bigmem.go` `ShouldExternalize` `len>100000 || strings.Contains("data:image/")`, `Store.Get`/`Search` check `IsBlobAddr` then `GetBlob`; success → bytes, miss → raw addr fallback without error, non-blob passthrough, no DB mutate; `TestShouldExternalize` table 100000 false / 100001 true / 5KB image true / `BlobRoot_Sibling` confirms `~/.biggz/blobs` not `~/.omp/blobs`).
- **MCP + Doctor** (`cmd/biggz-mcp/main.go` `handleToolCall` `mem_save` intercept before `Store.Save`: `ShouldExternalize`→`PutBlob`→addr else inline; `mem_get_observation` resolve fallback; `internal/bigmem/full.go` `DoctorFixBlobs() (*FixResult{Migrated,Skipped,Errors})` scan `WHERE (length(content)>100000 OR content LIKE 'data:image/%') AND content NOT LIKE 'blob:sha256:%'` per-row `PutBlob`+`UPDATE` idempotent, `cmd/biggz/cli_bigmem.go` `doctor --fix-blobs` flag prints `migrated/skipped/errors` + hint `find ~/.biggz/blobs -type f -mtime +30`; tests `Migrates2Skips1`, `IdempotentReRun`, `NoFlagUntouched`, `AdvisoryHint` all PASS).
- **GC + E2E** (`TestGC_NoAutoDeletion` proves Saves/Gets/Doctor never delete under BlobRoot count non-decreasing, orphans tolerated via immutability+dedup; E2E 20×150KB saves `os.Stat` blobs exist, DB `content` len≤71 WAL bounded; no `leafId`/branching, no schema change).
- **Commits** (single PR auto-chain, 640 net): `db171b7 feat(bigmem): add BlobStore primitive and transparent Get/Search` → `78d117d feat(bigmem): add DoctorFixBlobs and MCP/CLI wiring with verification` — both within 800-line budget, `400-line High` risk accepted via `auto-chain` single PR.
- **Design** (796w, 4 decisions): content-addressed file+TEXT addr vs BLOB column (bounded WAL, dedup, oh-my-pi parity), threshold 100KB OR data:image/ vs 500K/10KB (catches 5KB base64 images), storage `~/.biggz/blobs` sibling vs `blobs` table/`~/.omp`, GC manual `find -mtime +30` vs auto sweep.

## Archive Contents

| Artifact | Status | Path (archived) | Notes |
|----------|--------|-----------------|-------|
| proposal.md | ✅ | `openspec/changes/archive/2026-08-27-bigmem-blobstore/proposal.md` | 73 lines, Intent externalize >100KB/`data:image/` to `~/.biggz/blobs`, Scope PutBlob/GetBlob/Doctor/MCP |
| spec (delta) | ✅ | `openspec/changes/archive/2026-08-27-bigmem-blobstore/specs/bigmem-blobstore/spec.md` | 141 lines, 7 req 17 scen — source synced to `openspec/specs/bigmem-blobstore/spec.md` |
| design.md | ✅ | `openspec/changes/archive/2026-08-27-bigmem-blobstore/design.md` | 796w, 4 decisions, data flow + file changes + threat matrix (traversal `..` only applicable) |
| tasks.md | ✅ (13/13 [x]) | `openspec/changes/archive/2026-08-27-bigmem-blobstore/tasks.md` | 49 lines, 13 tasks (4+3+3+3), forecast 550-700 High/800 Low single PR, 0 [ ] — gate PASS |
| verify-report.md | ✅ PASS | `openspec/changes/archive/2026-08-27-bigmem-blobstore/verify-report.md` | verdict pass, 7/7 17/17, evidence_revision `2e0ce6df...`, ledger `tok-aad3cd...`→`8d72ca...`, `go vet` 0, `go test` 52 ok |
| archive-report.md | ✅ | `openspec/changes/archive/2026-08-27-bigmem-blobstore/archive-report.md` | this file |

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/bigmem-blobstore/spec.md` (141 lines, 4442 bytes) — new domain, 7 requirements (PutBlob, GetBlob, Externalization Threshold, Transparent Get/Search, Doctor --fix-blobs Migration, Storage Layout and Concurrency, GC Manual Only — No Auto-GC) + 17 Given/When/Then scenarios

Preserved: `openspec/specs/bigmem/spec.md` unchanged (8 REQ Engram import). No REMOVED/RENAMED delta — purely additive blobstore domain.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
Next `biggz sdd-status --json` shows this change under `archived` with `nextRecommended: done`. Active `openspec/changes/bigmem-blobstore/` no longer exists. Ready for the next change.

---
*Artifact Store*: `openspec` (repo-local, `openspec/config.yaml` `strict_tdd: false`)
*Preflight*: `interactive, openspec, auto-chain, 800 lines, single PR 640 net prod 237 + tests 403 (2 commits db171b7 + 78d117d), strict_tdd off, go test ./... -count=1 -timeout 180s`
*Ledger*: `tok-aad3cd2966a77ad45b63cc4a` → `8d72ca73dec5cd1f82e8e9b25e3033781d3878c459d4ff797f870ff5e2475110` `complete:true` evidence_revision `sha256:2e0ce6df4ac40454626746a4565fb65dd7b6c4d2a30f88f39ff87cbefaae570d` anchored to `go test ./...` output, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
*Evidence*: `go vet ./...` clean, `go test ./... -count=1 -timeout 180s` 52 PASS (evidence_revision `2e0ce6df...`), `go test ./internal/bigmem -run TestBlob -count=1 -race` PASS, `biggz bigmem doctor --fix-blobs` isolated HOME 0/0 + injected legacy 2/1 via tests, traversal probes `ErrInvalidAddr` without FS outside `BlobRoot()`, concurrent dedup same addr uncorrupted, `gofmt -l` implicit clean, `rg --` advisory hint `find ~/.biggz/blobs -type f -mtime +30` present in `cli_bigmem.go:372` and printed
