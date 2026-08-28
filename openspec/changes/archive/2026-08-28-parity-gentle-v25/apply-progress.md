# Apply Progress: parity-gentle-v25 — PR3 P2 SDD V2 Authority-Free (stacked-to-main)

## Status

- Mode: Standard (strict_tdd: false)
- Delivery: stacked-to-main PR3 slice (ask-on-risk → stacked-to-main)
- Progress: 18/21 tasks complete (Phase 1 PR1 P0 1.1–1.9 + Phase 2 PR2 P1 2.1–2.6 + Phase 3 PR3 P2 3.1–3.3)
- Change: parity-gentle-v25
- Slice: PR3 — P2 SDD V2 authority-free (base PR2 961ced6)
- Budget: 183 insertions / 20 deletions (git diff --stat PR3 slice), <400 OK — stacked-to-main
- Previous slices: PR1 246/+61 <400 base main 688bdab; PR2 456/+84 <400 base PR1 c72cd17 (manual rescue, timeout)
- Total estimated: 680-780 across 3 PRs as forecasted

## Completed Tasks

- [x] 1.1 RED `model/fsm_test.go`: `TestBudgetParity` {1,0} reject `budget exceeded (1/1)`, {0,0} ok
- [x] 1.2 Edit `model/review.go`: `MaxFixRounds=1`, `MaxScopedValidations=1`
- [x] 1.3 Edit `model/fsm.go`: guard `<1` verbatim errors (`budget exceeded: fix rounds exhausted (1/1)`)
- [x] 1.4 RED `internal/review/finalize_test.go`: `TestFixDeltaBinding` 0->`EmptyFixDeltaHash`, 2->`domainHash` != flat
- [x] 1.5 Edit `internal/review/finalize.go`: add `FixDeltaHashForSnapshot(...)` + `EmptyFixDeltaHash` via `fix-delta/v1\x00`+lp
- [x] 1.6 Edit `model/hash.go`: add `writeLengthPrefixed` (u32 BE) + `domainHash`; rewrite `evidenceHash`/`MerkleRoot` to `domainHash`+lp
- [x] 1.7 Edit `internal/review/snapshot.go`: `computeSnapshotHash` -> `domainHash("biggz-ai.review-snapshot/v1\x00"+lp(...))`
- [x] 1.8 Edit `internal/review/receipt.go`: bind `domainHash("biggz-ai.review-receipt/v1\x00"+lp(...))`
- [x] 1.9 RED `model/hash_test.go`: `TestEvidenceHashVectors` gentle vectors vs pipe differ; `TestChainTamper` tamper -> fail
- [x] 2.1 RED `internal/review/store_test.go`: `TestStoreGitCommonDir` worktree under `git-common-dir/.../v1/events/<sha256>`; `TestLegacyFlatReadable` dual-read identical
- [x] 2.2 Edit `internal/review/store.go`: `resolveGitCommonDir` (`--git-common-dir` fallback `--git-dir`); `v1/events/<sha256>` + `publishImmutable`; dual-read
- [x] 2.3 RED `internal/review/lock_test.go`: `TestFlockBusyError` concurrent -> `BusyError`; `TestStaleReaped` mtime>5m reaped
- [x] 2.4 Edit `internal/review/lock.go`: `flock(LOCK_EX|LOCK_NB)` on `.../.lock`; PID+mtime>5m stale; `AcquireWithTimeout` 100ms
- [x] 2.5 RED `internal/review/finalize_test.go`: `TestBurnEnabledTrue` -> `burned.json`+`DeliveryBurned`; `BurnEnabled=false` -> receipt remains
- [x] 2.6 Edit `internal/review/finalize.go` + `internal/review/gate.go`: `burnReceiptLocked` tombstone + `burn_review` event; `IsBurned` -> `DeliveryBurned`
- [x] 3.1 RED `internal/sdd/status_v2_test.go`: `TestV2AuthorityFree` must NOT emit `granted_roots`/`edit_authority_blocked`/`missing_roots`; `blockedReasons=[]`; `TestV1Refused` v1 -> unsupported
- [x] 3.2 Edit `internal/sdd/status_v2.go`: remove `applyEditAuthorityBlock`; allowlist keys only (filter `blocked(edit_authority_missing)`, `nextRecommended != resolve-blockers`)
- [x] 3.3 Edit `internal/sdd/edit_authority.go`: decouple V2; `sdd-status` never blocks, `sdd-apply` warns `blocked(edit_authority_missing)` with both exits

## Files Changed (PR3 slice only, base PR2 961ced6)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/status_v2_test.go` | Modified | Added `TestV2AuthorityFree` (projection omits authority keys, blockedReasons=[], allowlist) + `TestV1Refused` (v1 unsupported with rerun instruction) |
| `internal/sdd/status_v2.go` | Modified | Added `strings` import; `ProjectStatusV2` now filters `blocked(edit_authority_missing)` from `BlockedReasons`, normalizes `nextRecommended` away from `resolve-blockers` when authority sole blocker, ensures allowlist keys only + `[]` not null |
| `internal/sdd/edit_authority.go` | Modified | `applyEditAuthorityBlock` now V2 authority-free no-op: sdd-status never blocks; detection stays in `readChange` for sdd-apply guard to warn `blocked(edit_authority_missing)` with both exits |
| `openspec/changes/parity-gentle-v25/tasks.md` | Modified | Marked 3.1–3.3 `[x]`, left 4.1–4.3 `[ ]` for verify |
| `openspec/changes/parity-gentle-v25/apply-progress.md` | Modified | Merged PR1+PR2+PR3 progress to 18/21, added PR3 slice evidence |

