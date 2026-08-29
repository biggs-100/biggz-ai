# Design: Parity Gentle 69 — Ledger Budget

## Technical Approach

Port gentle `e8cc0fcc→782e8dfe` in 3 stacked-to-main PRs (`<400` lines). Add CAS verify-before-commit in `withStoreLock`, dual-budget single predicate + `2×MaxAttempts` refund, rescope wedge on carried cumulatives, and `declaredArtifactStore` routing. Preserve FIXED: domainHash, GitCommonDir/v1/events, flock, burn, SDD v2.

## Architecture Decisions

| # | Decision | Options | Tradeoff | Choice |
|---|----------|---------|----------|--------|
| 1 | CAS verify point | A) `commit()` `loadRecord(rev)` before `writeLedgerHead` B) wrapper C) caller check | A `+8` lines, keeps `record-<sha>.json`+`withStoreLock`; B duplicates; C leaks to 6 sites | **A** `cas_store.go:375` fail closed on mismatch, HEAD unchanged |
| 2 | Budget owner | A) inline×3 B) `runtimeChangedLineBudgetExceeded(s,delta)` C) method | A violates spec (duplication); B pure func, grep-single inequality; C couples model | **B** `RuntimeAttempt.ChangedLines` + `RuntimeStore/Status.CumulativeChangedLines` |
| 3 | Refund cap | A) count all interrupted B) `DeliveredIncrement`+`Refunded<=MaxAttempts` C) persisted counter | A over-counts `0` lines; B matches gentle 2243/2217; C new state | **B** `2×` total, no new field |
| 4 | Rescope wedge | A) `new<old` B) `new>carried` for both `&&` C) `>=` | A violates carried; C admits `==`; B matches 1087/1425 | **B** `MaxAttempts>cumAtt && MaxLines>cumLines`, preserve slice |
| 5 | Locator | A) `declaredArtifactStore` reads `openspec/config.yaml` B) env C) flag | A SDD v2, testable; B/C surface | **A** `openspec`→fs, `engram`→`bigmem:sdd/…`, `hybrid`→merge filesystem-wins, `none`→empty via `NormalizeArtifactStore` |

## Data Flow

```
withStoreLock → loadRecord(revision) ─mismatch→ CAS refuse
      │match └→ write record-<sha>.json → writeLedgerHead

Acquire/Settle → runtimeChangedLineBudgetExceeded(cum+delta,max) → blocked or admit
      └→ DeliveredIncrement → Refunded>Max? block

declaredArtifactStore → config sdd.artifact_store → {openspec|engram|hybrid|none} → resolveArtifactPaths
Rescope → wedge check → mutate Max* only
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/sddattempt/cas_store.go` | Modify | Verify-before-commit replay in `commit()` |
| `internal/sddattempt/sddattempt.go` | Modify | Budget fields, single predicate, refund helpers, rescope wedge, cap checks |
| `internal/sdd/status.go` | Modify | `declaredArtifactStore`, store-branched `resolveArtifactPaths` |
| `internal/sdd/engram_status.go` | Modify | Hybrid filesystem-wins routing, `bigmemArtifactPaths` |
| `internal/review/capture.go` | Modify | `wrapRuntimeCandidateUnavailable`, `Binary files differ` typed marker (defer if over) |
| `internal/sdd/status_v2.go` | Verify | `NormalizeArtifactStore` alias, no change if ok |

## Interfaces / Contracts

```go
type RuntimeAttempt struct{ ChangedLines int `json:"changed_lines,omitempty"` }
type RuntimeStore struct{ CumulativeChangedLines int `json:"cumulative_changed_lines,omitempty"` }
type RuntimeStatus struct{ CumulativeChangedLines int `json:"cumulative_changed_lines,omitempty"` }
func runtimeChangedLineBudgetExceeded(s *RuntimeStore, d int) bool { return s.CumulativeChangedLines+d > s.MaxLines }
func runtimeAttemptDeliveredIncrement(interrupted bool, c int) int
func runtimeRefundedAttempts(s *RuntimeStore) int
func declaredArtifactStore(ws string) ArtifactStore
func resolveArtifactPaths(root string, store ArtifactStore) ArtifactPaths
func wrapRuntimeCandidateUnavailable(err error) error
type RuntimeRecordRejectedError struct{ Cause error }
```

`commit()` sig unchanged; `record-<sha>.json`+`HEAD` atomic replace (`sha256Hex(canonicalRecordPayload)`) unchanged.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | CAS stale refuses; `cum+delta>max`; wedge `5,700` rejects `7,800` admits; store defaults | `go test ./internal/sddattempt -run TestCAS\|TestBudget\|TestRescope -count=1` |
| Integration | Acquire `300+150` blocked `300+80` ok; `2×` cap; hybrid wins; `Binary files differ` typed | `go test ./internal/sdd ./internal/review -count=1` + `bigmemStoreRootOverride` |
| E2E | Each PR `<400` (PR1 `≤20`), `go vet` + `go test ./... -timeout 180s` green, no FIXED regression | CI |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|----------|---------------|-----------------|-------------------|
| Documentation-like paths | N/A — no doc execution; binary marker only classifies candidate-unavailable | — | — |
| Git repository selection | **Applicable** — `git --git-common-dir` + `isNotGitRepoError`→machine fallback | Clone/worktree/no-git | RED: worktree commondir, fallback vs permission fail |
| Commit state | N/A — ledger is `record-*.json`+`HEAD` | — | — |
| Push state | N/A — no push | — | — |
| PR commands | N/A — no `gh pr` composition | — | — |

Propagate applicable rows unchanged to tasks RED tests.

## Migration / Rollout

No migration. `omitempty` fields decode `0`; `MaxLines==0` defaults `400`; cumulative recomputed as `sum(ChangedLines)` if absent. Missing config → `openspec`. Each PR `git revert` safe.

## Open Questions

- [ ] Config key `sdd.artifact_store` vs `artifact_store` — read both, prefer `sdd.`
- [ ] `SnapshotBuilder` name alias to `wrapRuntimeCandidateUnavailable` if differs
- [ ] PR1 `≤30` acceptable if import needed

## PR Slices

| PR | Scope | Cap | Files |
|----|-------|-----|-------|
| PR1 | Ledger verify-before-commit | `≤20` (`≤30` allow) | `cas_store.go` |
| PR2 | Budget+refund single predicate, `2×` cap | `<400` | `sddattempt.go` |
| PR3 | Locator+hybrid+rescope wedge (+taxonomy) | `<400` | `status.go`, `engram_status.go`, `Rescope`, `capture.go` |
