# Proposal: sdd-parity-rescope-grant-ledger

## Intent
Close 7 gaps vs `gentle-ai` (2026-08-31). Fix ledger dup, permissive rescope, missing topology, weak passive proof; add grant sugar + read-only marker.

## Scope
### In Scope
- G1 `internal/sdd/attempt.go:12` guard fail-closed (15)
- G2 rescope narrowing `Widened`/`Exhausted` + guards (`sddattempt.go:1992`, 55)
- G3 `ForInstance` `cas_store.go:86` (25)
- G4 topology `cross_common_dir_runtime_target` (`edit_authority.go`+`status.go:473`, 85)
- G5 passive 8 MiB/shebang/MDX/exec (`risk.go:165`, 55)
- G7 `readOnlyMarkerAfterToken` (20)

### Out of Scope
- `RuntimeHandoff` (single-worktree)
- `FinalVerifyAttestation` (review-free)
- Full `treeBlobSizes`/`git grep` (150+ LOC)
- Hybrid `research.go:39` — already compliant

## Capabilities
### New Capabilities
- None

### Modified Capabilities
- `sdd`: ledger guard + narrowing rescope + `ForInstance`
- `sdd-status`: read-only + topology `resolve-blockers`
- `review`: bounded passive proof

## Approach
| G | Options | Decision |
|---|---------|----------|
|1|delete vs guard|Guard `ErrLegacyRetired` + pointer `biggz sdd-attempt`|
|2|wedge vs verbatim|Verbatim: `Active==0&&!DecisionRequired&&!Complete&&terminal&&!drifted` → `new<=old→Widened`, `new<=cum→Exhausted`; carry `Cumulative*`|
|3|explicit vs sugar|`ForInstance(instance) (Store,error)` 1..128 trimmed, scopes `grantedRootsFor`; keep explicit|
|4|off vs verbatim|`resolveExistingPath→gitRootOf→OpenRuntimeStore→SameFile` per `editTargetTokens`; block `apply/verify/remediate`; memoize; `context.Context`|
|5|full vs adapted|Adapted: `ClassifyRisk` unchanged; `isPassiveContentFile` ≤8 MiB, NUL/utf8, `#!`, MDX, `exec`; over-budget→not passive|
|7|—|`(?i)^\s*\(read-only\)` per-token|

## Alternatives & Won't Port
- **Handoff**: needs `commonDir`+`CandidateTree` multi-worktree CAS; single clone `record-*.json` has no caller → won't port.
- **Attestation**: needs review authority + digest; budget-bounded → won't port.
- Rejected: hard delete, wedge-only, full scan.

## Affected Areas
| Area | Impact |
|------|--------|
| `internal/sdd/attempt.go` | Guard 15 |
| `internal/sddattempt/sddattempt.go:1992` | Rescope 55 |
| `internal/sddattempt/cas_store.go:86` | Grant 25 |
| `internal/sdd/edit_authority.go` | Marker 20 + topology 85 |
| `internal/sdd/status.go:473` | Wiring |
| `internal/review/risk.go:165` | Passive 55 |

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Drift stub `false` | Med | TODO + refuse empty `candidateTree` |
| Git latency | Low | Memoize per status |
| Passive regression | Low | Gate `.md/.mdx/.rst/.adoc/.png/.jpg/.gif`; fixture `with-shebang.md` |
| Dual API drift | Low | Shared validator + equiv test |

## Rollback Plan
`git revert` single PR. CAS additive, no migration. Guard revert restores `AttemptBegin`.

## Dependencies
`git rev-parse --git-common-dir`, `os.SameFile`; CAS `HEAD/LOCK`; `go test ./...`.

## Success Criteria
- [ ] Guard `ErrLegacyRetired` with pointer
- [ ] Rescope rejects `Widened`/`Exhausted`, preserves `Cumulative*`
- [ ] `ForInstance` equiv + invalid reject
- [ ] Topology blocks foreign `commonDir`, read-only filtered
- [ ] Passive shebang/MDX/exec escalate, >8 MiB fail-closed
- [ ] Single PR <400 tracked (<800 with tests), tests pass

## Assumptions
Single-worktree→Handoff out; review-free→Attestation out; stub→`candidateTree` false ok; adapted 8 MiB sufficient; `auto-chain stacked-to-main`.

## Work Units
255 tracked + ~180 test = passes 800.
