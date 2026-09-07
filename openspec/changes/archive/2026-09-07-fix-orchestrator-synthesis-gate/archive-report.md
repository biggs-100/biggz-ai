# Archive Report — fix-orchestrator-synthesis-gate

**Change**: `fix-orchestrator-synthesis-gate`
**Archived to**: `openspec/changes/archive/2026-09-07-fix-orchestrator-synthesis-gate/`
**Archive date**: 2026-09-07 (ISO, UTC)
**Mode**: `openspec` / stacked-to-main (PR1 → PR3 auto-chain, apply-progress PR1+PR2+PR3)
**Verdict**: PASS — 6/6 requirements, 32/32 scenarios, 14/14 tasks (artifact) / 12/12 per snapshots, `sdd-verify-validate` PASS, ledger sha256:263aaece71a13d9eb3898e4a64486267df7937fc9e3b0b1cea9c20d2197732fe
**Production code modified by archive**: none (spec sync + folder move + this report only)

## Final-State Authority

This report is the terminal record at close. Intermediate snapshots are history, not current state:

1. **Native review authority** — no `reviewGate` receipt governs this change; no explicit review artifact failed validation. Treated as `disabled/unmanaged` (demanding a receipt while no review governs would deadlock, not safeguard).
2. **Persisted tasks artifact** — `tasks.md` inspected at archive time: 14 checkboxes, 0 unchecked (`- [ ]` 0, `- [x]` 14), all phases complete (Phase 1: 1.1–1.4, Phase 2: 2.1–2.4, Phase 3: 3.1–3.3, Phase 4: 4.1–4.3). `sdd-apply` marked completion; no stale-checkbox reconciliation needed. This outranks snapshot counts per hierarchy — orchestrator launch `12/12` and `verify-report` `12` are stale counts; final artifact is `14/14`.
3. **Explicit final-state facts (orchestrator launch)** — `proposal.md`, `specs/sdd-discipline-gates/spec.md`, `specs/orchestrator/spec.md`, `specs/sdd/spec.md`, `design.md`, `tasks.md` (12/12 per launch, actual 14/14 per artifact), `apply-progress.md` (PR1+PR2+PR3 merged, stacked-to-main), `verify-report.md` (PASS, 6/6 32/32, sha256:263aaece71..., ledger tok-9d8a69d886a1076c341a19e1 revision f8a6f0b45...). These are the close-state account and outrank snapshot wording where they agree; where they disagree with the tasks artifact (12 vs 14), the artifact wins per gate.
4. **Intermediate snapshots** (`apply-progress.md` PR1–PR3, `verify-report.md` PASS 6/6 32/32 at verification time) — valid at write time, cited with attribution where numbers overlap; not evidence of final state where higher sources disagree.

Contradictions: task count 12 (launch + verify) vs 14 (persisted tasks artifact). Recorded explicitly here: both statements, sources, and times as above. Higher-ranked artifact (14) reported as current; 12 attributed to snapshots at write time. No other unrankable contradictions. No silent resolution.

## Gates

### Task Completion Gate: PASS

- `tasks.md` inspected at archive time in `openspec/changes/archive/2026-09-07-fix-orchestrator-synthesis-gate/tasks.md`: `- [ ]` count 0, `- [x]` count 14 (Phase 1: 1.1–1.4, Phase 2: 2.1–2.4, Phase 3: 3.1–3.3, Phase 4: 4.1–4.3).
- `verify-report.md` Completeness table at verification time: Tasks total 12, complete 12, incomplete 0 (snapshot; artifact now shows 14/14 due to counting of forecast rows vs implementation tasks — both indicate 0 unchecked).
- `apply-progress.md` Status `12/12 tasks complete. Ready for verify` at apply time (snapshot; artifact now 14/14).
- Archived trail contains no stale unchecked implementation tasks. `sdd-apply` owns checkbox completion; archive required no exceptional stale-checkbox reconciliation.

### Verification Gate: PASS (no CRITICAL)

