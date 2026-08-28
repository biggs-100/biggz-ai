# Design: parity-gentle-v25 — 6 Invariantes Fail-Closed (Gentle v2.5.0-rc.1)

## Technical Approach

Verbatim port gentle 3e2e8c24. Six invariants → 3 PRs `stacked-to-main` <400 lines. Helpers minimal (`writeLengthPrefixed`, `domainHash`). Dual-read preserves legacy flat; `BurnEnabled` reversible. Covers proposal 444w + specs core-review 7scen, review 5scen, sdd-status 3scen.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| **I1 Budget** `MaxFixRounds`/`MaxScopedValidations` 3/5→1 | A: FSM only B: const+guard+ledger | A hides constants | **B** — `model/review.go` const=1; `fsm.go` guard `<1`; ledger fallback |
| **I2 FixDelta** | A: `payloadSHA256("fix-delta:%d")` B: `domainHash("fix-delta/v1\x00"+lp(...))` | A not content-bound | **B** — `finalize.go:FixDeltaHashForSnapshot`; zero→`EmptyFixDeltaHash`; `Validate()` rejects flat |
| **I3 Chain** hash | A: pipe `\|` concat B: `domainHash(domain+"\x00"+lp)` | A collision-prone | **B** — `model/hash.go:writeLengthPrefixed` (u32 BE len) + domains `review-evidence/v1`, `review-merkle/v1`, `review-snapshot/v1` |
| **I4 Store+Lock** | A: `--git-dir`+`O_EXCL` B: `--git-common-dir`+`flock` | A splits worktrees, racy | **B** — `store.go:resolveGitCommonDir` fallback; dual-read; `lock.go:flock(LOCK_EX\|LOCK_NB)`+5m stale |
| **I5 Burn** | A: `os.Remove` B: `burned.json`+`BurnEnabled` | A silent loss | **B** — `finalize.go:BurnEnabled=true`→tombstone; `IsBurned()`→`DeliveryBurned` |
| **I6 SDD V2** | A: `applyEditAuthorityBlock` in status B: authority-free | A blocks `sdd-status` | **B** — `status_v2.go` allowlist; `edit_authority.go` warn only in `sdd-apply` |

## Data Flow

**BudgetCounters→FSM:**
```
BudgetCounters{FixRounds,ScopedValidations} → guardTable(BudgetCheck)
 → FixRounds>=1 reject "budget exceeded (1/1)" else ChangesSubmitted; ScopedValidations>=1 reject else ReReview
```

**FixDelta:**
```
(baseTree,candidate,pathsDigest,cumulative,ledgerIDs) → FixDeltaHashForSnapshot
 → cumulative==0 ? EmptyFixDeltaHash : domainHash("fix-delta/v1\x00"+lp(fields)) → receipt.FixDeltaHash → computeHash binds
```

**Chain:**
```
Evidence{Pos,Ts,Kind,Payload,Prev} → lp(Pos,Ts,Kind,Payload,Prev)
 → domainHash("biggz-ai.review-evidence/v1\x00"+lp) linked via PrevHash
 → MerkleRoot=domainHash("biggz-ai.review-merkle/v1\x00"+lp(lastHash))
 → Snapshot.Hash=domainHash("biggz-ai.review-snapshot/v1\x00"+lp(base,candidate,paths))
```

**Store+Lock:**
```
Open(repo,lineage) → git --git-common-dir (fallback --git-dir)
 → <commonDir>/biggz/review-transactions/<lineage>/v1/events/<sha256>
 → Append: flock .lock → publishImmutable → HEAD atomic; LoadChain dual-read commonDir→flat
```

