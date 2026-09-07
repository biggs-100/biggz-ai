# Tasks: Fix Orchestrator Synthesis Gate

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 620-750 (additions+deletions) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 Go canonical gate -> PR2 JS dual gate + docs -> PR3 lint + parity |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Go gate 120s + envelopes | PR1 -> main | `go test ./internal/sdd -run TestShouldBlock` | 30s allow / 121s block / history block | `internal/sdd/synthesis_gate.go` revert alone |
| 2 | JS dual gate + docs | PR2 -> main | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` | `message_end`->`tool_call` allow; missing->block+bypass | `internal/assets/pi/biggz-synthesis-gate.js` + `biggz-orchestrator*.md` revert |
| 3 | Lint + parity + CI | PR3 -> main | `go test ./... && node --test` | 8 phases 2 syn->7 blocks; `go vet` `node --check` | `tests/transcript-lint` removable |

## Phase 1: Foundation - Go Canonical Gate

- [x] 1.1 RED: Add failing `TestShouldBlock` in `internal/sdd/synthesis_gate_test.go` - missing/history/121s->block, `turn_start`->block, body-token->allow, thin->allow, child/recall->allow
- [x] 1.2 Restore `internal/sdd/synthesis_gate.go` globals `currentTurnMarkdown`/`currentTurnTime` + `SetCurrentTurnMarkdown` + `HasSynthesis` 4 markers+table
- [x] 1.3 Restore `ShouldBlock` 120s + `CheckSynthesisPrecondition` + `BuildBlockedEnvelope`/`FormatFallback` context+fallback
- [x] 1.4 Restore `ShouldBlockApplyAdmission` (ignores child/recall) + label-only `IsCheckpointAsk` + `HasSessionRecall`; keep `synthesis.go` `sanitizePlain` whitelist

## Phase 2: Core - JS Dual Gate + Docs

- [x] 2.1 RED: Flip `internal/assets/pi/biggz-synthesis-gate.test.mjs` - `execute`->`{isError:true}` no orig, `tool_call`->`{block:true}` on missing/history/121s
- [x] 2.2 Remove passthrough in `internal/assets/pi/biggz-synthesis-gate.js`; restore `wrapSingleTool` strict `currentTurnMarkdown`+120s check sweep `pi.tools`/`_tools`/`getAllTools`
- [x] 2.3 Restore `pi.on("tool_call")` second guard + `message_end`/`message_update` buffer + `turn_start` resets + `PI_SUBAGENT_CHILD=1` + `## Session Recall` narrow + thin `BIGGZ_ADVISE=1`->`pi.notify`
- [x] 2.4 Un-retire `biggz-orchestrator.md`/`workflow.md`/`delegation.md`: delete `ENFORCEMENT RETIRED`, restore `INVALID and will be blocked` + 12x `REMINDER` + self-check before `question`

## Phase 3: Integration - Lint + Docs

- [x] 3.1 Restore `docs/architecture.md` 3-layer defense un-retired
- [x] 3.2 Create `tests/transcript-lint` with 8-phase fixture: 2 syntheses->7 blocks; complete->0 blocks
- [x] 3.3 RED: streaming race/thin/translated-marker/recall-outside-current/child-admission threat cases

## Phase 4: Verification - Parity + CI

- [x] 4.1 Run focused: `go test ./internal/sdd -run TestShouldBlock` + `node --test biggz-synthesis-gate.test.mjs` verifies REQ-DG-1/2, REQ-ORCH-001, REQ-SDD
- [x] 4.2 Run full CI: `go test ./... && go vet ./... && node --check` - docs contain markers+`INVALID` no `ENFORCEMENT RETIRED`
- [x] 4.3 Parity: Go/JS same verdicts; `BuildBlockedEnvelope` preserves question via `FormatFallback`