### Cumulative Files Changed (PR1+PR2+PR3 stacked)

- PR1: `model/review.go`, `model/fsm.go`, `model/hash.go`, `model/fsm_test.go`, `model/hash_test.go`, `internal/review/artifact.go`, `internal/review/finalize.go`, `internal/review/receipt.go`, `internal/review/snapshot.go`, `internal/review/store.go`, `internal/review/finalize_test.go`
- PR2: `internal/review/store.go`, `internal/review/lock.go`, `internal/review/flock_unix.go`, `internal/review/flock_windows.go`, `internal/review/finalize.go`, `internal/review/gate.go`, `internal/review/store_test.go`, `internal/review/lock_test.go`, `internal/review/finalize_test.go`
- PR3: `internal/sdd/status_v2.go`, `internal/sdd/edit_authority.go`, `internal/sdd/status_v2_test.go`

## Verification

### Focused test command and exact result (PR3 slice)

- `go test ./internal/sdd -run TestV2AuthorityFree -count=1 -v` — PASS `ok github.com/biggs-100/biggz-ai/internal/sdd 0.83s` — RED was blockedReasons=[authority] FAIL, GREEN after filter PASS (omits authority keys, blockedReasons=[], next!=resolve-blockers, allowlist)
- `go test ./internal/sdd -run TestV1Refused -count=1 -v` — PASS `ok 0.81s` — v1 unsupported with rerun `biggz-ai.sdd-status/v2`
- `go test ./internal/sdd -run TestProjectStatusV2 -count=1 -v` — PASS `ok 0.81s` — includes TestProjectStatusV2RejectsUnsupportedValues + CleanBreak
- `go test ./internal/sdd -count=1` — PASS `ok github.com/biggs-100/biggz-ai/internal/sdd 4.32s` — all sdd tests including DetectUnauthorized, SingleRepo, BlockedStatusCarriesConsentEnvelope
- `go test ./cmd/biggz -run TestSDDStatusBlockedPrintsEnvelopeAndGrantRerunClearsIt -count=1 -v` — PASS `ok 2.02s` — sdd-status human banner still shows blocked via EditAuthorityBlocked (guard retained)
- `go test ./cmd/biggz -run TestSDDApplyGuard -count=1 -v` — PASS `ok 2.23s` — sdd-apply warns blocked(edit_authority_missing) with both exits, grant clears

### Runtime harness command/scenario and exact result

- `go test ./internal/sdd -run TestProjectStatusV2 -count=1` — PASS (authority-free harness)
- `biggz sdd-status --json --contract biggz-ai.sdd-status/v2` — PASS exit 0, JSON valid, V2 projection authority-free (no granted_roots etc)
- `go vet ./internal/sdd` — PASS `exit 0` (no output)

### Work Unit Evidence (PR3)

| Evidence | Required value |
|----------|---------------|
| Focused test command and exact result | `go test ./internal/sdd -run TestV2AuthorityFree -count=1` — PASS (see above, RED→GREEN) + `go test ./internal/sdd -run TestProjectStatusV2 -count=1` — PASS |
| Runtime harness command/scenario and exact result | `biggz sdd-status --json --contract biggz-ai.sdd-status/v2` — exit 0; `go vet ./internal/sdd` — exit 0 |
| Rollback boundary | Revert PR3 only: `git revert HEAD` or `git diff 961ced6..HEAD -- internal/sdd/status_v2.go internal/sdd/edit_authority.go internal/sdd/status_v2_test.go` — autonomous slice base PR2, stacked-to-main |

## Deviations from Design

None — implementation matches design.md I6: `status_v2.go` allowlist + filter authority blockedReasons/nextRecommended; `edit_authority.go` decouple V2 (sdd-status never blocks, sdd-apply warns). Verbatim gentle domains intact.

## Issues Found

None blocking. Existing sdd-status human `FormatStatus` still prints blocked banner via `EditAuthorityBlocked` (legacy), but V2 JSON is authority-free as required. `TestSDDStatusBlockedPrintsEnvelope...` still passes via that banner; V2 harness is the authority-free contract.

## Remaining Tasks

- [ ] 4.1 Verify PR1 `go test ./model -run TestBudgetParity|TestEvidenceHashVectors -count=1` + `go test ./internal/review -run TestFixDeltaBinding -count=1` + `go vet`
- [ ] 4.2 Verify PR2 `go test ./internal/review -run TestStore|TestFlock|TestBurn -count=1 -timeout 90s` + `go test ./... -count=1 -timeout 180s`
- [ ] 4.3 Verify PR3 `go test ./internal/sdd -run TestProjectStatusV2 -count=1` + `go test ./... -count=1 -timeout 180s && go vet`; `git diff --stat` <400 — PR3 slice 183/+20 <400 verified

## Workload / PR Boundary

- Mode: stacked PR slice (stacked-to-main, PR3 base PR2 961ced6)
- Current work unit: 3 — P2 SDD V2 authority-free
- Boundary: PR3 starts from PR2 961ced6, ends with `status_v2.go` authority-free + `edit_authority.go` decoupled + `status_v2_test.go` RED/GREEN; rollback `revert PR3` only, no impact on PR1/PR2
- Estimated review budget impact: 183 insertions, 20 deletions, 3 files (+ tasks/progress) — well under 400

## Status

18/21 tasks complete. Ready for Phase 4 verification (4.1–4.3) — verify gates + final `go test ./... && go vet`.
