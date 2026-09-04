# Apply Progress: sdd-discipline-gates — Units 1–2

**Change**: sdd-discipline-gates
**Unit**: 2 — JS mirror + node parity tests (PR 2, stacked-to-main)
**Attempt**: tok-e3ab5840e633db52102d2815 (shared work-unit units-2-4, left OPEN for Units 3–4 — not settled)
**Status**: Units 1–2 complete — ready for Unit 3

## Completed Tasks (Phase 1: 1.1–1.5)

- [x] 1.1 `ShouldBlock` in `internal/sdd/synthesis_gate.go` now requires `IsCheckpointAsk`; `HasOptions` alone never blocks. Doc comments updated (header + `ShouldBlock`).
- [x] 1.2 Free-text fixture (`{"question":...}`, no options/token) returns false with/without synthesis — RED then GREEN.
- [x] 1.3 Preflight option-ask fixture (options-bearing, no checkpoint token) returns false even with missing/empty synthesis and expired window — RED (`ShouldBlock should be false for preflight option-ask even without synthesis`) then GREEN after fix.
- [x] 1.4 Option-bearing checkpoint without synthesis blocks; with valid synthesis in-window passes (REQ-DG-4 regression pinned).
- [x] 1.5 Child (`PI_SUBAGENT_CHILD=1`) and Session Recall bypasses verified unchanged for option-bearing checkpoints (threat-matrix row).

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/synthesis_gate.go` | Modified | `ShouldBlock` gate condition narrowed from `!IsCheckpointAsk && !HasOptions` to `!IsCheckpointAsk`; comments updated |
| `internal/sdd/synthesis_gate_test.go` | Modified | Added `TestShouldBlock_ReqDG1_OptionBearingNonCheckpoint` + `TestShouldBlock_ReqDG1_BypassUnchanged` |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/sdd/ -run 'Synthesis'` → PASS (`ok github.com/biggs-100/biggz-ai/internal/sdd`); `go test ./internal/sdd/ -run 'TestShouldBlock' -v` → all 7 test funcs PASS |
| RED proof (pre-fix) | New `TestShouldBlock_ReqDG1_OptionBearingNonCheckpoint` FAILED pre-fix (`synthesis_gate_test.go:200`), PASSED post-fix |
| Runtime harness command/scenario and exact result | N/A — unit gate only, no runtime boundary (per tasks.md Unit 1 row) |
| Rollback boundary | `internal/sdd/synthesis_gate.go` + `internal/sdd/synthesis_gate_test.go` only — revert both to HEAD |
| `go vet ./internal/sdd/` | Clean (no output) |
| `git diff --stat` | 2 files, 75 insertions(+), 6 deletions(–) — within 400-line budget |

## Deviations from Design

None — implementation matches design (narrow to `IsCheckpointAsk`; `ShouldBlockApplyAdmission`, JS mirror, preflight admission left untouched for Units 2–4).

## Scope Guard

Units 2–4, installer TUI, `session_guard`, `edit_authority` untouched. `ShouldBlockApplyAdmission` intentionally unchanged (separate write-admission contract with its own pinning test, still passing).

## Remaining

- Unit 3: `HasExplicitPreflight` + dispatcher admission + tests
- Unit 4: fallback envelope wiring + full verification

---

# Unit 2 record: JS mirror + node parity tests (Phase 2: 2.1–2.4)

## Completed Tasks (Phase 2: 2.1–2.4)

- [x] 2.1 `BlockedFallbackEnvelope` + `BuildBlockedEnvelope` in `internal/sdd/synthesis_gate.go`: `Block:false` when allowed; on block `Reason` (same text as `CheckSynthesisPrecondition`), `Context` (`synthesis required before checkpoint ask: ` + full question string), `Fallback` (`FormatFallback(env)`, prompt + options verbatim). `ShouldBlock`/`CheckSynthesisPrecondition` untouched.
- [x] 2.2 `TestBlockedEnvelope_ReqDG2_FallbackVerbatim` — RED (build failed: `undefined: BuildBlockedEnvelope`) then GREEN.
- [x] 2.3 JS mirror: both gate sites (`wrapSingleTool` execute + `tool_call` guard) narrowed from `!isCheckpointAsk && !hasOptions` to `!isCheckpointAsk`; checkpoint-block paths now emit `blockedEnvelope(params, reason)` (`{block, reason, context, fallback}` via existing `formatFallback`) appended to `isError` text / `block` reason + notify (REQ-DG-2, nothing swallowed); stale comments updated. Fix B `hasOptions`-blocks test flipped to REQ-DG-1 allow-behavior (helper assertions kept).
- [x] 2.4 Parity test `REQ-DG-1/REQ-DG-2 parity vs Go fixtures` — RED (`parity: preflight option-ask without synthesis must allow`, actual `true` vs expected `undefined`) then GREEN after mirror fix.

