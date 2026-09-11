```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:8c9b5b7ab4d91e19ea8eb44ff1791622a3a2ff57902749e7e46e27d55c61d16d
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 3/3
scenarios: 11/11
test_command: go test ./internal/bigmem -count=1 -timeout 180s
test_exit_code: 0
test_output_hash: sha256:9ab0d4adb5a9ae94e1ab37238b993eef4ae407f888de5fa6cfbee1f07230eba1
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: fix-bigmem-recall-friction — **Slice 1 (tasks 1.1–1.4) + Phase 1b (tasks 1b.1–1b.5) = PR 1** (Ghost-WAL liveness, defect #4). Phases 2–5 are not started and are NOT verified here.
**Version**: N/A
**Mode**: Standard — `strict_tdd: false`; RDD enabled. No receipt or approval claim is made by this report; no verdict is claimed beyond evidence produced in this run.
**Revision verified**: branch `fix/bigmem-recall-friction-1-ghost-wal`, HEAD `bed2d881` + uncommitted slice/Phase-1b diff. The verifier made no repository writes except this report; `git status --short` before and after is unchanged (` M bigmem.go`, ` M bigmem_test.go`, `?? ghost_test.go`, `?? liveness_other.go`, `?? liveness_windows.go`, `?? openspec/changes/fix-bigmem-recall-friction/`).
**Supersedes**: the previous `verify-report.md` (evidence_revision `sha256:cb8a2182…`, written before Phase 1b). W1 and W2 from that report are closed — see "Stale-Report Findings: Disposition".
**Platform contract of this run** (accepted limitation): Windows evidence is **executed**; non-Windows evidence is **source inspection only** (no WSL, no cross-compilation — explicitly out of scope this round).
**Evidence digest recipe**: `cat focused_v.txt suite.txt vet.txt gofmt.txt stale.txt build.txt | sha256sum` = `sha256:8c9b5b7a…c61d16d` (files under `%TEMP%\bm-verify2`; per-file digests inline below).

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total (change) | 21 |
| Tasks complete (change) | 9 — Phase 1 (1.1–1.4) + Phase 1b (1b.1–1b.5) |
| Tasks incomplete (change) | 12 — Phases 2–5 (2.1–2.4, 3.1–3.3, 4.1–4.3, 5.1–5.2), outside this scope |
| Slice 1 + 1b acceptance scope | 9/9 tasks complete; `strict_tdd: false` → no Strict-TDD config gate applies |

### Build & Tests Execution (raw output, this run)
**Build**: ✅ Passed — `go build ./...` → exit 0, no output (`sha256:e3b0c442…b7852b855` = digest of empty output).

**Focused tests**: ✅ exit 0
```text
$ go test ./internal/bigmem -run 'Ghost|Liveness|ReclaimCheckpoint' -count=1
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	1.632s
exit 0 · sha256:9624e354…a99b75b

$ go test ./internal/bigmem -run 'Ghost|Liveness|ReclaimCheckpoint' -count=1 -v   # same pattern, per-test
14 top-level PASS / 1 SKIP (TestGhostWAL_NonWindows_LiveHolderKeepsFiles — skips ON Windows, by design)
--- PASS: TestGhostWAL_Stale_Removed (0.04s)
--- SKIP: TestGhostWAL_NonWindows_LiveHolderKeepsFiles (0.00s)
--- PASS: TestGhostWAL_Inconclusive_PreservesFiles (0.01s)
--- PASS: TestGhostWAL_Inconclusive_RecoveredFallback (0.10s)     # Phase 1b / W2 closure
--- PASS: TestGhostWAL_LiveHolder_UsesPrimary (0.07s)
--- PASS: TestGhostWAL_LeftoverProbe_ReclaimWhenDead (0.01s)
--- PASS: TestProbeDBLiveness_Matrix (0.00s)  [no holder / live holder / inconclusive]
--- PASS: TestClassifyGhostWAL_Matrix (0.08s) [6 subtests]
--- PASS: TestGhostWAL_ReclaimKeepsData (0.06s)
--- PASS: TestReclaimCheckpointErrorNeverFailsOpen (0.01s)
--- PASS: TestGhostWAL_Fresh_Kept / SaveSearch_Checkpoint / WALBounded / IsGhostWAL(+Robust)
PASS
exit 0 · sha256:953645b6…82b0c6358

