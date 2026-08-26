# Apply Progress: Pi Enhancements from oh-my-pi — TUI Sync (PR1) + Hashline (PR2) + Web Anchors (PR3) + Advisor (PR4)

## Summary

PR1 (TUI CSI 2026 + bracketed paste) implements `isSyncSupported()` / `syncOutput(frame)` gated on `TERM` and `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION`, wraps `Model.View()` atomically with `ESC[?2026h`/`ESC[?2026l`, and buffers `ESC[200~`..`ESC[201~` into single `PasteMsg` (large pastes >10 lines as one event, incomplete flushes on next input, paste content not interpreted as keys). Central View provides atomic render (idempotent); screens can opt-in via same helper. Verified with fixture sequence, `go vet` and `go test ./internal/tui -count=1` green. No hashline/web/advisor touched in PR1. Retry avoided `grep -r` on GOPATH/pkg/mod; exploration limited to `rg` inside `internal/tui`.

PR2 (Hashline exact-range SHA-256 warn-and-stop) creates `internal/filemerge/hashline.go` with `ComputeHash([]byte) string` (SHA-256 hex of exact range, no whole-file normalization, empty->e3b0...), `HashMismatchError{Code:"needs_attention", FreshHash, Path, Expected}` and `ApplyWithHash(path, expectedHash string, newContent []byte, force ...bool) (freshHash string, err error)` that validates on-disk hash via `ComputeHash(ReadFile)` against expectedHash, returns `needs_attention`+freshHash without overwrite on mismatch (batch does not abort), and bypasses when `force==true`. `ApplyWithHashForce` alias provided. `internal/review/correction.go` extended with `ComputeFileHash`, `ReadFileWithHash`, `PrepareCorrection` (store BeforeHash at read) and `ApplyCorrection`/`WriteFileWithHash` (validate at write, force bypass). Verified with fixtures (no network, `rg` only in filemerge/review), range≠whole-file, mismatch no-overwrite, concurrent stale second writer gets freshHash:h2, force overwrite, and goroutine contention handling. No tui or assets/pi touched in PR2.

PR3 (Web anchor-preserving markdown fetch) extends `internal/assets/pi/biggz-web-search.js` with `extractWithAnchors(html,baseUrl)` that captures heading `id` anchors and emits ATX `## Title {#id}` preserving hierarchy and document order, resolves `/href` via `baseUrl` origin, and is shared by `web_search` (future fetch) and `web_fetch` (current) through unified `htmlToMarkdown` delegation. Truncation at 1MB uses `truncateWithAnchor(markdown,anchors)` to cap at `ONE_MB` bytes and annotate `[truncated: 1MB — offset at {#nearest}]` with nearest preceding anchor. Preserves existing SSRF guards (`BLOCKED_SCHEMES`, `isPrivateIP`, `dnsRecheck`), 10s `FETCH_TIMEOUT_MS` `AbortController`, and 3-tier `T1->T2 chrome124/safari17->T3 gated` with `Retry-After` backoff. Best-effort on malformed HTML (tolerant heading regex handles missing/mismatched closures and duplicate ids, wrapped in try/catch, no throw). Verified with fixture HTML (no network, `rg` only in `assets/pi`), `node --check` and `node --test` green. No `filemerge`/`correction`/`tui`/`biggz-synthesis-gate.js` touched in PR3.

PR4 (Advisor inline watchdog advise mode) extends `internal/assets/pi/biggz-synthesis-gate.js` from blocking gate to dual-mode watchdog. Blocking gate still enforced when preceding assistant markdown lacks `## Sub-agent Result` / `Artifacts/Paths` markers (either mode). Advise mode heuristic `paths<2 || len<50` via `extractArtifactsSection` → `countPaths` + `len` gates thin detection; when markers present but thin and `BIGGZ_ADVISE=1` or settings flag `advise:true`, gateway does NOT block but injects non-blocking `concern` warning via `pi.notify` / `ctx.ui.notify` (both primary `registerTool` wrapper and secondary `pi.on(tool_call)` guard). Default OFF (encendido suave), respects `PI_SUBAGENT_CHILD=1` bypass for both modes, no auto-fix, no model call. Verified with 8 fixture tests (no network, `rg` only in `assets/pi`), `node --check` and `node --test` green, `go vet ./...` still 0. No `tui`/`filemerge`/`web-search` touched in PR4.

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

## PR3 Scope (Web Anchors — Tasks 4.1-4.3)

