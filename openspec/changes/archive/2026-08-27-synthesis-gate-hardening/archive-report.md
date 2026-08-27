# Archive Report — 2026-08-27-synthesis-gate-hardening

**Change**: `2026-08-27-synthesis-gate-hardening`
**Archived to**: `openspec/changes/archive/2026-08-27-synthesis-gate-hardening/`
**Archive date**: 2026-08-27 (ISO)
**Mode**: `openspec` (preflight: `interactive | openspec | auto-chain | 800`)
**Delivery**: `auto-chain` / single PR (Low risk, 280–450 lines, 800 budget → ~13% tracked diff)
**Verdict**: PASS — 4/4 requirements, 14/14 scenarios, 11/11 tasks, 2 Go + 9 Node focal tests green

## Final-State Authority

This report is the terminal record at close. Per the hierarchy, higher-ranked sources outrank intermediate snapshots:

1. **Native review authority** — no `reviewGate` receipt present; workload Low risk single PR under 400-line budget (forecast 280–450, actual tracked diff ~108 lines + 945 verified) required no native RAR receipt. Treated as `disabled/unmanaged` per kill-switch coexistence (demanding a receipt while no review governs would deadlock). No explicit review artifact failed validation.
2. **Persisted tasks artifact** — `tasks.md` 11/11 `[x]` (Phase 1: 1.1–1.3, Phase 2: 2.1–2.3, Phase 3: 3.1–3.3, Phase 4: 4.1–4.2). No stale unchecked tasks.
3. **Explicit final-state facts (orchestrator launch, 2026-08-27)** — outrank `apply-progress`/`verify-report` snapshots dated 2026-08-26T20:43–20:57Z:
   - `apply` 11/11 Standard Mode single PR; gate JS (`internal/assets/pi/biggz-synthesis-gate.js`) and `internal/assets/biggz/biggz-orchestrator.md` verified without edit (commit `b0d2fc1` baseline preserved, 535 lines gate + 693 lines template with 12× REMINDER, 4 markers)
   - Created `internal/assets/biggz/orchestrator_test.go` (86 lines, 2 tests / 6 subtests) — drift guard via `assets.FS`
   - Modified `docs/architecture.md` +22 lines `### Synthesis Gate (3-layer defense)` subsection
   - Verification: `go vet ./internal/assets/biggz` 0, `go test ./internal/assets/biggz -count=1` PASS 2/2, `node --check` 0, `node --test .../biggz-synthesis-gate.test.mjs` 9/9 pass
   - `biggz sdd-verify-validate` admitted; focal CI verde; 2 flaky Windows failures in `internal/install` excluded and documented (pre-existing, unrelated `gofmt` stash)
   - Stash `stash@{0} "temp stash unrelated gofmt changes synthesis-gate"` preserves 82 unrelated `gofmt` files — not part of this change
4. **Intermediate snapshots** (`apply-progress.md`, `verify-report.md`) — valid at write time (2026-08-26 20:43–20:57Z), cited with attribution where numbers overlap; not evidence of final state where higher sources disagree.

Contradictions: none unrankable. Final-state facts corroborated by repository evidence (see Changed Files, Commands). No silent resolution required.

## Gates

### Task Completion Gate: PASS
- `tasks.md` inspected at archive time: 0 unchecked (`- [ ]` count 0), 11 checked (`- [x]` count 11). `sdd-apply` already marked completion; no stale-checkbox reconciliation needed.
- `apply-progress.md` 11/11 confirmed with evidence per task (see progress file). Archived trail contains no stale unchecked tasks.

### Native Review Receipt Gate: PASS (disabled/unmanaged)
- No `reviewGate` artifact governs this Low-risk change; `400-line budget risk: Low`, `Chained PRs recommended: No`, single PR, no PR opened. No `scope-changed`/`invalidated` state.
- `verify-report` CRITICAL 0, blockers 0 — no blocking receipt required. Gate considered `disabled/unmanaged` as fallback (kill-switch off, demanding receipt would deadlock).

