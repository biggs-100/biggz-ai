# Proposal: Complete Subagent Report

## Intent
4-line synthesis drops preview/diff/decisions/commands/validation; typed failures show raw JSON. `ask_user_question`/`question` needed for preflight (4 groups) but lacks UI-limit checks (header 16/label 60/questions≤4/options 2–4), loses preview in fallback, no compaction persistence. Fix reporting + question reliability without breaking `hasSynthesis` or 400-line budget.

## Scope

### In Scope
- Rich template (Preview/Diff/Decisions/Commands/Validation), 4 markers kept
- Gate warning-by-default; block only if markers missing; read loop >50KB
- Failure synthesis (human summary)
- `validateQuestionEnvelope` pre-flight
- Single ownership: orchestrator only
- Persistence `biggz-ai.pending-question/v1` + fallback preview
- Close G1–G14 (P0 template/gate/ownership/persistence; P1 validation/loop/failures)
- 3 PRs stacked-to-main, <400 lines each

### Out of Scope
- Rename `engram`→`bigmem` (keep alias)
- New phases / budget / review-gate redesign
- TUI restyle beyond synthesis

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `orchestrator`: synthesis completeness, failure synthesis, question ownership, pending lifecycle
- `pi-integration`: gate warning/block, envelope validation, source resolution `currentTurnMarkdown→ctx.history→lastAssistant(120s)`

## Approach
Extend checkpoint with 6 optional sections, keep 4 markers green. Gate warns (`concern: synthesis is thin`) when `Artifacts/Paths`<2 or <50 chars; blocks (`isError:true`) only if markers missing. Add read-loop for >50KB. Centralize asks in orchestrator; validate envelope first. Persist `biggz-ai.pending-question/v1` (hybrid dual-write, readback equal). Keep `engram` alias. Ship stacked.

**PRs:** PR1 template+gate+loop+alias. PR2 failure synthesis+validation+ownership. PR3 persistence+fallback+E2E.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | Rich template, `REMINDER`/`INVALID` rule |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | 4-marker gate, warning-default, 120s chain |
| `internal/sdd/*`, `internal/orchestrator/*` | Modified | Validation, synthesis, persistence |
| `openspec/specs/orchestrator/spec.md` | Modified | Completeness + ownership |
| `openspec/specs/pi-integration/spec.md` | Modified | Envelope limits + gate |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| False block breaks preflight | Low | Warn-default; block only missing markers |
| Template bloat | Low | Optional sections, bullets/tables |
| Stacked diff overlap | Med | Rebase/retarget per slice |
| Compaction loss | Low | Dual-write + readback compare |

## Rollback Plan
Revert PR3→PR1. Delete `sdd/*/pending-question` + `state.yaml` entry. Gate reverts via single commit. Alias needs no rename reversal. No migration.

## Dependencies
- SDD Status v2 + `biggz sdd-status --json --instructions`
- BigMem SQLite + `capture_prompt:false`
- `ask_user_question`/`question` + `node --test` harness

## Success Criteria
- [ ] Synthesis always has 4 markers; rich sections when artifacts>0
- [ ] Gate warns on thin, blocks only on missing markers (`isError:true`)
- [ ] `validateQuestionEnvelope` rejects header>16/label>60/questions>4/options∉[2,4]
- [ ] Failures render as human synthesis, not raw JSON
- [ ] Pending survives compaction (readback equal)
- [ ] 3 PRs <400 lines, CI green (`go vet`, `go test ./...`, `node --test biggz-synthesis-gate.test.mjs`)
