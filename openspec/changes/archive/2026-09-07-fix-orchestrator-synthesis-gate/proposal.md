# Proposal: Fix Orchestrator Synthesis Gate

## Intent

Restore fail-closed gate retired 2026-09-04 (`ShouldBlock`/`BuildBlockedEnvelope` deleted, JS passthrough). `branch-worktree-cleanup` did 8 delegations but only 2 syntheses → 6 missing checkpoints unblocked. Require synthesis after **every** sub-agent, plain chat FIRST, same turn.

## Scope

### In Scope
- Go `synthesis_gate.go`: `ShouldBlock`/`CheckSynthesisPrecondition`/`BuildBlockedEnvelope`/`ShouldBlockApplyAdmission` + `SetCurrentTurnMarkdown` 120s; `HasSynthesis` 4 markers + table, `IsCheckpointAsk` label-only
- JS `biggz-synthesis-gate.js`: `{isError:true}`/`{block:true}`, strict `currentTurnMarkdown` 120s, thin `count<2||len<50` warn (`BIGGZ_ADVISE=1`), resets + sweep
- Docs `biggz-orchestrator*.md`: `INVALID and will be blocked` + 12× REMINDER + self-check
- Tests: Go/JS blocking fixtures + transcript lint (8 phases → 7 blocks)
- CI `go test`/`node --test`/`go vet`/`node --check`

### Out of Scope
- Body-text `IsCheckpointAsk`, history satisfying block, marker translation, new deps

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `sdd-discipline-gates`: REQ-DG-1/2 fail-closed + fallback
- `orchestrator`: REQ-ORCH-001 120s + template + INVALID
- `sdd`: Gate markers + 120s + ApplyAdmission

## Approach

- **Go** (`cf840430^`): label-only (`label/value/id/name/title`), `ShouldBlock=!child&&!recall&&isCheckpoint&&(!HasSynthesis||>120s)`, `BuildBlockedEnvelope`=`context`+`FormatFallback`, admission ignores bypass
- **JS**: unwrap passthrough, `currentTurnMarkdown` only, thin warn if `BIGGZ_ADVISE=1`, keep `PI_SUBAGENT_CHILD=1`, preflight/`## Session Recall` narrow, `turn_start` reset, `message_end` buffer, sweep
- **Docs**: restore blocking wording + self-check before `question`

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/synthesis_gate.go` | Modified | Blocking + 120s state |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Fail-closed + thin advise |
| `internal/sdd/synthesis.go` | Modified | Marker invariant |
| `internal/assets/biggz/biggz-orchestrator*.md` | Modified | Blocking + REMINDER + check |
| `*synthesis_gate*test*` | Modified | Passthrough→blocking |
| `docs/architecture.md` | Modified | 3-layer defense |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Streaming race | Med | `message_end` buffer + `currentTurnUpdateTime` |
| History false pass | Low | `currentTurnMarkdown` only |
| Body-token false block | Low | Label-only scan |
| Marker translation | Low | English whitelist |
| Thin spam | Med | Warn only `BIGGZ_ADVISE=1` |
| Bypass too wide | Low | Same-turn narrow |

## Rollback Plan

`git revert cf840430+944e284e` — no migration, advise passthrough. Verify tests green.

## Dependencies

None

## Success Criteria

- [ ] Checkpoint w/o `HasSynthesis` same turn ≤120s → Go `block:true` + JS `isError:true`/`block:true` + fallback, orig not called
- [ ] History-only/expired 121s → block; rich current → allow; non-checkpoint → never block
- [ ] Thin (`<2||<50`) → `pi.notify` only if `BIGGZ_ADVISE=1`
- [ ] `PI_SUBAGENT_CHILD=1` / preflight / same-turn `## Session Recall` → allow
- [ ] Transcript lint 8 phases → 7 blocks; CI green