### Verification Gate: PASS
- `verify-report.md` verdict `pass`, `schema: biggz-ai.verify-result/v1`, `evidence_revision: sha256:bc492...`, `blockers:0`, `critical_findings:0`, `requirements: 4/4`, `scenarios: 14/14`.
- `biggz sdd-verify-validate` admitted (per orchestrator final-state fact; interim report already validated by `sdd-verify`).
- Focal tests green at close: `go vet ./internal/assets/biggz` 0, `go test ./internal/assets/biggz -count=1` PASS (TestOrchestratorSynthesisTemplateInvariant ×4 subtests + TestOrchestratorSynthesisTemplateGuardsDrift), `node --check` 0, `node --test .../biggz-synthesis-gate.test.mjs` 9 pass 0 fail (heuristic helpers, scenario 1 block isError:true originalCalled==false, scenario 2 thin+advise warn, scenario 3 thin silent, scenario 4 rich, child bypass, settings flag, no-model-call, same-turn race).
- Full `go test ./...` 2 FAIL in `internal/install` documented as pre-existing flaky Windows (`TestDeployMCPMergeIntoSettings_WritesBiggzServer`, `TestProvisionBigMemMCP_WritesBothFiles`) — unrelated to gate (no files touched in `internal/install`), excluded per instruction. Focused gate slice is authoritative at close.

## Specs Synced

Sync executed BEFORE archive move (Task Completion Gate passed). Merge preserved untouched requirements.

| Domain | Main Spec | Action | Details |
|--------|-----------|--------|---------|
| orchestrator | `openspec/specs/orchestrator/spec.md` | Updated (ADDED ×2) | Preserved existing `Explicit Intent Required` (5 scenarios). Appended `Post-Delegation Human Checkpoint Synthesis` (MUST 4 markers same-turn, INVALID blocked, param-only not counted, thin passes) with 4 scenarios + `Orchestrator Synthesis Template Invariant` (example block + INVALID + REMINDER convergence, drift guard) with 2 scenarios. Total requirements: 1 → 3. No REMOVED/RENAMED. |
| pi-integration | `openspec/specs/pi-integration/spec.md` | Updated (MODIFIED ×1, ADDED ×1) | Replaced `Advisor Inline Watchdog Advise Mode` with hardened version: 4-marker strictness, source priority `currentTurnMarkdown → ctx.history → lastAssistant` 120s window, `{isError:true, text:"Please synthesize..."}` no `original()` + `notify`, thin `count<2 \|\| len<50` via `extractArtifactsSection`, `BIGGZ_ADVISE=1`/settings gate off default, only `PI_SUBAGENT_CHILD=1` bypass, no orchestrator bypass, no auto-fix/model call. Added 1 scenario (Same-turn buffer race) → 5 → 6 scenarios. Appended `Synthesis Gate Verification and CI` (4 gate scenarios + `go vet`/`go test`/`node --check`/`node --test` green) with 2 scenarios. Total requirements: 1 → 2. |

Source of truth now reflects new behavior:
- `openspec/specs/orchestrator/spec.md` (43 → 82 lines, +39)
- `openspec/specs/pi-integration/spec.md` (41 → 58 lines, +17, hardened)

Delta specs preserved in archive at `specs/orchestrator/spec.md` and `specs/pi-integration/spec.md` for audit.

## Changed Files (at close)

Tracked implementation diff under final-state authority (single PR, no migration, revert boundary):