- `verify-report.md` verdict `pass`, `schema: biggz-ai.verify-result/v1`, `evidence_revision: sha256:263aaece71a13d9eb3898e4a64486267df7937fc9e3b0b1cea9c20d2197732fe`, `blockers: 0`, `critical_findings: 0`, `requirements: 6/6`, `scenarios: 32/32`.
- `verify-report.md` Build & Tests: `go test ./internal/sdd -count=1 -timeout 60s -v && node --test internal/assets/pi/biggz-synthesis-gate.test.mjs && go test ./internal/assets/biggz -count=1` → exit 0, 84 passed / 0 failed / 0 skipped; `go vet ./... && node --check ...` → exit 0.
- Ledger evidence settled as `sha256:263aaece71a13d9eb3898e4a64486267df7937fc9e3b0b1cea9c20d2197732fe` via `biggz sdd-attempt settle` (token `tok-9d8a69d886a1076c341a19e1`, revision `f8a6f0b453372dbbe403bc9c4dd08c5d6ba37cb1bfaa6098c8b0a992fe7b96d5`).
- Archive does not accept overrides for CRITICAL issues — none exist, so no override was needed or applied. No warnings at verification time (WARNING 0, SUGGESTION only coverage threshold note).

### Native Review Receipt Gate: PASS (disabled/unmanaged)

- No `reviewGate` receipt required for this change; kill-switch off path `disabled/unmanaged` applies. No pending/malformed/scope-changed/invalidated/escalated review state blocks archive. No automatic reviewer launch required.

## Scope Delivered

Restore fail-closed synthesis gate retired 2026-09-04 (`ShouldBlock`/`BuildBlockedEnvelope` deleted, JS passthrough). `branch-worktree-cleanup` did 8 delegations but only 2 syntheses → 6 missing checkpoints unblocked. Require synthesis after **every** sub-agent, plain chat FIRST, same turn, 120s window.

- **REQ-DG-1 — Checkpoint-scoped synthesis block** (sdd-discipline-gates): `ShouldBlock` MUST scan ONLY option `label/value/id/name/title` bilingual `proceed|adjust|stop|continue|correct + continuar|ajustar|detener|parar|corregir|proseguir|proceder|procede`; body MUST NOT signal. `HasOptions` alone MUST NOT block. `HasSynthesis` requires 4 English markers (`## Sub-agent Result`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) plus `**What was done:**` OR `| Topic | Decision |`. Strict same-turn `currentTurnMarkdown` via `SetCurrentTurnMarkdown` reset on `turn_start/agent_start`; 120s window; history MUST NOT satisfy; thin `count<2||len<50` warn only `BIGGZ_ADVISE=1`; bypasses `PI_SUBAGENT_CHILD=1`, same-turn `## Session Recall`, narrow preflight. Go canonical, JS mirror.
- **REQ-DG-2 — Blocked-path fallback envelope**: Go `BuildBlockedEnvelope` + JS `blockedEnvelope` emit `context` + full question via `FormatFallback`; JS wraps `registerTool` + sweep `pi.tools/_tools/getAllTools` for `ask_user_choice/ask_user_question/question`; `execute` → `{isError:true}`, `tool_call` → `{block:true}` without calling original.
- **REQ-DG-5 — Transcript lint regression**: CI lint replays `branch-worktree-cleanup` 8-phase fixture (2 syntheses → 7 expected blocks when missing same-turn); complete 8 valid → 0. Threat cases: streaming race, thin, translated markers, recall outside current, child admission.
- **REQ-ORCH-001 — Blocking Synthesis Checkpoint (120s)** (orchestrator): after EVERY sub-agent emit `## Sub-agent Result` with 4 markers in current turn BEFORE checkpoint ask; gate checks `HasSynthesis` + `IsCheckpointAsk` label-only + 120s `currentTurnMarkdown` via `SetCurrentTurnMarkdown`; missing/expired → block `isError:true`/`{block:true}` + `BuildBlockedEnvelope`/`blockedEnvelope` + `FormatFallback`; `## Session Recall` same-turn narrow bypass; docs MUST state `INVALID and will be blocked` + 12× `REMINDER: synthesis markdown is separate chat markdown emitted FIRST, adjacent, same turn, before tool call` + self-check re-read labels before `question/ask_user_choice`.
- **Orchestrator Synthesis Template Invariant**: `biggz-orchestrator.md` keeps 4-marker example + `| Topic | Decision |` table + `- [ ]` checklist + `◆ Phase · Status · Next` lifecycle + `INVALID and will be blocked`; no `ENFORCEMENT RETIRED` passthrough; `engram` alias equals `bigmem`.
- **Synthesis Gate Markers and 120s Window** (sdd): `internal/sdd/synthesis_gate.go` globals `currentTurnMarkdown/currentTurnTime` + `SetCurrentTurnMarkdown`, `HasSynthesis` (4 markers+table English whitelist via `sanitizePlain`), `HasSessionRecall` (`## Session Recall`), `IsChildBypass` (`PI_SUBAGENT_CHILD==1`), `IsCheckpointAsk` label-only bilingual, `HasOptions` heuristic, `ShouldBlock` (`!child&&!recall&&isCheckpoint&&!expired&&!HasSynthesis`), `CheckSynthesisPrecondition` message, `ShouldBlockApplyAdmission` (ignores child/recall), `BlockedFallbackEnvelope`/`BuildBlockedEnvelope`/`FormatFallback` + `HasSessionRecall` narrow; JS mirrors strict `currentTurnMarkdown` ≤120s, thin warn only `BIGGZ_ADVISE=1`, `message_end` buffer + `turn_start` reset + sweep.