## Files Changed (Unit 2)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/synthesis_gate.go` | Modified | Added `BlockedFallbackEnvelope` + `BuildBlockedEnvelope` (REQ-DG-2, reuses `FormatFallback`) |
| `internal/sdd/synthesis_gate_test.go` | Modified | Added `TestBlockedEnvelope_ReqDG2_FallbackVerbatim` |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Checkpoint-only gate (2 sites) + `blockedEnvelope` helper, exposed via `pi._biggzSynthesisGate`; comments updated |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modified | Flipped Fix B hasOptions test to REQ-DG-1; added parity test |

## Work Unit Evidence (Unit 2)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → 25 pass / 0 fail; `go test ./internal/sdd/ -run 'TestBlockedEnvelope_ReqDG2' -v` → PASS; full `go test ./internal/sdd/` → PASS (`ok ... 12.420s`); `go vet ./internal/sdd/` → clean |
| RED proof (pre-fix) | Go: `undefined: BuildBlockedEnvelope` build-fail; JS: `parity: preflight option-ask without synthesis must allow` (`actual: true, expected: undefined`) — both GREEN post-fix |
| Runtime harness command/scenario and exact result | N/A — mirror parity only, no runtime boundary (per tasks.md Unit 2 row) |
| Rollback boundary | `internal/sdd/synthesis_gate.go` + `internal/sdd/synthesis_gate_test.go` + `internal/assets/pi/biggz-synthesis-gate.js` + `internal/assets/pi/biggz-synthesis-gate.test.mjs` only — revert to HEAD |
| `git diff --stat` (Units 1+2 cumulative, uncommitted) | 4 files, 241 insertions(+), 33 deletions(–) — within shared 400-line attempt budget |

## Deviations from Design

None — implementation matches design (envelope `{context, fallback}` via existing formatters; JS mirrors Go checkpoint-only gate; `ShouldBlockApplyAdmission`, preflight admission untouched for Units 3–4).

## Scope Guard (Unit 2)

Units 1 (done, untouched logic), 3, 4, installer TUI, `session_guard`, `edit_authority`, `preflight.go`, `status.go`, `continue.go` untouched. `hasOptions` JS helper kept (tests reference it) but no longer gates.

---

# Unit 3 record: HasExplicitPreflight + dispatcher admission + tests (Phase 3: 3.1–3.4)

## Completed Tasks (Phase 3: 3.1–3.4)

- [x] 3.1 `HasExplicitPreflight(cwd, home...) bool` in `internal/sdd/preflight.go`: cache hit (`GetPreflightPrefs` ok) OR disk read ok (`ReadSddPreflightToDisk` ok) = true; silent defaults alone = false. `ResolvePreflightPrefs` behavior unchanged.
- [x] 3.2 RED (`undefined: HasExplicitPreflight/CheckPhaseEntryPreflight/NextPhaseChecked` build-fail) then GREEN: defaults-only→false, cache→true, disk→true in `internal/sdd/preflight_admission_test.go`.
- [x] 3.3 Phase-entry gate: shared `CheckPhaseEntryPreflight` admission helper in `internal/sdd/status.go` returns `blocked(preflight_missing)` + `resolve-blockers` until explicit; `NextPhaseChecked` in `internal/sdd/continue.go` enforces it and launches nothing on block, else normal `NextPhase` routing. Status reads (`Status`/`StatusWithOptions`/`NextPhase`) untouched.
- [x] 3.4 RED then GREEN newly-blocked-flows row: status-read without preflight succeeds; phase entry without preflight blocks (`blocked(preflight_missing)` + `resolve-blockers`); explicit preflight admits to normal routing (`spec`).

## Files Changed (Unit 3)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/preflight.go` | Modified | Added `HasExplicitPreflight(cwd, home...) bool` (cache > disk > false) |
| `internal/sdd/status.go` | Modified | Added `PreflightMissingReason` + shared `CheckPhaseEntryPreflight` admission helper (phase entry only) |
| `internal/sdd/continue.go` | Modified | Added gated `NextPhaseChecked` phase entry (block → error, no launch; admit → `NextPhase`) |
| `internal/sdd/preflight_admission_test.go` | Created | REQ-DG-3 tests: defaults/cache/disk explicitness, admission helper, read-vs-entry distinction |
| `openspec/changes/sdd-discipline-gates/tasks.md` | Modified | Phase 3 tasks 3.1–3.4 marked [x] |

