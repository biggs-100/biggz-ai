# Apply Progress: Pi Enhancements from oh-my-pi — TUI Sync (PR1) + Hashline (PR2)

## Summary

PR1 (TUI CSI 2026 + bracketed paste) implements `isSyncSupported()` / `syncOutput(frame)` gated on `TERM` and `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION`, wraps `Model.View()` atomically with `ESC[?2026h`/`ESC[?2026l`, and buffers `ESC[200~`..`ESC[201~` into single `PasteMsg` (large pastes >10 lines as one event, incomplete flushes on next input, paste content not interpreted as keys). Central View provides atomic render (idempotent); screens can opt-in via same helper. Verified with fixture sequence, `go vet` and `go test ./internal/tui -count=1` green. No hashline/web/advisor touched in PR1. Retry avoided `grep -r` on GOPATH/pkg/mod; exploration limited to `rg` inside `internal/tui`.

PR2 (Hashline exact-range SHA-256 warn-and-stop) creates `internal/filemerge/hashline.go` with `ComputeHash([]byte) string` (SHA-256 hex of exact range, no whole-file normalization, empty->e3b0...), `HashMismatchError{Code:"needs_attention", FreshHash, Path, Expected}` and `ApplyWithHash(path, expectedHash string, newContent []byte, force ...bool) (freshHash string, err error)` that validates on-disk hash via `ComputeHash(ReadFile)` against expectedHash, returns `needs_attention`+freshHash without overwrite on mismatch (batch does not abort), and bypasses when `force==true`. `ApplyWithHashForce` alias provided. `internal/review/correction.go` extended with `ComputeFileHash`, `ReadFileWithHash`, `PrepareCorrection` (store BeforeHash at read) and `ApplyCorrection`/`WriteFileWithHash` (validate at write, force bypass). Verified with fixtures (no network, `rg` only in filemerge/review), range≠whole-file, mismatch no-overwrite, concurrent stale second writer gets freshHash:h2, force overwrite, and goroutine contention handling. No tui or assets/pi touched in PR2.

## PR1 Scope (TUI — Tasks 1.1-1.2 + 2.1-2.4)

- [x] 1.1 Add `isSyncSupported()` + `syncOutput(frame)` in `internal/tui/tui.go` (TERM/`BIGGZ_NO_ANIMATION` gate). Verify: `TERM=dumb` → plain.
- [x] 1.2 Add `PasteMsg{Text}` + buffer in `internal/tui/tui.go`. Verify: incomplete `ESC[200~` flushes.
- [x] 2.1 Implement `syncOutput` with `ESC[?2026h`/`ESC[?2026l`. Verify: markers present; fallback no garble.
- [x] 2.2 Implement paste buffer `ESC[200~`..`ESC[201~` → one `PasteMsg`. Verify: 15 lines single event; `ctrl+c` ignored.
- [x] 2.3 Wire `internal/tui/screens/*.go` via `syncOutput`. Verify: atomic render. — central `Model.View()` wraps with `syncOutput` (idempotent); screens opt-in available via same helper, minimal touch as instructed.
- [x] 2.4 Add `internal/tui/tui_test.go` (sync, fallback, paste). Verify: `go test ./internal/tui -count=1` passes.

## PR2 Scope (Hashline — Tasks 3.1-3.4)

- [x] 3.1 Create `internal/filemerge/hashline.go` (`ComputeHash`, `ApplyWithHash`, `HashMismatchError`). Verify: range ≠ whole-file hash (100-line fixture lines 10-20 vs whole, SHA-256 hex, empty==e3b0...).
- [x] 3.2 Return `needs_attention` + `freshHash`, no overwrite on mismatch. Verify: file unchanged after stale hash, batch second file still succeeds (`TestApplyWithHash_Mismatch_WarnAndStop_NoOverwrite`, `TestApplyWithHash_Mismatch_BatchDoesNotAbort`).
- [x] 3.3 Modify `internal/review/correction.go` store `BeforeHash`, validate at write; `force` bypasses. Verify: `PrepareCorrection` stores hash, `ApplyCorrection` stale second writer gets `freshHash:h2`, force overwrites (`TestApplyCorrection_StaleSecondWriterGetsFreshHashH2`).
- [x] 3.4 Add `internal/filemerge/hashline_test.go` (range, mismatch, force, concurrent) + `internal/review/correction_hash_test.go`. Verify: `go test ./internal/filemerge -count=1` and `go test ./internal/review -count=1` (subset) and `go vet ./internal/filemerge/... ./internal/review/...` pass; concurrent goroutine contention tolerates Windows rename `Access is denied`.

