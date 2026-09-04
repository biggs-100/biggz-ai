# Archive Report — sdd-discipline-gates

**Change**: `sdd-discipline-gates`
**Archived to**: `openspec/changes/archive/2026-09-04-sdd-discipline-gates/`
**Archive date**: 2026-09-04 (ISO, UTC)
**Mode**: `openspec` / auto-chain (stacked-to-main, PR 1 → PR 5)
**Verdict**: PASS WITH WARNINGS — 4/4 requirements, 11/11 scenarios, 19/19 tasks, `sdd-verify-validate` PASS
**Production code modified by archive**: none (spec sync + folder move + this report only)

## Final-State Authority

This report is the terminal record at close. Intermediate snapshots are history, not current state:

1. **Native review authority** — no `reviewGate` receipt governs this change; no explicit review artifact failed validation. Treated as `disabled/unmanaged` (demanding a receipt while no review governs would deadlock, not safeguard).
2. **Persisted tasks artifact** — `tasks.md` 19/19 `[x]`, 0 unchecked at archive time. `sdd-apply` marked completion; no stale-checkbox reconciliation needed.
3. **Explicit final-state facts (orchestrator launch)** — `sdd-verify-validate` PASS (4 req / 11 scen); known pre-existing failure `TestSDDStatusJSONEnvelopeDerivesStructuredFields` is out-of-scope; ledger notes below are the close-state account and outrank snapshot wording.
4. **Intermediate snapshots** (`apply-progress.md` Units 1–5, `verify-report.md` PASS WITH WARNINGS) — valid at write time, cited with attribution where numbers overlap; not evidence of final state where higher sources disagree.

Contradictions: none unrankable. No silent resolution required.

## Gates

### Task Completion Gate: PASS
- `tasks.md` inspected at archive time: `- [ ]` count 0, `- [x]` count 19 (Phase 1: 1.1–1.5, Phase 2: 2.1–2.4, Phase 3: 3.1–3.4, Phase 4: 4.1–4.3, Phase 5: 5.1–5.3).
- Archived trail contains no stale unchecked tasks.

### Verification Gate: PASS (no CRITICAL)
- `verify-report.md` verdict `pass_with_warnings`, `schema: biggz-ai.verify-result/v1`, `blockers: 0`, `critical_findings: 0`, `requirements: 4/4`, `scenarios: 11/11`.
- `sdd-verify-validate` PASS per orchestrator final-state fact (4 req / 11 scen).
- Archive does not accept overrides for CRITICAL issues — none exist, so no override was needed or applied.

## Scope Delivered

Fail-closed SDD discipline gates (Go canonical, JS mirror, installer TUI untouched):

- **REQ-DG-1 — Checkpoint-scoped synthesis block**: `ShouldBlock` (Go `internal/sdd/synthesis_gate.go`, JS `internal/assets/pi/biggz-synthesis-gate.js`) requires `IsCheckpointAsk`; `HasOptions` alone never blocks. Free-text asks and Session Preflight option-asks never block. Go/JS parity pinned by tests.
- **REQ-DG-2 — Blocked-path fallback envelope**: on block, same-turn payload carries attempted `context` plus full question `fallback` via existing `FormatFallback` / `formatFallback` — nothing swallowed. Exposed as Go `BlockedFallbackEnvelope` + `BuildBlockedEnvelope` and JS `blockedEnvelope`.
- **REQ-DG-3 — Explicit-preflight admission**: `HasExplicitPreflight(cwd)` (cache hit OR disk read ok; silent defaults alone = false); `ResolvePreflightPrefs` behavior unchanged. Dispatcher phase-entry only (`status.go` shared `CheckPhaseEntryPreflight` + `continue.go` `NextPhaseChecked`) returns `blocked(preflight_missing)` + `resolve-blockers`, no launch until explicit. Status reads unaffected.
- **REQ-DG-4 — Parity and regression guard**: synthesis-first checkpoints still pass; missing-synthesis checkpoints still block; zero installer/TUI files changed.
- **Unit 5 (tasks-only, no spec requirement) — Ledger multi-unit lifecycle + CLI hardening**: new non-terminal `progress` settle outcome (intermediate checkpoints stay open/admitted, only final `passed` completes the ledger; consumes budget like failure; `Begin`/`Finish` untouched); `sddAttemptRun` flag loop fail-closed (`error: unknown flag <f>`, exit 1, mirroring `sddStatusRun`); `HelpText` documents positional `<change>`, cwd always `os.Getwd()`, no `--cwd`/`--change` flags, plus `progress` semantics.

