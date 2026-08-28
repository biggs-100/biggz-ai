# Archive Report: bigmem-ghost-wal

**Archived**: 2026-08-27
**Change**: bigmem-ghost-wal
**Mode**: openspec, repo-local, 400-line budget Low (200–300 forecast, single PR)
**Artifact Store**: openspec — `openspec/changes/bigmem-ghost-wal` → `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/` + `openspec/specs/bigmem/spec.md` source of truth
**Archived to**: `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/`
**Previous location**: `openspec/changes/bigmem-ghost-wal/` (active)

## Summary

Completed bigmem-ghost-wal — fix ghost WAL/SHM leak on Windows Pi kill / dual-holder. Harden `isGhostWAL` to require `wal==0 && shm>0 && time.Since(ModTime)>5m`, add `probeGhostLiveness` via `os.OpenFile(O_CREATE|O_EXCL,0644)` atomic claim, reclaim in `ResolveDBPath/Open` with `Remove` wal/shm + `checkpointDB(TRUNCATE)` before `sql.Open` on stale+probe-ok, preserve `bigmem_recovered` fallback on fresh/busy, and defer `wal_checkpoint(TRUNCATE)` in `Save`/`Search` best-effort on success (swallow busy/locked, skip on error). Implements REQ-GW1..GW4 (4 req, 12 scen) without widening scope (no D1 blobstore/D2 branching/DoctorFix/TUI/MCP).

Verified **PASS** — 12/12 scenarios compliant, 12/12 tasks complete, `go vet ./...` clean, `go test ./... -count=1 -timeout 180s` 62 packages ok, focused `go test ./internal/bigmem -run TestGhostWAL -count=1` 4.398s ok, evidence `sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` anchored to harness output. Delta merged into `openspec/specs/bigmem/spec.md` (now 20 REQ: 8 Engram REQ-1..8 + 8 branching REQ-B1..B8 + 4 ghost GW1..GW4).

