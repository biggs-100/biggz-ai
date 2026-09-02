# Archive Report: UI Pi Pretty v2

**Change**: `ui-pi-pretty-v2`
**Archived**: 2026-09-02
**Archived to**: `openspec/changes/archive/2026-09-02-ui-pi-pretty-v2/`
**Mode**: Standard (`strict_tdd: false`)
**Artifact Store**: `openspec`
**Delivery**: stacked-to-main — 5 slices (PR1 sync → PR2 pills → PR3 footer → PR4 diff → PR5 gallery/mouse/a11y), each <400 lines and revertible via `git revert HEAD`

## Summary

Polish Pi harness after 4 ports (sync, hashline, web anchors, advisor). Vanilla Pi minimal; oh-my-pi proves harness wins. Delivered 5 upgrades (<400 lines each) — streaming, pills, footer, diff, gallery/a11y — with no ledger/SDD change. All work is harness-only TUI/Pi layers.

Shipped as 5 stacked PRs to `main`:
- **PR1 9d5906e (sync)**: 16ms trailing `AfterFunc` throttle, idempotent CSI 2026, guard `isSyncSupported()`
- **PR2 94daa1f (pills)**: lipgloss tokens + `biggz-tool-pills.js` collapse `>3 → … +N`, spinner freeze
- **PR3 00360e7 (footer)**: `biggz-footer.js` + `extension-api.js` powerline `branch|change|lineage|lens 1/4|budget 1/1`, `›`→`▕`/`/` fallback
- **PR4 074c7fa (diff)**: `diff.go` + `sergi/go-diff` split >100 else unified, 1MB cap, word highlight
- **PR5 c3c201a (gallery/mouse/a11y)**: `scripts/gallery` 80/100c deterministic, `BIGGZ_MOUSE=1` opt-in, `BIGGZ_NO_ANIMATION`/`TERM=dumb` guards, `ansi.Strip`

Each slice stacked-to-main revertible (`git revert 5..1`), kill-switch `BIGGZ_PRETTY=0` and `BIGGZ_NO_ANIMATION=1` verified.

## Spec Compliance

**Verdict**: `PASS` (0 CRITICAL, 0 blockers)

Per `verify-report.md` `evidence_revision sha256:4e836e39eab82f7bbe105fad11979099d70c2e08a1af1acd903b82e0b3084bcf`, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `verify-report.md` `PASS 7/7 21/21`:

| Metric | Value |
|--------|-------|
| Requirements | `7/7` (tui 4 + pi-integration 3) |
| Scenarios | `21/21` compliant, 0 PARTIAL, 0 UNTESTED, 0 FAILING |
| Build | `go vet ./...` → exit 0 |
| Tests | `go test ./... -count=1 -timeout 180s` PASS + `node --test internal/assets/pi/*.test.mjs` 42/42 PASS + focused `TestSyncOutput` 12 PASS + `TestRenderDiff` 5 PASS + `TestPill` 3 PASS + `TestAnimation` 4 PASS |
| Gallery | `go run ./scripts/gallery && git diff --exit-code docs/gallery` exit 0, second run zero diff, 80/100 deterministic |
| Guards harness | `TERM=dumb` zero CSI/ANSI, `BIGGZ_PRETTY=0` plain, `BIGGZ_NO_ANIMATION=1` tick nil, `PI_SUBAGENT_CHILD=1` bypass, `BIGGZ_MOUSE` default off → opt-in verified |
| Tasks | `21/21` [x] (PR1 3/3, PR2 4/4, PR3 4/4, PR4 3/3, PR5 4/4, Verify 3/3) |
| Critical findings | 0 |

Compliance matrix (21 scenarios, all COMPLIANT):

