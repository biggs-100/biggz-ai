# Apply Progress — 2026-08-27-synthesis-gate-hardening

**Change:** 2026-08-27-synthesis-gate-hardening
**Mode:** Standard (strict_tdd: false)
**Artifact store:** openspec
**Delivery:** auto-chain | single PR (chained PRs: No, 280–450 líneas Low risk, budget 800)
**Chain strategy:** pending (single PR; split→stacked-to-main)

## Phase 1: Foundation — Pi Gate Hardening

- [x] 1.1 Audit `internal/assets/pi/biggz-synthesis-gate.js`: 4 markers, source `currentTurn→history→lastAssistant` 120s, thin `count<2||len<50`, advise `BIGGZ_ADVISE=1`, bypass only `PI_SUBAGENT_CHILD=1`, expose `_biggzSynthesisGate`
  - Evidence: file audited 535 lines; `hasSynthesis` checks `## Sub-agent Result` + `Artifacts/Paths` + `Risks` + `Next`; `checkSynthesisPrecondition` order `currentTurnMarkdown` (≤120000 ms / same-turn) → `ctx.history` → `lastAssistant`; `isThinSynthesis` via `getArtifactsMetrics` <2 or <50; `isAdviseEnabled` via `BIGGZ_ADVISE=1|true` or settings; `isChildBypass` only `PI_SUBAGENT_CHILD=1`; `_biggzSynthesisGate` exposes `hasSynthesis`, `extractArtifactsSection`, `countPaths`, `getArtifactsMetrics`, `isThinSynthesis`, `isAdviseEnabled`, `checkSynthesisPrecondition`, `_test`.
  - Action: no edit required — implementation already satisfies spec/design (b0d2fc1 hardening).

- [x] 1.2 Fix gate: block `{isError:true, text:"Please synthesize..."}` no `original()` + `notify` error; thin advise `concern: synthesis is thin` warning only, no model call; `recordText` handles race, resets after `original()`
  - Evidence: `pi.registerTool` wrapper returns `{content:[{type:"text", text:"Please synthesize before asking — missing ## Sub-agent Result block..."}], isError:true}` without calling `original()`, notifies `ctx.ui.notify` + `pi.notify` error; thin path `emitConcern` warning + allow; `recordText` accumulates `currentTurnMarkdown` and resets after successful `original()`; `pi.on("tool_call")` secondary guard. Verified by `node --test` scenario 1 asserts `isError:true` and `originalCalled==false`; scenario 2/3 thin behavior; no `callModel` invoked.

- [x] 1.3 Verify helpers `hasSynthesis`/`isThinSynthesis`/`getArtifactsMetrics`/`checkSynthesisPrecondition`; Given `Artifacts: -` Then thin true; Given 3 paths 120 chars Then thin false
  - Evidence: helper `isThinSynthesis(thinMarkdown)` true (count=1 len<50), `richMarkdown` count 3 len≥50 false, `hasSynthesis(missing)` false, `hasSynthesis(rich)` true, `getArtifactsMetrics` validated in `node --test` helper suite (9 tests).

## Phase 2: Orchestrator Template + Integration Test

- [x] 2.1 Verify `internal/assets/biggz/biggz-orchestrator.md` has copy-paste block with 4 markers, `INVALID and will be blocked`, and 12× `REMINDER: synthesis markdown is separate...`
  - Evidence: `grep -c "REMINDER: synthesis markdown is separate"` → 12; `grep -n "INVALID and will be blocked"` → 2 hits (copy-paste blocks); `## Sub-agent Result` 4 hits, `Artifacts/Paths` 2, `Risks / Open Questions` 2, `Next Recommended` 2; file 693 lines already contains required invariant. No edit required.

- [x] 2.2 Create `internal/assets/biggz/orchestrator.test.go` reading template, asserting 4 markers + `INVALID` + `REMINDER`; Given drift removes marker When `go test` Then fail
  - Created: `internal/assets/biggz/orchestrator_test.go` (package `biggZ_test`, 86 lines) — reads embedded `assets.FS.ReadFile("biggz/biggz-orchestrator.md")`, asserts `## Sub-agent Result` / `**Artifacts/Paths:**` / `**Risks / Open Questions:**` / `**Next Recommended:**` + canonical header `## Sub-agent Result: {phase/agent}`, `INVALID and will be blocked`, `synthesis markdown is separate`, `separate chat markdown emitted FIRST` + `Do NOT put synthesis inside the tool's question param`, counts `REMINDER` ≥12, and drift guard for marker counts. File uses Go 1.25, `gofmt` clean.
  - Note on naming: delivered as `orchestrator_test.go` (Go idiomatic `_test.go`) instead of `orchestrator.test.go` so `go test` discovers it; content satisfies spec intent and design File Changes row.

