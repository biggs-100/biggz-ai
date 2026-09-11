# Apply Progress — fix-bigmem-recall-friction

- **Change:** fix-bigmem-recall-friction
- **Slice:** 1 + Phase 1b (Phase 1 — Ghost-WAL liveness, PR 1 of the feature-branch-chain); Phase 2 — Searchable close (PR 2 of the chain; see the Phase 2 section); Phase 3 — FTS sanitization (PR 3 of the chain; see the Phase 3 section)
- **Status:** complete (tasks 1.1–1.4) — runtime harness evidence + bookkeeping finished in this continuation run (the previous apply run hit the 20-minute task-mode timeout after the code was green, before harness evidence and bookkeeping). Phase 1b (tasks 1b.1–1b.5) added in a later continuation run; see the Phase 1b section for evidence; its one off-surface residual (`TestGhostWAL_Stale_Removed`) was resolved in a follow-up run via the liveness seam — no residual remains. Phase 2 (tasks 2.1–2.4) complete in a later continuation run — RED, implementation, exactly-once runtime harness and rollback evidence in the Phase 2 section.
- **Mode:** Standard (`strict_tdd: false` for this project; no Strict-TDD cycle table applies)
- **Branch / base:** `fix/bigmem-recall-friction-1-ghost-wal` (tracker: `fix/bigmem-recall-friction`, base `bed2d881`) — uncommitted, left dirty for human review
- **Phase 2 branch / base:** `fix/bigmem-recall-friction-2-searchable-close`, base = PR 1 branch at `5d2d9871` — uncommitted, left dirty for human review
- **Phase 3 branch / base:** `fix/bigmem-recall-friction-3-fts-sanitize`, base = PR 2 branch at `a381065e` — uncommitted, left dirty for human review
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

## Phase 2 — Searchable close (REQ-SC1)