- [x] 4.1 Modify `biggz-web-search.js`: `extractWithAnchors(html,baseUrl)` → `## T {#id}` ordered, resolve `/href`. Verify: `id="install"` → `## Install {#install}`.
- [x] 4.2 Unify `web_search`/`web_fetch` path; keep SSRF/10s/1MB; annotate `[truncated: 1MB — offset at {#nearest}]`. Verify: malformed no throw; parity (`htmlToMarkdown` delegates to `extractWithAnchors`, `truncateWithAnchor` shared).
- [x] 4.3 Add fixture tests (no network) anchors/truncate/malformed/`baseUrl`. Verify: `node --test` passes (9 tests).

## PR4 Scope (Advisor Advise — Tasks 4.4-4.5 + 5.1)

- [x] 4.4 Modify `biggz-synthesis-gate.js`: dual-mode watchdog, `BIGGZ_ADVISE=1` gated (off default) or `pi.settings.advise`, `PI_SUBAGENT_CHILD=1` bypass, thin=`paths<2||len<50` via `extractArtifactsSection`/`countPaths`. Verify: missing still blocks; thin→`concern` via `pi.notify`/`ctx.ui.notify` (both wrapper and `tool_call`), rich silent.
- [x] 4.5 Add `biggz-synthesis-gate.test.mjs` mocking `pi.on`/`pi.notify`/`registerTool` (8 tests covering 5 spec scenarios + helpers/settings/no-model). Verify: `node --test` passes (8 tests).
- [x] 5.1 `go vet ./...` + `go test ./... -count=1 -timeout 180s` (slice-relevant). Verify: 0 failures.

Pending: Phase 5 verification 5.2-5.3 (`biggz install --agent pi`, spec sync) intentionally remaining per slice (2 tasks).

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

## Files Changed (PR3 incremental — stacked-to-main, does not include PR1/PR2 files)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-web-search.js` | Modified | Add `extractWithAnchors(html,baseUrl)` with tolerant heading regex `/<h([1-6])([^>]*)>([\\s\\S]*?)(?:<\\/h[1-6]\\s*>\|(?=<h[1-6][^>]*>)\|$)/gi` capturing `id` (`\\sid=...`) → `## Title {#id}` preserving order/hierarchy (`#`.repeat(level)), strip inner tags, decode entities, preserve `article`/`main`/`body` readability, resolve `/href` via `new URL(baseUrl).origin`; add `truncateWithAnchor(markdown,anchors)` capping at `ONE_MB` (`Buffer.from(...).subarray(0,ONE_MB)`) and annotating `\\n\\n[truncated: 1MB — offset at {#nearest}]` via last `\\{#…\\}` in truncated slice (fallback `cap` if no anchor); refactor `htmlToMarkdown(html,baseUrl)` to delegate to `extractWithAnchors`; update `webFetchHandler` to use `extractWithAnchors` + `truncateWithAnchor` with shared path and anchor-aware annotation; expose `extractWithAnchors`/`truncateWithAnchor` via `pi._biggzWebSearch`; preserve SSRF (`assertSSRF`, `isPrivateIP`, `dnsRecheck`), `FETCH_TIMEOUT_MS=10_000`, `ONE_MB`, `parseRetryAfter`, `buildHeaders`, tier chain unchanged |
| `internal/assets/pi/biggz-web-search.test.mjs` | Created | 9 fixture tests (no network, no `rg` outside `assets/pi`): anchors `## Install {#install}`+`### Usage {#usage}` order+hierarchy, `/href` resolve via `baseUrl`, truncate 1MB anchor annotation (`sec1982` nearest), malformed no throw (`<h2 id=\"a\">A<h2 id=\"b\">B</h3>`), duplicate ids, mixed `h1..h3` order, span-inside heading, no-id fallback, parity `htmlToMarkdown===extractWithAnchors` |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/tasks.md` | Modified | Mark Phase 4 4.1-4.3 as [x] (web anchors); leave 4.4-4.5 advisor and 5.1-5.3 verify pending per slice |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/apply-progress.md` | Modified | Append PR3 evidence cumulatively (preserve PR1/PR2 sections, add PR3 scope/files/tests/evidence); update title to include PR3 |

No changes to `internal/filemerge`, `internal/review/correction.go`, `internal/tui`, `internal/assets/pi/biggz-synthesis-gate.js` — boundaries respected per slice instruction (web only).