| Requirement | Scenarios | Covering Tests | Result |
|-------------|-----------|----------------|--------|
| PRETTY-V2-TUI-01 — Throttled Sync 16ms/CSI 2026 | Throttle coalesces burst; Guard disables sync; Idempotent nesting | `TestSyncOutput_ThrottleCoalesceBurst` (3/16ms→1 CSI), `TestSyncOutput_Fallback_*`/`GuardPrettyOff`/`ThrottleGuardZeroCSI` (zero ESC), `TestSyncOutput_Idempotent*` (single ESC[?2026l) | ✅ |
| PRETTY-V2-TUI-02 — Tool Pills +N collapse | Collapse beyond 3; Spinner reduced-motion; Kill-switch plain | `TestPillCollapse` + `pills.test.mjs collapse 5->3+2 hidden`, `TestPillSpinnerStatic`/`TestAnimation*`+`pills.test.mjs spinner frozen`, `TestPillPrettyAndDumb` + `BIGGZ_PRETTY=0` harness | ✅ |
| PRETTY-V2-TUI-03 — Responsive Inline Word Diff | Split above threshold; Unified below; Cap and fallback | `TestRenderDiff_SplitAt120` (120→old│new), `TestRenderDiff_UnifiedAt80` (80 unified), `TestRenderDiff_CapFallback` 1.2MB cap + `TestRenderDiff_MalformedNoPanic` | ✅ |
| PRETTY-V2-TUI-04 — Gallery + Reduced-Motion/Dumb | Gallery matches viewport 80/100; Reduced-motion kills ticks/sync; Dumb strips ANSI | `scripts/gallery` `HelpOverlayWidth`+`VisibleWidth` zero diff, `tickCmd` nil + frames lack ESC ( `TestAnimation*`+`TestSyncOutput_Fallback`), `View ansi.Strip` + `pills.test.mjs dumb strips ANSI` zero `\x1b[` | ✅ |
| PRETTY-V2-PI-01 — Powerline Footer + Nerd fallback | Footer segment order; NerdFont fallback; Kill-switch disables footer | `biggz-footer.test.mjs order branch▕change▕lineage▕lens▕budget`, `nerd fallback ▕/ zero glyphs`, `kill-switch BIGGZ_PRETTY=0 no injection` | ✅ |
| PRETTY-V2-PI-02 — Pill Streaming via Extension API | Throttled incremental pill; Collapsible preserves order via API; Subagent child bypass | `extension-api 16ms throttle coalesce` single write, `collapsible 4->3+1 hidden order`, `PI_SUBAGENT_CHILD bypass` early return | ✅ |
| PRETTY-V2-PI-03 — Opt-In Mouse Gating | Default mouse off; Opt-in enables mouse; Guard overrides opt-in | `isMouseAllowed() false` when unset, true when `BIGGZ_MOUSE=1 && pretty && !dumb && !no-animation`, `isMouseAllowed() false` when `BIGGZ_MOUSE=1 + PRETTY=0/NO_ANIMATION/dumb` → no `enableMouse` | ✅ |

Design coherence verified: 16ms `AfterFunc` trailing coalesce vs `tea.Tick` jitter, lipgloss tokens in `styles.go` vs inline ANSI, `sergi/go-diff` `DiffMain` word-level vs `difflib`, separator registry `extension-api.js` vs hardcoded, `BIGGZ_MOUSE=1` opt-in vs always-on, `HelpOverlay(w)` vs golden snapshot — all per `design.md`.

## Spec Sync

Delta specs merged into main specs (source of truth) BEFORE archive move. ADDED requirements appended, no REMOVED (requires Reason/Migration) or RENAMED. Preserved all OTHER requirements.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| tui | **Updated** | 4 ADDED requirements (PRETTY-V2-TUI-01 Throttled Sync Streaming 3 scen + PRETTY-V2-TUI-02 Tool Pills +N 3 scen + PRETTY-V2-TUI-03 Responsive Diff 3 scen + PRETTY-V2-TUI-04 Gallery/Mouse/A11y 3 scen = 12 scenarios) appended to existing 16 requirements (Synchronized Output Rendering, Bracketed Paste Handling, Reduced-Motion/Gentleman-Cute, Model Picker, Screen Registration, Dashboard Tiles, Shared Table Styles, Animation/SyncOutput, Testing via teatest, POLISH-TUI-01..07) → now 20 requirements. Preserved 296→372 lines. Old requirements intact. | `openspec/specs/tui/spec.md` ✅ |
| pi-integration | **Updated** | 3 ADDED requirements (PRETTY-V2-PI-01 Powerline Footer 3 scen + PRETTY-V2-PI-02 Pill Streaming 3 scen + PRETTY-V2-PI-03 Opt-In Mouse 3 scen = 9 scenarios) appended to existing 6 requirements (Advisor Advise Mode, Synthesis Gate Verification, Question Envelope, POLISH-PI-01, POLISH-PI-02, REQ-PS4) → now 9 requirements. Preserved 118→175 lines. Old requirements intact. | `openspec/specs/pi-integration/spec.md` ✅ |

**Totals**: `7 ADDED requirements`, `21 scenarios` merged. No MODIFIED/REMOVED/RENAMED semantics needed (no such deltas in this change). Verification: `grep -n PRETTY-V2 tui/spec.md` shows 4 hits at 298/317/336/355; `grep -n PRETTY-V2 pi-integration/spec.md` shows 3 hits at 120/139/158; `grep -c Requirement:` tui 20 / pi-integration 9; `grep -c POLISH-TUI-07` 1 / `REQ-PS4` 1 still present.

## Archived Artifacts

All SDD artifacts preserved in `openspec/changes/archive/2026-09-02-ui-pi-pretty-v2/` (audit trail, never delete or modify):

| Artifact | Path | Status | Notes |
|----------|------|--------|-------|
| Proposal | `proposal.md` | ✅ 449w, 3.5K | Intent (5 harness upgrades <400 each, no ledger), scope (sync streaming/pills/footer/diff/gallery/mouse/a11y), approach (5 stacked PRs <400 auto-chain), Affected Areas, Risks, Rollback `BIGGZ_PRETTY=0`/`BIGGZ_NO_ANIMATION`, Success Criteria |
| Design | `design.md` | ✅ 783w, 6.2K | 6 architecture decisions (AfterFunc 16ms vs Tick, tokens vs ANSI, sergi/go-diff vs difflib, separator registry, mouse opt-in, HelpOverlay vs golden), data flow, File Changes (10 rows), Interfaces (`tui.go` `pendingFrame/syncMu/AfterFunc`, `RenderDiff`, `PillStyle`, `extension-api.js` `SEPARATORS/getSeparator`), Guards Matrix, Testing Strategy, Threat Matrix N/A |
| Specs (delta source) | `specs/tui/spec.md` | ✅ 615w, 4.8K, 4 req 12 scen | PRETTY-V2-TUI-01..04 deltas (source for merge → main) |
| Specs (delta source) | `specs/pi-integration/spec.md` | ✅ 455w, 3.5K, 3 req 9 scen | PRETTY-V2-PI-01..03 deltas (source for merge → main) |
| Tasks | `tasks.md` | ✅ 21/21 [x] | PR1 3/3 + PR2 4/4 + PR3 4/4 + PR4 3/3 + PR5 4/4 + Verify 3/3; 0 unchecked at archive (`grep -c "^- \[x\]"` 21, `grep -c "^- \[ \]"` 0) |
| Apply Progress | `apply-progress.md` | ✅ PR1-5, stacked-to-main, evidence_revision `794b70c` | Cumulative evidence chain `9d5906e→94daa1f→00360e7→074c7fa→c3c201a`, PR5 ~32 code lines + 38 gallery fixtures deterministic, `go vet`/`go test`/`node --test` PASS |
| Verify Report | `verify-report.md` | ✅ `verdict: pass`, `7/7` req `21/21` scen, `blockers: 0` `critical_findings: 0`, `evidence_revision sha256:4e836e39...` `build_output_hash sha256:e3b0c44...` | PASS at verify time, 0 CRITICAL, 3 SUGGESTIONs (fixtures generated, double-strip idempotent, load-time capture guarded by per-site `isMouseAllowed()`) |
| Archive Report | `archive-report.md` | ✅ (this file) | Merge + archive confirmation |

Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS). Active changes directory no longer contains `ui-pi-pretty-v2` (verified via `ls openspec/changes/` → only `archive/`).

