# Proposal: fix-orchestrator-checkpoint-synthesis

## Intent

Restore strict Post-Delegation Human Checkpoint invariant. Users: orchestrator in `interactive` mode after delegation. JS gate (`biggz-synthesis-gate.js`) drifted to relaxed `history fallback allow + warning` (639-670, 750-850, 994-1030), letting orchestrator `ask_user_question` without same-turn `## Sub-agent Result` — losing artifacts/risks/next and bypassing `SavePendingDualWrite` (`pending.go`). Evidence: explore `e51aee1b`, `synthesis_gate.go:HasSynthesis` 4 markers / `ShouldBlock = !child && !recall && checkpoint && ≤120s && !HasSynthesis` remains strict truth; JS must match.

## Scope

### In Scope
- Revert `biggz-synthesis-gate.js` to strict: only `currentTurnMarkdown ≤120s + HasSynthesis` satisfies block; history only for advise.
- Remove `emitHistoryFallbackWarning` allow-path in `wrapSingleTool` + `tool_call` handler → `isError:true`/`block:true` + notify.
- Rewrite `biggz-synthesis-gate.test.mjs`: 5 tests `allow with history fallback` → `block when only history has synthesis`.
- Preserve `PI_SUBAGENT_CHILD=1` and `## Session Recall` preflight bypass.

### Out of Scope
- `polish-wait-visuals` (next chained change).
- `pi-subagents` FleetView, `chain_strategy`, `pending.go`/`synthesis.go` logic.
- Full rewrite of `biggz-orchestrator.md` duplication (2x checkpoint block) — note only as drift risk.

## Capabilities

### New Capabilities
- None — invariant restore only.

### Modified Capabilities
- `orchestrator`: checkpoint synthesis invariant (same-turn 4 markers).
- `pi-integration`: JS strict gate + test contract.

## Approach

**Option A — strict restore (chosen):** `currentTurnMarkdown` sole blocking source, identical to `synthesis_gate.go:ShouldBlock`. Advise (`BIGGZ_ADVISE=1`) may fallback `currentTurn→history→lastAssistant` for `concern: thin` only, never for block.

Rejected: **Option C — pending-only** (enforce `SavePendingDualWrite` earlier without fixing gate) — masks bug, checkpoint still leaks.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Remove fallback allow, enforce strict block |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modified | 5 tests → strict expectation |
| `internal/assets/biggz/biggz-orchestrator.md` | Note | Drift risk — no edit now |
| `internal/sdd/synthesis_gate.go` | Ref | Truth — unchanged |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Streaming race — markdown 500ms before `tool_call` missed | Med | Keep `currentTurnMarkdown` buffer + `message_end`/`update` tracking; 120s window unchanged |
| Preflight regression — first ask blocked despite no synthesis ever | Low | Keep `anySynthesis==""` allowance in both handlers; dedicated test |

## Rollback Plan

`git revert HEAD` → restores JS + test to relaxed. No migration/BigMem cleanup. No `PI_SYNTHESIS_GATE_STRICT` shim now.

## Dependencies

None — P0 isolated.

## Success Criteria

- [ ] Checkpoint `ask` with only history synthesis + empty `currentTurn` → **blocked** (`isError:true` / `{block:true}`).
- [ ] Checkpoint `ask` with `currentTurn` synthesis ≤120s → **allowed**.
- [ ] Preflight (no synthesis anywhere) → allowed. `PI_SUBAGENT_CHILD=1` and `## Session Recall` bypass intact.
- [ ] `node --test biggz-synthesis-gate.test.mjs` PASS.
- [ ] `go test ./internal/sdd -run TestHasSynthesis -count=1` PASS.

## Estimate

<100 lines (`gate.js` ~30, `test.mjs` ~40). Single PR, no chaining.
