# Tasks: fix-orchestrator-checkpoint-synthesis

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 70-100 (gate.js ~30, test.mjs ~40) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |
| Review budget | 800 lines |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Gate strict restore (JS) | PR 1 (single) | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` | N/A — in-process marker check, no runtime deploy | `internal/assets/pi/biggz-synthesis-gate.js` revert restores relaxed gate |
| 2 | Test contract rewrite (5 tests → block) | PR 1 (same) | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` + `go test ./internal/sdd -run TestHasSynthesis -count=1` | N/A — unit harness only | `internal/assets/pi/biggz-synthesis-gate.test.mjs` revert restores allow expectations |

## Phase 1: Setup

- [x] 1.1 Verify `internal/assets/pi/biggz-synthesis-gate.js` lines 630-670, 750-850, 994-1030 and Go truth `internal/sdd/synthesis_gate.go:HasSynthesis/ShouldBlock` — Done: mismatch documented
- [x] 1.2 Create branch from `main` and confirm specs `specs/orchestrator/spec.md` (REQ-001/002/003/005/007) and `specs/pi-integration/spec.md` (REQ-001/004/005/006/008) — Done: branch ready

## Phase 2: Gate Fix

- [x] 2.1 Edit `internal/assets/pi/biggz-synthesis-gate.js:639-670` — make `checkSynthesisPrecondition` strict: only `currentTurnMarkdown + hasSynthesis + now-currentTurnUpdateTime ≤120s` returns true; delete `getCurrentTurnSynthesis/getSynthesisSource` fallback allow — Done: Given history-only synthesis When checkpoint ask Then block isError:true
- [x] 2.2 Edit `wrapSingleTool` `750-850` — replace `emitHistoryFallbackWarning` allow with `return {isError:true}+notify` — Done: Given currentTurn empty history has markers When ask Then blocked
- [x] 2.3 Edit `pi.on("tool_call")` `994-1030` — same strict block `{block:true, reason}` + notify — Done: Given tool_call without currentTurn synthesis Then block:true
- [x] 2.4 Preserve `anySynthesis==""` preflight allow, `checkSessionRecallInCurrentTurn()` same-turn Recall bypass, `PI_SUBAGENT_CHILD=1` child bypass, `BIGGZ_ADVISE=1` thin concern fallback only for advise — Done: Given preflight/child/Recall When ask Then allow per REQ-003/004/005

## Phase 3: Tests

- [x] 3.1 Rewrite 5 tests in `internal/assets/pi/biggz-synthesis-gate.test.mjs` — `allow with history fallback` → `block when only history has synthesis` assert `isError:true`, `originalCalled==false`, notify has `Please synthesize before asking` — Done: Given currentTurn="" lastAssistant rich ≤120s When checkpoint proceed Then blocked
- [x] 3.2 Keep coverage: rich currentTurn ≤120s → allow; expired 121s → block; thin+BIGGZ_ADVISE=1 → allow+concern thin; thin without flag → silent allow; preflight anySynthesis=="" → allow — Done: node --test exit 0

## Phase 4: Docs

- [x] 4.1 Add parity comment in `internal/assets/pi/biggz-synthesis-gate.js` header referencing `internal/sdd/synthesis_gate.go:ShouldBlock` as truth; note `internal/assets/biggz/biggz-orchestrator.md` drift risk (no edit) — Done: comment visible
- [x] 4.2 No edits to `internal/sdd/pending.go` / `synthesis.go` — Done: doc-only, zero diff

## Phase 5: Verify

- [x] 5.1 Run `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` — Done: PASS 22/22, no fallback-warning, strict block on history-only
- [x] 5.2 Run `go test ./internal/sdd -run TestHasSynthesis -count=1` — Done: PASS (no tests to run, HasSynthesis truth unchanged) + go test ./internal/sdd full PASS except pre-existing TestReadLoopLarge unrelated
- [ ] 5.3 Run `biggz sdd-verify-validate --requirements 8 --scenarios 16` on verify report draft — Done: validator PASS, no staged files (deferred to verify phase)
