# Design: Complete Subagent Report

## Technical Approach

Close G1-G14 keeping 4-marker invariant (`## Sub-agent Result`, `**What was done:**`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`). Add 6 optional rich sections (Preview/Diff/Decisions/Commands/Validation/Failure) — bullets/tables, omitted when empty. Harden Pi gate: block only if markers missing via strict `currentTurnMarkdown`; history fallback only for non-blocking `concern: synthesis is thin` (count<2‖len<50, behind `BIGGZ_ADVISE=1`). Orchestrator owns all checkpoint asks; validate envelopes pre-dispatch with plain fallback. Persist pending via `biggz-ai.pending-question/v1` dual-write (BigMem+`state.yaml`, readback equality, compaction reload). Add >50KB read-loop and humanize failures. 3 PRs stacked-to-main (<400 LOC each), `engram==bigmem`, `hasSynthesis` compat.

Covers delta specs `orchestrator/spec.md` (rich template, read-loop, ownership/persistence) and `pi-integration/spec.md` (gate strictness, `validateQuestionEnvelope`, advise).

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|---|---|---|---|
| Template | A mandatory 10 fields B 4+6 optional omit-empty C separate artifact | A bloats; C splits audit; B keeps gate green + value when artifacts>0 | B: 4 required + optional Preview/Diff/Decisions/Commands/Validation/Failure |
| Gate block vs advise | A block on thin B warn-default (block only missing) | A false-blocks preflight; B preserves hard gate, advise opt-in | B: `currentTurnMarkdown` strict `isError:true` only if marker missing; thin→allow+`concern` |
| Source resolution | A `currentTurn→history→lastAssistant` for block B strict `currentTurn` for block, history only for advise | A leaks stale synthesis; B fixes streaming race + false positives | B: `checkSynthesisPrecondition` = `currentTurn` only; `getCurrentTurnSynthesis` fallback for advise |
| Truncation >50KB | A single read B loop offset/limit + verify | A leaves preview truncated; B guarantees length | B: loop `read(offset,limit)` until `len>=expected`, retry once |
| Envelope validation | A dispatch-then-catch B pre-dispatch validate→fallback | A shows native truncation; B deterministic `isError`+plain | B: `header≤16,label≤60,qs≤4,opts 2-4`; reject→`isError:true`+fallback |
| Pending persistence | A BigMem only B FS only C dual-write+readback | A/B lost on compaction/boot; C survives both | C: `biggz-ai.pending-question/v1` BigMem+`state.yaml`, equality retry, compaction reload→fallback markdown |

## Data Flow

```
sub-agent → orchestrator.RenderSynthesis → gate → ValidateEnvelope → pending.SavePendingDualWrite
   │ exec_summary,artifacts,risks,next      │hasSynthesis strict  │isCheckpointAsk │ BigMem+state.yaml
   │ failure JSON→human Failure             │currentTurn only     │header/label/qs  │ readback equality
   │ >50KB→ReadLoop(offset/limit)          │thin+BIGGZ_ADVISE→concern             │ compaction→fallback
                                           
General ask? ──→ bypass (no block). Checkpoint ask + missing marker? ──→ block isError + notify, no handler.
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | 4-marker block + `INVALID` + `REMINDER≥12`; add 6 optional placeholders, failure humanization, read-loop, `engram` alias |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Strict `hasSynthesis` 4-marker, `checkSynthesisPrecondition=currentTurn` only, `getCurrentTurnSynthesis` history fallback for advise, `isCheckpointAsk`, `isThinSynthesis`/`emitConcern`, `PI_SUBAGENT_CHILD=1` bypass |
| `internal/assets/biggz/orchestrator.test.go` | Modify | Assert markers+`Preview`+`INVALID`+`REMINDER≥12`+alias drift guard |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Modify | 4 gates (missing→isError, rich→pass, thin+advise→warn, thin silent) + loop/envelope/pending/alias |
| `internal/sdd/synthesis.go` | Create | `RenderSynthesis` (4+6 omit-empty, failure→human), `ReadLoop` (paginated + verify) |
| `internal/sdd/question.go` | Create | `ValidateQuestionEnvelope` (16/60/4/2-4), `FormatFallback` plain |
| `internal/sdd/pending.go` | Create | `biggz-ai.pending-question/v1`; `SavePendingDualWrite`, `VerifyEquality`, `LoadOnCompaction` |
| `openspec/specs/orchestrator/spec.md` | Modified | Deltas already present |
| `openspec/specs/pi-integration/spec.md` | Modified | Deltas already present |

## Interfaces / Contracts

```go
// internal/sdd/question.go — limits per spec
func ValidateQuestionEnvelope(q QuestionEnvelope) error // header≤16,label≤60,qs≤4,opts 2..4 → isError:true + limit name
func FormatFallback(q QuestionEnvelope) string

// internal/sdd/synthesis.go
func RenderSynthesis(r SubAgentResult) string // 4 required + 6 optional (omit empty), hasSynthesis compat
func ReadLoop(path string, cap int) (string, error)

// internal/sdd/pending.go — schema biggz-ai.pending-question/v1
type PendingQuestion struct{ Schema string; Envelope QuestionEnvelope; SynthesisMD string }
func SavePendingDualWrite(ch string, pq PendingQuestion) error // BigMem+state.yaml, equality retry
func LoadOnCompaction(ch string) (PendingQuestion, error)      // BigMem primary, FS fallback

// JS — biggz-synthesis-gate.js
pi._biggzSynthesisGate = { hasSynthesis, isCheckpointAsk, getArtifactsMetrics, isThinSynthesis, checkSynthesisPrecondition, getCurrentTurnSynthesis }
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit Go | Validate rejects 16/60/4/2-4, Render 4+6 omit-empty + humanized Failure, ReadLoop >50KB | `go test ./internal/sdd -run TestValidate\|TestSynthesis` |
| Unit JS | Gate: missing→isError no-handler, rich→pass, thin+advise→concern, general bypass | `node --test biggz-synthesis-gate.test.mjs` mockPi |
| Integration | Template invariant + alias, pending dual-write equality + compaction fallback | `go test ./internal/assets/biggz` + temp BigMem store |
| E2E | Thin/rich/failure+truncated→fallback re-emit | Pi harness; CI `go vet && go test && node --test` |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Gate is in-process JS interception of `ask_user_question`/`question` and BigMem/state.yaml persistence; no `git -C`, commit/push, or PR argument composition added. Stacked PRs reuse `branch-pr`/`chained-pr` with explicit `--head`.

## Migration / Rollout

No migration. `BIGGZ_ADVISE=1` off by default. `engram` remains alias. Rollback: revert PR3→PR1, delete `sdd/*/pending-question` + `state.yaml` entry. 3 PRs stacked-to-main, each <400 LOC, CI green.

## Open Questions

- [ ] GC `state.yaml` pending on answer vs keep until archive? Propose delete on answer.