| File | Action | Evidence |
|------|--------|----------|
| `internal/assets/biggz/orchestrator_test.go` | Created (86 lines, `package biggz_test`) | `go vet` 0, `go test` PASS 2 tests — reads `assets.FS.ReadFile("biggz/biggz-orchestrator.md")`, asserts 4 markers + canonical header `## Sub-agent Result: {phase/agent}` + `INVALID and will be blocked` + `REMINDER: synthesis markdown is separate` + `separate chat markdown emitted FIRST` + `Do NOT put synthesis inside the tool's question param` + REMINDER ≥12 + drift guard. Delivered as `orchestrator_test.go` (`_test.go` required for `go test` discovery) vs design's `orchestrator.test.go` dot notation — idiomatic deviation documented, intent preserved. |
| `docs/architecture.md` | Modified (+22 lines) | Added `### Synthesis Gate (3-layer defense)` after RDD: Layer 1 prompt invariant 4 markers + INVALID same-turn + 12× REMINDER, Layer 2 Pi gate blocking `isError:true`/`Please synthesize` + source priority 120s + same-turn buffer + thin advise heuristic + only `PI_SUBAGENT_CHILD=1`, Layer 3 tests/CI gates. Staged diff `git diff HEAD -- docs/architecture.md` shows insertion only. |
| `internal/assets/pi/biggz-synthesis-gate.js` | Verified (535 lines, no edit) | Audit confirms 4-marker `hasSynthesis`, `checkSynthesisPrecondition` order `currentTurnMarkdown` (≤120000 ms) → `ctx.history` → `lastAssistant` 120s, `isThinSynthesis` via `getArtifactsMetrics` `<2 \|\| <50`, `isAdviseEnabled` `BIGGZ_ADVISE=1\|true`/settings, `isChildBypass` only `PI_SUBAGENT_CHILD=1`, `_biggzSynthesisGate` helpers, `pi.registerTool` wrapper block `{isError:true}` + `notify` no `original()`, `pi.on("tool_call")` secondary guard, `recordText` + reset after `original()`, `node --check` 0. |
| `internal/assets/biggz/biggz-orchestrator.md` | Verified (693 lines, no edit) | `grep -c "REMINDER: synthesis markdown is separate"` 12, `grep -n "INVALID and will be blocked"` 2 blocks, `## Sub-agent Result` 4 hits — already hardened `b0d2fc1`. |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Verified (410 lines, 9 tests, no edit) | Covers 4 gate scenarios + helpers + child bypass + race; `node --test` 9 pass 0 fail, asserts `originalCalled==false` on block. |

**Unrelated stash** (not shipped): `stash@{0}` 82 files `gofmt`-only (`cmd/biggz-mcp/main.go`, `internal/agents/*`, `internal/tui/*`, etc.) — pure formatting alignment, excluded from gate per instruction. `git diff --stat HEAD` shows only `docs/architecture.md` 22 insertions tracked; untracked `orchestrator_test.go` is intentional new file; stash preserves the 82-file `gofmt` sweep.

## Archive Contents

- `proposal.md` ✅ (Intent: mandatory synthesis before decision; Scope: orchestrator.md + synthesis-gate.js + tests + docs; Approach: 3 layers prompt→gate→tests/CI; Success criteria 4 gate behaviors)
- `spec.md` ✅ (Summary 4 requirements R1–R4 across orchestrator ×2 and pi-integration ×2)
- `specs/orchestrator/spec.md` ✅ (Delta ADDED 2)
- `specs/pi-integration/spec.md` ✅ (Delta MODIFIED 1 + ADDED 1)
- `design.md` ✅ (Technical Approach 3 layers, Architecture Decisions prompt/gate/thin/tests, Data Flow buffer→wrapper→checkSynthesisPrecondition→block/advise, File Changes 5 rows, Interfaces `_biggzSynthesisGate`, Testing Strategy layers, Threat Matrix N/A)
- `tasks.md` ✅ (11/11 tasks complete, Review Workload Forecast Low / No chain, Suggested Work Unit PR 1)
- `apply-progress.md` ✅ (Phase 1–4 evidence with focused test results, Files Changed, Work Unit Evidence, Deviations, Issues Found, Workload boundary)
- `verify-report.md` ✅ (PASS 4/4 14/14, Build 0, Tests 2+9 green, Spec Compliance Matrix 14 rows, Correctness/Coherence implemented, Issues WARNING only 2 flaky excluded)
- `archive-report.md` ✅ (this file)

