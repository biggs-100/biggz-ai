# Design: fix-orchestrator-checkpoint-synthesis

## Technical Approach

Restore strict checkpoint invariant drifted in `biggz-synthesis-gate.js`. Only `currentTurnMarkdown` ≤120s with 4 markers satisfies `ask_user_question`/`question`. Mirrors Go truth `synthesis_gate.go:ShouldBlock`. Remove `emitHistoryFallbackWarning` allow paths (639-670, 750-850, 994-1030). History kept only for `BIGGZ_ADVISE=1` thin concern. No edit to `pending.go`/`synthesis.go`.

Maps to proposal Option A; satisfies REQ-001/002/003/005/007 + REQ-001/004/005/006/008.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| **Block source** | A) `currentTurn` only strict <br> B) `current→history→last` relaxed | A matches Go, prevents leaks; B loses pending persist | **A** — `checkSynthesisPrecondition` uses only `currentTurnMarkdown` |
| **History fallback** | A) remove all <br> B) keep for advise thin | A simpler, B preserves ergonomics | **B** — `getCurrentTurnSynthesis` fallback only for `isThinSynthesis` when `BIGGZ_ADVISE=1`, never for block |
| **Window & bypasses** | Preserve vs tighten 120s | Preserve keeps parity | **Preserve** 120s, `PI_SUBAGENT_CHILD=1`, `## Session Recall` same-turn, preflight `anySynthesis==""` allow |

`HasSynthesis` = `## Sub-agent Result` && `**Artifacts/Paths:**` && `**Risks / Open Questions:**` && `**Next Recommended:**` && (`**What was done:**` || `| Topic | Decision |`).
`ShouldBlock(q,md,now)` = `!IsChildBypass() && !HasSessionRecall(md) && IsCheckpointAsk(q) && now-currentTurnTime≤120s && !HasSynthesis(md)`.

## Data Flow

```
message_update/end → recordText → currentTurnMarkdown + currentTurnUpdateTime
                        (reset turn_start/agent_start/tool_execution_end)
      ↓
ask → wrapSingleTool.execute()─┬ IsChildBypass? → allow
      │                        ├ !IsCheckpointAsk? → allow (+advise check)
      │                        ├ checkSynthesisPrecondition(currentTurn) ?
      │                        │   hasSynthesis→allow | Recall→allow | anySynthesis==""→allow | else isError:true+notify
      │                        └ allow → SavePendingDualWrite (BigMem+state.yaml+VerifyEquality retry1)
      └ pi.on("tool_call") mirrors strict → {block:true}
history/last → advise-only, never for block
```

| `currentTurn` | `history/last` | age | Checkpoint | Result |
|---------------|----------------|-----|------------|--------|
| 4 markers | * | ≤120s | Y | `allow` → persist |
| 4 markers | * | >120s | Y | `block` expired |
| Empty | markers ≤120s | * | Y | `block` isError:true |
| Empty | Empty | * | Y | `allow` iff `anySynthesis==""` else `block` |
| Any | * | * | N | `allow` general bypass |
| `## Session Recall` | * | * | Y | `allow` same-turn |

- **Window:** `Date.now()-currentTurnUpdateTime` vs `time.Now().Sub(currentTurnTime)` strict ≤120s; 121s = block.
- **Sanitize:** `stripAnsi/stripOsc/TruncateToWidth` before `VisibleWidth`; table cell budget 17→80, lifecycle `◆ Phase · Status · Next` green/yellow/red+dim.
- **Race:** `recordText` accumulates `message_update` chunks; `message_end` finalizes; `currentTurn` window covers 500ms debounce. `turn_start` reset enforces same-turn.

## File Changes

| File | Action | Description | Est |
|------|--------|-------------|-----|
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | 639-670: `checkSynthesisPrecondition` → only `currentTurnMarkdown` ≤120s, delete fallback. 750-850: `wrapSingleTool` replace fallback with `isError:true`+`pi.notify`/`ctx.ui.notify`. 994-1030: `tool_call` same `block:true`. Keep `anySynthesis==""` + `checkSessionRecallInCurrentTurn`. | ~30 |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modify | Rewrite 5 `allow with history fallback` → `block when only history has synthesis` (`isError:true`, `originalCalled==false`). Keep rich/thin/child/preflight. | ~40 |
| `internal/sdd/synthesis_gate.go` | Ref | Truth unchanged; parity comment only | 0 |
| `internal/sdd/pending.go` `internal/sdd/synthesis.go` | Note | No edits; doc only — persist only on `allow` | 0 |
| `internal/assets/biggz/biggz-orchestrator.md` | Note | Drift risk noted, no edit | 0 |

## Interfaces / Contracts

```js
hasSynthesis(text) // 4 markers incl. table variant
checkSynthesisPrecondition(ctx) // STRICT only currentTurnMarkdown ≤120s
isCheckpointAsk(params) // {proceed,adjust,stop,continue,correct} in label/value/id/name/title
getCurrentTurnSynthesis(ctx) // current→history→last (120s) advise-only
isThinSynthesis(text) // markers && (count<2||len<50)
pi.on("message_end"|"message_update", e=>recordText(extractAssistantText(e)))
pi.on("tool_call",(ev,ctx)=> checkpoint? strict? {block:true}:allow)
wrapSingleTool(def) // idempotent _biggzGateWrapped, notify pi.notify/ctx.ui.notify
// Go: HasSynthesis(md) bool; ShouldBlock(q,md,now) bool; CheckSynthesisPrecondition(q,md)
// Pending: SavePendingDualWriteAt(root,change,pq) dual BigMem sdd/{change}/pending-q + state.yaml + VerifyEquality retry1
```

Bypasses: `PI_SUBAGENT_CHILD=1` → immediate allow (skip block+advise). `## Session Recall` only in `currentTurnMarkdown` bypasses synthesis (narrow preflight). `BIGGZ_ADVISE=1` thin concern fallback emits `concern: synthesis is thin` without blocking.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit JS | 5 strict blocks only-history → `isError:true`, `originalCalled==false` | `node --test biggz-synthesis-gate.test.mjs` `createMockPi`/`makeCtx` |
| Unit JS | Rich allow; expired block; preflight allow; child bypass; Recall same-turn allow; thin+advise concern | Same file isolated env |
| Unit Go | `HasSynthesis` incl. table, `ShouldBlock` formula, window | `go test ./internal/sdd -run TestHasSynthesis` |
| Integration | allow→dual-write identical bytes; block→no write | Temp `state.yaml` + BigMem harness |
| Manual | Notify, color, truncation width | `synthesis-gate-status` visual |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR or secret-path boundary. Gate is in-process marker check; no `rm -rf`/`git push --force`. If future adds shell/PR routing, re-evaluate `references/threat-matrix.md`.

## Migration / Rollout

No migration. Single PR <100 lines. `git revert HEAD` restores relaxed gate. Flag `PI_SYNTHESIS_GATE_STRICT` deferred. CI: `node --test` PASS + `go test` PASS. Next: `polish-wait-visuals`.

## Open Questions

- [ ] `biggz-thinking-wrap.js` duplication note sufficient vs separate fix
- [ ] Deprecate `getSynthesisSource` alias to avoid drift?