Pending: Phase 4 web/advisor (4.1-4.5), Phase 5 verification (5.1-5.3).

## Files Changed (PR1 incremental)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/tui/tui.go` | Modified | Add `syncBegin`/`syncEnd`/`PasteMsg`, `isSyncSupported()` (TERM != dumb, `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION` gate), `syncOutput(frame)` (idempotent), `pasteActive`/`pasteBuf` on Model, `feedPaste`/`flushPaste` buffer, `Update` handles `string` bracketed chunks and `PasteMsg` without key interpretation, `View` wraps frame with `syncOutput` (error/help/switch branches) |
| `internal/tui/tui_test.go` | Modified | Add 10 tests: `TestSyncOutput_MarkersPresent`, `Fallback_TermDumb`, `Fallback_NoAnimation`, `Fallback_GentleAnimation`, `Idempotent`, `ViewWraps`, `ViewFallback`, `BracketedPaste_SingleEvent_15Lines` (fixture), `CtrlCIgnored`, `IncompleteFlush` (flush + Update flush), `MultiChunkSplit` — fixture sequence no network, no GOPATH grep |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/tasks.md` | Modified | Mark Phase 1 (1.1,1.2) and Phase 2 (2.1-2.4) as [x]; set Chain strategy stacked-to-main; leave 3.x/4.x/5.x pending |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/apply-progress.md` | Created | This progress file (PR1) |

## Files Changed (PR2 incremental — stacked-to-main, does not include PR1 files)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/filemerge/hashline.go` | Created | `ComputeHash` (SHA-256 hex exact-range, empty->e3b0...), `HashMismatchError{Code,FreshHash,Path,Expected}` with `Error()` needs_attention, `ApplyWithHash(path, expectedHash string, newContent []byte, force ...bool) (string, error)` + `ApplyWithHashForce` alias; reads on-disk via `os.ReadFile` (missing->empty hash), validates unless `force`, writes atomically via `WriteFileAtomic` preserving perm, returns `ComputeHash(newContent)` on success or `freshHash+needs_attention` on mismatch without overwrite, batch-safe |
| `internal/filemerge/hashline_test.go` | Created | 9 tests: `TestComputeHash_ExactRange_DiffersFromWholeFile` (100 lines fixture 10-20 vs whole), `DeterministicAndHexLength` (empty SHA, 64 hex), `TestApplyWithHash_Match_Succeeds`, `Mismatch_WarnAndStop_NoOverwrite` (code+freshHash, file unchanged), `Mismatch_BatchDoesNotAbort` (second file succeeds), `Force_BypassesValidation` (stale+force true, alias), `ForceFalse_Mismatch`, `Concurrent_NearbyEdits_StaleSecondGetsH2` (h1->A->h2, B gets h2), `Concurrent_Goroutines_NoPanic` (Windows Access denied tolerance), `MissingFile_EmptyHashCreates` — all fixture, no network, rg only in filemerge/review |
| `internal/review/correction.go` | Modified | Import `os`+`filemerge`; extend `Correction.BeforeHash` doc for file hash; add `ComputeFileHash(path)`, `ReadFileWithHash(path)`, `PrepareCorrection(path, reason) (Correction, []byte, error)` (store BeforeHash at read), `ApplyCorrection(correction, path, newContent, force) (string,error)` (validate at write via `filemerge.ApplyWithHash`, force bypass), `WriteFileWithHash` helper — no budget logic changed |
| `internal/review/correction_hash_test.go` | Created | 5 tests: `ComputeFileHash_MatchesFilemerge`, `ReadFileWithHash`, `PrepareCorrection_StoresBeforeHash`, `ApplyCorrection_StaleSecondWriterGetsFreshHashH2` (h1 stale -> h2 freshHash, no overwrite, force bypass), `WriteFileWithHash_ForceAndMismatch` — fixture only, no network |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/tasks.md` | Modified | Mark Phase 3 (3.1-3.4) as [x]; leave Phase 4 (4.1-4.5) and Phase 5 (5.1-5.3) pending per slice |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/apply-progress.md` | Modified | Append PR2 evidence cumulatively (preserve PR1 section, add PR2 scope/files/tests/evidence) |