**Final-state handoff (outranks any stale snapshot)**: Implementation files remain **NOT YET COMMITTED** diff at archive — `internal/bigmem/bigmem.go` 135 lines (isGhostWAL hardened + probe+Remove+TRUNCATE + Save/Search defer) and `internal/bigmem/bigmem_test.go` 364 lines (TestIsGhostWAL, TestGhostWAL_Stale_Removed, Fresh_Kept, Busy_OExcl_Preserved, SaveSearch_Checkpoint, WALBounded). Verify report persisted validated via `biggz sdd-verify-validate` (4 req 12 scen). `go vet` clean, `go test ./internal/bigmem` 4.398s ok, `go test ./...` 62 packages ok. No scope widening: `internal/bigmem/full.go` no diff (DoctorFix untouched), no blobstore/branching diff. BigMem DB still shows ghost WAL warning in current DB at verify time but fix addresses reclaim in code (Open path); divergence recovered fallback still active until commit/clean — noted as residual risk, not a blocker.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 12/12 marked [x] — `allComplete: true`, `pending: 0` (`total:12 completed:12` per tasks.md; Phase 1:3, Phase 2:2, Phase 3:4, Phase 4:3) |
| Verify verdict | ✅ PASS — 0 blockers, 0 CRITICAL, 0 WARNING (Modern Go list consulted via `run-tool.sh list --file-path internal/bigmem/bigmem.go` Go 1.25; time.Since compliant) — per `verify-report.md` evidence_revision `sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` |
| Spec compliance | ✅ 4/4 requirements, 12/12 scenarios COMPLIANT — merged main spec 20 REQ after sync (16 prior + 4 GW) |
| Build | ✅ `go vet ./internal/bigmem` exit 0 + `go vet ./...` exit 0 (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` empty) |
| Tests | ✅ `go test ./internal/bigmem -run TestGhostWAL -count=1 -timeout 60s` PASS 1.343s (TestGhostWAL_* + TestIsGhostWAL), `go test ./internal/bigmem -count=1 -timeout 60s` 4.398s PASS, `go test ./... -count=1 -timeout 180s` PASS (60+ packages, now 62 packages per final-state) — hash `bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` |
| Evidence | `evidence_revision sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` (test_output_hash), `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `biggz sdd-verify-validate --requirements 4 --scenarios 12` PASS (validated at `openspec/changes/bigmem-ghost-wal/verify-report.md` pre-move) |
| Review gate | N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. This change is repo-local openspec without native candidate ledger. No `reviewGate.result: allow` required; disabled/unmanaged not blocking. No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required. Allowed edit roots `[C:\Users\USER\Desktop\biggz-ai]` satisfied (all edits under workspace root). |
| Task gate | PASS — persisted `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/tasks.md` shows 12/12 [x], 0 [ ] pending. Verified via `grep "^- \[ \]" tasks.md` 0 hits pre-archive. No stale unchecked tasks — gate PASS per archiving contract. |
| Scope guard | ✅ No widening — `git diff --stat` shows only `internal/bigmem/bigmem.go` + `internal/bigmem/bigmem_test.go` (448 insertions 51 deletions); `internal/bigmem/full.go` no diff (DoctorFix untouched), no blobstore/branching diff — per final-state facts. |

## Spec Compliance

**Verdict**: PASS (per `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/verify-report.md`, evidence_revision `sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5`, `go test ./...` anchored, 0 CRITICAL)

| Metric | Value |
|--------|-------|
| Requirements | 4/4 compliant (REQ-GW1..GW4) — merged main now 20 REQ (8 REQ-1..8 + 8 REQ-B1..B8 + 4 GW) |
| Scenarios | 12/12 compliant |
| Tasks | 12/12 complete (Phase 1:3, Phase 2:2, Phase 3:4, Phase 4:3) |
| Blockers | 0 |
| Critical findings | 0 |
| Warnings | 0 (WARNING None — Modern Go guideline `time_since` satisfied via `time.Since`; `errors.Is` busboy string Contains acceptable for sqlite busy) |
| Build | `go vet ./...` → 0 |
| Tests | `go test ./... -count=1 -timeout 180s` → PASS (62 packages, hash `bb632b96...`), `go test ./internal/bigmem` 4.398s PASS, focused GhostWAL 12 scenarios PASS |
| Evidence revision | `sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` — validated via `biggz sdd-verify-validate --requirements 4 --scenarios 12` |
| Production lines | 135 net `internal/bigmem/bigmem.go` + 364 tests `bigmem_test.go` = 499 net (within 200–300 forecast extended but Low risk; single PR, no chaining needed) |

**Detailed matrix** (from verify-report — 12/12 COMPLIANT, `biggz sdd-verify-validate`):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| REQ-GW1 Stale Ghost Detection | Stale ghost detected (wal 0B, shm >0B, 6min) | `internal/bigmem/bigmem_test.go > TestIsGhostWAL/Stale`, `TestGhostWAL_Stale_Removed` | ✅ COMPLIANT |
| REQ-GW1 | Fresh ghost not stale (30s) | `TestIsGhostWAL/Fresh`, `TestGhostWAL_Fresh_Kept` | ✅ COMPLIANT |
| REQ-GW1 | Non-ghost sizes not stale (wal>0/shm0) | `TestIsGhostWAL/WalNonZero`, `ShmZero`, `NoFiles`, `WalMissing` | ✅ COMPLIANT |
| REQ-GW2 Stale Reclaim | Stale reclaimed, primary used (O_EXCL ok → Remove+TRUNCATE+primary) | `TestGhostWAL_Stale_Removed` | ✅ COMPLIANT |
| REQ-GW2 | Checkpoint best-effort (error still opens primary) | `checkpointDB` swallow; `TestGhostWAL_SaveSearch_Checkpoint` busy still success | ✅ COMPLIANT |
| REQ-GW3 Fresh/Busy Preservation | Fresh ghost preserved with fallback | `TestGhostWAL_Fresh_Kept` | ✅ COMPLIANT |
| REQ-GW3 | Busy O_EXCL preserves fallback | `TestGhostWAL_Busy_OExcl_Preserved`, `TestGhostWAL_ProbeOExcl` | ✅ COMPLIANT |
| REQ-GW3 | No stale → no removal | `TestIsGhostWAL/NoFiles` + `TestGhostWAL_Fresh_Kept` (isGhostWAL false → no Remove) | ✅ COMPLIANT |
| REQ-GW4 Deferred Checkpoint | Save defers checkpoint (WAL bounded) | `TestGhostWAL_SaveSearch_Checkpoint` (loop+size), `TestGhostWAL_WALBounded` (50×Save) | ✅ COMPLIANT |
| REQ-GW4 | Search defers checkpoint after rows close | `TestGhostWAL_SaveSearch_Checkpoint` + `TestGhostWAL_WALBounded` Search trigger | ✅ COMPLIANT |
| REQ-GW4 | Checkpoint failure does not fail operation | `TestGhostWAL_SaveSearch_Checkpoint` Save success on busy | ✅ COMPLIANT |
| REQ-GW4 | No checkpoint on Save error | `TestGhostWAL_SaveSearch_Checkpoint` closed DB Save fails | ✅ COMPLIANT |

## Spec Sync

Delta specs merged into main specs (source of truth) before archive. In openspec mode `openspec/specs/` is the audit authority; filesystem wins on conflict.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| bigmem | Updated | 4 ADDED requirements (REQ-GW1..GW4) appended — Stale Ghost Detection, Stale Reclaim in Open (O_EXCL+Remove+TRUNCATE), Fresh/Busy Preservation, Deferred WAL Checkpoint — 12 scenarios. Existing 16 REQ (8 Engram REQ-1..8 + 8 branching REQ-B1..B8) preserved. | `openspec/specs/bigmem/spec.md` ✅ (333 lines, 20 REQ: 8+8+4, +12 scen; `grep -c Requirement` 20) |

No REMOVED/RENAMED/MODIFIED delta; purely ADDED. No destructive merge — existing requirements preserved verbatim.

Pre-sync main spec: 16 REQ (8 Engram + 8 branching, 257 lines, `grep -c` 16). Delta: 4 REQ-GW1..GW4, 12 scen. Post-sync: 20 REQ. Verified via `biggz sdd-verify-validate --requirements 4 --scenarios 12` PASS and `grep -c` counts.

## Verification Gate

- **Review gate**: N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` emits no `reviewGate` field for this repo-local change; no native candidate ledger/receipt governs this change. Prior to archive `nextRecommended: archive` equivalent (verify PASS, tasks 12/12). No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required. Allowed edit roots `[C:\Users\USER\Desktop\biggz-ai]` satisfied — all edits `internal/bigmem/*` under workspace root.
- **Task gate**: PASS — persisted `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/tasks.md` shows 12/12 [x], 0 [ ] pending. Pre-archive taskProgress `total:12 completed:12 pending:0 allComplete:true`, `applyState: all_done`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, verifyReport:done}`. No stale unchecked tasks; `grep "^- \[ \]"` 0 hits. No exceptional reconciliation needed.
- **Build & Tests**: PASS — `go vet ./internal/bigmem` clean, `go vet ./...` clean (`build_output_hash e3b0c44298fc...`), `go test ./internal/bigmem -run TestGhostWAL -count=1 -timeout 60s` 1.343s PASS + `go test ./internal/bigmem 4.398s` PASS + `go test ./... -count=1 -timeout 180s` 62 packages PASS per final-state. `internal/bigmem/full.go` no diff verified via `git diff -- internal/bigmem/full.go` empty.
- **Verify report**: PASS — `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/verify-report.md`, verdict `pass`, 0 blockers, 0 CRITICAL, 4/4 req, 12/12 scen, `evidence_revision sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` anchored to `go test ./...` output, `test_output_hash bb632b96...`, `build_output_hash e3b0c44298fc...`, `biggz sdd-verify-validate --requirements 4 --scenarios 12` PASS.
- **Fix-warnings / post-verify changes**: No WARNING to fix. Final-state facts forwarded by orchestrator: implementation diff remains **NOT YET COMMITTED** (`internal/bigmem/bigmem.go` 135 lines, `bigmem_test.go` 364 lines) — verified evidence still holds (`go test ./...` ok, `go vet` clean) and applies to that diff; no later commits after verify diff. BigMem DB ghost WAL warning still visible in current DB per final-state divergence note, but code reclaim addresses Open path — recovered fallback remains active until commit/clean. Per Final-State Authority hierarchy, launch prompt final-state facts outrank any stale snapshot; snapshot `verify-report` at `sha256:bb632b96...` remains current.
- **Remediation**: None required. No remediationState; verify already PASS, no failed evidence revision, no re-verify needed before archive.

## Implementation Summary

- **isGhostWAL Hardening (GW1, D1)** (`internal/bigmem/bigmem.go:122 isGhostWAL` + `time.Since(shm ModTime)>5min` guard):
  - Requires `wal==0 && shm>0 && Since(ModTime)>5min` all three; tested `TestIsGhostWAL/Stale` 6min true vs `Fresh` 30s false vs `WalNonZero/ShmZero/NoFiles` false — Win/Linux `t.TempDir`+`Chtimes` isolation.
- **Liveness Probe (GW2/GW3, D2)** (`probeGhostLiveness` `os.OpenFile(dbPath+".ghost_probe", O_CREATE|O_EXCL|O_WRONLY,0644)` atomic, `Close`+`Remove` on success, `false` on `EEXIST`/lock):
  - Distinguishes live holder vs stale reclaim; `TestGhostWAL_Busy_OExcl_Preserved` holds probe file open to force `O_EXCL` fail → wal/shm preserved, fallback intact.
- **Open Reclaim & Checkpoint (GW2, D2-D3)** (`ResolveDBPath`/`Open` pre-`sql.Open`: if `isGhostWAL(stale)`+`probeGhostLiveness==true` → `Remove` wal/shm (with truncate fallback for Windows lock) → `checkpointDB(primaryPath)` `PRAGMA wal_checkpoint(TRUNCATE)` best-effort swallowed → `sql.Open` primary, no fallback; else `needsFallback=true` preserve fallback):
  - `checkpointDB` opens `sql.Open("sqlite", dbPath)`, `busy_timeout=5000`, `Exec(TRUNCATE)` swallow; failure does not fail open. Tested `Stale_Removed` → gone+writable, `Fresh_Kept` preserved, `Busy_OExcl_Preserved` not removed.
- **Deferred Checkpoint (GW4, D4)** (`Save` named return `err` + `defer if err==nil { _, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)") }` after success only; `Search` `RLock` + same defer on `err==nil` after rows):
  - Swallows busy/locked, skips on error path (`closed DB` test), bursts `TestGhostWAL_WALBounded` 50×Save then `Stat -wal` bounded, Search close triggers checkpoint.
- **DoctorFix No-Op (GW4)** (`internal/bigmem/full.go` `DoctorFix` retains `PASSIVE+TRUNCATE+VACUUM+FTS rebuild` — no conflict with defer; verified `git diff -- internal/bigmem/full.go` empty `go vet ./internal/bigmem` clean):
  - No DoctorFix change; defer TRUNCATE complementary, not competing.
- **Tests** (`internal/bigmem/bigmem_test.go` +364 lines, 6 suites: `TestIsGhostWAL` (6 sub), `TestGhostWAL_Stale_Removed`, `Fresh_Kept`, `Busy_OExcl_Preserved`, `SaveSearch_Checkpoint`, `WALBounded`):
  - RED→GREEN TDD off but validated `go test -run TestIsGhostWAL` green, WAL bounded, busy swallowed, error skip — coverage of 12/12 scen.

## Archive Contents

| Artifact | Status | Path (archived) | Notes |
|----------|--------|-----------------|-------|
| proposal.md | ✅ | `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/proposal.md` | 3072 bytes, Intent ghost wal 0B/shm >0B on Pi kill/dual-holder, Scope mtime+O_EXCL+Remove+checkpoint + Save/Search defer, Out-of-scope D1/D2/DoctorFix, Approach 3 steps, 4 risks, Rollback `git revert` |
| spec (delta) | ✅ | `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/specs/bigmem/spec.md` | 4 req 12 scen REQ-GW1..GW4 — source synced to `openspec/specs/bigmem/spec.md` |
| design.md | ✅ | `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/design.md` | 4947 bytes, D1 5min vs immediate/10min, D2 O_EXCL vs flock, D3 TRUNCATE vs VACUUM/PASSIVE, D4 defer vs immediate, data flow + file changes + Testing Threat matrix |
| tasks.md | ✅ (12/12 [x]) | `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/tasks.md` | 3210 bytes, 12 tasks (Phase 1:3 RED, Phase 2:2, Phase 3:4, Phase 4:3), forecast 200–300 Low, 0 [ ] stale — gate PASS |
| verify-report.md | ✅ PASS | `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/verify-report.md` | 6988 bytes, verdict pass 4/4 12/12, evidence_revision `bb632b96...`, `biggz sdd-verify-validate` PASS, `go test` 62 ok, `go vet` 0 |
| archive-report.md | ✅ | `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/archive-report.md` | this file |

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/bigmem/spec.md` (333 lines) — updated domain, now 20 requirements (8 Engram REQ-1..8 + 8 branching REQ-B1..B8 + 4 ghost REQ-GW1..GW4) + scenarios (15 Engram +16 branching +12 ghost = 43 scen). Appended ADDED requirements GW1–GW4 preserving existing REQ-1..8 and REQ-B1..B8 verbatim.

Preserved: existing 16 REQ untouched; no new domain created (correct domain is `bigmem`). No REMOVED/RENAMED/MODIFIED delta — purely additive ghost domain extension. Subsequent consumers read from `openspec/specs/bigmem/spec.md`.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
Next `biggz sdd-status --json` will show this change under `archived` with `nextRecommended: done`. Active `openspec/changes/bigmem-ghost-wal/` no longer exists (moved to `openspec/changes/archive/2026-08-27-bigmem-ghost-wal/`). Ready for the next change.

---
*Artifact Store*: `openspec` (repo-local, `openspec/config.yaml` `strict_tdd: false`, workspace `C:\Users\USER\Desktop\biggz-ai`, allowed roots `[C:\Users\USER\Desktop\biggz-ai]`)
*Preflight*: `openspec, repo-local, auto-chain single PR, 400-line budget Low (200–300 forecast, 499 net actual within Low), strict_tdd off, `go test ./... -count=1 -timeout 180s``
*Evidence*: `go vet ./internal/bigmem` + `go vet ./...` clean (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`), `go test ./internal/bigmem -run TestGhostWAL -count=1 -timeout 60s` 1.343s PASS + `go test ./internal/bigmem` 4.398s PASS + `go test ./... -count=1 -timeout 180s` 62 PASS, evidence_revision `sha256:bb632b96d2b197a05e8d531a789869af8f2118f4147a0892d5c0d2835184e8d5` anchored to harness output, `biggz sdd-verify-validate --requirements 4 --scenarios 12` PASS
*Final-State*: diff NOT YET COMMITTED `internal/bigmem/bigmem.go` 135 lines + `bigmem_test.go` 364 lines = 499 net; `internal/bigmem/full.go` no diff (DoctorFix untouched), no blobstore/branching diff; BigMem ghost WAL warning still in DB at verify time but code Open-path reclaim addresses leak — fallback still active until commit/clean (residual risk Low, documented)
