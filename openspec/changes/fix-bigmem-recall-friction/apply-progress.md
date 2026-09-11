# Apply Progress — fix-bigmem-recall-friction

- **Change:** fix-bigmem-recall-friction
- **Slice:** 1 + Phase 1b (Phase 1 — Ghost-WAL liveness, PR 1 of the feature-branch-chain)
- **Status:** complete (tasks 1.1–1.4) — runtime harness evidence + bookkeeping finished in this continuation run (the previous apply run hit the 20-minute task-mode timeout after the code was green, before harness evidence and bookkeeping). Phase 1b (tasks 1b.1–1b.5) added in a later continuation run; see the Phase 1b section for evidence; its one off-surface residual (`TestGhostWAL_Stale_Removed`) was resolved in a follow-up run via the liveness seam — no residual remains.
- **Mode:** Standard (`strict_tdd: false` for this project; no Strict-TDD cycle table applies)
- **Branch / base:** `fix/bigmem-recall-friction-1-ghost-wal` (tracker: `fix/bigmem-recall-friction`, base `bed2d881`) — uncommitted, left dirty for human review
- **Environment:** Go 1.26.1 windows/amd64

## Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 1.1 RED `ghost_test.go` classify/probe matrix (fresh, stale-idle, live-holder, inconclusive) | [x] | `TestClassifyGhostWAL_Matrix`, `TestProbeDBLiveness_Matrix` PASS |
| 1.2 `liveness_windows.go` (share-0 probe) + `liveness_other.go` (O_EXCL) | [x] | files exist; matrix PASS; runtime share-0 probe output below |
| 1.3 `classifyGhostWAL` → live holder primary / dead reclaim / inconclusive recovered | [x] | `TestGhostWAL_*` PASS + runtime Case A / Case B below |
| 1.4 Integration: held handle → no removal, no fallback; checkpoint error never fails Open | [x] | `TestGhostWAL_LiveHolder_UsesPrimary`, `TestGhostWAL_ReclaimKeepsData`, `TestReclaimCheckpointErrorNeverFailsOpen` PASS + runtime A/B below |

## Files Changed (slice 1 = 676 authored changed lines; size exception below)

| File | +/- | Purpose |
|------|-----|---------|
| `internal/bigmem/bigmem.go` | +65/-47 | `classifyGhostWAL` + `ResolveDBPath` switch: live holder → primary (no touch, no fallback); proven dead → `reclaimStaleWAL` (Remove wal/shm + `.ghost_probe` + best-effort `wal_checkpoint(TRUNCATE)`); inconclusive → recovered fallback intact; `ghostReclaimCheckpoint` test seam |
| `internal/bigmem/liveness_windows.go` | +39 (new) | `probeDBLiveness` via `windows.CreateFile` share-mode 0: `ERROR_SHARING_VIOLATION`/`ERROR_LOCK_VIOLATION` ⇒ live holder; success/missing ⇒ proven dead |
| `internal/bigmem/liveness_other.go` | +26 (new) | legacy O_CREATE\|O_EXCL claim probe for non-Windows (failure is inconclusive, never "dead") |
| `internal/bigmem/ghost_test.go` | +426 (new) | 7 tests: classify/probe matrix, live-holder primary, inconclusive preserves files, reclaim-when-dead, reclaim-keeps-data, checkpoint-error-never-fails-Open, leftover-probe reclaim |
| `internal/bigmem/bigmem_test.go` | +0/-73 | removed obsolete tests of the old single-file probe (`TestGhostWAL_Busy_OExcl_Preserved`, `TestGhostWAL_ProbeOExcl`) |

## Focused Test Command + Result

```
$ go test ./internal/bigmem -run 'Ghost|Liveness' -count=1 -v
--- PASS: TestGhostWAL_Stale_Removed (0.05s)
--- PASS: TestGhostWAL_Fresh_Kept (0.01s)
--- PASS: TestGhostWAL_SaveSearch_Checkpoint (0.12s)
--- PASS: TestGhostWAL_WALBounded (0.57s)
--- PASS: TestGhostWAL_LiveHolder_UsesPrimary (0.07s)
--- PASS: TestGhostWAL_Inconclusive_PreservesFiles (0.01s)
--- PASS: TestGhostWAL_LeftoverProbe_ReclaimWhenDead (0.01s)
--- PASS: TestProbeDBLiveness_Matrix (0.00s)
--- PASS: TestClassifyGhostWAL_Matrix (0.08s)
--- PASS: TestGhostWAL_ReclaimKeepsData (0.07s)
--- PASS: TestIsGhostWAL_Robust (0.01s)
--- PASS: TestIsGhostWAL (0.09s)
ok  github.com/biggs-100/biggz-ai/internal/bigmem  1.736s
```
exit code 0, 12/12 PASS.