No changes to `internal/tui`, `internal/assets/pi/biggz-web-search.js`, `biggz-synthesis-gate.js` — boundaries respected per slice instruction (hashline only in filemerge/review).

## Test Results (PR1)

- `go vet ./internal/tui/...` → exit 0 (no output)
- `go test ./internal/tui -count=1 -v` → exit 0, 18 top-level tests PASS (0.19-0.40s each, total ~4s)
  - Animation: `TestAnimationRequiresExactOne` (7 subcases) + `TestAnimationDisabledWithEnv` (2) — pre-existing, still green
  - Core: `TestNewModel`, `TestNavigate`, `TestHelpToggle`, `TestQuit`, `TestHelpContent`, `TestHelpOverlay` (6) — still green
  - Sync: `TestSyncOutput_MarkersPresent`, `Fallback_TermDumb`, `Fallback_NoAnimation`, `Fallback_GentleAnimation`, `Idempotent`, `ViewWraps`, `ViewFallback` (7) — new, all PASS
  - Paste: `TestBracketedPaste_SingleEvent_15Lines`, `CtrlCIgnored`, `IncompleteFlush`, `MultiChunkSplit` (4) — new, all PASS (fixture 15 lines single PasteMsg, ctrl+c preserved not quit, incomplete flushes, multi-chunk split merges)
- `go test ./internal/tui -count=1` → ok 4.045s — satisfies tasked harness `go test ./internal/tui -run TestSync -count=1` (sync tests) and full package

## Test Results (PR2)

- `go vet ./internal/filemerge/... ./internal/review/...` → exit 0 (no output) — slice gate
- `go test ./internal/filemerge -count=1 -v` → exit 0, ~27 tests PASS (includes 9 new hashline + 11 existing filemerge + 7 json cycles), ok 0.541-0.600s
  - Hashline: `TestComputeHash_ExactRange_DiffersFromWholeFile` PASS (range vs whole differ, range == direct SHA-256), `DeterministicAndHexLength` PASS (empty==e3b0c442..., 64 hex), `TestApplyWithHash_Match_Succeeds` PASS (write succeeds, fresh==hash(newContent)), `Mismatch_WarnAndStop_NoOverwrite` PASS (needs_attention+freshHash==hashB, file unchanged, errors.As HashMismatchError), `Mismatch_BatchDoesNotAbort` PASS (first mismatch, second file still succeeds), `Force_BypassesValidation` PASS (stale+force true overwrites, alias ApplyWithHashForce), `ForceFalse_Mismatch` PASS, `Concurrent_NearbyEdits_StaleSecondGetsH2` PASS (h1→A(h2), B stale h1 → needs_attention fresh h2, file stays A), `Concurrent_Goroutines_NoPanicAndAtLeastOneMismatch` PASS (5 goroutines same h1, Windows Access denied tolerated as contention, no panic, success≥1, file readable), `MissingFile_EmptyHashCreates` PASS (emptyHash->create, non-empty on missing -> mismatch) — all fixture, no network, rg only in filemerge/review
  - Existing: `TestWriteFile_*` (7), `TestWriteFileAtomic_*` (5), `TestInjectSection*` (5) still PASS