$ go test ./internal/bigmem -run 'TestGhostWAL_Stale_Removed' -count=1 -v
--- PASS: TestGhostWAL_Stale_Removed (0.05s)
exit 0 · sha256:2c09d34a…c06906147
```

**Package suite**: ✅ exit 0
```text
$ go test ./internal/bigmem -count=1 -timeout 180s
ok  	github.com/biggs-100/biggz-ai/internal/bigmem	14.842s
exit 0 · sha256:9ab0d4ad…7230eba1
```

**Static checks**: ✅ exit 0
```text
$ go vet ./internal/bigmem   -> exit 0, empty output  (sha256:e3b0c442…b7852b855)
$ gofmt -l internal/bigmem   -> exit 0, empty output  (sha256:e3b0c442…b7852b855)
```

**Modern-Go check**: `use-modern-go` `list` was consulted for the touched Go files (`liveness_other.go`, `bigmem.go`, `ghost_test.go`; slice files `liveness_windows.go`, `bigmem_test.go` reviewed against the same catalog). No applicable modernization surfaced in the Phase 1b diff (no slice/map/context/loop idioms involved).

**Coverage**: ➖ Not available (no coverage gate defined for this slice; no coverage % claimed).

### Spec Compliance Matrix — REQ-GW1 / GW2 / GW3, per platform

Legend: ✅ executed + inspected · 🔎 inspection-only (this run) · ⚠️ PARTIAL · 🚫 structurally unreachable by design.

| Requirement (sub-clause) | Scenario | Windows — executed (this run) | Non-Windows — 🔎 inspection only | Covering tests (all PASS on Windows) |
|---|---|---|---|---|
| REQ-GW1 — live holder MUST NOT be classified stale, no warning, no fallback | Live holder is not ghost | ✅ `ghostLiveHolder` → primary, wal/shm untouched, stderr clean | ⚠️ never classified `ghostStale` (files safe), but classified `ghostInconclusive` → warning + recovered fallback still run (clauses "no warning / no fallback" unattainable when no holder is observable) | `TestClassifyGhostWAL_Matrix/live_holder_is_not_stale`, `TestGhostWAL_LiveHolder_UsesPrimary` |
| REQ-GW1 | Stale ghost detected | ✅ `ghostStale` when probe proves death | 🚫 unreachable: O_EXCL success is not proof (→ `ghostInconclusive`); `TestClassifyGhostWAL_Matrix/stale_idle_is_reclaimable` encodes `wantOther=ghostInconclusive` | matrix + `TestGhostWAL_Stale_Removed` |
| REQ-GW1 | Fresh ghost not stale | ✅ `ghostNone`, shape gate platform-neutral | 🔎 same code path, platform-neutral | `TestClassifyGhostWAL_Matrix/fresh…`, `TestGhostWAL_Fresh_Kept` |
| REQ-GW1 | Non-ghost sizes not stale | ✅ `ghostNone` | 🔎 platform-neutral | matrix `/wal_nonzero…`, `/shm_zero…`, `TestIsGhostWAL*` |
| REQ-GW2 — reclaim only after probe **proves** death | Stale reclaimed, primary used | ✅ `reclaimStaleWAL` (seam-forced proof) removes wal/shm + TRUNCATE checkpoint; primary opened | 🚫 unreachable: no proof mechanism exists → no reclaim (recorded tradeoff) | `TestGhostWAL_Stale_Removed`, `TestGhostWAL_ReclaimKeepsData` (seam), `TestGhostWAL_LeftoverProbe_ReclaimWhenDead` |
| REQ-GW2 — checkpoint failure must never fail `Open` | Checkpoint best-effort | ✅ injected checkpoint error: `ResolveDBPath` still returns primary, files still reclaimed | 🔎 reclaim itself unreachable off-Windows; best-effort clause moot | `TestReclaimCheckpointErrorNeverFailsOpen` (seam) |
| REQ-GW2 | Live holder blocks reclaim | ✅ wal/shm not removed, no fallback | ⚠️ files preserved (safe half ✅); "primary without fallback" half unattainable | `TestGhostWAL_LiveHolder_UsesPrimary` |
| REQ-GW3 — fresh = normal path | Fresh ghost normal open | ✅ no removal, no warning, no fallback | 🔎 platform-neutral | `TestGhostWAL_Fresh_Kept` |
| REQ-GW3 — inconclusive preserves wal/shm **and** recovered fallback | Inconclusive probe preserves fallback | ✅ warning emitted, resolved to `bigmem_recovered`, wal/shm + probe marker survive, seeded row readable | 🔎 this IS the off-Windows path; checked-in tests run there (`NonWindows_LiveHolderKeepsFiles`, `Inconclusive_PreservesFiles`) | `TestGhostWAL_Inconclusive_PreservesFiles` — `TestGhostWAL_Inconclusive_RecoveredFallback` (both halves now checked-in) |
| REQ-GW3 | No stale → no removal side-effect | ✅ `ghostNone` branch leaves files untouched | 🔎 platform-neutral | `TestGhostWAL_Fresh_Kept` + matrix |
| REQ-GW3 — live holder exception | Live holder opens primary without fallback | ✅ no warning, no fallback, wal/shm untouched | ⚠️ exception cannot trigger; fallback runs (files kept) | `TestGhostWAL_LiveHolder_UsesPrimary` |

**Compliance summary**: Windows (platform of the reported defect): **11/11 scenarios COMPLIANT, all via checked-in tests executed in this run** (incl. the formerly missing W2 assertion). Non-Windows: by inspection, 4 scenarios platform-neutral (fresh/non-ghost paths), 2 served by the conservative branch (`inconclusive` ✅), 5 have positive clauses structurally unreachable by design (safe direction). Counters declare 3/3 and 11/11 per the validator's declared==authoritative convention; the per-scenario matrix above is authoritative.

### Stale-Report Findings: Disposition
| Finding | Status now | Evidence |
|---------|-----------|----------|
| **W1** — off-Windows, O_EXCL success reported `proven=true` → `ghostStale` → `reclaimStaleWAL` could `os.Remove` a live holder's wal/shm (Unix removes open files) | ✅ **CLOSED** | `liveness_other.go:20-38` — O_EXCL success now returns `(holder=false, proven=false)`; probe failure also inconclusive; doc comment cites REQ-GW2/GW3 + tradeoff. `bigmem.go:296-308` — `!proven → ghostInconclusive`; `bigmem.go:572-577` — inconclusive → no removal, fallback preserved. Regression guard checked in: `TestGhostWAL_NonWindows_LiveHolderKeepsFiles` (asserts recovered resolution + wal/shm preserved; SKIPs on Windows — inspected, not executed this round). Matrix encodes the shift: `wantOther: ghostInconclusive` for stale-idle and `no holder` off-Windows. Note: this closure is verified by **source inspection + checked-in test inspection**; the implementer's WSL RED/GREEN run is recorded in `apply-progress.md` but is not claimed as this verifier's evidence. |
| **W2** — no checked-in assertion for the REQ-GW3 inconclusive→recovered fallback | ✅ **CLOSED** | `TestGhostWAL_Inconclusive_RecoveredFallback` (ghost_test.go): seam-forced inconclusive → asserts (a) stderr contains `ghost WAL/SHM detected`, (b) `ResolveDBPath` returns the recovered path, (c) `-wal`/`-shm`/`.ghost_probe` markers survive, (d) seeded row readable through the recovered store. **Executed on Windows in this run — PASS.** |
| **W3** (stale report) — non-Windows cases CI-invisible | Addressed to the extent possible | Non-Windows semantics now have explicit checked-in expectations that execute on non-Windows (`TestGhostWAL_NonWindows_LiveHolderKeepsFiles`, matrix `wantOther`, probe `no holder` branch). Inspected, not executed this round. |

### New-Gap Analysis — off-Windows inconclusive semantics vs the spec
- **Is any REQ-GW\* clause unserved?** One positive path becomes unreachable off-Windows: REQ-GW2's "Stale reclaimed, primary used" (stale corpus + probe success → reclaim). Previously it ran off-Windows, but on the incorrect premise (`O_EXCL` success = proof of death) that violated REQ-GW1's necessary condition ("classify as stale ghost **only when** … the primary liveness probe **proves** no live holder"). The fix makes the necessary condition honest, so off-Windows the system can no longer satisfy the positive scenario — it takes the `ghostInconclusive` branch instead.
- **Consistency with the spec as written**: ✅ consistent. REQ-GW2 gates reclaim on *proof* ("MUST reclaim only after the liveness probe proves the previous holder is dead"); an unobservable holder means no proof, so NOT reclaiming is the spec-compliant direction. REQ-GW3's MUSTs (inconclusive → wal/shm kept + recovered fallback preserved) are exactly what the new semantics do. No MUST-NOT clause is violated; the unserved surface is a positive best-effort behavior whose precondition the platform probe cannot establish.
- **Residual gap (WARNING W-1)**: the spec text carries **no platform qualifier**, and the repo ships/tests linux+darwin. Read platform-neutrally, REQ-GW2's positive reclaim scenario and the live-holder exception (REQ-GW1/GW3 "open primary, no warning, no fallback") are now Windows-only. The tradeoff (safety over aggressiveness; POSIX advisory-lock probe as out-of-scope follow-up) is recorded in `liveness_other.go` and `apply-progress.md`, consistent with the spec's normative core — but the spec/design still do not state the scope, so a reader of the spec alone cannot derive the Windows-only capability. Recommend scoping REQ-GW\* or documenting it in design (SUGGESTION).

### Phase 1b Test Blast Radius & Seam Independence
**File-by-file blast radius (Phase 1b only, vs slice 1):**
| File | Phase 1b delta | Nature |
|------|----------------|--------|
| `internal/bigmem/liveness_other.go` | rewrite of the probe body (~+12/−5) | semantics: O_EXCL success `(false,true)` → `(false,false)`; docs |
| `internal/bigmem/bigmem.go` | +4/−1 | new seam `var ghostProbeDBLiveness = probeDBLiveness` (line 313) + 1 call site in `classifyGhostWAL` (line 300); mirrors `ghostReclaimCheckpoint` |
| `internal/bigmem/ghost_test.go` | ~+145 (new file; slice 1 had it at ~430) | `forceProbeSeam` helper; `TestGhostWAL_NonWindows_LiveHolderKeepsFiles`; `TestGhostWAL_Inconclusive_RecoveredFallback`; seam-forcing in `ReclaimKeepsData`/`CheckpointError`; platform-aware matrix fields |
| `internal/bigmem/bigmem_test.go` | +7/−2 (vs slice 1's +0/−73) | **only** `TestGhostWAL_Stale_Removed` changed: comment block + `forceProbeSeam(t, false, true)` replacing the probe-file removal; every assertion kept |
| Production files not in Phase 1b | — | `liveness_windows.go` and `isGhostWAL` untouched |

**Evidence the stale test is platform-independent and does not mask a platform defect:**
1. **Seam restored**: `forceProbeSeam` saves `orig := ghostProbeDBLiveness` and restores it via `t.Cleanup` (ghost_test.go:69-74); no test using it calls `t.Parallel()`, so no seam can leak into or race another test. (The only `t.Parallel()` tests in the package, `TestIsGhostWAL`/`TestIsGhostWAL_Robust`, never touch the probe seam and run after the sequential tests by Go's testing rules.)
2. **No other test in `bigmem_test.go` was altered by Phase 1b**: `git diff` vs HEAD attributes every hunk to either slice 1's documented removals (`TestGhostWAL_Busy_OExcl_Preserved`, `TestGhostWAL_ProbeOExcl`, −73 lines) or the single `TestGhostWAL_Stale_Removed` hunk; the arithmetic is exact: slice 1 `+0/−73` + Phase 1b `+7/−2` = current `+7/−75` (`git diff --numstat`).
3. **The seam does not mask a platform defect**: it replaces only the probe outcome. `classifyGhostWAL` (shape gate + switch) and `reclaimStaleWAL` still run for real, so the test asserts *what reclaim does once death is proven* (REQ-GW2), which is exactly the clause under test. The platform probe keeps its own per-OS contract in `TestProbeDBLiveness_Matrix` (Windows no-holder `(false,true)` / off-Windows no-holder `(false,false)`; live holder Windows-only; inconclusive both OSes) and `TestClassifyGhostWAL_Matrix` (`wantOther`). Forcing the seam therefore cannot hide a probe regression: the probe's contract is asserted independently, and the semantics change under test (off-Windows `proven=false`) is itself encoded in the checked-in non-Windows expectations.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|-------------|--------|-------|
| REQ-GW1 | ✅ Implemented | `isGhostWAL` unchanged (shape+freshness); `classifyGhostWAL` adds the honest 3-way probe verdict (`bigmem.go:289-308`); live holder → primary, no touch, no fallback (`ResolveDBPath` switch, `bigmem.go:572-577`); Windows probe = share-0 `CreateFile` (sharing violation ⇒ live; success/missing ⇒ proven dead; else inconclusive) |
| REQ-GW2 | ✅ Implemented | `reclaimStaleWAL` (`bigmem.go:319-330`): Remove wal/shm/`.ghost_probe` + best-effort TRUNCATE, only on `ghostStale`; checkpoint error discarded → can never fail `Open` |
| REQ-GW3 | ✅ Implemented | `ghostInconclusive → needsFallback=true` keeps wal/shm and preserves warning + merge/promote fallback; fresh → `ghostNone` normal path; live holder exception (Windows) opens primary |
| Non-Windows probe | ✅ Honest by construction | `liveness_other.go`: O_EXCL success is inconclusive, not proof; comment states the rationale and the recorded tradeoff |
| Modern Go | ✅ Consulted | `use-modern-go list` run for touched files; nothing applicable surfaced |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| D1 Windows share-0 probe | ✅ | As designed |
| D1 non-Windows O_EXCL | ✅ with documented deviation | Slice 1 kept it as a "death" signal; Phase 1b corrects it to inconclusive (safety), matching D1's rationale that the O_EXCL probe "misses live holders". Positive reclaim off-Windows is given up deliberately; POSIX lock probe is an out-of-scope follow-up (documented in code + apply-progress) |
| D1 branch policy (holder → primary; proven dead → reclaim; inconclusive → recovered) | ✅ | Verified in code + executed tests (seam-proven reclaim, real Windows probe for holder/inconclusive) |
| D2–D5, Data Flow, Threat Matrix | ➖ N/A | Phases 2–5 not started |
| Review workload | ✅ recorded | PR 1 = 849 changed lines vs 400 budget; maintainer-approved `size:exception` in `tasks.md` (verified by `apply-progress.md` + `tasks.md` text) |

### Issues Found
**CRITICAL**: None. (No failing test, no `UNTESTED`/`FAILING` scenario with a checked-in-expectation gap on Windows; the W2 gap that previously forced a runtime-only claim is closed.)

**WARNING**:
- **W-1 — Spec text remains platform-unqualified while positive REQ-GW\* behavior is now Windows-only.** Off Windows, the live-holder exception and the positive reclaim path are structurally unreachable (probe cannot observe a holder). Files are never at risk (the hazardous direction is closed) and every MUST-NOT is honored, but a spec reader cannot derive the platform scope; REQ-GW2's positive scenario is unserved off-Windows. Recorded tradeoff exists (code + apply-progress) but the spec/design were not updated.
- **W-2 — Non-Windows verification is inspection-only in this round.** The checked-in non-Windows expectations were not executed (no WSL/cross-compilation by explicit instruction). Window-limited but accepted; re-run on Linux CI would close it.

**SUGGESTION**:
- **S-1** — Scope REQ-GW\* to Windows in the spec delta, or state the non-Windows limitation in `design.md`, so the Windows-only positive path is derivable from the artifacts (mirrors stale-report S2, still open).
- **S-2** — Track the POSIX advisory-lock probe (fcntl on SQLite lock bytes) as the follow-up that restores non-Windows reclaiming; it is already named out-of-scope in `liveness_other.go`.
- **S-3** — (Carried) Clarify design D1's "(handle held through `os.Remove`)" wording — the probe handle closes before best-effort removal; the TOCTOU window is benign (failed removal leaves files; primary still opens).

### Limits of This Verification
- **Executed (this run, Windows/amd64, Go 1.26.1)**: focused ghost/liveness/reclaim tests (14 PASS, 1 by-design SKIP), full `internal/bigmem` suite, `TestGhostWAL_Stale_Removed` verbose, `go vet`, `gofmt -l`, `go build ./...`. All exit 0; hashes recorded above.
- **Inspected only**: non-Windows probe semantics and every non-Windows-specific expectation (`liveness_other.go`, matrix `wantOther`, `TestGhostWAL_NonWindows_LiveHolderKeepsFiles`); the Phase 1b blast-radius accounting (diff/numstat, since slice 1 was never committed, Phase-1b-vs-slice-1 isolation rests on exact arithmetic + hunk attribution); design/tasks/apply-progress conformance.
- **Not verified**: Phases 2–5 (unstarted); cross-platform execution (explicitly out of scope this round); CI wiring that would execute the non-Windows tests; the implementer's WSL runs were not re-run and are not used as this verifier's evidence.
- **Tree state**: uncommitted for human review (no commits/branches created); only this report was added.

### Verdict
**PASS WITH WARNINGS** — On Windows (the platform of the reported defect), all 11 REQ-GW1/GW2/GW3 scenarios pass with checked-in tests executed in this run, including the reclaim path (seam-proven), the checkpoint-failure path, and the REQ-GW3 inconclusive fallback that previously lacked a checked-in assertion. Stale-report findings **W1 and W2 are CLOSED**; the Phase 1b fix introduces no new violating behavior — its cost is that off-Windows positive reclaim and the live-holder exception become structurally unreachable (safe direction, recorded tradeoff), which the unqualified spec text still does not state (**W-1**). This is not a rounded-up PASS: off-Windows claims rest on source inspection only (**W-2**), and the spec/platform scope mismatch remains open as W-1/S-1. Change-level verify/archive remain blocked until Phases 2–5 complete.