## Package Suite Command + Result

```
$ go test ./internal/bigmem -count=1 -timeout 180s
ok  github.com/biggs-100/biggz-ai/internal/bigmem  17.664s   (exit 0)
```

## Static Checks

```
$ go vet ./internal/bigmem          # exit 0, no output
$ gofmt -l internal/bigmem          # no output (clean)
```

## Runtime Harness Evidence

Fixed binaries built from the working tree into a temp dir (never installed, never pointed at the real store):

```
$ go build -o /tmp/bm-harness/bin/biggz-fixed.exe ./cmd/biggz          # exit 0
$ go build -o /tmp/bm-harness/bin/biggz-mcp-fixed.exe ./cmd/biggz-mcp  # exit 0
```

Temp store root `/tmp/bm-harness/rt/homeA` (`USERPROFILE` override; real `~/.biggz/bigmem` never opened — its mtime stayed at the pre-harness value 17:00:46). Holder = PowerShell process holding `bigmem.db`, `bigmem.db-wal`, `bigmem.db-shm` open with `FileShare.ReadWrite` (deny delete) — the held-handle variant permitted by the task; no real `biggz-mcp` was driven through stdio.

### Case A — live holder + stale ghost shape (REQ-GW1): primary used, warning absent, no fallback

Ghost shape forged: `wal = 0 B`, `shm = 32768 B`, both backdated 6 min (stale).

```
$ powershell probe.ps1 -Db '...\homeA\.biggz\bigmem\bigmem.db'   # separate process, share-0 open
share0-open-FAILED (live holder detected): ...porque está siendo utilizado en otro proceso

$ USERPROFILE=<temp-home> biggz-fixed.exe bigmem search "primary-only-marker-AAA7" --project ghost-harness
  obs-1789078030712731200-1 [note] ghost-harness-seed (0s)          # EXIT=0
stderr: (0 bytes)                                                    # ghost warning ABSENT

$ USERPROFILE=<temp-home> biggz-fixed.exe bigmem save "ghost-harness-live-write" "live-write-marker-BBB9" --type note --project ghost-harness
Saved: obs-1789078072328524800-1                                     # EXIT=0
stderr: (0 bytes)

after: bigmem_recovered/  -> ABSENT (fallback never taken)
       wal -> present, 0 B, sha256 unchanged
       shm -> present, 32768 B (content refreshed by the CLI's own SQLite connection: normal WAL-index
              activity, not the ghost path; size preserved, not removed, not truncated)
```

```
before run: bigmem.db.ghost_probe = 21 B (marker), wal/shm stale shape re-forged, holder live
$ biggz-fixed.exe bigmem search "primary-only-marker-AAA7" ...   # EXIT=0, stderr 0 bytes
after run:  bigmem.db.ghost_probe = 21 B  -> SURVIVED  => reclaimStaleWAL did NOT run (live holder branch)
            wal/shm present; recovered ABSENT
```

### Case B — holder provably dead (REQ-GW2): ghost files reclaimed, data intact

```
$ powershell probe.ps1 -Db '...\homeA\.biggz\bigmem\bigmem.db'   # after releasing holder
share0-open-SUCCEEDED (no live holder)                            # provably dead

shape re-forged stale (+ ghost_probe marker 21 B present),
$ USERPROFILE=<temp-home> biggz-fixed.exe bigmem get obs-1789078030712731200-1
ID:        obs-1789078030712731200-1
Title:     ghost-harness-seed
...
Content:   primary-only-marker-AAA7                              # EXIT=0, stderr 0 bytes

after: bigmem.db.ghost_probe -> GONE  (reclaim removed it  => ghostStale branch ran)
       wal/shm                -> GONE  (reclaimed + TRUNCATE checkpoint)
       bigmem_recovered/      -> ABSENT
data intact: search "live-write-marker-BBB9"   -> obs-1789078072328524800-1
             search "primary-only-marker-AAA7" -> obs-1789078030712731200-1
```