**SDD V2:**
```
ChangeStatus → ProjectStatusV2 allowlist → omit granted_roots/missing_roots → blockedReasons=[]; next≠resolve-blockers
 → sdd-apply warns blocked(edit_authority_missing) with both exits
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `model/review.go` | Modify | `MaxFixRounds=1`, `MaxScopedValidations=1` |
| `model/fsm.go` | Modify | Guard `<1`, budget error verbatim |
| `model/hash.go` | Modify | `writeLengthPrefixed`+`domainHash`; rewrite `evidenceHash`/`MerkleRoot` |
| `internal/review/finalize.go` | Modify | `FixDeltaHashForSnapshot`+`EmptyFixDeltaHash`; `BurnEnabled` tombstone |
| `internal/review/receipt.go` | Modify | Bind `FixDeltaHashForSnapshot` in `Validate()` |
| `internal/review/snapshot.go` | Modify | `computeSnapshotHash` via `lp`+domain |
| `internal/review/store.go` | Modify | `resolveGitCommonDir`, `v1/events/<sha256>`, dual-read |
| `internal/review/lock.go` | Modify | `flock(LOCK_EX\|LOCK_NB)` on `.lock`, stale PID+mtime |
| `internal/sdd/status_v2.go` | Modify | Remove `applyEditAuthorityBlock`; authority-free |
| `internal/sdd/edit_authority.go` | Modify | Decouple V2; warn only in `sdd-apply` |
| `internal/review/gate.go` | Modify | `DeliveryBurned` when `IsBurned()` |

## Interfaces / Contracts

```go
const MaxFixRounds=1; const MaxScopedValidations=1
func writeLengthPrefixed(fields ...[]byte) []byte // u32 BE len+bytes
func domainHash(domain string, payload []byte) string // "sha256:"+hex(sha256(domain+"\x00"+payload))
func evidenceHash(e Evidence) string // domainHash("biggz-ai.review-evidence/v1", lp(...))
func MerkleRoot(c []Evidence) string // domainHash("biggz-ai.review-merkle/v1", lp(lastHash))
const EmptyFixDeltaHash="sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
var BurnEnabled=true
func FixDeltaHashForSnapshot(baseTree,candidate,pathsDigest string,cumulative int,ids []string) string
func resolveGitCommonDir(repo string)(string,error) // --git-common-dir fallback --git-dir
func (s *Store) IsBurned() bool; func IsChainBurned(ValidatedChain) bool
func (fl *FileLock) Acquire() error // flock LOCK_EX|LOCK_NB → BusyError
func ProjectStatusV2(ChangeStatus)(StatusV2Projection,error) // omits granted_roots/missing_roots
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Budget 0→ok 1→reject; chain vectors ≠pipe; FixDelta zero vs flat | `TestBudgetParity`, `TestEvidenceHashVectors`, `TestFixDeltaBinding` |
| Integration | Worktree→commonDir, flat readable, flock serialize, burn tombstone, SDD omit keys | `store_test.go` worktree; `lock_test.go`; `finalize_test.go` BurnEnabled; `gate_test.go` DeliveryBurned; `status_v2_test.go` |
| E2E | `go test ./... -count=1 -timeout 180s` + `go vet`; 21 gates; v1 contract refused | CI |

## Threat Matrix

No arbitrary shell; only `git rev-parse` (safe). Domain threats below.

| Boundary | Applicable | Response | RED Tests |
|----------|------------|----------|-----------|
| Chain tamper | Yes — integrity | `domainHash+lp` fail-closed | Tamper→Validate fail; pipe≠vector |
| Store split | Yes — loss | `GitCommonDir`+dual-read | Worktree file under commonDir; legacy returns same |
| Lock bypass | Yes — double burn | `flock`+BusyError+stale | Concurrent→BusyError; stale>5m reaped |
| Burn loss | Yes — resurrection | `burned.json`+`IsBurned` | true→tombstone+DeliveryBurned; false→receipt remains |
| Budget overrun | Yes — flood | const 1+guard | Round2 rejected 1/1 |
| SDD leak | Yes — block | V2 omit keys; apply warns | JSON lacks keys; blockedReasons=[] despite `../other` |

## Migration / Rollout

No migration. Dual-read, `BurnEnabled` reversible. Revert P2→P0 via `git revert`.

**3 PRs stacked-to-main <400:**
- **PR1 Budget+FixDelta+Chain** — `model/*`, `finalize.go` FixDelta, `snapshot/receipt.go`: vectors, binding
- **PR2 Store/Lock+Burn** — `store.go`, `lock.go`, `finalize burn`, `gate.go`: commonDir, flock, tombstone
- **PR3 SDD V2** — `status_v2.go`, `edit_authority.go`: authority-free

Gates `go test`+`go vet` per PR; 21 gates on PR3.

## Open Questions

- [ ] `ledgerIDs` order sorted vs insertion — confirm gentle sorts
- [ ] `writeLengthPrefixed` Position encoding (string vs u32) — align vectors