Out of scope (untouched per proposal): body-text `IsCheckpointAsk`, history satisfying block, marker translation, new deps.

## Specs Synced

Sync executed BEFORE archive move (Task Completion Gate passed). Merge preserved untouched requirements, applied ADDED/MODIFIED per delta:

| Domain | Main Spec | Action | Details |
|--------|-----------|--------|---------|
| sdd-discipline-gates | `openspec/specs/sdd-discipline-gates/spec.md` | Updated | 2 MODIFIED (REQ-DG-1, REQ-DG-2 replaced in place), 1 ADDED (REQ-DG-5 appended), 0 REMOVED/RENAMED. Preserved REQ-DG-3 (Explicit-preflight admission) and REQ-DG-4 (Parity and regression guard). Total 5 requirements (was 4). |
| orchestrator | `openspec/specs/orchestrator/spec.md` | Updated | 2 MODIFIED (REQ-ORCH-001 — Blocking Synthesis Checkpoint 120s, Orchestrator Synthesis Template Invariant). Preserved 31 other requirements (Explicit Intent, Post-Delegation Human Checkpoint, Single Ownership, Path Validation, Bounded Writer, Sealed Explorer, etc.). Total 33 requirements unchanged count. |
| sdd | `openspec/specs/sdd/spec.md` | Updated | 1 MODIFIED (Synthesis Gate Markers and 120s Window). Preserved 26 other requirements (Preflight Normalization, Preflight Disk, Sync Phase Lifecycle, Sync Execution Contract, REQ-G1-01..G7-01, ReviewOffer, Hook lineage, Archive hygiene, REQ-SDD-001..004, REQ-SD-S1..S5, etc.). Total 27 requirements unchanged count. |

Source of truth now reflects the new behavior:
- `openspec/specs/sdd-discipline-gates/spec.md` (was 105 lines, now 161 lines; +56, REQ-DG-5 added, REQ-DG-1/2 expanded)
- `openspec/specs/orchestrator/spec.md` (was 734 lines, now 754 lines; +20, REQ-ORCH-001 expanded with REMINDER/self-check, Template Invariant expanded with retired-wording absent)
- `openspec/specs/sdd/spec.md` (was 432 lines, now 469 lines; +37, Synthesis Gate expanded with label-only, thin, admission, HasOptions)

Deltas preserved in archive at `specs/{domain}/spec.md` for audit (3 delta files).

## Files Changed (at close)