## Task Completion Gate

- **Persisted tasks artifact**: `openspec/changes/archive/2026-09-02-ui-pi-pretty-v2/tasks.md` (moved from `openspec/changes/ui-pi-pretty-v2/tasks.md`)
- **Check**: `grep -c "^- \[x\]"` → 21, `grep -c "^- \[ \]"` → 0. All 21 tasks `[x]` (Phase1 1.1-1.3 3/3, Phase2 2.1-2.4 4/4, Phase3 3.1-3.4 4/4, Phase4 4.1-4.3 3/3, Phase5 5.1-5.4 4/4, Phase6 6.1-6.3 3/3). No stale checkboxes for completed work.
- **Reconciliation**: No exceptional mechanical reconciliation needed. `tasks.md` at HEAD prior to archive had 0 `[ ]` with proof in `apply-progress.md` FINAL and `verify-report.md` PASS; `sdd-verify` reports PASS 7/7 21/21; `sdd-archive` validates 21/21 before move.
- **Gate**: PASS — `sdd-apply` marked completed tasks correctly; `sdd-archive` validates no stale unchecked tasks. No blocker.
- **Active changes verification**: `ls openspec/changes/` shows only `archive/` subdirectory, no active `ui-pi-pretty-v2`. `ls -R archive/2026-09-02-ui-pi-pretty-v2` shows 7 artifacts + specs (2 domains).