Active changes directory no longer contains this change: `openspec/changes/` now holds only `archive/` (verified `ls`).

## Verification (Archive-Time Checks)

- [x] Main specs updated correctly (orchestrator 1→3 requirements, pi-integration 1→2, preserved untouched)
- [x] Change folder moved to `openspec/changes/archive/2026-08-27-synthesis-gate-hardening/` (ISO prefix matches change date; today's system date 2026-08-26 would double-prefix — user explicitly requested `2026-08-27` target, consistent with existing archive naming `2026-07-27-*` single prefix)
- [x] Archive contains all artifacts (proposal, spec, specs/, design, tasks, apply-progress, verify-report, archive-report)
- [x] Archived `tasks.md` has no unchecked tasks (11/11 `[x]`)
- [x] No staged files for this change (staged diff empty pre-archive; post-archive only spec sync edits to `openspec/specs/*`)
- [x] No CRITICAL verify blockers

## Commands Run (evidence)

| Command | Result | Summary |
|---------|--------|---------|
| `go vet ./internal/assets/biggz` | passed (0, prior `go vet ./...` also 0) | Clean |
| `go test ./internal/assets/biggz -count=1 -v` | passed (2 tests, 6 subtests, ~0.44s) | `contains_copy-paste_block_with_4_markers`, `contains_INVALID_and_will_be_blocked_rule`, `contains_12x_REMINDER_convergence`, `synthesis_separate_from_tool_param`, `GuardsDrift` |
| `node --check internal/assets/pi/biggz-synthesis-gate.js` | passed (0) | Syntax clean |
| `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` | passed (9 pass 0 fail, ~95–110ms) | 4 gate scenarios + child bypass + race + helpers |
| `biggz sdd-verify-validate` | admitted (per orchestrator final-state fact; interim verify already validated) | PASS 4/4 14/14 |
| `git stash list` | — | `stash@{0}: On master: temp stash unrelated gofmt changes synthesis-gate` (82 files) |

Tracked diff at close: `docs/architecture.md` +22; `internal/assets/biggz/orchestrator_test.go` untracked new; `git diff --stat HEAD` pre-sync 22 insertions only (spec sync adds orchestrator/p-i spec merges, not counted in app diff). No staged changes before archive.

## Residual Risks / Open Questions

- **Thin heuristic noise**: `count<2 || len<50` via `extractArtifactsSection` is loose by design; when `BIGGZ_ADVISE=1` enabled, advise may emit `concern: synthesis is thin` on legitimately small but complete syntheses. Mitigation: advise is warning-only (never block), off by default.
- **Full-suite flaky on Windows**: `go test ./...` fails 2 in `internal/install` pre-existing (not introduced by this change, documented `internal/install` not touched). Mitigation: excluded per instruction, focused gate CI green; future fix should stabilize `TestDeployMCPMergeIntoSettings_WritesBiggzServer` / `TestProvisionBigMemMCP_WritesBothFiles` Windows paths.
- **Stash preservation**: 82 `gofmt` files stashed separately — not shipped with this change. Risk low (formatting only) but next change should re-apply or drop stash cleanly to avoid bit-rot.
- **Naming deviation**: `orchestrator.test.go` (dot) vs `orchestrator_test.go` (underscore) — underscore is Go-idiomatic and `go test`-discoverable; file name change does not affect gate behavior, but future docs should use underscore.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. Source of truth synced, audit trail preserved, stash holds unrelated formatting. Ready for next change.

## Key Learnings

1. Prompt-gate convergence via 12× REMINDER and 4-marker invariant prevents synthesis bypass drift.
2. Same-turn `currentTurnMarkdown` buffer resolves streaming race before `ctx.history` lands.
3. Thin heuristic (`countPaths <2 || len <50`) gated behind `BIGGZ_ADVISE=1` avoids noisy blocking.
4. Go integration test via `assets.FS` guards template drift without importing template path.
5. `node --test` fixture coverage of 4 scenarios plus child bypass validates blocking/advise without model calls.
