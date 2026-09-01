# Proposal: fix-bigmem-recall-recency

## Intent
Stale "en que nos quedamos?" — `search --query "session"` uses `ORDER BY rank` (`bigmem.go:1844` BM25) not recency `search --query ""` (`ORDER BY updated_at DESC` @1801). Returned 2026-08-27 obs-178780... not 2026-09-01 obs-178829.... Make recency the only valid latest-context path; immune despite 621c4e5,27d2ad7,c9d831a.

## Scope

### In Scope
- `biggz recall` / `biggz bigmem recent` → latest session_summary + git log + sdd-status via `updated_at DESC`
- Harden Session Boot Recall gate (`biggz-orchestrator-workflow.md`): helper + `biggz_mem_context(5)` before FTS
- Guardrail (`bigmem-protocol.md` + `APPEND_SYSTEM.md` via `install.go`): "for recency use empty query ordered by updated_at, not FTS term search"
- CLI/docs (`cli_bigmem.go`, `docs/architecture.md`): rank vs recency

### Out of Scope
- Change FTS rank; vector ranking; schema/sdd-init

## Capabilities

### New Capabilities
- `recall-recency`: helper+gate via `updated_at DESC` without FTS

### Modified Capabilities
- `bigmem`: ordering docs + help guardrail
- `orchestrator`: gate hardening, ban FTS for latest
- `cli`: wire `recall`/`recent`

## Approach
| Opt | Summary | Pros | Cons |
|-----|---------|------|------|
| A | Keep rank, add `Search("",opts)` helper + guardrails + docs | Zero risk, minimal, trivial revert | Prompt-dependent |
| B | `--order rank|recency` flag | Flexible | Larger surface, needs discipline |
**Recommend A+C** (A + docs/hardening, no B): minimal risk, fixes misuse at UX/docs.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modified | Add `Recent()` wrapper; rank unchanged |
| `cmd/biggz/cli_bigmem.go` | Modified | Dispatch `recent`/`recall` |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | Gate mandates helper |
| `internal/assets/biggz/bigmem-protocol.md` | Modified | WHEN TO SEARCH guardrail |
| `internal/install/install.go` | Modified | Inject guardrail |
| `docs/architecture.md` | Modified | Rank/recency contract |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Ignores guardrail | Med | Gate blocks; test helper+guardrail |
| Duplicates path | Low | Reuse `Search("",…)` |
| Docs drift | Low | Tests 1801 vs 1844 |

## Rollback Plan
Revert helper; `Search()` unchanged. Remove dispatch, revert workflow/protocol, reinstall `APPEND_SYSTEM.md`. No DB migration. `git revert` + `go test`.

## Dependencies
- `sdd-init/biggz-ai` (2026-08-29); `internal/bigmem`, `cli_bigmem.go`; keep 621c4e5,27d2ad7,c9d831a

## Success Criteria
- [ ] `search --query ""` = `updated_at DESC`; `search "session_summary"` = `rank` unless helper
- [ ] `biggz recall`/`recent` returns latest (2026-09-01 > 2026-08-27) + git log + sdd-status, never stale
- [ ] Prompt has "for recency use empty query ordered by updated_at, not FTS term search"
- [ ] Manual "en que nos quedamos?" returns latest; `go test ./internal/bigmem -run TestSearch` + `assets/biggz` pass

## Proposal question round
1) dual alias or single? 2) git log+sdd-status or BigMem only? 3) ban FTS or warn? 4) limit=5? Assumptions: A+C. Correct or second round before spec.