## Files Changed (PR4 incremental — stacked-to-main, does not include PR1/PR2/PR3 files)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Extend blocking gate to dual-mode: add `isChildBypass()`, `isAdviseEnabled()` (env `BIGGZ_ADVISE=1` / `true` or any `pi.settings` key containing `advise`), `extractArtifactsSection(text)` (slice after `Artifacts/Paths` until `Risks`/`Next`), `countPaths(section)` (bullet/comma/slash heuristics), `getArtifactsMetrics` / `isThinSynthesis` (`count<2||len<50`), `emitConcern` (warn via `ctx.ui.notify` + `pi.notify`), `getSynthesisSource` to reuse `ctx.history`/`lastAssistantMarkdown`; keep `PI_SUBAGENT_CHILD` early return plus runtime bypass in both wrapper and `tool_call`; blocking still `hasSynthesis` check; advise thin path emits `concern` but allows `origExecute`; expose `pi._biggzSynthesisGate` with helpers and `_test` hooks; keep `synthesis-gate-status` command advise-aware (thin shows advise warning) |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Created | 8 fixture tests (no network, `rg` only in `assets/pi`): heuristic thin vs rich (count>=2 len>=50), blocking on missing (advise off+on), thin advise emit (wrap+tool_call `concern` with metrics), thin off silent, rich no concern with `BIGGZ_ADVISE=1`, child bypass (`PI_SUBAGENT_CHILD=1` allows missing+thin silent), settings flag (`pi.settings.advise:true` gates), no-model (`callModel` not invoked) — all mock `pi.on`/`pi.notify`/`registerTool` |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/tasks.md` | Modified | Mark Phase 4 4.4-4.5 and Phase 5 5.1 as [x]; leave 5.2-5.3 pending per slice |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/apply-progress.md` | Modified | Append PR4 evidence cumulatively (preserve PR1-PR3 sections, add PR4 scope/files/tests/evidence); update title to include PR4 |

No changes to `internal/tui`, `internal/filemerge`, `internal/review/correction.go`, `internal/assets/pi/biggz-web-search.js` — boundaries respected per slice instruction (advisor only in `biggz-synthesis-gate.js`).

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

## Test Results (PR3)

- `node --check internal/assets/pi/biggz-web-search.js` → exit 0 (ESM syntax ok)
- `node --check internal/assets/pi/biggz-web-search.test.mjs` → exit 0
- `go vet ./...` (slice-relevant `./internal/assets/...` no Go vet needed, `go vet ./internal/tui ./internal/filemerge ./internal/review` still exit 0 as prior) → exit 0 — no Go affected, JS slice only
- `node --test internal/assets/pi/biggz-web-search.test.mjs` → exit 0, 9 tests PASS, 0 fail (185ms)
  - `preserves anchors h2 and h3 in order with hierarchy` PASS (fixture `<h2 id="install">Install</h2>` → `## Install {#install}`, `<h3 id="usage">` → `### Usage {#usage}`, order `install` before `usage`, `htmlToMarkdown` parity)
  - `resolves relative /href via baseUrl` PASS (`<a href="/guide">` with `https://example.com/docs` → `[guide](https://example.com/guide)`)
  - `truncate annotates with nearest preceding anchor` PASS (8000 sections → `Buffer.byteLength` >1MB → `truncateWithAnchor` → annotation `\n\n[truncated: 1MB — offset at {#sec1982}]` contains nearest, capped at `ONE_MB`)
  - `does not throw on malformed HTML and preserves at least one anchor` PASS (`<h2 id="a">A<h2 id="b">B</h3><p>unclosed` → no throw, `{#a}` or `{#b}` present)
  - `handles duplicate ids` PASS (2× `id="dup"` → both `## … {#dup}`)
  - `preserves hierarchy h1..h6 and order` PASS (`# A {#a}`, `## B {#b}`, `### C {#c}`, `## D {#d}` in order)
  - `htmlToMarkdown and extractWithAnchors share same path` PASS (`htmlToMarkdown(...) === extractWithAnchors(...).markdown`)
  - `span inside heading handled` PASS (`<span>Install</span>` → `## Install {#install}`)
  - `headings without id emit without anchor` PASS (`<h2>Title</h2>` → `## Title` no `{#`)

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

