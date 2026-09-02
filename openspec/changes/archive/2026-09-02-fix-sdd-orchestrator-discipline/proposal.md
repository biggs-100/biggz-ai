# Proposal: fix-sdd-orchestrator-discipline

## Intent

Orchestrator permits fast-forward inline and auto-continue without `proceed/adjust/stop`, bypasses SD Agent Authority (`sdd-*` vs `general`), skips reads of `biggz-orchestrator-workflow.md`/`delegation.md`, and misses RDD `biggz review + receipt` gate before `verify`.

## Scope

### In Scope
- Blocking synthesis checkpoint: 4 markers + `IsCheckpointAsk`/`HasSynthesis`/120s window before `proceed/adjust/stop` or `continue/correct`
- SD Agent Authority: SDD phases only via `sdd-*` agents; `general`/`explore` blocked
- Orchestrator must read `biggz-orchestrator-workflow.md` and `delegation.md` before delegating
- RDD gate (`biggz review` + receipt) before `verify`
- Fix fast-forward inline and auto-continue to require explicit token

### Out of Scope
- New SDD phases or artifact types
- Changes to review lens logic beyond gating
- BigMem store migration

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `orchestrator`: harden delegation discipline, synthesis gate, SD Agent Authority, pre-delegation reads, and RDD gate before verify

## Approach

Harden `biggz-orchestrator.md` + workflow/delegation docs fail-closed. Update `internal/sdd/synthesis_gate.go` (bilingual tokens, 120s). Add SD Authority guard rejecting `general` for SDD phases. Inject mandatory pre-delegation reads. Add RDD gate in verify preflight (`biggz review --receipt`). Fix Work Routing Ladder to disallow size-only SDD.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/biggz/biggz-orchestrator*.md` | Modified | Orchestrator workflow/delegation docs: gates + authority |
| `internal/sdd/synthesis_gate.go` | Modified | Gate enforcement + marker validation |
| `internal/sdd/verify*.go` | Modified | RDD gate before verify |
| `internal/orchestrator/*` | Modified | SD Agent Authority enforcement |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Gate blocks legitimate auto flow | Med | Keep auto exception only when `auto` explicitly chosen; still surface failures |
| Over-strict agent authority breaks delegation | Low | Fallback: log and block, suggest correct `sdd-*` agent |
| Review gate flakiness | Low | Pre-push disabled path reports `unmanaged` not fabricated PASS |

## Rollback Plan

Revert orchestrator docs and `synthesis_gate.go` to prior commit; disable RDD pre-verify check via flag; no data migration needed.

## Dependencies

- `biggz review` + receipt validation already exists
- `biggz sdd-status` v2 contract

## Success Criteria

- [ ] Every delegated sub-agent emits `## Sub-agent Result` with 4 markers before checkpoint ask; gate blocks otherwise
- [ ] SDD phases via `general` are rejected with SD Agent Authority error
- [ ] Orchestrator reads workflow + delegation docs before routing (verified via launch prompt)
- [ ] `verify` blocked without valid RDD review receipt when RDD enabled
- [ ] No fast-forward inline or auto-continue without `proceed/adjust/stop`