## Work Unit Evidence (Unit 3)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/sdd/ -run 'Preflight' -v` → PASS (5 new ReqDG3 tests + 2 pre-existing VerifyPreflight tests, 0 fail) |
| RED proof (pre-fix) | New tests FAILED pre-impl (`undefined: HasExplicitPreflight`, `undefined: CheckPhaseEntryPreflight`, `undefined: NextPhaseChecked` build-fail) — GREEN post-impl |
| Full package | `go test ./internal/sdd/` → PASS (`ok ... 12.136s`); `go vet ./internal/sdd/` → clean; `gofmt -l` → no output |
| Runtime harness command/scenario and exact result | `NextPhaseChecked` without preflight → `blocked(preflight_missing)` + `resolve-blockers` error, no launch (covered in `TestPhaseEntryPreflight_ReqDG3_ReadVsEntry`); `biggz sdd-status` reads unaffected by construction (no derive-path change) |
| Rollback boundary | `internal/sdd/preflight.go`, `status.go`, `continue.go` + `internal/sdd/preflight_admission_test.go` only — revert/delete restores silent defaults |
| Unit 3 diff | 3 tracked files, 44 insertions(+), 0 deletions(-) + 1 new test file — within shared 400-line attempt budget |

## Deviations from Design

None — implementation matches design (separate `HasExplicitPreflight`; `ResolvePreflightPrefs` unchanged; shared admission helper; phase-entry-only gating; reads unaffected).

## Scope Guard (Unit 3)

Units 1–2 (done, untouched), Unit 4, installer TUI, `session_guard`, `edit_authority`, JS mirror, CLI entry points untouched. Attempt tok-e3ab5840e633db52102d2815 left OPEN for Unit 4 — not settled.

---

# Unit 4 record: Verification (Phase 4: 4.1–4.3)

**Change**: sdd-discipline-gates
**Unit**: 4 — fallback envelope wiring verification + full suite (PR 4, stacked-to-main)
**Attempt**: tok-e3ab5840e633db52102d2815 (shared work-unit units-2-4, left OPEN for Unit 5 settle — not settled)
**Status**: Units 1–4 complete — ready for Unit 5

No code changes in Unit 4 — fallback envelope wiring (`BlockedFallbackEnvelope`/`BuildBlockedEnvelope` Go, `blockedEnvelope` JS) and dispatcher `blocked(preflight_missing)` + `resolve-blockers` admission were implemented in Units 2–3; Unit 4 is verification only.

## Completed Tasks (Phase 4: 4.1–4.3)

- [x] 4.1 `go test -count=1 ./internal/sdd/...` → PASS (`ok ... 12.233s`); `go vet ./internal/sdd/` → clean
- [x] 4.2 `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → 25 pass / 0 fail (parity green)
- [x] 4.3 `git diff --stat` shows 7 files, 285 insertions(+), 33 deletions(–); `grep -iE 'install|tui'` on changed names → none (REQ-DG-4 scope guard holds)

## Files Changed (Unit 4)

| File | Action | What Was Done |
|------|--------|---------------|
| `openspec/changes/sdd-discipline-gates/tasks.md` | Modified | Phase 4 tasks 4.1–4.3 marked [x] |
| `openspec/changes/sdd-discipline-gates/apply-progress.md` | Modified | Unit 4 record appended |

## Work Unit Evidence (Unit 4)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/sdd/...` → PASS (`ok github.com/biggs-100/biggz-ai/internal/sdd 12.233s`); `node --test .../biggz-synthesis-gate.test.mjs` → 25 pass / 0 fail |
| Runtime harness command/scenario and exact result | Blocked checkpoint ask → same-turn fallback + `blocked(preflight_missing)` → `resolve-blockers` paths covered by Unit 2–3 tests (no new runtime boundary in Unit 4) |
| Rollback boundary | No production files touched — revert the two openspec tracking files only |
| `git diff --stat` (Units 1–4 cumulative, uncommitted) | 7 files, 285 insertions(+), 33 deletions(–) — within shared 400-line attempt budget; zero installer/TUI files |

## Deviations from Design

None — implementation matches design; Unit 4 verification-only as planned.

## Scope Guard (Unit 4)

Production code untouched. Attempt tok-e3ab5840e633db52102d2815 left OPEN for Unit 5 settle — not settled.

---

# Unit 5 record: Ledger multi-unit lifecycle + CLI hardening (Phase 5: 5.1–5.3)

**Change**: sdd-discipline-gates
**Unit**: 5 — ledger multi-unit lifecycle + CLI hardening (PR 5, stacked-to-main)
**Attempt**: tok-e3ab5840e633db52102d2815 (shared open attempt — left OPEN, final settle happens after per orchestrator instruction)
**Status**: Units 1–5 complete — ready for verify