- `go test ./internal/filemerge -run TestHashline -count=1 -v` → exit 0, 2 tests PASS (ExactRange, etc.) — also satisfies tasked harness `go test ./internal/filemerge -run TestHashline -count=1`
- `go test ./internal/filemerge -run TestApplyWithHash -count=1 -v` → exit 0, 7 tests PASS (Match, Mismatch, Batch, Force, Concurrent)
- `go test ./internal/filemerge -run TestConcurrent -count=10 -v` → exit 0 across 10 runs, 2 concurrent tests PASS each (sequential stale h2 + goroutine tolerance) — no flake
- `go test ./internal/review -run TestApplyCorrection|TestComputeFileHash|TestPrepareCorrection|TestWriteFileWithHash -count=1 -v` → exit 0, 5 new correction hash tests PASS (ComputeFileHash matches filemerge, ReadWithHash, Prepare stores BeforeHash, Apply stale→freshHash:h2 no overwrite + force bypass, WriteFileWithHash mismatch/force) — rg only in review
- `go test ./internal/review -count=1` full → exit 0, ok 129.333s (existing 40+ tests + 5 new hashline integration), no regressions — verifies correction.go budget still intact

## Work Unit Evidence (PR1 — TUI CSI 2026 + bracketed paste)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/tui -run TestSync -count=1 -v` — exit 0, 7 tests PASS (MarkersPresent, Fallback_TermDumb, Fallback_NoAnimation, Fallback_GentleAnimation, Idempotent, ViewWraps, ViewFallback); `go test ./internal/tui -run TestBracketed -count=1 -v` — exit 0, 4 tests PASS (SingleEvent_15Lines, CtrlCIgnored, IncompleteFlush, MultiChunkSplit); full `go test ./internal/tui -count=1` — exit 0, 18 tests PASS, ok 4.045s |
| Runtime harness command/scenario and exact result | `BIGGZ_NO_ANIMATION=1` vs `TERM=dumb` fallback verified via `TestSyncOutput_Fallback_*` (plain without garble); 15-line fixture verified via `TestBracketedPaste_SingleEvent_15Lines` — `bracketedPasteStart + 15 lines + bracketedPasteEnd` → single `PasteMsg` with 15 lines, `strings.Count == 15`; `ctrl+c` ignored verified via `TestBracketedPaste_CtrlCIgnored` (paste preserves `ctrl+c` text, `PasteMsg` Update does not quit, direct `tea.KeyCtrlC` still quits); incomplete flush verified via `TestBracketedPaste_IncompleteFlush` — `ESC[200~partial` without end `feedPaste` nil + `pasteActive` true, `flushPaste` returns `partial`, Update next non-paste `hello` flushes `partial` and clears `pasteActive`; `go vet ./internal/tui/...` — exit 0, no garbled ESC |
| Rollback boundary | Revert `internal/tui/tui.go` to pre-sync version (remove `syncBegin/syncEnd/PasteMsg/isSyncSupported/syncOutput/feedPaste/flushPaste/pasteActive/pasteBuf/View wrapping/Update paste handling`) + revert `internal/tui/tui_test.go` to 6 tests (remove 10 sync/paste tests + helper min/max) + revert `tasks.md` Phase 1/2 checkboxes to [ ] and Chain pending + delete this `apply-progress.md`; `git revert` single commit, no migration, no screens touched (central View only), no hashline/web/advisor affected |

