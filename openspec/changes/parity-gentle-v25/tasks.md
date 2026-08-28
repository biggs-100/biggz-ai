# Tasks: parity-gentle-v25 — 6 Invariants Fail-Closed

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 680-780 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1->PR2->PR3 stacked-to-main |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | P0 Budget+FixDelta+chain | PR1 base `main` | `go test ./model -run TestBudgetParity -count=1` | `go test ./... -count=1 -timeout 180s && go vet` | Revert `model/*` + `internal/review/finalize.go,receipt.go,snapshot.go` |
| 2 | P1 GitCommonDir+flock+Burn | PR2 base PR1 | `go test ./internal/review -run TestStore -count=1` | `git worktree add /tmp/wt1 && go test ./internal/review -count=1` | Revert `internal/review/store.go,lock.go,gate.go,finalize.go` |
| 3 | P2 SDD V2 authority-free | PR3 base PR2 | `go test ./internal/sdd -run TestV2 -count=1` | `biggz sdd-status --json --contract biggz-ai.sdd-status/v2` | Revert `internal/sdd/status_v2.go,edit_authority.go` |

## Phase 1: PR1 P0 — Budget + FixDelta + Chain

- [x] 1.1 RED `model/fsm_test.go`: `TestBudgetParity` {1,0} reject `budget exceeded (1/1)`, {0,0} ok
- [x] 1.2 Edit `model/review.go`: `MaxFixRounds=1`, `MaxScopedValidations=1`
- [x] 1.3 Edit `model/fsm.go`: guard `<1` verbatim errors
- [x] 1.4 RED `internal/review/finalize_test.go`: `TestFixDeltaBinding` 0->`EmptyFixDeltaHash`, 2->`domainHash` != flat; `Validate` rejects flat
- [x] 1.5 Edit `internal/review/finalize.go`: add `FixDeltaHashForSnapshot(...)` + `EmptyFixDeltaHash`
- [x] 1.6 Edit `model/hash.go`: add `writeLengthPrefixed` (u32 BE) + `domainHash`; rewrite `evidenceHash`/`MerkleRoot` to `domainHash`+lp
- [x] 1.7 Edit `internal/review/snapshot.go`: `computeSnapshotHash` -> `domainHash("biggz-ai.review-snapshot/v1\x00"+lp(...))`
- [x] 1.8 Edit `internal/review/receipt.go`: bind `FixDeltaHashForSnapshot` in `Validate`/`computeHash`; legacy fallback when cumulative==0
- [x] 1.9 RED `model/hash_test.go`: `TestEvidenceHashVectors` gentle vectors vs pipe differ; `TestChainTamper` tamper -> fail

## Phase 2: PR2 P1 — Store + Flock + Burn

- [x] 2.1 RED `internal/review/store_test.go`: `TestStoreGitCommonDir` worktree under `git-common-dir/.../v1/events/<sha256>`; `TestLegacyFlatReadable` dual-read identical
- [x] 2.2 Edit `internal/review/store.go`: `resolveGitCommonDir` (`--git-common-dir` fallback `--git-dir`); `v1/events/<sha256>` + `publishImmutable`; dual-read
- [x] 2.3 RED `internal/review/lock_test.go`: `TestFlockBusyError` concurrent -> `BusyError`; `TestStaleReaped` mtime>5m reaped
- [x] 2.4 Edit `internal/review/lock.go`: `flock(LOCK_EX|LOCK_NB)` on `.../.lock`; PID+mtime>5m stale; `AcquireWithTimeout` 100ms
- [x] 2.5 RED `internal/review/finalize_test.go`: `TestBurnEnabledTrue` -> `burned.json`+`DeliveryBurned`; `BurnEnabled=false` -> receipt remains
- [x] 2.6 Edit `internal/review/finalize.go` + `internal/review/gate.go`: `burnReceiptLocked` tombstone + `burn_review` event; `IsBurned` -> `DeliveryBurned`

## Phase 3: PR3 P2 — SDD V2 Authority-Free

- [x] 3.1 RED `internal/sdd/status_v2_test.go`: `TestV2AuthorityFree` must NOT emit `granted_roots`/`edit_authority_blocked`/`missing_roots`; `blockedReasons=[]`; `TestV1Refused` v1 -> unsupported
- [x] 3.2 Edit `internal/sdd/status_v2.go`: remove `applyEditAuthorityBlock`; allowlist keys only
- [x] 3.3 Edit `internal/sdd/edit_authority.go`: decouple V2; `sdd-status` never blocks, `sdd-apply` warns `blocked(edit_authority_missing)` with both exits

## Phase 4: Verification & Gates

- [x] 4.1 Verify PR1 `go test ./model -run TestBudgetParity|TestEvidenceHashVectors -count=1` + `go test ./internal/review -run TestFixDeltaBinding -count=1` + `go vet` — PASS (model, review, vet 0, gate 21/21)
- [x] 4.2 Verify PR2 `go test ./internal/review -run TestStore|TestFlock|TestBurn -count=1 -timeout 90s` + `go test ./... -count=1 -timeout 180s` — PASS (review 153s, filemerge 0.6s, go vet 0)
- [x] 4.3 Verify PR3 `go test ./internal/sdd -run TestProjectStatusV2 -count=1` + `go test ./... -count=1 -timeout 180s && go vet`; `git diff --stat` <400 — PASS (SDD V2 authority-free, 183+20 <400, p1 246, p2 456 with 800 exception)
