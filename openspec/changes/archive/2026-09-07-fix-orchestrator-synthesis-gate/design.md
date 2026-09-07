# Design: Fix Orchestrator Synthesis Gate

## Technical Approach

Restore fail-closed gate deleted `cf840430`+`944e284e`. Go `synthesis_gate.go` canonical (`ShouldBlock` 120s same-turn, label-only); JS `biggz-synthesis-gate.js` mirrors with `isError:true`/`block:true`+fallback; docs un-retire blocking contract; tests flip passthrough to blocking plus 8→7 transcript lint. Covers REQ-DG-1/2, REQ-ORCH-001, REQ-SDD.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Go globals `currentTurnMarkdown`/`currentTurnTime` + `SetCurrentTurnMarkdown` vs ctx threading | Globals add state but match Pi buffer, no harness plumbing | **Restore `ShouldBlock(question,md,now)` with `now-currentTurnTime<=120s && HasSynthesis(currentTurnMarkdown)`, reset on turn boundaries. Go canonical.** |
| JS dual block `wrapSingleTool.execute` + `tool_call` vs single | Single bypasses via cross-extension `registerTool` isolation | **Re-harden both: execute `{isError:true}`, tool_call `{block:true,context,fallback}` + sweep `pi.tools`/`_tools`/`getAllTools`; strict `currentTurnMarkdown` only, thin `count<2||len<50` warns only `BIGGZ_ADVISE=1`, `PI_SUBAGENT_CHILD=1` bypass.** |
| Restore `INVALID and will be blocked`+12× REMINDER vs keep retired wording | Retired wording lost contract → 6/8 syntheses skipped | **Un-retire `biggz-orchestrator*.md`, remove `ENFORCEMENT RETIRED`, add label re-read self-check. Keep 4 markers+table+`◆`.** |
| Transcript lint (8 phases→7 blocks) vs unit only | Post-hoc but catches drift deterministically | **Add CI lint scanning `IsCheckpointAsk` without preceding same-turn `HasSynthesis`; 0 blocks when complete.** |

## Data Flow

```
sub-agent done → orchestrator emits synthesis (4 markers+table)
              → message_end/message_update → recordText → currentTurnMarkdown + now
              → checkpoint call (ask_user_choice/ask_user_question/question)
              → IsCheckpointAsk(label-only bilingual) + HasSynthesis(currentTurnMarkdown) + 120s
              → ShouldBlock=!child&&!recall&&isCheckpoint&&(!hasSyn||expired)
                  true → BuildBlockedEnvelope/blockedEnvelope → {isError:true}/{block:true}+context+FormatFallback (no orig call)
                  false→ allow → reset buffer on tool_execution_end/turn_start
```