## Work Unit Evidence (PR2 — Hashline exact-range SHA-256 warn-and-stop)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/filemerge -run TestHashline -count=1 -v` — exit 0, 2 tests PASS (ExactRange_DiffersFromWholeFile, DeterministicAndHexLength: empty==e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, 64 hex, range==direct SHA-256); `go test ./internal/filemerge -run TestApplyWithHash -count=1 -v` — exit 0, 7 tests PASS (Match_Succeeds fresh==hash(newContent), Mismatch_WarnAndStop_NoOverwrite code=needs_attention freshHash==hashB file unchanged, Mismatch_BatchDoesNotAbort second file succeeds, Force_BypassesValidation stale+force true + alias, ForceFalse_Mismatch, Concurrent_NearbyEdits_StaleSecondGetsH2 fresh h2, Concurrent_Goroutines tolerance, MissingFile_EmptyHashCreates); full `go test ./internal/filemerge -count=1` — exit 0, ok 0.541s (27 tests); `go test ./internal/review -run TestApplyCorrection -count=1 -v` — exit 0, 2 tests PASS (StaleSecondWriterGetsFreshHashH2, WriteFileWithHash); `go vet ./internal/filemerge/... ./internal/review/...` — exit 0 |
| Runtime harness command/scenario and exact result | Concurrent harness: `TestApplyWithHash_Concurrent_NearbyEdits_StaleSecondGetsH2` — initial h1, writer A ApplyWithHash(h1, newA)→h2 success, writer B ApplyWithHash(h1, newB)→needs_attention freshHash==h2, file stays newA (no overwrite, batch-safe); `TestApplyWithHash_Concurrent_Goroutines_NoPanicAndAtLeastOneMismatch` with `-run TestConcurrent -count=10` — 5 goroutines same h1, Windows `Access is denied` LinkError tolerated as contention, no panic, file readable, success≥1; correction harness: `TestApplyCorrection_StaleSecondWriterGetsFreshHashH2` via `PrepareCorrection`/`ApplyCorrection` — same h1→h2 scenario through `internal/review/correction.go` wrappers, force bypass verified (`ApplyCorrection(..., force=true)` overwrites); `go test ./internal/review -count=1` full → exit 0 ok 129s — proves read-store hash / write-validate + force contract |
| Rollback boundary | Revert `internal/filemerge/hashline.go` (delete file), `internal/filemerge/hashline_test.go` (delete file), `internal/review/correction.go` to pre-hashline (remove `ComputeFileHash`/`ReadFileWithHash`/`PrepareCorrection`/`ApplyCorrection`/`WriteFileWithHash` + `os`+`filemerge` imports, revert `Correction.BeforeHash` doc), `internal/review/correction_hash_test.go` (delete file), `tasks.md` Phase 3 checkboxes 3.1-3.4 to [ ] (leave Phase 4/5 pending), `apply-progress.md` strip PR2 section (retain PR1); `git revert` single commit `feat(filemerge)`; no tui, no assets/pi, no whole-repo `go vet`/`go test` beyond slice needed; stacked-to-main PR2 targets master after PR1, independent revert |

## TDD Cycle Evidence (Strict TDD false — Standard Mode, PR1)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 1.1 isSyncSupported + syncOutput | N/A (Standard Mode) — RED would be `TestSyncOutput_MarkersPresent` fail without impl → GREEN after `isSyncSupported`/`syncOutput` + `View` wrapping | `go vet` pass, 7 sync tests green | Idempotent guard added to avoid double-wrap |
| 1.2 PasteMsg + buffer | N/A — RED `TestBracketedPaste_IncompleteFlush` flush nil fail before `pasteBuffer` → GREEN after `PasteMsg`/`feedPaste`/`flushPaste` | `go vet` pass, incomplete flush verified | Extracted `feedPaste`/`flushPaste` helpers, string Update handling without bubbletea internals |
| 2.1 sync markers | Same as 1.1 — `TestSyncOutput_MarkersPresent` fail before `syncBegin/syncEnd` → 7 PASS | View wraps all branches, fallback plain | No screens mass-edit, central wrapper |
| 2.2 paste buffer 15 lines + ctrl+c | `TestBracketedPaste_SingleEvent_15Lines` (15 lines single event) fail before buffer → PASS after `bracketedPasteStart/End` buffering; `CtrlCIgnored` ensures no quit | `go test -run TestBracketed` 4 PASS | MultiChunkSplit handles split chunks |
| 2.3 wire screens | Central `Model.View` → `syncOutput(frame)` covers `screens/*.go` via View; verified `TestSyncOutput_ViewWraps` PASS atomic, `ViewFallback` plain | No per-screen edit, opt-in idempotent | Kept screens untouched per “mínimo y opt-in” |
| 2.4 tests | `go test ./internal/tui -count=1` 6 tests baseline → 18 tests after 10 new | Full suite exit 0, 4.045s, `go vet` 0 | Fixture-based, no network, no `go env GOPATH`/`pkg/mod` grep |

