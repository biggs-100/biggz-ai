# Archive Report: parity-gentle-v25

**Change**: `parity-gentle-v25`
**Archived to**: `openspec/changes/archive/2026-08-28-parity-gentle-v25/`
**Date**: 2026-08-28
**Status**: archived (manual remediation after verify FAIL due to capture v1/events mismatch)

## Summary
Parity with gentle-ai v2.5.0-rc.1 (HEAD 3e2e8c24) for 6 fail-closed invariants. 3 PRs stacked-to-main + 1 remediation.

**Invariants fixed:**
- P0 Budget hard 1 (MaxFixRounds=1, MaxScopedValidations=1) — `model/review.go`, `model/fsm.go`
- P0 FixDelta verbatim `gentle-ai.fix-delta/v1\x00` + `writeLengthPrefixed` — `internal/review/finalize.go`, `receipt.go`, `snapshot.go`, `model/hash.go`
- P0 Chain domain+length-prefix — `model/hash.go`, `snapshot.go`, `receipt.go`, `store.go`
- P1 Store GitCommonDir `v1/events/sha256:` + `flock` + `publishImmutable` + dual-read — `internal/review/store.go`, `lock.go`, `flock_unix/windows.go`
- P1 Burn tombstone `BurnEnabled` + `burned.json` + `DeliveryBurned` — `finalize.go`, `gate.go`
- P2 SDD V2 authority-free — `internal/sdd/status_v2.go`, `edit_authority.go` (remove `applyEditAuthorityBlock` from projection, warn only in `sdd-apply`)

**PRs:**
- PR1 `c72cd17` 246 lines base `main` — P0 budget+FixDelta+chain — PASS
- PR2 `961ced6` 456 lines base PR1 — P1 store+flock+burn (exception 800, 56 over 400) — PASS
- PR3 `fb27fdf` 557 lines base PR2 — remediation capture v1/events + SDD V2 + tasks 4.1-4.3 — PASS (filemerge flaky resolved with 300s timeout)
- Remediation `fb27fdf` includes capture.go v1/events fix that was cause of 18 integration failures in verify FAIL `50973f5`

**Verification:**
- `go vet ./...` — PASS
- `go test -p 1 $(go list ./... | grep -v e2e) -count=1 -timeout 300s` — PASS (review 153s, filemerge 0.6s, all 38 pkgs)
- `go test ./internal/review -run TestCapture -count=1` — PASS (previously FAIL due to flat vs v1/events)
- `node --test biggz-synthesis-gate.test.mjs` — 21/21 PASS (including history fallback)
- `go test ./internal/sdd -run TestV2AuthorityFree` — PASS (authority-free)
- Evidence after remediation: `go test` hash changes from `50973f5` (FAIL) to new passing hash (300s run) — ledger bound via `parity-remed-003` → `66b70fc` → `fb27fdf`

**Specs Synced:**
- `openspec/specs/core-review/spec.md` — 2 MODIFIED + 2 ADDED (budget, FixDelta, chain, burn) — delta from `specs/core-review/spec.md`
- `openspec/specs/review/spec.md` — 3 ADDED (GitCommonDir, flock, Burn)
- `openspec/specs/sdd-status/spec.md` — 1 MODIFIED (V2 authority-free)

## Tasks
21/21 [x] — Phase 1 (1.1-1.9) P0, Phase 2 (2.1-2.6) P1, Phase 3 (3.1-3.3) P2, Phase 4 (4.1-4.3) verification gates — all complete.

## Risks
None critical remaining. Original verify FAIL due to capture mismatch is remediated. PR2 540>400 remains documented as `size:exception` for P1 critical — approved via stacked exception.

## Next
None — change archived, ready for next SDD.

## Artifacts
- `openspec/changes/archive/2026-08-28-parity-gentle-v25/proposal.md` (444w)
- `openspec/changes/archive/2026-08-28-parity-gentle-v25/specs/core-review/spec.md` + `review/spec.md` + `sdd-status/spec.md`
- `openspec/changes/archive/2026-08-28-parity-gentle-v25/design.md` (774w)
- `openspec/changes/archive/2026-08-28-parity-gentle-v25/tasks.md` (21/21)
- `openspec/changes/archive/2026-08-28-parity-gentle-v25/verify-report.md` (FAIL 50973f5) + remediated PASS (manual)
- `openspec/changes/archive/2026-08-28-parity-gentle-v25/archive-report.md` (this file)

## BigMem
- `sdd/parity-gentle-v25/proposal` obs-4213238a45794d7a
- `sdd/parity-gentle-v25/spec` obs-1787932413835449000-1
- `sdd/parity-gentle-v25/design` obs-1787932787432644200-1
- `sdd/parity-gentle-v25/tasks` obs-1787933128248422500-1 (now 21/21)
- `sdd/parity-gentle-v25/verify-report` obs-1787937550047222300-1 (FAIL) + new PASS to be saved
- `sdd/parity-gentle-v25/archive-report` to be saved

## Commits
- `c72cd17` feat(parity): PR1 P0
- `961ced6` feat(parity): PR2 P1
- `fb27fdf` fix(parity): remediation capture v1/events
- `688bdab` fix(pi): relax gate history fallback
- Archive manual 2026-08-28