## Verification Evidence (Final State)

Final-state facts per `verify-report.md` at `2026-09-02` and repository evidence at archive time, highest rank:

- **Build**: `go vet ./...` exit 0 (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` empty output — clean). `build_command: go vet ./...` `build_exit_code: 0`.
- **Tests (authoritative, `evidence_revision sha256:4e836e39eab82f7bbe105fad11979099d70c2e08a1af1acd903b82e0b3084bcf`)**:
  - `go test ./... -count=1 -timeout 180s` → PASS `ok` all 40+ packages (cmd/biggz 85s, internal/tui 11s, screens 8s, styles 0.6s, sdd 20s, etc.)
  - `node --test internal/assets/pi/*.test.mjs` → 42 tests PASS (footer 6, pills 5, synthesis-gate 23, web-search 8) 285ms
  - `go test ./internal/tui -run TestSyncOutput -count=1 -v` → 12 PASS (ThrottleCoalesceBurst 3→1 CSI, Fallback_TermDumb/NoAnimation/Gentle/PrettyOff/GuardPiSubagent zero CSI, Idempotent/DoubleWrap single ESC[?2026l, MarkersPresent)
  - `go test ./internal/tui -run TestRenderDiff -count=1 -v` → 5 PASS (SplitAt120 two cols, UnifiedAt80 single col, CapFallback 1MB, MalformedNoPanic)
  - `go test ./internal/tui/screens -run TestPill/TestAnimation` → 3 PASS pill collapse + 4 PASS animation tick nil / spinner static
  - `go run ./scripts/gallery && git diff --exit-code docs/gallery` → exit 0, second run zero diff (19 files 80.ansi + 19 files 100.ansi deterministic, VisibleWidth<=w via `TruncateToWidth`)
  - Guards harness: `TERM=dumb go test ./internal/tui -run TestSyncOutput` PASS zero ANSI, `BIGGZ_PRETTY=0` plain, `BIGGZ_NO_ANIMATION=1` tick nil, `PI_SUBAGENT_CHILD=1` bypass PASS, `BIGGZ_MOUSE` unset→false / `BIGGZ_MOUSE=1`→true only when pretty/dumb/animation guards pass
- **PR Line Budget (<400 each, stacked-to-main)**:
  - PR1 9d5906e (sync) 284 ins /27 del 4 files ✅ <400 (tui.go + tui_test.go + screens/help.go + scripts/gallery)
  - PR2 94daa1f (pills) 342 ins /41 del 6 files ✅ <400 (styles.go+polish.go+pills.go+pills_test.go+biggz-tool-pills.js)
  - PR3 00360e7 (footer) 284 ins /114 del 4 files ✅ <400 (biggz-footer.js+extension-api.js+footer.test.mjs)
  - PR4 074c7fa (diff) 389 ins 5 files ✅ <400 (diff.go+diff_test.go+review.go+go.mod/sum)
  - PR5 c3c201a (gallery/mouse/a11y) 36 ins/5 del 3 code files ✅ <400 (code-only; 41 files 804/1034 with fixtures, fixtures generated not counted)
  - Each slice one linear commit on `master`, revertible via `git revert HEAD` or `git revert 5..1` reverse; kill-switches `BIGGZ_PRETTY=0`/`BIGGZ_NO_ANIMATION=1` verified.
- **Hashes (ledger-not-required, filesystem verify)**: `evidence_revision sha256:4e836e39eab82f7bbe105fad11979099d70c2e08a1af1acd903b82e0b3084bcf` (combined `go test + node --test` output), `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (`go vet` empty), `test_output_hash` same as `evidence_revision`. No ledger binding required for `openspec` filesystem archive (task: "No ledger reset needed, just archive").
- **Ledger**: Task attests "No ledger reset needed, just archive" — archival is mechanical filesystem operation, not ledger-settled delivery. Verify evidence is filesystem `evidence_revision` hash, not `biggz sdd-attempt settle`.

## Residual Risks

| Risk | Severity | Note / Mitigation |
|------|----------|-------------------|
| Gallery fixtures are generated assets (38 files 80/100 ansi) counted as 804+/1034- in PR5 diff | INFO | Code-only <400 verified; fixtures deterministic via `go run ./scripts/gallery` second run zero diff. Review skips fixture diff as code review counts code-only. Mitigated. |
| `tui.go` View `ansi.Strip(syncOutput(frame))` when `TERM=dumb` double-strip (syncOutput already strips) | SUGGESTION | Idempotent no-op, no double-wrap due to `Contains` check — safe. Not a blocker. |
| `biggz-question-mouse.js` `const BIGGZ_MOUSE = isMouseAllowed()` evaluated at module load | SUGGESTION | Dynamic per-site `isMouseAllowed()` at all 3 patch sites ensures correctness after env change in tests; no stale capture. Mitigated. |
| None — 0 CRITICAL, 0 blockers at verify time; all guards (`BIGGZ_PRETTY=0`, `BIGGZ_NO_ANIMATION`, `TERM=dumb`, `PI_SUBAGENT_CHILD`, `BIGGZ_MOUSE=1` opt-in) verified. | None | No open risks for Next Steps beyond next change. |

## Source of Truth Updated

The following specs now reflect the shipped behavior (preserved requirements unchanged, new requirements merged before archive):

- `openspec/specs/tui/spec.md` — **Updated**, now 20 requirements (16 existing + 4 PRETTY-V2-TUI-01..04) — 7 ADDED scenarios verified, old TUI requirements (`Synchronized Output Rendering`, `Bracketed Paste Handling`, `Reduced-Motion/Gentleman-Cute`, `Model Picker`, `Screen Registration`, `Dashboard Tiles`, `Shared Table Styles`, `Animation/SyncOutput`, `Testing via teatest`, `POLISH-TUI-01..07`) preserved
- `openspec/specs/pi-integration/spec.md` — **Updated**, now 9 requirements (6 existing + 3 PRETTY-V2-PI-01..03) — old Pi requirements (`Advisor Inline Watchdog`, `Synthesis Gate Verification`, `Question Envelope`, `POLISH-PI-01`, `POLISH-PI-02`, `REQ-PS4`) preserved

Other main specs (`agent-install`, `agent-registry`, `bigmem`, `cli`, `complexity-gates`, `component-catalog`, `core-review`, `extension-api`, `filemerge`, `orchestrator`, `pi-web-search`, `planner`, `plugin-system`, `policy`, `review-authority`, `review-gates`, `review-lenses`, `review-lifecycle`, `runtime`, `sdd`, `state-persistence`, `system-diagnostics`, etc.) unchanged and preserved.

## SDD Cycle Complete

Change `ui-pi-pretty-v2` has been fully planned, implemented, verified, and archived:

`proposal` (449w intent 5 harness upgrades) → `spec` (2 deltas: tui 4 req + pi-integration 3 req = 7/21) → `design` (783w, 6 decisions, 10 file rows, Guard Matrix) → `tasks` (21, 5 PR slices stacked-to-main within 400 budget, auto-chain) → `apply` (21/21 tasks: PR1 9d5906e sync → PR2 94daa1f pills → PR3 00360e7 footer → PR4 074c7fa diff → PR5 c3c201a gallery/mouse/a11y, each `go vet` + focused `go test` + `node --test` green before merge) → `verify` (PASS 7/7 21/21, `go vet` exit 0 + `go test ./...` + `node --test` 42/42 + gallery deterministic, 0 CRITICAL) → `archive` (2 delta→main sync + mechanical folder move `openspec/changes/ui-pi-pretty-v2/` → `openspec/changes/archive/2026-09-02-ui-pi-pretty-v2/` + this report).

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks. Audit trail preserved in `openspec/changes/archive/2026-09-02-ui-pi-pretty-v2/` — never delete or modify archived changes.

## Commands Run (Archive Phase)

- `cat openspec/changes/ui-pi-pretty-v2/tasks.md | grep -E "^- \[ \]"` → 0 unchecked, `grep -c "^- \[x\]"` → 21 ✅ Task Gate PASS
- `read openspec/specs/tui/spec.md` (296 lines) + `read openspec/specs/pi-integration/spec.md` (118 lines) → preserved originals verified
- `edit openspec/specs/tui/spec.md` append PRETTY-V2-TUI-01..04 (76 lines ADDED, 4 req 12 scen) → `grep -n PRETTY-V2` 4 hits 298/317/336/355, `grep -c Requirement:` 20, `grep -c POLISH-TUI-07` 1, `wc -l` 372 ✅
- `edit openspec/specs/pi-integration/spec.md` append PRETTY-V2-PI-01..03 (57 lines ADDED, 3 req 9 scen) → `grep -n PRETTY-V2` 3 hits 120/139/158, `grep -c Requirement:` 9, `grep -c REQ-PS4` 1, `wc -l` 175 ✅
- `mv openspec/changes/ui-pi-pretty-v2 openspec/changes/archive/2026-09-02-ui-pi-pretty-v2` → exit 0, `ls -la openspec/changes/` shows only `archive/`, `ls -R archive/2026-09-02-ui-pi-pretty-v2` shows 6 artifacts + specs (2 domains) ✅
- `write archive-report.md` → this file, 21/21 tasks evidence, 7/7 21/21 compliance, hashes `sha256:4e836e39...`/`sha256:e3b0c44...`, rollback boundaries (5 PRs stacked-to-main)
- Verification readback: `git diff --stat HEAD -- openspec/specs/tui/spec.md openspec/specs/pi-integration/spec.md` → `2 files 133 insertions(+)`, `git status --short` shows spec sync `M` + archive untracked `??` as expected for untracked change folder mechanical move, `ls openspec/changes/archive/2026-09-02-ui-pi-pretty-v2/` confirms proposal/design/specs/tasks/apply-progress/verify-report/archive-report
- No ledger operation per task contract: "No ledger reset needed, just archive" — filesystem archive only