Implementation diff (revert boundary per `apply-progress.md` work-unit records; archive modified no production code beyond specs sync):

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/synthesis_gate.go` | Modified (PR1) | Restored `currentTurnMarkdown`/`currentTurnTime` globals, `SetCurrentTurnMarkdown`, `ShouldBlock` 120s strict same-turn, `CheckSynthesisPrecondition`, `BlockedFallbackEnvelope`+`BuildBlockedEnvelope` with `FormatFallback`, `ShouldBlockApplyAdmission` (ignores child/recall), kept label-only `IsCheckpointAsk` + `HasOptions` advise-only |
| `internal/sdd/synthesis_gate_test.go` | Modified (PR1) | Added `TestShouldBlock`, `TestShouldBlock_OptionBearingAndBypass`, `TestShouldBlock_SessionRecallAndChild`, `TestShouldBlock_TurnResetAndBodyThin`, `TestShouldBlockApplyAdmission_NoBypasses`, `TestBlockedEnvelope_ReqDG2_FallbackVerbatim`, `TestCheckSynthesisPrecondition_Message` — RED then GREEN |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified (PR2) | Removed `ENFORCEMENT RETIRED` passthrough (cf840430/944e284e), restored `wrapSingleTool` strict `currentTurnMarkdown` ≤120s `checkSynthesisPrecondition` with `isError:true` + `blockedEnvelope` via `FormatFallback`, restored `pi.on("tool_call")` second defense `{block:true,context,fallback}`, `message_end`/`message_update`/`assistant_message` buffer + `turn_start`/`agent_start` reset + `tool_execution_end` reset, sweep `pi.tools/_tools/getAllTools`+`getToolDefinition`, `PI_SUBAGENT_CHILD=1` bypass (child checkpoint still blocks), `## Session Recall` narrow same-turn, thin `count<2||len<50` warn only via `pi.notify` when `BIGGZ_ADVISE=1` |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modified (PR2+PR3) | PR2: 16 passthrough tests → 25 blocking (execute→isError:true no orig, tool_call→block:true, message_end buffer, turn_start reset, child bypass, Session Recall narrow, thin BIGGZ_ADVISE warn, history-only/expired block, sweep, envelope fallback). PR3: +3 RED threat (translated markers, streaming race + history-only recall, child admission + thin parity) → 28 tests |
| `internal/assets/biggz/biggz-orchestrator.md` | Modified (PR2) | Removed retired blockquote, restored `Post-Delegation Human Checkpoint (MANDATORY...)` + `INVALID and will be blocked` + self-check `re-read ONLY question text + options`, Additional rules blocking, Language Boundary 120s, +5 REMINDER (total 6, combined 18) |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified (PR2) | Removed retired wording, added `INVALID and will be blocked` + self-check + 6 REMINDER |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modified (PR2) | Un-retired Ask contract to `blocking — synthesis required` + `gate blocks if missing (120s)`, +4 REMINDER after Delivery +1 after synthesis separate (total 5) + self-check |
| `docs/architecture.md` | Modified (PR3) | Restored 3-layer defense un-retired: removed `ENFORCEMENT RETIRED` blockquote, restored `Go canonical + Pi gate (blocking + thin advise)` with 120s, 4 markers+table whitelist, label-only bilingual, `HasOptions` never blocks, thin warn, narrow recall, child bypass except child checkpoint, `message_end` buffer, `INVALID and will be blocked` 12× REMINDER, hard gate b0d2fc1 language boundary |
| `internal/sdd/transcript_lint_test.go` | Created (PR3) | Transcript lint replaying `branch-worktree-cleanup` 8-phase fixture: `lintTranscript` strict same-turn, 9 tests (7Blocks, Complete_0Blocks, StreamingRace, ThinWarnOnly, TranslatedMarkers, RecallOutsideCurrent, ChildAdmission, Parity, BlockedEnvelope) |
| `openspec/changes/fix-orchestrator-synthesis-gate/tasks.md` | Created/Updated | 14 tasks across 4 phases, all checked |
| `openspec/changes/fix-orchestrator-synthesis-gate/apply-progress.md` | Created | PR1+PR2+PR3 merged evidence (focused + full CI + parity + vet + node --check) |
| `openspec/specs/sdd-discipline-gates/spec.md` | Updated (archive sync) | MODIFIED 2 + ADDED 1 |
| `openspec/specs/orchestrator/spec.md` | Updated (archive sync) | MODIFIED 2 |
| `openspec/specs/sdd/spec.md` | Updated (archive sync) | MODIFIED 1 |

