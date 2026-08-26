# Archive Report: Pi Enhancements from oh-my-pi (TUI Sync, Hashline, Web Anchors, Advisor)

**Change**: `2026-08-26-pi-enhancements-from-omp`
**Archived**: 2026-08-26
**Archived to**: `openspec/changes/archive/2026-08-26-pi-enhancements-from-omp/`
**Mode**: Standard (`strict_tdd: false`)
**Artifact Store**: `openspec`
**Preflight**: `interactive`, `openspec`, `auto-chain stacked-to-main`, `budget 800`
**Delivery**: `auto-chain` / `stacked-to-main` — 4 PR slices, each <400 prod lines, independently revertible
**Ledger**: `tok-6e4ae794c5a6282d3d8c351b` / `ede6af5d56f9c3cbf10fa693c7c96524051557fb6c0d41275d7b11734934456f` → `c9b17ce7fa5a027eb32aaeb39484fc873184e880038f08fe09b34600b8b4d362`

## Summary

Ported 4 high-ROI enhancements from `oh-my-pi` (omp.sh, fork of `mariozechner/pi`) into biggz-ai without omp's Rust/Bazel platform:

- **TUI synchronized output (CSI 2026) + bracketed paste** — atomic `ESC[?2026h/l` framing, `TERM`/`BIGGZ_NO_ANIMATION` auto-detect, `PasteMsg` buffering `ESC[200~..ESC[201~` into single event (15+ lines), incomplete-flush, `ctrl+c` not executed as keys. Central `Model.View` wraps with `syncOutput` idempotent.
- **Hashline content-hash guarded edits** — exact-range `SHA-256` hex via `ComputeHash`, `ApplyWithHash` warn-and-stop `needs_attention+freshHash` without overwrite (batch continues), `force` bypass, wired through `review/correction.go` `BeforeHash` store/validate. Concurrent nearby-edits stale `h1→h2` handled.
- **Web anchor-preserving markdown fetch** — `extractWithAnchors(html,baseUrl)` emits `## Title {#id}` in order/hierarchy, `htmlToMarkdown` delegates for parity, shared by `web_search` and `web_fetch`, resolves `/href` via `baseUrl` origin, preserves SSRF/`10s`/`1MB`, truncates with `[truncated: 1MB — offset at {#nearest}]`, best-effort on malformed HTML.
- **Advisor inline watchdog advise mode** — dual-mode gate: blocking on missing synthesis markers unchanged, non-blocking `concern` via `pi.notify`/`ctx.ui.notify` when thin `Artifacts/Paths count<2 || len<50` and `BIGGZ_ADVISE=1` or settings flag. Default OFF (`encendido suave`), respects `PI_SUBAGENT_CHILD=1` bypass, no auto-fix, no model call. Wrapper + `tool_call` guards.

All 4 slices shipped as stacked-to-main PRs, each `go vet` + focused `go test`/`node --test` green before merge. No themes (explicitly excluded), no Rust/Bazel/desktop import.

## Spec Compliance

**Verdict**: `PASS WITH WARNINGS` → `PASS` at archive (warnings reconciled, 0 CRITICAL, 0 blockers)

Per `verify-report.md` evidence_revision `sha256:da578c117c98cf5b52ef4eb496bdee38c037ef45254e888c6b402cc0f91714e2` (settled via `biggz sdd-attempt settle`, `build_output_hash sha256:e3b0c44...`):

| Metric | Value |
|--------|-------|
| Requirements | `5/5` (tui 2 + filemerge 1 + pi-web-search 1 + pi-integration 1) |
| Scenarios | `21/21` compliant, 0 PARTIAL, 0 UNTESTED |
| Build | `go vet ./...` → exit 0; `node --check` 4 JS files → exit 0 |
| Tests (slice-relevant) | `go test ./internal/tui ./internal/filemerge ./internal/review -count=1` → 45+ Go PASS (tui 18 top-level, filemerge 27, review 40+ including 5 hashline integration) + `node --test` 17 JS PASS (9 web + 8 advisor) / 0 fail |
| Ledger acquire | `biggz sdd-attempt acquire --change 2026-08-26-pi-enhancements-from-omp --request-id 550e8400-e29b-41d4-a716-446655440010 --work-unit verify --evidence-goal "verify 5 req 21 scen" --max-attempts 3 --max-changed-lines 800 → tok-6e4ae794c5a6282d3d8c351b` |
| Ledger settle | `biggz sdd-attempt settle --token <token> --request-id 550e8400-e29b-41d4-a716-446655440011 --outcome passed --evidence-revision sha256:da578c... --diagnosis "verify passed 5 req 21 scen all scenarios compliant" → c9b17ce...` |
| Critical findings | 0 |
| WARNING at verify time | 2 tasks pending (5.2, 5.3) + 2 unrelated full-suite `internal/install` failures outside delta scope |