- **Scope:** tasks 2.1–2.4 (defect #1: MCP `mem_session_summary`/`SessionEnd` wrote only the `sessions` row and created no observation, so a closed session was not searchable in recall; the bash fallback did produce a searchable observation — that asymmetry was the bug).
- **Files:** table below. Authored changed lines: 152 tracked (+135/−17) + 182 new = **334** (under the 400 budget; PR 1's `size:exception` does NOT extend here).
- **Mode:** Standard (`strict_tdd: false`); RDD enabled — every PASS claimed below is raw output from this run.

### Implementation

| File | +/- | Purpose |
|------|-----|---------|
| `internal/bigmem/bigmem.go` | +24/−6 | `SessionSummaryObsID(sessionID)` = `"session-summary-" + TrimSpace(id)`; `SaveCtx` routes `type==session_summary && session_id!=""` to that deterministic PK (upsert `ON CONFLICT(id) DO UPDATE`) and bypasses the topic_key + hash-window dedup phases so two sessions with identical text never merge into one row |
| `internal/bigmem/full.go` | +39/−3 | `SessionEnd` dual-write: `sessions` UPDATE commits first, then the observation upsert via the `sessionSummaryUpsert` seam (mirrors `ghostProbeDBLiveness`), retry once after `sessionSummaryRetryDelay` = 50 ms; persistent failure returns an explicit error while the `sessions` row stays intact. Store lock released before the upsert (SaveCtx takes it) — no deadlock |
| `internal/sdd/session_guard.go` | +11/−6 | `tryMCPSave` drops its duplicate `store.SaveCtx` (SessionEnd now dual-writes); `saveViaBash` gains `sessionID` and appends `--session-id <id>` when non-empty |
| `cmd/biggz/cli_bigmem.go` | +9/−2 | `save` parses `--session-id` (skips the active-session fallback) + usage strings updated |
| `internal/bigmem/session_close_test.go` | +182 (new) | 4 tests: dual-write exactly-once + recency findability; same-content two sessions stay separate; retry-once/seam failure keeps `sessions` row; SaveCtx deterministic routing |
| `internal/sdd/session_guard_test.go` | +11 | bash-fallback args assert `--session-id sess-1` rides the command |
| `cmd/biggz/cli_bigmem_test.go` | +41 | `TestBigmemSave_SessionIDRoutesToUpsert`: double CLI close → same printed id + exactly 1 row via `Search("")`, content updated in place |

Why the phase-2 bypass matters: without it the hash-window dedup would match two different sessions' identical summaries (`normalized_hash + project + scope + type + title` within the window) and update one row — leaving the second close unsearchable. `TestSessionEnd_SameContentTwoSessionsStaySeparate` is the guard.

### RED evidence (before the fix)

```
$ go test ./internal/bigmem -run 'TestSessionEnd' -count=1 -v
=== RUN   TestSessionEnd_DualWriteExactlyOnce
    session_close_test.go:22: after first close: session_summary rows for sess-close-1 = 0
    session_close_test.go:24: after first close want exactly 1 session_summary row, got 0
--- FAIL: TestSessionEnd_DualWriteExactlyOnce (0.08s)
=== RUN   TestSessionEnd_SameContentTwoSessionsStaySeparate
    session_close_test.go:80: session sess-a must have exactly 1 session_summary, got 0
--- FAIL: TestSessionEnd_SameContentTwoSessionsStaySeparate (0.03s)
FAIL
FAIL	github.com/biggs-100/biggz-ai/internal/bigmem	0.784s
```

### Runtime harness — exactly-once through the store API (raw `-v` output)

```
$ go test ./internal/bigmem -run 'TestSessionEnd_DualWriteExactlyOnce|TestSessionEnd_ObservationWriteRetriesOnceThenFailsVisibly|TestSessionEnd_SameContent|TestSaveCtx_SessionSummary' -count=1 -v
--- PASS: TestSessionEnd_DualWriteExactlyOnce (0.12s)
    session_close_test.go:25: after first close: session_summary rows for sess-close-1 = 1
    session_close_test.go:35: recall: id=session-summary-sess-close-1 type=session_summary session_id=sess-close-1 updated_at=2026-09-11T05:19:08Z content="summary v1"
    session_close_test.go:50: after repeat close: session_summary rows for sess-close-1 = 1 (content=summary v2)
--- PASS: TestSessionEnd_SameContentTwoSessionsStaySeparate (0.08s)
--- PASS: TestSessionEnd_ObservationWriteRetriesOnceThenFailsVisibly (0.08s)
--- PASS: TestSaveCtx_SessionSummaryRoutesToDeterministicID (0.10s)
PASS
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	1.036s
```

### Runtime harness — cross-process CLI (`go run` from this working tree, temp `USERPROFILE`; the real `~/.biggz/bigmem` was never opened)

```
$ export USERPROFILE=<os-temp-home> HOME=<os-temp-home>
$ go run ./cmd/biggz bigmem save "Session summary" "cli close v1" --type session_summary --scope project --project biggz-ai --session-id sess-cli-rt
Saved: session-summary-sess-cli-rt
$ go run ./cmd/biggz bigmem save "Session summary" "cli close v2" --type session_summary --scope project --project biggz-ai --session-id sess-cli-rt
Saved: session-summary-sess-cli-rt
$ go run ./cmd/biggz bigmem recent --type session_summary --json
[
  {
    "id": "session-summary-sess-cli-rt",
    "title": "Session summary",
    "type": "session_summary",
    "content": "cli close v2",
    "session_id": "sess-cli-rt",
    "project": "biggz-ai",
    "scope": "project",
    "revision_count": 2,
    "duplicate_count": 2,
    "last_seen_at": "...",
    "created_at": "...",
    "updated_at": "..."
  }
]
```

One row, updated in place (`revision_count: 2`), searchable via empty-query recency. The installed `biggz` on PATH is an OLD build and is NOT cited as evidence anywhere in this section.

### Commands + results (this run, after all edits)

```
$ go test ./internal/bigmem ./internal/sdd ./cmd/biggz -run 'Session|Close' -count=1
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	3.040s
ok  	github.com/biggs-100/biggz-ai/internal/sdd	4.107s
ok  	github.com/biggs-100/biggz-ai/cmd/biggz	0.602s
$ go test ./internal/bigmem ./internal/sdd ./cmd/biggz -count=1 -timeout 180s
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	27.163s
ok  	github.com/biggs-100/biggz-ai/internal/sdd	24.403s
ok  	github.com/biggs-100/biggz-ai/cmd/biggz	67.144s
$ go vet ./internal/bigmem ./internal/sdd ./cmd/biggz     # exit 0, no output
$ gofmt -l internal/bigmem internal/sdd cmd/biggz         # no output (clean)
$ go build ./...                                          # BUILD_OK
```

### Notes / follow-ups

- MCP `mem_session_summary`/`mem_session_end` call `store.SessionEnd`, so the dual-write reaches the MCP path without touching `cmd/biggz-mcp/main.go` (out of this slice's surface); their existing `writeError` path now surfaces a persistent observation failure explicitly.
- Consequence of the documented routing: any `session_summary` save with a non-empty session id upserts per session — repeated CLI manual summaries attached to the same session (including the `manual-save-<project>` pseudo-session) update one row instead of stacking rows. This matches design D2 ("exactly-once per session id") and the REQ-SC1 idempotency scenario; recorded here because it is a behavior change for repeated manual closes.
- `SessionSummaryObsID` runs only for `TrimSpace(session_id) != ""`; a session id with surrounding whitespace keys off the trimmed id while the stored `session_id` column keeps the raw value (pre-existing normalization behavior, no change made).
- `TestSessionGuard_MCPUsesMCP` still proves the MCP path (SessionEnd → HasSessionSummary true) with the duplicate Save gone; `TestSessionGuard_RetrySucceeds` still sees exactly 2 `bigmemOpen` calls (guard-level retry unchanged).

### Rollback boundary (Phase 2)

Revert `internal/bigmem/bigmem.go` (routing + dedup bypass + `SessionSummaryObsID`), `internal/bigmem/full.go` (`SessionEnd` dual-write + seam + retry), `internal/sdd/session_guard.go` (drop duplicate save + `--session-id`), `cmd/biggz/cli_bigmem.go` (`--session-id` flag) and delete `internal/bigmem/session_close_test.go`; revert the two test additions in `internal/sdd/session_guard_test.go` / `cmd/biggz/cli_bigmem_test.go`. → back to `SessionEnd` writing only the `sessions` row (PR 1 state at `5d2d9871`). Self-contained: no other file depends on the new symbols; slice 1 is untouched.

## Phase 3 — FTS sanitization (REQ-FTS1)

- **Scope:** tasks 3.1–3.3 (defect #3: any-mode multi-token queries with hyphens were parsed as FTS syntax — `gentle-pi OR ruido` made FTS error with `no such column: pi`, the LIKE fallback compared the whole raw string and the search returned zero silently).
- **Files:** table below. Authored changed lines: 193 tracked (+193/−19) + 171 new = **383** (under the 400 budget; slice 1's `size:exception` does NOT extend here). SDD ledger updates (tasks.md + this section) stay outside the authored count per the convention recorded in slice 2.
- **Mode:** Standard (`strict_tdd: false`); RDD enabled — every PASS below is raw output from this run.

### Implementation

| File | +/- | Purpose |
|------|-----|---------|
| `internal/bigmem/bigmem.go` | +53/−19 | `sanitizeFTSTerms(query, mode) (fts string, hasTokens bool)`: per token strip `"`, drop tokens without a letter or digit, wrap the rest in `"..."`, join `AND` (all/default) or `OR` (any); `hasFTSAlnum` helper; `SearchCtx` FTS branch guards on `hasTokens` — punctuation-only queries go straight to LIKE so zero stays an explicit empty result, never a MATCH parse error |
| `internal/bigmem/fts_sanitize_test.go` | +171 (new) | `TestSanitizeFTSTerms` (12-case table) + 5 integration tests: any-mode hyphen hits, all-mode AND requires both tokens, explicit zero (nil error), accent fold (`sesión`/`sesion`), ordering preserved (rank vs `updated_at DESC`) |
| `cmd/biggz-mcp/main.go` | +10 | `mem_search` zero envelope: `{"results":[],"zero_results":true}` + `hint` (retry `match_mode=any`) on all/default-mode zero only |
| `cmd/biggz-mcp/main_test.go` | +72 | `TestMemSearch_ZeroEnvelope` (3 subtests) + `resultText` helper to unescape the JSON text block |
| `cmd/biggz/cli_bigmem.go` | +4 | `search` zero path prints the retry hint (`Retry with --match-mode any`) when mode != any and query non-empty |
| `cmd/biggz/cli_bigmem_test.go` | +54 | `TestBigmemSearch_ZeroHintAndHyphenHit` (3 subtests: all-zero hint, any-zero silent, hyphenated hits) |

Deliberate, documented nuances (verify should read these):

1. **"letterless" implemented as "no letter and no digit".** Design D4 says "drop letterless tokens"; `2026`-style digit tokens are letterless but unicode61 tokenizes digits into real FTS tokens (probed: quoted `"123"`/`"2026"` parse fine, `"2026-09-11"` matches its note). Dropping them would move digit-only queries off rank-ordered FTS → REQ-RR2 regression. The design's cited letterless example (`*`) still drops. Quoting is the actual hyphen/operator fix; dropping is for tokens that tokenize away (`***`), which would only zero an AND expression.
2. **Zero-result signal placement.** Design Interfaces list only `sanitizeFTSTerms` for `bigmem.go`, so no new store API was added: the store delivers an explicit empty result set with nil error (no MATCH parse error hidden behind the LIKE fallback), and the surfaces emit the signal (`zero_results` in the MCP envelope; `No results.` + hint on the CLI). The CLI hint rides **stdout** (primary response channel, mirrors the in-band MCP hint); the pre-existing project hint stays on stderr untouched.

### RED evidence (before the fix)

```
$ go test ./internal/bigmem -run 'TestSearch_FTS' -count=1 -v
=== RUN   TestSearch_FTS_HyphenAnyModeHits
    fts_sanitize_test.go:45: any-mode "gentle-pi ruido" must return "Hyphen note", got map[]
    fts_sanitize_test.go:45: any-mode "gentle-pi ruido" must return "Ruido note", got map[]
    fts_sanitize_test.go:45: any-mode "gentle-pi ruido" must return "Combo note", got map[]
--- FAIL: TestSearch_FTS_HyphenAnyModeHits (0.08s)
```

Root-cause probe (raw FTS5, before the fix): `MATCH 'gentle-pi'` → `SQL logic error: no such column: pi`; `MATCH '"gentle-pi"'` → 1 row; `MATCH 'gentle-pi OR ruido'` (old any-mode expression) → same error; `MATCH '"gentle-pi" OR "ruido"'` → 2 rows. Accent check: `sesion` and `"sesión"` both matched the `sesión` note (unicode61 folding verified, not assumed).

### Focused test command + result (this run)

```
$ go test ./internal/bigmem ./cmd/biggz-mcp ./cmd/biggz -run 'FTS|Match|Search' -count=1
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	2.930s
ok  	github.com/biggs-100/biggz-ai/cmd/biggz-mcp	0.755s
ok  	github.com/biggs-100/biggz-ai/cmd/biggz	0.306s
```

Verbose: `TestSanitizeFTSTerms`, `TestSearch_FTS_*` (5), `TestMemSearch_ZeroEnvelope`, `TestBigmemSearch_ZeroHintAndHyphenHit` all PASS.

### Package suite command + result (this run)

```
$ go test ./internal/bigmem ./internal/sdd ./cmd/biggz ./cmd/biggz-mcp -count=1 -timeout 240s
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	28.877s
ok  	github.com/biggs-100/biggz-ai/internal/sdd	23.511s
ok  	github.com/biggs-100/biggz-ai/cmd/biggz	63.214s
ok  	github.com/biggs-100/biggz-ai/cmd/biggz-mcp	3.832s
```

### Static checks

```
$ go vet ./internal/bigmem ./cmd/biggz-mcp ./cmd/biggz   # exit 0, no output
$ gofmt -l internal/bigmem cmd/biggz-mcp cmd/biggz       # no output (clean)
```

### Runtime harness evidence

Fixed binaries built from this working tree into `C:\Users\USER\AppData\Local\Temp\tmp.lUasNDQSRS\bin`; `USERPROFILE`/`HOME` pointed at the temp home `...\tmp.lUasNDQSRS\home`. The real store `C:\Users\USER\.biggz\bigmem` was never referenced by any harness command.

```
$ go build -o $H/bin/biggz-fixed.exe ./cmd/biggz          # exit 0
$ go build -o $H/bin/biggz-mcp-fixed.exe ./cmd/biggz-mcp  # exit 0
$ biggz-fixed.exe bigmem save "Hyphen seed" "marcador gentle-pi unico" --type note --scope project --project probe
Saved: obs-1789137349277529000-1                          # EXIT=0
$ biggz-fixed.exe bigmem save "Accent seed" "la sesión de cierre" --type note --scope project --project probe
Saved: obs-1789137349366100700-1                          # EXIT=0
```

CLI any-mode hyphen `gentle-pi ruido` (the defect query):

```
$ biggz-fixed.exe bigmem search "gentle-pi ruido" --match-mode any --project probe
  obs-1789137349277529000-1 [note] Hyphen seed (0s)       # EXIT=0
```

CLI accent `sesion` (all default) → `obs-1789137349366100700-1 [note] Accent seed`, EXIT=0.

CLI all-mode zero (`gentle-pi qqq-inexistente`), stdout/stderr split:

```
--stdout--
No results.
No all-mode matches. Retry with --match-mode any to broaden the search.
--stderr--
No results for "gentle-pi qqq-inexistente" in project "probe". Try --all or --project biggz-ai.
```

CLI any-mode zero → stdout `No results.` only (no retry hint), EXIT=0.

MCP stdio (`tools/call` piped; no handshake required):

```
> mem_search {"query":"gentle-pi qqq-inexistente","project":"probe"}
{"result":{"content":[{"text":"{\"hint\":\"No matches in match_mode=all. Retry with match_mode=any to broaden the search.\",\"results\":[],\"zero_results\":true}","type":"text"}]}}

> mem_search {"query":"qqq-inexistente zzz-inexistente","match_mode":"any","project":"probe"}
{"result":{"content":[{"text":"{\"results\":[],\"zero_results\":true}","type":"text"}]}}

> mem_search {"query":"gentle-pi ruido","match_mode":"any","project":"probe"}
... "title":"Hyphen seed" ...                            # hit present, EXIT=0
```

Negative control — OLD installed `biggz` (PATH build), same temp store, same defect query:

```
$ biggz bigmem search "gentle-pi ruido" --match-mode any --project probe
No results.                                               # defect reproduced on the old build
```

### Rollback boundary (Phase 3)

Revert `internal/bigmem/bigmem.go` (`sanitizeFTSTerms`/`hasFTSAlnum` + the `SearchCtx` FTS-branch guard), `cmd/biggz-mcp/main.go` (zero envelope block), `cmd/biggz/cli_bigmem.go` (retry-hint line), and **delete** `internal/bigmem/fts_sanitize_test.go`; revert the two test additions in `cmd/biggz-mcp/main_test.go` (`TestMemSearch_ZeroEnvelope` + `resultText`) and `cmd/biggz/cli_bigmem_test.go` (`TestBigmemSearch_ZeroHintAndHyphenHit`). → back to raw MATCH expressions (any-mode hyphen miss) and the plain `[]` search response at `a381065e` (PR 2 state). Self-contained: no other file depends on the new symbols; slices 1–2 files are untouched by this slice.

## Remaining Tasks

Phase 1b residual: RESOLVED — `internal/bigmem/bigmem_test.go > TestGhostWAL_Stale_Removed` now forces `(holder=false, proven=true)` through the `forceProbeSeam` helper (`ghost_test.go`), keeping the REQ-GW2 reclaim assertions intact and platform-independent (see the Phase 1b section for the cross-platform audit + evidence). No residual remains in Phase 1b.

Phases 4–5 (not started, out of this run's scope):

- Phase 4 (PR 4): summary read + recall discipline — tasks 4.1–4.3
- Phase 5: verification — tasks 5.1–5.2

## Notes

- Prior interrupted-run ledger facts preserved: outcome `interrupted` omitted `evidence_revision`; interrupted attempts do not consume budget (`remaining_attempts=2`); ledger revision `bf3cf0dcb1d1004c56dfcf0ba2d220601f0006264dcbae41a74d737f406f5e9e`.
- No commit/push/branch changes; tree dirty for human review. Harness artifacts live under `/tmp/bm-harness/` (outside the repo).