Tracked diff at close (per `git diff --stat HEAD` at archive time, before archive-report write): 11 files, ~1661 insertions(+), ~351 deletions(–) across 8 production files + 3 spec files. Per-PR budgets: each PR slice ≤400–500 lines with stacked-to-main chain (total per apply-progress within auto-chain budget). Zero destructive merges; all untouched requirements preserved.

## Test Evidence (at close, per `verify-report.md` at verification time, attributed)

| Command | Exit | Evidence |
|---|---|---|
| `go test ./internal/sdd -count=1 -timeout 60s -v && node --test internal/assets/pi/biggz-synthesis-gate.test.mjs && go test ./internal/assets/biggz -count=1` (focused + full) | 0 | PASS 19 suites internal/sdd (TestHasSynthesis, TestIsCheckpointAsk, TestIsCheckpointAskEnvelopeLabelsOnly, TestHasOptionsAdviseOnly, TestShouldBlock 5 suites, TestBlockedEnvelope, TestCheckSynthesisPrecondition, TestTranscriptLint 9 suites, etc.) + 48+ total internal/sdd tests 21.8s ; 5 suites internal/assets/biggz (TemplateInvariant, GuardsDrift, LazyFiles, SessionRecall, Alias) 0.6s ; 28/28 JS (heuristic, blocking missing/history/121s, thin advise, child bypass, preflight, envelope, parity, translated, streaming race, recall, child admission) 0.13s — overall 84 passed / 0 failed / 0 skipped |
| `go test ./internal/sdd -count=1` + `go test ./internal/assets/biggz -count=1` + `node --test` | 0 | Same as above, ledger settled sha256:263aaece71a13d9eb3898e4a64486267df7937fc9e3b0b1cea9c20d2197732fe via `biggz sdd-attempt settle` token tok-9d8a69d886a1076c341a19e1 revision f8a6f0b453372dbbe403bc9c4dd08c5d6ba37cb1bfaa6098c8b0a992fe7b96d5 |
| `go vet ./... && node --check internal/assets/pi/biggz-synthesis-gate.js && node --check internal/assets/pi/biggz-synthesis-gate.test.mjs` | 0, clean | build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 |
| `go test -run TestShouldBlock` + `TestTranscriptLint_*` + parity | 0 | REQ-DG-1/2/5, REQ-ORCH-001, REQ-SDD, synthesis gate markers 4+table, label-only, HasOptions never blocks, 120s/turn-reset/child/recall/thin, ApplyAdmission ignores bypass, CheckSynthesisPrecondition message, transcript lint 7 blocks vs 0 blocks |
| `grep` docs | pass | `biggz-orchestrator.md` 18 REMINDER (orchestrator 6 + workflow 7 + delegation 5) ≥12, `INVALID and will be blocked` present in all 3 docs, `ENFORCEMENT RETIRED` 0 occurrences, self-check `re-read ONLY the question text + options` present in all 3, `docs/architecture.md` 3-layer defense restored (2 INVALID, 0 ENFORCEMENT, HasSynthesis markers present) |

Spec compliance: 6/6 requirements, 32/32 scenarios compliant per verification matrix (REQ-DG-1 ×9, REQ-DG-2 ×4, REQ-DG-5 ×2, REQ-ORCH-001 ×5, Orchestrator Template Invariant ×3, Synthesis Gate ×9). No code changes made by verification. Ledger `evidence_revision` sha256:263aaece71... bound via `biggz sdd-attempt settle` matches persisted evidence.

**Modern Go Guidelines**: considered via `pwsh .\scripts\run-tool.ps1 list --file-path internal/sdd/synthesis_gate.go` (40+ guidelines); no violations; `HasSynthesis` uses `strings.Contains`, `ShouldBlock` uses `time.Since`, `SetCurrentTurnMarkdown` uses `time.Now()` idiomatically.