Bypasses: `PI_SUBAGENT_CHILD=1` allow (ignored by `ShouldBlockApplyAdmission`), same-turn `## Session Recall` allow, non-checkpoint never blocks.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/sdd/synthesis_gate.go` | Modify | Restore `currentTurnMarkdown`/`currentTurnTime`, `SetCurrentTurnMarkdown`, `ShouldBlock`, `CheckSynthesisPrecondition`, `BuildBlockedEnvelope`/`BlockedFallbackEnvelope`, `ShouldBlockApplyAdmission`; keep label-only `IsCheckpointAsk`, `HasOptions` advise-only |
| `internal/sdd/synthesis.go` | Modify | Keep `sanitizePlain` English marker whitelist, `RenderSynthesis` invariant |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Remove passthrough, restore `wrapSingleTool`+`tool_call` blocks, `blockedEnvelope` 120s, thin warn, `turn_start`/`message_end` buffer+sweep |
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | Remove retired header, restore `INVALID and will be blocked`, 12× REMINDER, self-check |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | Un-retire wording, align 120s rule |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modify | Un-retire Ask contract |
| `internal/sdd/synthesis_gate_test.go` | Modify | Add `TestShouldBlock` fixtures (missing/history/expired/body-token/turn-reset/bypass) |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modify | Flip to blocking assertions, sweep+thin tests |
| `tests/transcript-lint` | Create | 8-phase fixture (2 syntheses→7 blocks) |
| `docs/architecture.md` | Modify | Restore 3-layer defense |

## Interfaces / Contracts

```go
var currentTurnMarkdown string; var currentTurnTime time.Time
func SetCurrentTurnMarkdown(md string)
func HasSynthesis(md string) bool // 4 markers + prose|table
func HasSessionRecall(md string) bool
func IsChildBypass() bool // PI_SUBAGENT_CHILD==1
func HasOptions(q string) bool // "\"options\"" only, never gates
func IsCheckpointAsk(q string) bool // label/value/id/name/title bilingual, body ignored
func ShouldBlock(q,md string, now time.Time) bool
func CheckSynthesisPrecondition(q,md string) (bool,string)
type BlockedFallbackEnvelope struct{Block bool; Reason,Context,Fallback string}
func BuildBlockedEnvelope(q,md string, now time.Time, env QuestionEnvelope) BlockedFallbackEnvelope
func ShouldBlockApplyAdmission(q,md string, now time.Time) bool // ignores child/recall
func FormatFallback(env QuestionEnvelope) string
```

```js
// biggz-synthesis-gate.js mirrors Go
recordText via message_end/message_update → currentTurnMarkdown/currentTurnUpdateTime
hasSynthesis, isCheckpointAsk(params), hasOptions(params)
checkSynthesisPrecondition(ctx) // strict ≤120s
blockedEnvelope(params,reason) // {block:true,context,fallback}
wrapSingleTool(def) // {isError:true}
pi.on("tool_call") // {block:true} sweep safe
pi.on("turn_start") // clear
```

Markers stay English verbatim (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`, `◆`, `- [ ]`).

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit Go | 4 markers+table, label-only vs body, HasOptions never blocks, 120s/turn-reset/child/recall/thin | `go test ./internal/sdd -run TestShouldBlock` |
| Unit JS | helpers, execute/tool_call blocking, sweep load-order, thin warn BIGGZ_ADVISE=1 | `node --test biggz-synthesis-gate.test.mjs` |
| Integration | same-turn allow vs history/expired/body-token block, fallback verbatim | parity fixtures Go+JS same verdict |
| Lint E2E | 8 phases 6 missing →7 blocks; complete →0 | transcript lint + `go vet` `node --check` |

## Threat Matrix

Generic routing/shell matrix: N/A — no git/push/PR/executable boundary.

Synthesis-gate:

| Threat | Response | Planned RED Tests |
|--------|----------|-------------------|
| Streaming race (markdown ms before tool_call) | `message_end` buffer + `currentTurnUpdateTime` 120s; never `ctx.history` for block | immediate after markdown→allow; missing→block |
| History false pass | strict `currentTurnMarkdown` only; reset on `turn_start`/`tool_execution_end` | history-only→block; post-reset→block |
| Body-token false positive (`para continuar`) | label-only scan `label/value/id/name/title` | body-only→allow; label→block |
| Translated markers | English whitelist via `sanitizePlain` | Spanish content+English markers→pass; translated→fail |
| Thin spam (`count<2||len<50`) | warn only `BIGGZ_ADVISE=1` via `pi.notify` | thin+advise→warn allow; no advise→silent |
| Preflight bypass widening | narrow same-turn `## Session Recall` only; `ShouldBlockApplyAdmission` ignores child/recall | recall outside current→still block; child admission still blocks |

## Migration / Rollout

No migration. Flags: default blocking, `BIGGZ_ADVISE=1` thin, `PI_SUBAGENT_CHILD=1` bypass. Rollback `git revert cf840430^..944e284e`; verify `go test` `node --test` `go vet` `node --check` green.

## Open Questions

- [ ] `message_end` shape stable on Pi 0.84.x? Fallback `assistant_message` kept.
- [ ] OpenCode `question` parity via harness intercept follow-up for `ShouldBlockApplyAdmission`?