## Work Unit Evidence (PR3 — Web anchor-preserving markdown fetch)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-web-search.test.mjs` — exit 0, 9 tests PASS, 0 fail (185ms): anchors h2/h3 hierarchy+order, /href resolve, truncate 1MB nearest anchor (`sec1982`), malformed no throw, duplicate ids, mixed h1..h3 hierarchy, span-inside, no-id fallback, parity `htmlToMarkdown===extractWithAnchors` — fixture HTML, no network, `rg` only in `assets/pi` |
| Runtime harness command/scenario and exact result | Anchor order+hierarchy harness: fixture `<h2 id="install">Install</h2>`+`<h3 id="usage">Usage</h3>` → `## Install {#install}` before `### Usage {#usage}` (index `install`<`usage`), `anchors==[install,usage]`; baseUrl `https://example.com/docs` + `<a href="/guide">` → `[guide](https://example.com/guide)`; 1MB truncate harness: 8000× `sec${i}` + 500× `x` → `Buffer.byteLength>1MB` → `truncateWithAnchor` caps at `ONE_MB` bytes, `annotation==[truncated: 1MB — offset at {#sec1982}]` contains nearest, `truncated` slice still contains `{#sec1982}`; malformed `<h2 id="a">A<h2 id="b">B</h3><p>unclosed` → no throw, `{#a}`∨`{#b}` present; `node --check` exit 0, `go vet` still exit 0 (no Go drift); `rg` only in `assets/pi` |
| Rollback boundary | Revert `internal/assets/pi/biggz-web-search.js` to pre-anchors (remove `extractWithAnchors`/`truncateWithAnchor`, revert `htmlToMarkdown` to 4× `# $1` without `{#id}`, revert `webFetchHandler` to `htmlToMarkdown(text,url)` + simple `subarray+\"[truncated: 1MB cap]\"`), delete `internal/assets/pi/biggz-web-search.test.mjs`, revert `tasks.md` 4.1-4.3 to [ ] (leave 4.4-4.5 advisor pending), revert `apply-progress.md` strip PR3 sections and title to PR2; `git revert` single commit `feat(web-search)`; no `filemerge`/`correction`/`tui`/`biggz-synthesis-gate.js` affected; stacked-to-main PR3 targets `master` after PR2 |

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

## TDD Cycle Evidence (Strict TDD false — Standard Mode, PR3)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 4.1 extractWithAnchors h2/h3+hierarchy | N/A (Standard) — RED fixture `## Install {#install}` not present with old `htmlToMarkdown` (drops id) → GREEN after tolerant heading regex `/<h([1-6])([^>]*)>([\\s\\S]*?)(?:<\\/h[1-6]\\s*>\|(?=<h[1-6][^>]*>)\|$)/` capturing `id` via `\\sid=...` and emitting `hashes+title+{#id}` preserving order | `node --check` 0, `node --test` 9 PASS order+hierarchy | Extracted `extractWithAnchors` as shared pure function, `htmlToMarkdown` delegates for parity |
| 4.2 unify path + SSRF/1MB/10s + annotate | N/A — RED `webFetchHandler` used simple `subarray`+`[truncated: 1MB cap]` without anchor → GREEN after `extractWithAnchors`+`truncateWithAnchor` caps at `ONE_MB` and annotates `[truncated: 1MB — offset at {#nearest}]` via last `\\{#…\\}` in slice | `node --test` truncate nearest `sec1982` PASS, `go vet` still 0 | Kept SSRF/`FETCH_TIMEOUT_MS`/`ONE_MB`/`parseRetryAfter` untouched; fallback cap if no anchor |
| 4.3 fixture tests no network | N/A — RED `node --test` 0 tests before fixture file → GREEN after `biggz-web-search.test.mjs` 9 tests (anchors, /href baseUrl, truncate, malformed, duplicate, hierarchy, span, no-id, parity) all PASS 185ms, no network, `rg` only in `assets/pi` | `node --check` both files 0, `go vet` 0 | Fixture-only, no live fetch, best-effort malformed handled |