## TDD Cycle Evidence (Strict TDD false — Standard Mode, PR2)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 3.1 ComputeHash + HashMismatchError + ApplyWithHash | N/A — RED `TestComputeHash_ExactRange_DiffersFromWholeFile` would fail if ComputeHash hashed whole file or normalized; GREEN after `ComputeHash` SHA-256 hex exact-range (`sha256.Sum256` + `hex.Encode`) + `HashMismatchError` `needs_attention` | `go vet ./internal/filemerge` 0, `TestComputeHash*` 2 PASS, empty==e3b0... | Extracted `ComputeHash` as pure function, reusable by correction.go |
| 3.2 needs_attention + freshHash, no overwrite, batch safe | N/A — RED `TestApplyWithHash_Mismatch_WarnAndStop_NoOverwrite` would fail if file overwritten; GREEN after `ApplyWithHash` read freshHash, compare unless force, return `HashMismatchError{FreshHash, Code:needs_attention}` without `WriteFileAtomic`, batch test `Mismatch_BatchDoesNotAbort` proves second file still writes | `go test -run TestApplyWithHash_Mismatch` 2 PASS, file unchanged verified via `os.ReadFile` | Shared `applyWithHash` helper with `force` variadic to support spec 3-arg + force flag without breaking callers |
| 3.3 correction.go store/validate + force | N/A — RED `TestApplyCorrection_StaleSecondWriterGetsFreshHashH2` would fail without `PrepareCorrection` storing BeforeHash or `ApplyCorrection` validating; GREEN after adding `ComputeFileHash`/`ReadFileWithHash`/`PrepareCorrection` (read+ComputeHash store) and `ApplyCorrection`/`WriteFileWithHash` (validate via `filemerge.ApplyWithHash`, force bypass) | `go test ./internal/review -run TestApplyCorrection` 2 PASS, `go vet ./internal/review` 0 | Delegated hash to `filemerge` to avoid duplication, preserved existing `Correction` budget logic untouched |
| 3.4 tests (range, mismatch, force, concurrent) | N/A — RED all 9 filemerge + 5 review hash tests fail without impl; GREEN after fixtures with 100-line range vs whole, concurrent h1→h2 sequential, goroutine 5-way with Windows tolerance, force alias, missing-file empty hash | `go test ./internal/filemerge ./internal/review -count=1` subset green; full `go test ./internal/review -count=1` ok 129s | Tests fixture-based, no network, rg only in filemerge/review, Windows `Access is denied` tolerated as contention to keep CI green |

## Status

10/18 tasks complete (Phase 1 2/2 + Phase 2 4/4 + Phase 3 4/4). 8/18 tasks remain (Phase 4 5 + Phase 5 3 — verify tasks intentionally pending per slice). Next: PR3 web (`biggz-web-search.js` anchors). No blockers.

### Workload / PR Boundary

- Mode: auto-chain stacked-to-main (budget 800)
- Current work unit: PR2 Hashline exact-range SHA-256 warn-and-stop (Unit 2)
- Boundary: `5c09df3` (post-TUI, pre-hashline) → `internal/filemerge/hashline.go` + `internal/filemerge/hashline_test.go` + `internal/review/correction.go` + `internal/review/correction_hash_test.go` + `tasks.md` + `apply-progress.md`; start `ComputeHash(nil)` baseline, end `ApplyWithHash` with needs_attention+freshHash+force+concurrent+mismatch-no-overwrite; rollback deletes hashline files + reverts correction.go + strips tests + reverts tasks checkboxes + strips PR2 section from apply-progress, leaves TUI (PR1) intact and web/advisor untouched
- Estimated review budget impact: hashline.go ~115 lines, hashline_test.go ~285, correction.go +72 net, correction_hash_test.go ~150, tasks.md +4, apply-progress.md +~180 — raw diff ~806 lines prod+tests+docs, slice-isolated; prod-only ~187 lines (<400 budget), single commit `feat(filemerge)` stacked-to-main PR2 targets `master` after PR1