### Negative control — OLD installed binary (`C:\Users\USER\go\bin\biggz`, `biggz-ai dev`, 2026-09-07)

Byte-identical twin stores (same seed, same forged shape), same holder protocol:

| Binary + store | Holder | `bigmem_recovered` created | `[bigmem] warning: ghost WAL/SHM detected` | wal/shm after |
|---|---|---|---|---|
| OLD `biggz` + `homeD` run 1 | live | **YES** (silent fallback) | no (recovered did not pre-exist) | preserved (0 / 32768) |
| OLD `biggz` + `homeD` run 2 (shape restored) | live | already existed | **YES — exact user line:** `[bigmem] warning: ghost WAL/SHM detected; using recovered DB at C:\...\homeD\.biggz\bigmem_recovered\bigmem.db (primary C:\...\homeD\.biggz\bigmem\bigmem.db blocked)` | preserved |
| FIXED build + `homeE` (identical twin) | live | **NO** | no (stderr 0 bytes) | preserved (0 / 32768) |

Context: with **no** holder the old binary reclaims like the fixed one; the defect is specifically the **live-holder misclassification** (failed removal attempts flip the old code into the recovered fallback).

### Harness limitations (stated explicitly)

- Simulated live holder (held handles), not a driven real `biggz-mcp`: permitted by the task; both binary actors were real builds from the actual toolchain.
- The CLI's own SQLite connection refreshes shm on open, so byte-identity is not a valid "untouched" assertion; branch identity is proven by the surviving `ghost_probe` marker + absent `bigmem_recovered` + absent warning.

## Size Exception (maintainer-approved)

Slice 1 is **676** authored changed lines (65+47+73+426+39+26) against the 400-line budget (Review Workload Forecast: `High` risk, `auto-chain`/`feature-branch-chain`). The human maintainer explicitly approved a `size:exception` for this slice; compressing `ghost_test.go` was rejected to keep reclaim-keeps-data and checkpoint-error-never-fails-Open coverage. Slices 2–4 keep the 400-line target.

## Rollback Boundary

Revert `internal/bigmem/ghost_test.go`, `internal/bigmem/liveness_windows.go`, `internal/bigmem/liveness_other.go`, the `classifyGhostWAL`/`reclaimStaleWAL`/`ghostReclaimCheckpoint` block and the `ResolveDBPath` switch in `internal/bigmem/bigmem.go`, and restore the removed `TestGhostWAL_Busy_OExcl_Preserved`/`TestGhostWAL_ProbeOExcl` tests in `internal/bigmem/bigmem_test.go` → back to shape-only + O_EXCL probe behavior (pre-slice HEAD). No other file depends on the new symbols; rollback is self-contained to these five files.

## Phase 1b — Cross-platform liveness honesty (verify findings W1/W2)

- **Scope:** tasks 1b.1–1b.5 (verify-report W1/W2). Off Windows the O_EXCL claim probe never opens `bigmem.db`, so O_EXCL success is NOT proof of death; the old mapping (`proven=true` → `ghostStale` → `reclaimStaleWAL`) let `os.Remove` delete a live holder's wal/shm, which succeeds on Unix even while the files are open — a direct REQ-GW2/GW3 violation. Windows `liveness_windows.go` (share-0) is untouched and correct as-is.
- **Files:** `internal/bigmem/liveness_other.go` (fix, ≈+12/−5), `internal/bigmem/bigmem.go` (+4/−1 seam, verified via `git diff --stat` minus the slice-1 record), `internal/bigmem/ghost_test.go` (≈+145; untracked file, so `git diff` cannot show a slice-separated stat — estimate from the edit blocks). Total ≈160 authored changed lines, slightly above the ~150 soft target; the W2 end-to-end test body is the bulk. Not a rewrite.

### The fix

