# Proposal: rdd-auto-enabled-post-verify

## Intent
Enforce RDD ON + auto-review on block, never auto-disable. Fix `ReviewOffer=nil` (`status.go:523`, `engram_status.go:246,342`) and naive hook (`pre-push:8-28`) ghost `019fbb3a-*`.

## Scope
### In Scope
- Wire `ReviewOffer` `status.go`+`engram_status.go`: iff `applyState==all_done && verifyReport==done && passing && RDD enabled`; `biggz review start --lineage <change>-<shortsha>`
- Hook: `ls -t` + `git merge-base --is-ancestor HEAD`, fallback newest; keep space-fix grep
- Guard `archive.go:12-40`: comment + test no `RDDDisable`
- Orchestrator auto-run iff `gate allowed:false` && `auto-chain`, else offer only
- Doc `rm -rf .git/biggz/review-transactions/019fbb3a-*` after `Temp` check

### Out of Scope
- Flip default OFF (`rdd.go:280-322` stays ON)
- Auto-run every verify PASS
- Auto-delete ghosts in code
- Receipt-only like gentle (`gate.go:642-680`)

## Capabilities
### New Capabilities
- None
### Modified Capabilities
- `sdd-status`: emit `ReviewOffer` via V2 allowlist (`status_v2.go:48-53`)
- `rdd-gate`: lineage-aware pre-push
- `sdd-archive`: never-disable invariant

## Approach
| # | Approach | Verdict |
|---|----------|---------|
| 1 | ON + offer-only + `ls -t` + manual rm | Reject: no auto-run |
| 2 | Installer-only (`install.go:410-560`) eager | Reject: breaks w/o install |
| 3 | **Hybrid**: core ON (`rdd.go:280-322`) + `ensureRDDEnabled` + conditional offer + hook + guard | **Select** 260 LOC |

## Alternatives & Won't Port
- Gentle `rdd_mode.go:681-712` OFF not ported
- Gentle `review_offer.go` → add conditional auto-run on `allowed:false`
- Gentle receipt-only not ported

## Affected Areas
| Area | Impact | Desc |
|------|--------|------|
| `status.go:523` | Mod | Wire ReviewOffer, `pathquote.Quote` |
| `engram_status.go:246,342` | Mod | Mirror |
| `status_v2.go:48-53` | Keep | Allowlist |
| `pre-push:8-28` | Mod | Lineage-aware |
| `archive.go:12-40` | Mod | Guard + test |
| `install.go:410-560` | Keep | Defense |

## Risks
| Risk | Like | Mitigation |
|------|------|------------|
| Stale replay | Med | `ls -t` + `merge-base --is-ancestor` |
| Surprise enable | Low | Warn; `rdd.go:360-373` |
| Quoting | Low | `pathquote.Quote` |
| `rm` valid | Low | Limit `019fbb3a-*`, check `Temp` |

## Rollback Plan
`git revert <sha>` single PR restores stub+hook, removes guard.

## Dependencies
`RDDStatus` (`rdd.go`), `gate allowed:false` (`gate.go`), `auto-chain` 800

## Success Criteria
- [ ] `sdd-status --json` emits `reviewOffer` post-verify PASS when enabled
- [ ] Hook picks ancestor lineage, not ghost
- [ ] `archive.go` never `RDDDisable`
- [ ] Push blocks when unmanaged; auto-run unblocks (auto-chain)

## Assumptions
- Single-worktree (`git rev-parse --git-common-dir` for others)
- `auto-chain` per preflight

## Work Units
| Unit | LOC |
|------|-----|
| ReviewOffer | 70 |
| Hook | 40 |
| Archive guard | 30 |
| Orchestrator doc | 20 |
| Ghost doc | 10 |
| **Total 260 <400 single PR** |  |