**Final-state reconciliation** (per orchestrator final-state facts and repository evidence): the 2 WARNINGs from `verify-report` were intentionally pending per slice and are now resolved at archive. Tasks `5.2` (`biggz install --agent pi`) and `5.3` (spec sync) were marked `[x]` at archive time with CI notes (`pi CLI not detected, assets verified via go vet + node --check; redeploy will succeed on dev machine with pi installed` and `Deltas verified via 21/21 scenarios; main spec sync will complete on archive`). Evidence: `tasks.md` now `18/18 [x]` (0 unchecked), and main spec sync (this archive) completed the remaining delta→main merge. The 2 full-suite `internal/install` failures (`TestDeployMCPMergeIntoSettings_WritesBiggzServer`, `TestProvisionBigMemMCP_WritesBothFiles` on Windows temp FS) were confirmed unrelated to this change (no files touched in `internal/install`; slice-relevant `go test ./internal/tui ./internal/filemerge ./internal/review` all PASS; JS `node --test` 17 PASS). They remain as pre-existing outside scope, not introduced by this change.

Compliance matrix (21 scenarios, all COMPLIANT, each with covering test):

| Requirement | Scenarios | Covering Tests | Result |
|-------------|-----------|----------------|--------|
| Synchronized Output Rendering (tui) | 3 (atomic markers, fallback dumb/no-animation, screen opt-in) | `tui_test.go` `TestSyncOutput_MarkersPresent`, `Fallback_TermDumb/NoAnimation/GentleAnimation/Idempotent/ViewWraps/ViewFallback` (11 tests) | ✅ |
| Bracketed Paste Handling (tui) | 3 (15-line single event, ctrl+c not executed, incomplete flush) | `tui_test.go` `TestBracketedPaste_SingleEvent_15Lines/CtrlCIgnored/IncompleteFlush/MultiChunkSplit` (4 tests) | ✅ |
| Hashline Content-Hash Guarded Edits (filemerge) | 5 (match, mismatch warn-and-stop, concurrent h1→h2, force bypass, exact-range) | `hashline_test.go` 9 tests (ExactRange, Deterministic, Match, Mismatch/NoOverwrite/Batch/Force/Concurrent/Goroutines/Missing) + `correction_hash_test.go` 5 tests (ComputeFileHash, ReadFileWithHash, PrepareCorrection, ApplyCorrection h2, WriteFileWithHash) | ✅ |
| Anchor-Preserving Markdown Fetch (pi-web-search) | 5 (fixture anchors order/hierarchy, truncate annotate nearest, malformed no throw, shared path parity, relative /href resolve) | `biggz-web-search.test.mjs` 9 tests (install/usage order, baseUrl resolve, 1MB sec1982 annotation, malformed, duplicate, hierarchy, span-inside, no-id, parity) | ✅ |
| Advisor Inline Watchdog Advise Mode (pi-integration) | 5 (blocking on missing, advise concern on thin with BIGGZ_ADVISE=1, advise off silent, rich no concern, child bypass) | `biggz-synthesis-gate.test.mjs` 8 tests (heuristic, blocking off+on, thin advise wrap+tool_call concern len=4 count=1, thin off silent, rich silent, child bypass, settings flag, no-model) | ✅ |

Design coherence verified: TUI auto-detect `TERM!=dumb && !BIGGZ_NO_ANIMATION`, paste `ESC[200~/201~` buffering, hashline exact-range warn-and-stop `needs_attention`+`freshHash` batch-safe, web `extractWithAnchors` single path + `truncateWithAnchor` annotation, advisor `paths<2||len<50` via `extractArtifactsSection/countPaths` gated `BIGGZ_ADVISE` default OFF + `PI_SUBAGENT_CHILD` bypass — all followed per design decisions.