- `liveness_other.go`: O_EXCL success now returns `(holder=false, proven=false)` (inconclusive); a failed probe stays inconclusive too. The probe itself is kept (it still detects a concurrent probe race). Signature unchanged. The doc comment states that this probe cannot observe a live holder, cites REQ-GW2/REQ-GW3, and records the tradeoff + POSIX follow-up.
- `bigmem.go`: `var ghostProbeDBLiveness = probeDBLiveness` (same style as `ghostReclaimCheckpoint`), used by `classifyGhostWAL` — a seam for deterministic tests, not a refactor.

### RED evidence (before the fix; GOOS=linux test binary executed in WSL)

```
$ GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o bigmem.test ./internal/bigmem
$ wsl --exec /tmp/bigmem.test -test.run 'TestProbeDBLiveness_Matrix|TestGhostWAL_NonWindows_LiveHolderKeepsFiles' -test.v -test.count=1
--- FAIL: TestGhostWAL_NonWindows_LiveHolderKeepsFiles (0.01s)
    ResolveDBPath = ".../001/bigmem.db", want recovered fallback ".../bigmem_recovered/bigmem.db" (inconclusive MUST NOT claim the primary)
    wal MUST be preserved while a live holder may own it: ... bigmem.db-wal: no such file or directory
    shm MUST be preserved while a live holder may own it: ... bigmem.db-shm: no such file or directory
--- FAIL: TestProbeDBLiveness_Matrix/no_holder
    probeDBLiveness = (false, true), want (false, false) off Windows (REQ-GW2/GW3)
```

With a live holder open, the old code classified `ghostStale` and reclaim deleted wal/shm under the holder — the exact W1 defect, reproduced.

**Existing tests whose expectations changed as part of RED** (both in `ghost_test.go`):
1. `TestProbeDBLiveness_Matrix/no holder proves dead` → renamed `no holder`, platform-aware: Windows wants `(false,true)`, off Windows wants `(false,false)`.
2. `TestClassifyGhostWAL_Matrix/stale idle is reclaimable` → new `wantOther: ghostInconclusive` field carries the non-Windows expectation (the stale shape can no longer be `ghostStale` off Windows).

### GREEN (after the fix)

```
$ go test ./internal/bigmem -run 'Ghost|Liveness|ReclaimCheckpoint' -count=1   # Windows: ok 1.832s
$ go test ./internal/bigmem -count=1 -timeout 180s                              # Windows: ok 22.592s
$ go vet ./internal/bigmem                                                      # exit 0
$ gofmt -l internal/bigmem                                                      # clean
$ wsl --exec /tmp/bigmem.test -test.run 'Ghost|Liveness|ReclaimCheckpoint'      # Linux: all PASS/SKIP except TestGhostWAL_Stale_Removed (off-surface residual below)
```

### New checked-in tests

- `TestGhostWAL_NonWindows_LiveHolderKeepsFiles` (W1 regression guard, runs on Linux/CI): live holder + ghost shape + non-Windows probe ⇒ recovered fallback returned, wal/shm preserved (no removal).
- `TestGhostWAL_Inconclusive_RecoveredFallback` (W2 closure): seam forces inconclusive; the recovered DB is built the way a previous inconclusive resolve builds it (`safeCopyDB`); asserts (a) stderr contains `ghost WAL/SHM detected`, (b) `ResolveDBPath` returns the recovered path, (c) wal/shm + the planted `.ghost_probe` marker survive, (d) the seeded row is readable through the recovered store. The primary is switched to rollback-journal mode so SQLite's own WAL hygiene on the `mergeDB` read cannot be mistaken for the code under test (reclaim would still `os.Remove` the files, which the planted marker flags).
- `forceProbeSeam` helper + seam-forcing in `TestGhostWAL_ReclaimKeepsData` / `TestReclaimCheckpointErrorNeverFailsOpen` so the reclaim assertions stay meaningful on every platform now that non-Windows reclaiming is unreachable by design.

### Recorded tradeoff (deliberate — do NOT "fix")

Off Windows a genuinely-dead stale ghost no longer reclaims; it takes the recovered fallback (warning + `bigmem_recovered` merge/promote). Safety over aggressiveness: the alternative deletes a possibly-live holder's files. Follow-up OUT OF SCOPE: a POSIX advisory-lock probe (fcntl on SQLite's lock bytes) would restore non-Windows reclaiming; not implemented here.