## Completed Tasks (Phase 5: 5.1–5.3)

- [x] 5.1 `TestSettle_ProgressMultiUnitSingleFinalSettle` in `internal/sddattempt/acquire_settle_test.go` — RED (`invalid outcome "progress"`) then GREEN. Choice: **new non-terminal outcome value `progress`** (option A). `Settle` accepts `progress`; it closes the attempt exactly like the failure path (`NextAction "begin"`, `ActiveAttempt 0`, ledger stays open, no `Complete`), so the next acquire in the same `WorkUnit` scope is admitted and only the final `settle(passed)` completes. `progress` consumes one delivered attempt of budget like `failed` (fail-closed: no infinite free checkpoints); `Finish` (legacy begin/finish path) is untouched and still strict. Multi-unit runs keep one `WorkUnit` scope id across unit acquires — the scope-change guard stays fail-closed.
- [x] 5.2 Fail-closed fix chosen (no decision-not-to): `default` case in `sddAttemptRun` flag loop (`cmd/biggz/cli_sdd.go`) → `error: unknown flag <f>` exit 1, byte-mirroring `sddStatusRun`. RED proof: new `TestSDDAttemptUnknownFlagFailClosed` (`cmd/biggz/sdd_attempt_unknown_flag_test.go`, `--cwd`/`--change`/`--bogus`) FAILED pre-fix (exit 0, flag swallowed) via stash check, PASSES post-fix.
- [x] 5.3 `HelpText` (`internal/sddattempt/sddattempt.go`): `<change>` is positional (word after operation), no `--change`/`--cwd` flags, workspace root always `os.Getwd()`; `--outcome` documents `progress` as settle-only non-terminal checkpoint.

## Files Changed (Unit 5)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sddattempt/sddattempt.go` | Modified | `Settle` accepts `progress` + doc comments; `HelpText` positional-`<change>`/cwd clarification + settle `--outcome` line |
| `internal/sddattempt/acquire_settle_test.go` | Modified | Added `TestSettle_ProgressMultiUnitSingleFinalSettle` |
| `cmd/biggz/cli_sdd.go` | Modified | `default: unknown flag` fail-closed in `sddAttemptRun` flag loop |
| `cmd/biggz/sdd_attempt_unknown_flag_test.go` | Created | RED→GREEN unknown-flag test via `runSDDAttemptCLI` helper |
| `openspec/changes/sdd-discipline-gates/tasks.md` | Modified | Phase 5 tasks 5.1–5.3 marked [x] |
| `openspec/changes/sdd-discipline-gates/apply-progress.md` | Modified | Unit 5 record appended |

## Work Unit Evidence (Unit 5)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test -count=1 ./internal/sddattempt/ ./internal/sdd/` → PASS (both `ok`); `go test ./cmd/biggz/ -run TestSDDAttemptUnknownFlagFailClosed` → PASS |
| RED proof (pre-fix) | 5.1: `Settle(progress): invalid outcome "progress"` FAIL → GREEN post-fix. 5.2: stash of `cli_sdd.go` fix → `flag --cwd: exit code = 0, want 1` FAIL → GREEN after restore |
| Runtime harness command/scenario and exact result | Acquire→settle(progress)→acquire→settle(passed) two-unit lifecycle green in-process (test asserts mid-ledger `begin`/unblocked and final `Complete`); `sdd-attempt acquire/settle` CLI verbs unchanged (flag parse only, no verb behavior change) |
| Rollback boundary | `internal/sddattempt/sddattempt.go` + `acquire_settle_test.go` + `cmd/biggz/cli_sdd.go` + `cmd/biggz/sdd_attempt_unknown_flag_test.go` only — revert/delete restores strict 3-outcome settle + silent unknown flags |
| `go vet` | `go vet ./internal/sddattempt/ ./internal/sdd/ ./cmd/biggz/` → clean |
| `git diff --stat` (Units 1–5 cumulative, tracked) | 10 files, 359 insertions(+), 36 deletions(–) — under 400-line budget; zero installer/TUI files; Unit 5 tracked delta is 74 insertions, 3 deletions |

## Deviations from Design

None — Phase 5 scope (ledger lifecycle + CLI hardening) was recorded in tasks.md Unit 5 row; the `progress` outcome extends the acquire/settle bounded path only, `Begin`/`Finish` semantics unchanged.

## Scope Guard (Unit 5)

Units 1–4 (done, untouched logic), installer TUI, `session_guard`, `edit_authority`, JS mirror, preflight/dispatcher untouched. Attempt tok-e3ab5840e633db52102d2815 left OPEN — NOT settled per instruction; final settle happens after.