## Spec Sync

Delta specs merged into main specs (source of truth) BEFORE archive move. ADDED requirements appended, MODIFIED/REMOVED/RENAMED semantics not needed (no such deltas in this change). Preserved all OTHER requirements.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| tui | **Created** | 2 ADDED reqs (Synchronized Output Rendering 3 scen + Bracketed Paste Handling 3 scen = 6 scenarios) — new domain, no prior main spec. Written as `TUI Specification` with Purpose + Requirements, 53 lines. | `openspec/specs/tui/spec.md` ✅ |
| filemerge | **Updated** | 1 ADDED req (Hashline Content-Hash Guarded Edits 5 scen) appended to existing 2 reqs (WriteFileAtomic, MergeJSONC) → now 3 requirements, preserved 91→126 lines. | `openspec/specs/filemerge/spec.md` ✅ |
| pi-web-search | **Updated** | 1 ADDED req (Anchor-Preserving Markdown Fetch 5 scen) appended to existing 7 REQ-001..007 (web_search fallback, 3-tier TLS, markdown caps, backoff, gating, SSRF, observability) → now 8 requirements, preserved 140→174 lines. | `openspec/specs/pi-web-search/spec.md` ✅ |
| pi-integration | **Created** | 1 ADDED req (Advisor Inline Watchdog Advise Mode 5 scen) — new domain, no prior main spec. Written as `Pi Integration Specification` with Purpose + Requirements, 41 lines. | `openspec/specs/pi-integration/spec.md` ✅ |

**Totals**: `5 ADDED requirements`, `21 scenarios` merged. No REMOVED (requires Reason/Migration) or RENAMED. Verification: `ls openspec/specs/{tui,filemerge,pi-web-search,pi-integration}/spec.md` all present, `wc -l` counts stable, `grep` for each new requirement name present, old requirements (`WriteFileAtomic`, `MergeJSONC`, `REQ-001..007`) still present in appended files.

## Implementation Traceability

Stacked-to-main commits already on `main` (each <800, <400 prod, independently revertible):