## Archive Contents

- `proposal.md` ✅ (Intent: restore fail-closed gate retired 2026-09-04, 8 delegations 2 syntheses →6 missing; Scope: Go/JS/docs/tests/CI; Affected Areas 6 rows; Risks 6; Rollback `git revert cf840430+944e284e`)
- `specs/sdd-discipline-gates/spec.md` ✅ (delta, 4722 bytes, MODIFIED REQ-DG-1/2 + ADDED REQ-DG-5, preserved for audit)
- `specs/orchestrator/spec.md` ✅ (delta, 3736 bytes, MODIFIED REQ-ORCH-001 + Template Invariant)
- `specs/sdd/spec.md` ✅ (delta, 3693 bytes, MODIFIED Synthesis Gate Markers and 120s Window)
- `design.md` ✅ (Technical Approach: restore gate deleted cf840430+944e284e; Architecture Decisions 4 rows; Data Flow sub-agent→synthesis→checkpoint→ShouldBlock→envelope; File Changes 9 files; Interfaces Go/JS; Testing Strategy 4 layers; Threat Matrix 6 rows; Migration/Rollout; Open Questions)
- `exploration.md` ✅ (optional, from sdd-explore)
- `tasks.md` ✅ (14/14 complete at archive time; 0 unchecked; stacked-to-main 3 work units)
- `apply-progress.md` ✅ (PR1 Go canonical + PR2 JS dual + PR3 lint + parity, 129 lines, work-unit evidence tables + validation per PR)
- `verify-report.md` ✅ (PASS, 6/6 32/32, yaml header + completeness + build/tests + compliance matrix 32 rows + correctness + coherence + issues none + verdict PASS)
- `archive-report.md` ✅ (this file)

## Verification (Archive-Time Checks)

- [x] Main specs updated correctly (3 domains: sdd-discipline-gates 5 reqs, orchestrator 33 reqs, sdd 27 reqs; diffs show only MODIFIED/ADDED for this change, all untouched requirements preserved, proper Markdown hierarchy)
- [x] Change folder moved to `openspec/changes/archive/2026-09-07-fix-orchestrator-synthesis-gate/` (ISO prefix = today UTC 2026-09-07, single prefix, matches repo convention `YYYY-MM-DD-{change}`)
- [x] Archive contains all artifacts (proposal, specs/ (3 deltas), design, exploration, tasks, apply-progress, verify-report, archive-report)
- [x] Archived `tasks.md` has no unchecked implementation tasks (0 `- [ ]`, 14 `- [x]`; no stale-checkbox reconciliation required, though orchestrator/verify snapshots say 12 — artifact wins per Final-State Authority)
- [x] Active changes directory no longer contains this change (`openspec/changes/fix-orchestrator-synthesis-gate` → not found, `Test-Path` false)
- [x] No CRITICAL verify blockers; no production code touched by archive (only spec sync preserves history)
- [x] `openspec/config.yaml` `rules.archive`: none defined — no extra rules to apply (checked; no `rules.archive` section requiring warning/merger confirmation)
- [x] Archive is AUDIT TRAIL — no deletion/modification of archived change beyond this report; `.biggz-instance` not present (no linked repo instance to retain)

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/sdd-discipline-gates/spec.md` — REQ-DG-1 strict same-turn/label-only/thin/bypass, REQ-DG-2 dual-guard fallback envelope, REQ-DG-5 transcript lint 7 blocks
- `openspec/specs/orchestrator/spec.md` — REQ-ORCH-001 120s + INVALID + 12× REMINDER + self-check, Template Invariant un-retired (no ENFORCEMENT RETIRED)
- `openspec/specs/sdd/spec.md` — Synthesis Gate 4 markers+table, label-only bilingual, HasOptions never blocks, 120s window, turn_start reset, child/recall/preflight bypass, thin warn-only, CheckSynthesisPrecondition, ApplyAdmission ignores bypass, FormatFallback

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. Source of truth synced, audit trail preserved in `openspec/changes/archive/2026-09-07-fix-orchestrator-synthesis-gate/`. Ready for the next change.