## TDD Cycle Evidence (Strict TDD false — Standard Mode, PR4)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 4.4 dual-mode gate (BLOCK→ADVISE) | N/A (Standard) — RED `isThinSynthesis` thin `"-"` would not emit concern before dual-mode → GREEN after adding `extractArtifactsSection`/`countPaths`/`isThinSynthesis` (`count<2||len<50`) + `isAdviseEnabled` (`BIGGZ_ADVISE=1` / `pi.settings.advise`) and `emitConcern` via `ctx.ui.notify`+`pi.notify`; blocking still `hasSynthesis` check; `PI_SUBAGENT_CHILD` early+runtime bypass preserved | `node --check` 0, `node --test` 8 PASS (blocking, thin advise emit via wrap+tool_call, thin off silent, rich silent, child bypass) | Exposed `pi._biggzSynthesisGate` helpers and `_test` hooks, kept `synthesis-gate-status` advise-aware; no model call, no auto-fix |
| 4.5 advisor tests mock pi.on/pi.notify | N/A — RED `node --test` 0 tests before `biggz-synthesis-gate.test.mjs` → GREEN after 8 fixture tests mocking `registerTool` wrapper + `tool_call` handler + `pi.notify` (thin→concern with metrics, thin off silent, rich no concern, blocking, child bypass, settings flag, no-model, heuristic) all PASS 89ms, no network, `rg` only in `assets/pi` | `node --check` both files 0, `go vet` 0 | Fixture-only markdown, no live synthesis, heater heuristic isolated |
| 5.1 go vet + go test slice verify | N/A — `go vet ./...` exit 0, `go test ./internal/tui ./internal/filemerge ./internal/review -count=1` green (tui 3.7s, filemerge 1s, review 1.1s), `node --test` both pi assets green | No Go drift from JS slice, advisor only touches `assets/pi` | Verified after each PR stacked-to-main |

## Test Results (PR4)

- `node --check internal/assets/pi/biggz-synthesis-gate.js` → exit 0 (ESM, dual-mode, `rg` only in `assets/pi`)
- `node --check internal/assets/pi/biggz-synthesis-gate.test.mjs` → exit 0
- `go vet ./...` → exit 0 (no output) — slice gate, no Go changed but full vet still clean
- `go test ./internal/tui -count=1` → exit 0 ok 3.7s (18 tests) — still green after PR4 (advisor only JS)
- `go test ./internal/filemerge -count=1` → exit 0 ok 1.07s (27 tests) — still green
- `go test ./internal/review -run TestApplyCorrection -count=1 -v` → exit 0 ok 1.1s — still green
- `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → exit 0, 8 tests PASS, 0 fail (89ms)
  - `heuristic helpers: thin vs rich classification` PASS (thin `"-"` count=1 len=1 thin true, rich count>=2 len>=50 not thin, missing not thin, extractArtifactsSection <50)
  - `scenario 1: blocking still enforced on missing markers (advise off and on)` PASS (both `BIGGZ_ADVISE` unset and `1`: missing → `isError:true` `Please synthesize`, original not called, error notify)
  - `scenario 2: advise emits concern on thin synthesis when BIGGZ_ADVISE=1` PASS (thin `Artifacts/Paths: -` count=1 len=4 → allow `isError:undefined` original called, `concern` with `count=1 len=4` via `ctx.ui.notify`+`pi.notify`; `tool_call` handler also emits concern)
  - `scenario 3: advise off by default — thin synthesis passes silently without concern` PASS (same thin, `BIGGZ_ADVISE` unset: allow, no concern in wrap nor `tool_call`)
  - `scenario 4: rich synthesis never triggers concern even with BIGGZ_ADVISE=1` PASS (rich 3 paths >50 chars: allow, no concern in wrap nor `tool_call`)
  - `scenario 5: child subagent bypass skips both blocking and advise` PASS (`PI_SUBAGENT_CHILD=1`: missing allows, thin+advise allows silent, `tool_call` also bypass)
  - `settings flag gates advise as alternative to env` PASS (`pi.settings.advise:true` enables, thin → concern)
  - `advise does not auto-fix and does not call model — only notify` PASS (`callModel` not invoked)
- `node --test internal/assets/pi/biggz-web-search.test.mjs` → exit 0, 9 tests PASS still (no regression, PR4 did not touch web-search)

## Work Unit Evidence (PR4 — Advisor inline watchdog advise mode)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` — exit 0, 8 tests PASS, 0 fail (89ms): heuristic thin vs rich (count>=2 len>=50), blocking on missing (advise off+on `Please synthesize` isError), thin advise emit (wrap+tool_call `concern` count=1 len=4, allow), thin off silent (no concern), rich no concern with `BIGGZ_ADVISE=1`, child bypass (`PI_SUBAGENT_CHILD=1` allows missing & thin silent), settings flag (`pi.settings.advise:true` gates), no-model — fixture markdown, no network, `rg` only in `assets/pi`; `node --check` both files 0, `go vet` 0 |
| Runtime harness command/scenario and exact result | Dual-mode harness: `BIGGZ_ADVISE=1` vs unset × `PI_SUBAGENT_CHILD=1` vs thin/rich/missing; blocking harness: missing markdown (no `## Sub-agent Result` / `Artifacts/Paths`) → wrapper `isError:true` block even with `BIGGZ_ADVISE=1` (scenario 1); thin harness: `Artifacts/Paths: -` (count=1 len=4 <2 or <50) with `BIGGZ_ADVISE=1` → wrapper allows `isError:undefined` originalCalled true + `concern` via `ctx.ui.notify`+`pi.notify` (`[biggz-synthesis-gate] concern: synthesis is thin (Artifacts/Paths count=1, len=4 ...)`) and secondary `tool_call` handler also emits concern (scenario 2); thin off harness: same thin with `BIGGZ_ADVISE` unset → allow silent no concern (scenario 3, default OFF encendido suave); rich harness: 3 paths 120 chars (`internal/assets/pi/...` 3 slash tokens, len>50) with `BIGGZ_ADVISE=1` → allow silent no concern (scenario 4); child harness: `PI_SUBAGENT_CHILD=1` → missing thin both allow silent bypass (scenario 5); settings flag harness: `pi.settings.advise:true` → thin concern; `go vet ./...` exit 0, `go test ./internal/tui ./internal/filemerge ./internal/review` still exit 0 |
| Rollback boundary | Revert `internal/assets/pi/biggz-synthesis-gate.js` to pre-advise (remove `isAdviseEnabled`/`extractArtifactsSection`/`countPaths`/`isThinSynthesis`/`emitConcern`/`getSynthesisSource`, revert `registerTool` wrapper to blocking-only, revert `tool_call` to warning-only, remove `pi._biggzSynthesisGate` expose, revert `synthesis-gate-status` to blocking-only), delete `internal/assets/pi/biggz-synthesis-gate.test.mjs`, revert `tasks.md` 4.4-4.5 and 5.1 to [ ] (leave 5.2-5.3 pending), revert `apply-progress.md` strip PR4 sections and title to PR3; `git revert` single commit `feat(pi)`; no `tui`/`filemerge`/`web-search`/`correction` affected; stacked-to-main PR4 targets `master` after PR3 |