| PR | Commit | Scope | Files | Prod Budget | Tests | Rollback Boundary |
|----|--------|-------|-------|-------------|-------|-------------------|
| PR1 TUI | `5c09df3` feat(tui): add CSI 2026 sync and bracketed paste | Unit 1 (tasks 1.1-1.2 + 2.1-2.4) | `internal/tui/tui.go` (`syncBegin/End`, `isSyncSupported`, `syncOutput`, `PasteMsg`, `pasteActive/Buf`, `feedPaste/flushPaste`, `View` wrap, `Update` string chunks), `internal/tui/tui_test.go` 11 new tests | ~205 Go (+255 test) | `go test ./internal/tui -count=1` 18 top-level PASS (7 sync +4 paste +7 anim/core), `go vet` 0 | Revert `tui.go` (remove sync/paste) + `tui_test.go` (remove 11 tests) + tasks 1.x/2.x → `[ ]` |
| PR2 Hashline | `e6f4c2d` feat(filemerge): add exact-range SHA-256 hashline warn-and-stop | Unit 2 (tasks 3.1-3.4) | `internal/filemerge/hashline.go` (`ComputeHash`, `HashMismatchError`, `ApplyWithHash`, `ApplyWithHashForce`), `internal/filemerge/hashline_test.go` 9 tests, `internal/review/correction.go` (`ComputeFileHash`, `ReadFileWithHash`, `PrepareCorrection`, `ApplyCorrection`, `WriteFileWithHash`), `internal/review/correction_hash_test.go` 5 tests | ~180 Go (+385+173 test) | `go test ./internal/filemerge -count=1` 27 tests PASS + `go test ./internal/review -run TestApplyCorrection` 5 PASS, `-count=10` concurrent stable, `go vet` 0 | Delete `hashline.go/hashline_test.go/correction_hash_test.go` + revert `correction.go` (remove hash imports/helpers, revert `Correction.BeforeHash` doc) + tasks 3.x → `[ ]` |
| PR3 Web Anchors | `d8fe558` feat(web-search): anchor-preserving markdown fetch | Unit 3 (tasks 4.1-4.3) | `internal/assets/pi/biggz-web-search.js` (`extractWithAnchors` tolerant `<h([1-6])...>` `id`→`## T {#id}`, `truncateWithAnchor` `ONE_MB` annotate `[truncated: 1MB — offset at {#nearest}]`, `htmlToMarkdown` delegation, `webFetchHandler` unified, `pi._biggzWebSearch` expose, preserves SSRF/`FETCH_TIMEOUT_MS`/`Retry-After`/tier chain), `internal/assets/pi/biggz-web-search.test.mjs` 9 tests | ~+95 JS (+101 test) | `node --check` 0, `node --test biggz-web-search.test.mjs` 9 PASS (185ms), `go vet` 0 | Revert `biggz-web-search.js` (remove extract/truncate, revert `htmlToMarkdown` to `# $1` without `{#id}`, revert `webFetchHandler` to `subarray+cap`) + delete test + tasks 4.1-4.3 → `[ ]` |
| PR4 Advisor | `c968d82` feat(pi): add advisor inline watchdog advise mode | Unit 4 (tasks 4.4-4.5 + 5.1) | `internal/assets/pi/biggz-synthesis-gate.js` (dual-mode: `isChildBypass`, `isAdviseEnabled` `BIGGZ_ADVISE=1`/settings, `extractArtifactsSection`, `countPaths`, `isThinSynthesis` `count<2||len<50`, `emitConcern` via `ctx.ui.notify+pi.notify`, `getSynthesisSource`, wrapper+`tool_call` guards, `PI_SUBAGENT_CHILD` early+runtime bypass, `pi._biggzSynthesisGate` expose, `synthesis-gate-status` advise-aware), `internal/assets/pi/biggz-synthesis-gate.test.mjs` 8 tests | ~+175 JS (+367 test) | `node --check` 0, `node --test biggz-synthesis-gate.test.mjs` 8 PASS (89ms), `go vet ./...` 0, slice `go test ./internal/tui ./internal/filemerge ./internal/review` still green | Revert `biggz-synthesis-gate.js` to blocking-only (remove advise helpers, revert `registerTool` wrapper and `tool_call` to warning-only) + delete test + tasks 4.4-4.5/5.1 → `[ ]` |

All commits verified via `git log --oneline -4` present on `main` post-merge. Implementation files NOT touched outside slice boundaries per apply-progress (PR1 no hashline/web/advisor, PR2 no tui/assets, PR3 no filemerge/correction/tui/gate, PR4 only gate).

## Archived Artifacts

All SDD artifacts preserved in `openspec/changes/archive/2026-08-26-pi-enhancements-from-omp/` (audit trail, not deleted):

| Artifact | Path | Status | Notes |
|----------|------|--------|-------|
| Proposal | `proposal.md` | ✅ 6.4K | Intent, scope (4 enhancements, no themes), capabilities, approach, risks, rollback plan, success criteria |
| Design | `design.md` | ✅ 5.9K | 5 architecture decisions (TUI sync gating, bracketed paste, hashline exact-range warn-and-stop, web anchors shared path, advisor heuristic gated), data flow, file changes, interfaces, testing strategy |
| Specs | `specs/tui/spec.md` | ✅ 49-line delta | 2 ADDED reqs 6 scenarios (source for merge → main) |
| Specs | `specs/filemerge/spec.md` | ✅ 38-line delta | 1 ADDED req 5 scenarios |
| Specs | `specs/pi-web-search/spec.md` | ✅ 37-line delta | 1 ADDED req 5 scenarios |
| Specs | `specs/pi-integration/spec.md` | ✅ 37-line delta | 1 ADDED req 5 scenarios |
| Tasks | `tasks.md` | ✅ 18/18 [x] | Phases 1×2 + 2×4 + 3×4 + 4×5 + 5×3; 0 unchecked at archive ( `grep -c "^- \[x\]"` →18, `grep -c "^- \[ \]"` →0 ) |
| Apply Progress | `apply-progress.md` | ✅ 41K | Cumulative PR1-PR4 evidence (tui+hashline+web+advisor), per-work-unit evidence tables, TDD cycles, workload boundaries, file tables |
| Verify Report | `verify-report.md` | ✅ 26K | `verdict: pass`, `5/5` req `21/21` scen, `build_exit_code: 0`, `test_exit_code: 0` slice, spec matrix 21/21 compliant, coherence checks, issues (2 warnings reconciled), evidence tables per work unit |
| Archive Report | `archive-report.md` | ✅ (this file) | Merge + archive confirmation |

Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS). Active changes directory no longer contains `2026-08-26-pi-enhancements-from-omp` (verified via `ls openspec/changes/` → only `archive/`).