- [x] 2.3 Run `go vet ./internal/assets/biggz/... && go test ./internal/assets/biggz -count=1` — exit 0
  - `go vet ./internal/assets/biggz` → exit 0
  - `go test ./internal/assets/biggz -count=1 -v` → PASS (2 tests, 6 subtests) in 0.4s

## Phase 3: Unit Tests — 4 Gate Scenarios

- [x] 3.1 Verify `internal/assets/pi/biggz-synthesis-gate.test.mjs` covers 4 scenarios: missing→`isError:true` not-called, rich→pass, thin+`BIGGZ_ADVISE=1`→warn pass, thin no-flag→silent; plus child bypass + same-turn race
  - Evidence: `internal/assets/pi/biggz-synthesis-gate.test.mjs` 410 lines, 9 tests in suite `biggz-synthesis-gate advisor dual-mode — fixtures no network`.
  - Scenario 1 (missing → block) loops advise off/on, asserts `isError:true` + `Please synthesize` and `originalCalled==false`.
  - Scenario 2 (thin + BIGGZ_ADVISE=1 → warn pass) asserts allow + `concern: synthesis is thin` via `ctx.ui.notify`/`pi.notify`, plus `tool_call` handler path.
  - Scenario 3 (thin silent without flag) asserts allow without concern for both wrapper and `tool_call`.
  - Scenario 4 (rich never concern) asserts allow without concern even with flag.
  - Plus: child bypass (PI_SUBAGENT_CHILD=1 skips both), settings flag path (`pi.settings.advise`), no-model-call, same-turn race via `assistant_message` → `currentTurnMarkdown`.

- [x] 3.2 Cover helpers: Given `Artifacts: -` Then `isThinSynthesis` true; Given rich 3 paths 120 chars Then false; Given missing Then `hasSynthesis` false
  - Evidence: first test `heuristic helpers: thin vs rich classification` asserts exactly those cases plus `extractArtifactsSection` len <50, metrics count≥2 len≥50 for rich.

- [x] 3.3 Run `node --check .../biggz-synthesis-gate.js && node --test .../biggz-synthesis-gate.test.mjs` — all green, asserts `originalCalled==false` on block
  - `node --check internal/assets/pi/biggz-synthesis-gate.js` → exit 0
  - `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → 9 pass, 0 fail, duration ~95–110ms

## Phase 4: Documentation + CI Verification

- [x] 4.1 Update `docs/architecture.md` with `### Synthesis Gate (3-layer defense)` — prompt→gate→tests/CI, blocking vs advise, source priority, bypass
  - Modified: `docs/architecture.md` +22 lines insertion after RDD section, before OpenCode Plugins. Content: 3 layers (Prompt invariant 4 markers + INVALID same-turn + 12× REMINDER; Gate blocking `isError:true`/`Please synthesize`, source `currentTurn→history→lastAssistant` 120s, same-turn buffer fix, thin `count<2||len<50` advise gated `BIGGZ_ADVISE=1`/settings, only `PI_SUBAGENT_CHILD=1` bypass, expose `_biggzSynthesisGate`; Tests/CI unit + integration + `go vet`/`go test`/`node --check`/`node --test` + status command). No migration, rollback via revert.