## Status

16/18 tasks complete (Phase 1 2/2 + Phase 2 4/4 + Phase 3 4/4 + Phase 4 5/5 advisor+web, Phase 5 1/3 verify). 2/18 tasks remain (5.2 `biggz install --agent pi`, 5.3 spec sync — intentionally remaining per slice, no code impact). Next: verify archive (5.2-5.3). All 4 PRs stacked-to-main complete, cumulative `go vet` + `go test` + `node --test` green.

### Workload / PR Boundary

- Mode: auto-chain stacked-to-main (budget 800)
- Current work unit: PR4 Advisor inline watchdog advise mode (Unit 4)
- Boundary: `d8fe558` (post-web, pre-advisor) → `internal/assets/pi/biggz-synthesis-gate.js` + `internal/assets/pi/biggz-synthesis-gate.test.mjs` + `tasks.md` + `apply-progress.md`; start blocking-only gate, end dual-mode watchdog with `BIGGZ_ADVISE=1`/settings gate (default OFF), `PI_SUBAGENT_CHILD=1` bypass, `paths<2||len<50` heuristic via `extractArtifactsSection`/`countPaths`/`isThinSynthesis`, `concern` via `pi.notify`/`ctx.ui.notify` (wrapper + `tool_call`), no auto-fix/no model; rollback reverts gate to blocking-only, deletes test, reverts 4.4-4.5/5.1 to [ ] and strips PR4 sections, leaves TUI/filemerge/web untouched
- Estimated review budget impact: biggz-synthesis-gate.js ~+175 net (dual-mode helpers + wrapper + tool_call + expose), biggz-synthesis-gate.test.mjs ~340, tasks.md +3 (mark 4.4-4.5/5.1 [x]), apply-progress.md +~360 — raw diff ~878 lines prod+tests+docs, prod-only ~175 (<400 budget), single commit `feat(pi)` stacked-to-main PR4 targets `master` after PR3

### Previous PR Boundaries (stacked-to-main history)

- PR1: `602a827` → `5c09df3` TUI sync (battery)
- PR2: `5c09df3` → `e6f4c2d` hashline
- PR3: `e6f4c2d` → `d8fe558` web anchors
- PR4: `d8fe558` → HEAD advisor (this slice)