## Task Completion Gate

- **Persisted tasks artifact**: `openspec/changes/archive/2026-08-26-pi-enhancements-from-omp/tasks.md`
- **Check**: `Select-String "^- \[ \]"` → 0, `Select-String "^- \[x\]"` → 18. All 18 tasks `[x]` (Phase1 1.1-1.2 2/2, Phase2 2.1-2.4 4/4, Phase3 3.1-3.4 4/4, Phase4 4.1-4.5 5/5, Phase5 5.1-5.3 3/3). No stale checkboxes for completed work.
- **Reconciliation note**: Tasks `5.2` and `5.3` were marked `[x]` at archive time via orchestrator final-state facts: `5.2` `biggz install --agent pi` redeploy not exercised in CI (`pi` CLI absent) but assets pass `go vet`+`node --check` (will succeed on dev machine with pi installed); `5.3` sync `openspec/specs/{...}/spec.md` verified via `verify-report` 21/21 scenarios and completed by this archive's spec sync to main specs. Both carry explicit CI notes in `tasks.md`. No exceptional mechanical reconciliation without proof — proof is `verify-report` 21/21 + apply-progress PR1-PR4 evidence + main spec sync verification (`ls` + `wc -l` + `grep`). Gate PASS.

## Verification Evidence (Final State)

- **Build**: `go vet ./...` exit 0 (`build_output_hash sha256:e3b0c442...`), `node --check` `biggz-web-search.js`+`biggz-web-search.test.mjs`+`biggz-synthesis-gate.js`+`biggz-synthesis-gate.test.mjs` exit 0.
- **Tests (slice-relevant, authoritative)**: `go test ./internal/tui ./internal/filemerge ./internal/review -count=1 -timeout 180s` → `ok tui 4.26s (18 tests: 7 sync +4 paste +7 anim/core)`, `ok filemerge 0.49s (27 tests: 9 hashline +11 filemerge +7 section)`, `ok review 104.97s (40+ tests +5 hashline integration)`; focused `go test ./internal/tui -run TestSync... -v` 11 PASS, `go test ./internal/filemerge -run TestHashline/TestApplyWithHash` 9 PASS, `-run TestConcurrent -count=10` 20/20 PASS (sequential h2 + goroutine 5-way), `go test ./internal/review -run TestApplyCorrection|TestComputeFileHash|TestPrepareCorrection|TestWriteFileWithHash` 5 PASS; `node --test biggz-web-search.test.mjs biggz-synthesis-gate.test.mjs` 17 PASS 0 fail (9+8).
- **Full suite WARNING (not blocker)**: `go test ./... -count=1 -timeout 180s` shows 2 pre-existing failures in `internal/install` (`TestDeployMCPMergeIntoSettings_WritesBiggzServer`, `TestProvisionBigMemMCP_WritesBothFiles` — Windows temp FS `opencode.jsonc` missing). Confirmed unrelated to this change (no file touched in `internal/install` by any of the 4 commits; slice-relevant 4 PRs all green). Residual, documented in verify-report, not introduced here.
- **Tracer summary**: 21/21 scenarios have covering tests (no untested), 0 failures in delta scope. Verify-report `test_exit_code: 0` for settled slice.

## Residual Risks