Out of scope (untouched): installer/TUI screens, new preflight questions/defaults, `session_guard` / `edit_authority` behavior.

## Specs Synced

Sync executed BEFORE archive move (Task Completion Gate passed). No main spec existed for this domain, so the delta was promoted whole:

| Domain | Main Spec | Action | Details |
|--------|-----------|--------|---------|
| sdd-discipline-gates | `openspec/specs/sdd-discipline-gates/spec.md` | Created (new) | Copied from `openspec/changes/sdd-discipline-gates/specs/sdd-discipline-gates/spec.md` (105 lines): REQ-DG-1 (4 scenarios), REQ-DG-2 (2), REQ-DG-3 (3), REQ-DG-4 (2). No ADDED/MODIFIED/REMOVED merge against existing content — nothing to preserve. No REMOVED/RENAMED. |

Source of truth now reflects the new behavior:
- `openspec/specs/sdd-discipline-gates/spec.md` (new, 105 lines)

Delta spec preserved in archive at `specs/sdd-discipline-gates/spec.md` for audit.

## Files Changed (at close)

Implementation diff (revert boundary per `apply-progress.md` Unit records; archive modified no production code):

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/synthesis_gate.go` | Modified | `ShouldBlock` narrowed to `IsCheckpointAsk`; added `BlockedFallbackEnvelope` + `BuildBlockedEnvelope` via `FormatFallback` |
| `internal/sdd/synthesis_gate_test.go` | Modified | `TestShouldBlock_ReqDG1_OptionBearingNonCheckpoint`, `TestShouldBlock_ReqDG1_BypassUnchanged`, `TestBlockedEnvelope_ReqDG2_FallbackVerbatim` (+111) |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Checkpoint-only gate at 2 sites + `blockedEnvelope` helper via `formatFallback` |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modified | REQ-DG-1 allow-behavior flip + Go-fixture parity test (25/25) |
| `internal/sdd/preflight.go` | Modified | `HasExplicitPreflight(cwd, home...)` (cache > disk > false) |
| `internal/sdd/status.go` | Modified | `PreflightMissingReason` + shared `CheckPhaseEntryPreflight` admission helper |
| `internal/sdd/continue.go` | Modified | Gated `NextPhaseChecked` phase entry (block → error, no launch; admit → `NextPhase`) |
| `internal/sdd/preflight_admission_test.go` | Created (116 lines) | REQ-DG-3 explicitness + read-vs-entry admission tests |
| `internal/sddattempt/sddattempt.go` | Modified | `Settle` accepts non-terminal `progress` + `HelpText` positional-`<change>`/cwd/`progress` docs |
| `internal/sddattempt/acquire_settle_test.go` | Modified | `TestSettle_ProgressMultiUnitSingleFinalSettle` (+52) |
| `cmd/biggz/cli_sdd.go` | Modified | `sddAttemptRun` flag-loop `default: unknown flag` fail-closed (+3) |
| `cmd/biggz/sdd_attempt_unknown_flag_test.go` | Created (23 lines) | `TestSDDAttemptUnknownFlagFailClosed` (`--cwd`/`--change`/`--bogus` → exit 1) |

Tracked diff at close: 10 files, 359 insertions(+), 36 deletions(–) — under the 400-line budget; plus 2 new untracked test files (139 lines total). Zero installer/TUI files (`git diff --stat HEAD -- '*installer*' '*tui*' '*TUI*'` empty per verify).

## Test Evidence (at close, per `verify-report.md`)

| Command | Exit | Evidence |
|---|---|---|
| `go test ./internal/sdd/... ./internal/sddattempt/... ./cmd/biggz/ -run 'Attempt\|Synthesis\|Preflight\|Admission\|UnknownFlag'` | 0 (all 3 packages `ok`) | output sha256 `5ba1fd5d8bce550acc3a1acd5dd0f7b24ff291340ed08c3febecdf60d88b483e` |
| `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` | 0 (25 pass / 0 fail) | output sha256 `ee85e080c5ea4015afef8b0993c7401a6239ea4179dddf2fc04dbe60940bbb23` |
| `go vet ./internal/sdd/... ./internal/sddattempt/... ./cmd/biggz/` | 0, clean | output sha256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `go test ./internal/sdd/... -count=1` (full package) | 0 (`ok ... 12.273s`) | regression green |
| `git diff --stat` TUI filter | empty | zero installer/TUI files (REQ-DG-4) |
| `git diff --numstat` total | 10 tracked files, 359+/36− | under 400-line budget |

Spec compliance: 11/11 scenarios covered by passing runtime tests (REQ-DG-1 ×4, REQ-DG-2 ×2, REQ-DG-3 ×3, REQ-DG-4 ×2) plus Unit 5 tasks-only rows (`progress` lifecycle, unknown-flag fail-closed). No code changes made by verification. Attempt `tok-2b19463b50f8df04105619ec` (verify work-unit) NOT settled — orchestrator settles.

## Known Pre-Existing Failure (out-of-scope, NOT shipped, NOT fixed)

- `TestSDDStatusJSONEnvelopeDerivesStructuredFields` (`cmd/biggz/sdd_status_cli_test.go`) FAILS in this environment with `rdd_receipt_missing` (`blockedReasons: rdd_receipt_missing: lineage "json-change" unavailable ... not a git repository`, `nextRecommended = "resolve-blockers"` vs want `archive`, verify/archive `blocked` vs want ready).
- Stash-verified: fails identically on clean tree (`git stash --keep-index` → same FAIL → `git stash pop`). Pre-existing, unrelated to this change; full `./internal/sdd/...` suite itself is green. Excluded from the focused run above, which is fully green.
- Recorded here as out-of-scope per orchestrator instruction. Do NOT fix in this change; do NOT gate this archive on it (WARNING, not CRITICAL).

## Ledger Notes (Unit 5 lifecycle + post-final-settle dictamen, per `verify-report.md` §6)

- **Unit 5 fix (shipped)**: `Settle(passed)` previously set `Complete=true`, so `deriveAdmissionBlocked` refused the next `Acquire` (`blocked(corrupt_authority)` — reset required) and multi-unit apply (Units 1–4) could not run acquire→settle-per-unit without maintainer resets. Fixed via new non-terminal `progress` outcome (option A): mid-ledger stays open/admitted, only the final `settle(passed)` completes. `progress` consumes one delivered attempt of budget like `failed` (fail-closed, no infinite free checkpoints). `Begin`/`Finish` semantics untouched; single `WorkUnit` scope id across unit acquires; scope-change guard stays fail-closed.
- **Post-final-settle reset (by-design, NOT a bug, no code change)**: after a final `settle(passed)` the ledger is terminally complete (`blocked(corrupt_authority)`, "ledger is complete; reset required to continue", pinned by `TestAcquire_BlockedWhenComplete`). A new work-unit/objective (e.g. verify after apply) requires an explicit maintainer `sdd-attempt reset` (provenance-tracked) before the next `acquire`. Same code path as Unit 5, opposite intent: Unit 5 = premature terminal (fixed); post-final = correct terminal. If a future change wants apply→verify in one ledger scope without reset, that is a new spec requirement (e.g. scoped `verify` continuation token), not a fix.
- **Orchestrator action**: issue explicit `sdd-attempt reset` (new objective) before any verify-phase `acquire` when the apply scope already final-settled `passed`.

## Archive Contents

- `proposal.md` ✅
- `specs/sdd-discipline-gates/spec.md` ✅ (delta, preserved for audit)
- `design.md` ✅
- `tasks.md` ✅ (19/19 complete)
- `apply-progress.md` ✅ (Units 1–5 records with RED→GREEN proof per unit)
- `verify-report.md` ✅ (PASS WITH WARNINGS, 4/4 req, 11/11 scen)
- `archive-report.md` ✅ (this file)
- `_meta.yaml` ✅

## Verification (Archive-Time Checks)

- [x] Main spec created correctly (`openspec/specs/sdd-discipline-gates/spec.md`, 105 lines, diff-clean vs delta)
- [x] Change folder moved to `openspec/changes/archive/2026-09-04-sdd-discipline-gates/` (ISO prefix = today UTC, single prefix, matches repo convention `YYYY-MM-DD-{change}`)
- [x] Archive contains all artifacts (proposal, specs/, design, tasks, apply-progress, verify-report, archive-report, _meta)
- [x] Archived `tasks.md` has no unchecked tasks (0 `- [ ]`, 19 `- [x]`)
- [x] Active changes directory no longer contains this change
- [x] No CRITICAL verify blockers; no production code touched by archive
- [x] `openspec/config.yaml` `rules.archive`: none defined — no extra rules to apply

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. Source of truth synced, audit trail preserved. Ready for the next change.