- [x] 4.2 Final CI: `go vet ./... && go test ./... -count=1 -timeout 180s && node --check .../biggz-synthesis-gate.js && node --test .../biggz-synthesis-gate.test.mjs` — exit 0 (focused variant per follow-up)
  - Focused CI executed per follow-up (full `go test ./...` flaky 2 FAIL in `internal/install` pre-existing Windows — excluded):
    - `go vet ./internal/assets/biggz` → passed
    - `go test ./internal/assets/biggz -count=1` → passed (2 tests)
    - `node --check internal/assets/pi/biggz-synthesis-gate.js` → passed
    - `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → passed (9 tests)
  - Unfocused full `go vet ./...` previously passed (0) and `go test ./...` passed except 2 flaky `internal/install` Windows failures noted as not blocking per instruction.

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/biggz/orchestrator_test.go` | Created | Integration test reading embedded `biggz-orchestrator.md` via `assets.FS`, asserting 4 markers + `INVALID` + `REMINDER` 12× + drift guard (86 lines) |
| `docs/architecture.md` | Modified | Added `### Synthesis Gate (3-layer defense)` 22-line subsection after RDD, documenting prompt→gate→tests/CI, blocking vs advise, source priority 120s, bypass, helpers, CI gates |
| `internal/assets/pi/biggz-synthesis-gate.js` | Verified (no edit) | Already implements 4-marker blocking, source priority 120s, thin heuristic, advise gating, child-only bypass, helpers exposed — audit passed |
| `internal/assets/biggz/biggz-orchestrator.md` | Verified (no edit) | Already contains 12× REMINDER, 2 copy-paste blocks with 4 markers, INVALID rule — audit passed |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Verified (no edit) | Already covers 4 scenarios + helpers + child bypass + race — 9 tests green |
| `openspec/changes/2026-08-27-synthesis-gate-hardening/tasks.md` | Modified | Marked 11 tasks [x] complete |
| `openspec/changes/2026-08-27-synthesis-gate-hardening/apply-progress.md` | Created | This file (merge-ready, no overwrite) |

## Work Unit Evidence

| Evidence | Required value |
|----------|---------------|
| Focused test command and exact result | `go vet ./internal/assets/biggz` → exit 0; `go test ./internal/assets/biggz -count=1 -v` → PASS 2 tests/6 subtests 0.43s; `node --check internal/assets/pi/biggz-synthesis-gate.js` → exit 0; `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → 9 pass 0 fail |
| Runtime harness command/scenario and exact result | Manual scenarios exercised via `biggz-synthesis-gate.test.mjs` fixtures (no network): missing→`isError:true` not-called (advises off/on loops), rich→pass no warn, thin+`BIGGZ_ADVISE=1`→warn pass (count=1 len=4 metrics in concern), thin silent without flag→silent, child bypass `PI_SUBAGENT_CHILD=1`→skip both block and advise, same-turn `assistant_message`→`currentTurnMarkdown` race fix (elapsed <1000ms, allow) |
| Rollback boundary | Revert commit; 1 new + 1 modified + 3 verified files — no migration; stash `temp stash unrelated gofmt changes synthesis-gate` preserves unrelated gofmt diff |

## Deviations from Design

- Task 2.2 file delivered as `orchestrator_test.go` (underscore) rather than `orchestrator.test.go` (dot) to satisfy `go test` discovery (`_test.go` required). Content and intent match spec/design exactly; dot-named copy would be treated as regular source and never executed.
- Final CI run per follow-up used focused commands (`go vet ./internal/assets/biggz` + `node --check` + focused `go test`/`node --test`) instead of full `go test ./...`; 2 FAIL in `internal/install` on Windows are pre-existing flaky and excluded per instruction.

## Issues Found

- `internal/install` 2 FAIL (`TestDeployMCPMergeIntoSettings_WritesBiggzServer`, `TestProvisionBigMemMCP_WritesBothFiles`) are pre-existing Windows flaky — verified before and after change, not introduced by this change, excluded from gate per follow-up.
- No other issues; gate JS, orchestrator.md, and JS tests were already fully hardened — only Go integration test and docs were missing.

## Workload / PR Boundary

- Mode: single PR
- Current work unit: 3-layer gate (prompt+blocking+tests+docs) — PR 1
- Boundary: audited gate JS + verified orchestrator template (no edit) → created Go integration test + verified JS unit tests (no edit) → added architecture gate docs → focused verification (no full suite per instruction)
- Estimated review budget impact: ~108 changed lines tracked by git (22 docs + 86 test) + ~945 lines verified but not diffed; well under 800 budget (~13% of budget, Low risk); single PR no split

## Status

11/11 tasks complete. Ready for verify (`sdd-verify` → `biggz sdd-verify-validate`).
