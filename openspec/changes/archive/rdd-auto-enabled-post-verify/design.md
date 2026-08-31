# Design: rdd-auto-enabled-post-verify

## Technical Approach

Hybrid on Approach 3: keep RDD default ON (`internal/review/rdd.go:280-322` + `internal/install/install.go:505-560` `ensureRDDEnabled`), wire conditional `ReviewOffer` (`internal/sdd/status.go:523`, `internal/sdd/engram_status.go:246,342`) gated `applyState==all_done && verify PASS && RDD enabled` with `pathquote.Quote`, fix hook lineage (`pre-push:8-28`) via `ls -t`+`merge-base --is-ancestor`, guard `archive.go:12-40` never-disable, orchestrator auto-runs only `gate allowed:false && auto-chain`. ~260 LOC single PR `stacked-to-main`. Trace: SDD 5 req → D2/D3/D4, RDD 4 req → D1/D4.

## Architecture Decisions

### D1 — Default ON (rdd/spec.md Default ON, Install Defense)

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Keep `rdd.go` enabled default + `ensureRDDEnabled` clears stale `gen-*.json`, warns on explicit `disabled` | Low risk, correct | **Select** |
| Flip OFF like gentle `rdd_mode.go:681` | Breaks spec | Reject |
| Installer-only | Clone without install stays disabled | Reject |

### D2 — Conditional ReviewOffer (sdd/spec.md ReviewOffer)

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Wire `deriveChangeStatus`/`deriveBigMemChangeStatus` iff `all_done && verify done && Passing && enabled` → `ReviewOffer{Available:true, Invocation:"biggz review start --lineage "+pathquote.Quote(change+"-"+shortSHA)}` else `nil`; `status_v2.go:48-53` allowlist | No leak, byte-preserving | **Select** |
| Always emit | Pollutes planning | Reject |

`Passing=pass && 0 blockers && 8/8` (`verify.go:90`). Filesystem wins on hybrid merge.

### D3 — Hook lineage-aware (sdd/spec.md Hook lineage + space-tolerant)

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `ls -t $git_common/biggz/review-transactions/*` newest-first, filter `git merge-base --is-ancestor HEAD`; ghost `019fbb3a-*` only if ancestor; fallback newest; grep `[[:space:]]*` | Fixes stale replay | **Select** |
| Naive `for d; break` | Picks ghost alphabetically | Reject |

Template in install assets; `.git/hooks/pre-push` is generated copy.

### D4 — Archive guard + orchestrator + ghost (sdd/spec.md Archive/Orchestrator, rdd/spec.md Ghost)

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `archive.go` guard comment + test `grep RDDDisable==0` + mtime==T0; orchestrator auto-runs iff `allowed:false && auto-chain && offer` else surface offer; ghost `rm -rf 019fbb3a-*` manual after `grep -l Temp/biggz-smoke` | Respects `ask-on-risk`, never-disable | **Select** |
| Archive clears RDD / auto-run every PASS / code auto-delete | Violates spec / loops / risks valid lineage | Reject |

## Data Flow

```
verify-report+SpecCounts → parseVerifyResult → Passing
RDDStatus → effective Mode ────────────────────┐
                                              ▼
sdd-status → deriveChangeStatus → if all_done&&PASS&&enabled → ReviewOffer else nil
        → V2Projection (allowlist) → biggz sdd-status --json
        → sdd-verify/orchestrator → gate pre-push --json → allowed:false?
              ├── auto-chain → exec ReviewOffer.Invocation
              └── ask-on-risk → print offer
hook: ls -t → merge-base --is-ancestor HEAD? → select ancestor else skip ghost → none? fallback newest
```

## File Changes

| File | Action | Desc | LOC |
|------|--------|------|-----|
| `internal/sdd/status.go` | Modify | Wire ReviewOffer at `:523`, `pathquote.Quote`, shortSHA | 70 |
| `internal/sdd/engram_status.go` | Modify | Mirror at `:246,342` | 40 |
| `internal/sdd/archive.go` | Modify | Guard comment, `os.Rename` only | 30 |
| `pre-push` + `internal/install/assets/hooks/pre-push.tmpl` | Modify | `ls -t`+ancestor, space grep | 40 |
| `internal/install/install.go:505-560` | Modify | Doc idempotent `ensureRDDEnabled`+warning | 10 |
| `internal/sdd/*_test.go` | Add | PASS→offer, fail→nil, quoting, mtime, no-RDDDisable | 100 |

## Interfaces / Contracts

```go
type ReviewOfferBlock struct { Available bool `json:"available"`; Invocation string `json:"invocation"` } // status_v2.go:48 allowlist
type RDDStatusReport struct { EffectiveMode RDDMode; Source string } // rdd.go:280
type GateResult struct { Allowed bool; Delivery string } // gate.go
// hook: lineage=$(ls -t "$git_common/biggz/review-transactions"/* | head -n20)
// for c in $lineage; do git merge-base --is-ancestor "$(basename $c|cut -d- -f2)" HEAD && select; done
// grep: '"delivery"[[:space:]]*:[[:space:]]*"disabled"' , '"allowed"[[:space:]]*:[[:space:]]*false"'
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | RDD default, ReviewOffer predicate+quoting, archive mtime/no-disable | `rdd_test.go`, `status_test.go` table, `archive_test.go` |
| Integration | `sdd-status --json \| jq .reviewOffer` PASS | `go test ./internal/sdd -run ReviewOffer` |
| E2E | Hook ghost/fallback/space grep | `sh pre-push` temp `.git/biggz` |

## Threat Matrix

Shell/push-gate change → custom + reference applicability.

| Boundary | Cases | Applic. | Response | RED test |
|----------|-------|---------|----------|----------|
| Push state | tracking, first push, refspec | Applicable | Block iff `allowed:false` && not `delivery disabled`; space grep | Push variants same gate |
| Stale replay | Ghost `019fbb3a-*` not ancestor | Applicable | `ls -t`+ancestor filter, fallback newest | Ghost+real→real; all non-ancestor→newest |
| Surprise enable | Global `disabled` before install | Applicable | Warn before re-enable | Global disabled→warning |
| Quoting | `my change` injection | Applicable | `pathquote.Quote` | Space/quote→quoted invocation |
| Ghost rm | Wildcard deletes valid | Applicable | Limit `019fbb3a-*`, `grep -l Temp` before `rm` | `grep -R "rm.*019fbb3a" include *.go`==0 |

Ref: Documentation-like, Git repo selection, Commit state, PR commands = N/A (no exec/classification/routing).

## Migration / Rollout

No migration. Single PR `git revert <sha>`. Fresh via `rdd.go`; heal on `biggz install`. Ghost opt-in.

## Open Questions

- [ ] None. `shortSHA`=`git rev-parse --short HEAD`; `evidence_revision` validation.