### Off-surface residual — RESOLVED (seam, not platform gate)

`internal/bigmem/bigmem_test.go > TestGhostWAL_Stale_Removed` now forces the probe to `(holder=false, proven=true)` via the existing `forceProbeSeam` helper in `ghost_test.go` (same package, same test binary — no duplication), so the REQ-GW2 reclaim path runs deterministically on EVERY platform. Every original assertion is kept (stale wal/shm removed, `ResolveDBPath` returns the primary, primary writable via `Open` + `Save`); the seam is restored through the helper's `t.Cleanup`.

- **Why the seam beats a platform gate:** a `runtime.GOOS != "windows" { t.Skip }` gate would turn a cross-platform reclaim guarantee into a Windows-only assertion and silently drop reclaim coverage from Linux/CI. The seam keeps the test about WHAT the reclaim does (REQ-GW2), not about what the platform probe can prove; the probe itself already has per-OS expectations in `TestProbeDBLiveness_Matrix` / `TestClassifyGhostWAL_Matrix`.
- **Cross-platform audit (inspection, new `liveness_other.go` semantics = O_EXCL success ⇒ `(false, false)`):** every ghost-related test in the package either skips by design on the foreign OS (`TestGhostWAL_LiveHolder_UsesPrimary`, `TestGhostWAL_LeftoverProbe_ReclaimWhenDead` and the `live holder` subtests — Windows-only share-0), uses the seam (`TestGhostWAL_Inconclusive_RecoveredFallback`, `TestGhostWAL_ReclaimKeepsData`, `TestReclaimCheckpointErrorNeverFailsOpen`, now `TestGhostWAL_Stale_Removed`), encodes the non-Windows expectation explicitly (`TestGhostWAL_NonWindows_LiveHolderKeepsFiles`, `TestProbeDBLiveness_Matrix/no holder`, `TestClassifyGhostWAL_Matrix/stale idle is reclaimable` via `wantOther: ghostInconclusive`), or never consults the probe at all — the shape/freshness gate returns first (`TestIsGhostWAL`, `TestIsGhostWAL_Robust`, `TestGhostWAL_Fresh_Kept`, `TestGhostWAL_SaveSearch_Checkpoint`, `TestGhostWAL_WALBounded`, `TestResolveDBPath_MergeByMaxUpdatedAt` in `doctor_test.go`). **No remaining test depends on the old non-Windows `proven=true` semantics.**
- **Evidence (this run, Windows):** `go test ./internal/bigmem -run 'Ghost|Liveness|ReclaimCheckpoint' -count=1` → `ok ... 1.572s`; `go test ./internal/bigmem -count=1 -timeout 180s` → `ok ... 16.256s`; `go test ./internal/bigmem -run TestGhostWAL_Stale_Removed -count=1 -v` → `--- PASS: TestGhostWAL_Stale_Removed (0.19s)`; `go vet ./internal/bigmem` → exit 0; `gofmt -l internal/bigmem` → clean (no output).

## Remaining Tasks

Phase 1b residual: RESOLVED — `internal/bigmem/bigmem_test.go > TestGhostWAL_Stale_Removed` now forces `(holder=false, proven=true)` through the `forceProbeSeam` helper (`ghost_test.go`), keeping the REQ-GW2 reclaim assertions intact and platform-independent (see the Phase 1b section for the cross-platform audit + evidence). No residual remains in Phase 1b.

Phases 2–5 (not started, out of this run's scope):

- Phase 2 (PR 2): searchable close — tasks 2.1–2.4
- Phase 3 (PR 3): FTS sanitization — tasks 3.1–3.3
- Phase 4 (PR 4): summary read + recall discipline — tasks 4.1–4.3
- Phase 5: verification — tasks 5.1–5.2

## Notes

- Prior interrupted-run ledger facts preserved: outcome `interrupted` omitted `evidence_revision`; interrupted attempts do not consume budget (`remaining_attempts=2`); ledger revision `bf3cf0dcb1d1004c56dfcf0ba2d220601f0006264dcbae41a74d737f406f5e9e`.
- No commit/push/branch changes; tree dirty for human review. Harness artifacts live under `/tmp/bm-harness/` (outside the repo).