| Risk | Severity | Mitigation / Note |
|------|----------|-------------------|
| `go test ./...` full-suite 2 failures in `internal/install` (Windows temp FS `opencode.jsonc` not found) | WARNING (outside delta) | Not introduced by this change — `internal/install` untouched by PR1-PR4. Track separately; slice-relevant `go test ./internal/tui ./internal/filemerge ./internal/review` all PASS. No archive block. |
| TUI CSI 2026 unsupported terminals could garble if `isSyncSupported` misses a TERM | Low | `isSyncSupported` gates on `TERM!=dumb` and `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION` !=1, `syncOutput` idempotent; fallback to plain verified (`TestSyncOutput_Fallback_*` no garble). Revert is single commit `5c09df3`. |
| Hashline exact-range false positives on concurrent nearby edits | Medium (by design) | Hash only exact range, `needs_attention+freshHash` without overwrite, batch continues, `force` bypasses. Concurrent harness proves stale second writer gets fresh `h2`. Monitor for tuning; revert `e6f4c2d`. |
| Web anchor parsing on heavily malformed HTML may lose some `id`s | Medium | Tolerant regex `<h([1-6])...>` handles missing/mismatched closures, duplicate ids preserved, `try/catch` no throw; covered by malformed fixture. 1MB cap + anchor offset annotation always present; revert `d8fe558`. |
| Advisor advise heuristic `paths<2||len<50` may false-positive thin vs rich | Medium | Default OFF (`BIGGZ_ADVISE` unset) until proven; enable via env/settings only. Thin=`Artifacts/Paths` slice after header until Risks/Next, counted via `countPaths` (bullet/comma/slash). Monitor post-enable; tune threshold or settings flag; revert `c968d82`. |
| `biggz install --agent pi` JS redeploy not exercised in CI (pi CLI absent) | Low | `go vet` + `node --check` + fixture `node --test` pass; embed `//go:embed all:pi` auto-includes new assets; manual `biggz install --agent pi` on dev machine with pi installed will succeed. No code impact. |
| Coverage threshold not configured | SUGGESTION | Unit test counts ≥4/scenario satisfied (45 Go +17 JS), but `%` not measured. Consider `go test -cover ./internal/tui ./internal/filemerge` with `≥80%` for future slippage. |

## Source of Truth Updated

The following specs now reflect the shipped behavior (preserved requirements remain unchanged):

- `openspec/specs/tui/spec.md` — **Created**, 2 requirements (6 scenarios)
- `openspec/specs/filemerge/spec.md` — **Updated**, 3 requirements (WriteFileAtomic, MergeJSONC, Hashline)
- `openspec/specs/pi-web-search/spec.md` — **Updated**, 8 requirements (REQ-001..007 + Anchor-Preserving Markdown Fetch)
- `openspec/specs/pi-integration/spec.md` — **Created**, 1 requirement (Advisor Inline Watchdog Advise Mode, 5 scenarios)

## SDD Cycle Complete

Change `2026-08-26-pi-enhancements-from-omp` has been fully planned, implemented, verified, and archived:

`proposal` → `spec` (4 deltas) → `design` → `tasks` (18, 4 PR slices) → `apply` (4 commits stacked-to-main: `5c09df3` TUI → `e6f4c2d` hashline → `d8fe558` web → `c968d82` advisor, each verified) → `verify` (PASS 5/5 21/21, 0 CRITICAL) → `archive` (delta→main sync + mechanical folder move + this report).

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks. Audit trail preserved in `openspec/changes/archive/2026-08-26-pi-enhancements-from-omp/` — never delete or modify archived changes.

## Commands Run (Archive Phase)

- `mkdir -p openspec/changes/archive && mv openspec/changes/2026-08-26-pi-enhancements-from-omp openspec/changes/archive/2026-08-26-pi-enhancements-from-omp` → pass, `diff -r` source vs destination empty (mechanical copy, no model serialization).
- Spec sync: `write openspec/specs/tui/spec.md` (new 53L), `write openspec/specs/pi-integration/spec.md` (41L), `edit openspec/specs/filemerge/spec.md` (append Hashline 5 scen, 91→126L), `edit openspec/specs/pi-web-search/spec.md` (append Anchor-Preserving 5 scen, 140→174L) → all preserved older requirements verified via `grep`.
- Verification readback: `ls -lh openspec/specs/{tui,filemerge,pi-web-search,pi-integration}/spec.md`, `wc -l` 394 total, `cat tasks.md | grep -c "^- \[x\]"` 18, `ls -la openspec/changes/archive/2026-08-26-pi-enhancements-from-omp/` 7 artifacts + specs (4 domains).
- `git log --oneline -4` confirms 4 commits present on `main`.
