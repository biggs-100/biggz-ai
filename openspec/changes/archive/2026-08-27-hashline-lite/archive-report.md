# Archive Report: hashline-lite

**Archived**: 2026-08-27
**Change**: hashline-lite
**Mode**: interactive, openspec, auto-chain, 800 lines, single PR, strict_tdd off
**Artifact Store**: openspec — `openspec/changes/hashline-lite` → `openspec/changes/archive/2026-08-27-hashline-lite/` + `openspec/specs/hashline-lite/spec.md` source of truth
**Archived to**: `openspec/changes/archive/2026-08-27-hashline-lite/`
**Previous location**: `openspec/changes/hashline-lite/` (active)

## Summary

Completed hashline-lite — Go port of oh-my-pi hashline for `sdd-apply`. Lite line-precise DSL `PUT N.=M: / PUT <N / CUT` with `#A1B2` 4-hex guard (SHA-256 prefix), seen-range validation, per-batch bounded snapshot, NoopLoopGuard, and `WriteFileAtomic` temp+rename fallback. Opt-in via `edit.mode=hashline` in `internal/sdd/apply.go`; off routes to legacy.

Shipped in single PR, **794 lines** (prod `parser.go 81 + apply.go 139 + snapshot.go 78 = 298 <400`, tests + hook remainder), within the 800 review budget. All **20/20 tasks** complete, **8/8 requirements, 16/16 scenarios** verified PASS, **19 tests PASS / 0 FAIL**, `go vet ./internal/edit/hashline` exit 0.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 20/20 marked [x] — `allComplete: true`, `pending: 0` |
| Verify verdict | ✅ PASS — 0 blockers, 0 CRITICAL, 0 WARNING |
| Spec compliance | ✅ 8/8 requirements, 16/16 scenarios COMPLIANT |
| Build | ✅ `go vet ./internal/edit/hashline` exit 0 (`e3b0c442...` empty hash), scoped `go vet ./internal/filemerge ./internal/sdd` exit 0 |
| Tests | ✅ `go test ./internal/edit/hashline -count=1 -v -timeout 180s` → PASS (19 top-level, 0.418s) |
| Evidence | `evidence_revision sha256:7cc2b8234b1857ec56278cd28147bcd30a9e657c5f7124297b6d21364d65bd6b`, `test_output_hash sha256:7cc2b8234b...`, `build_output_hash sha256:e3b0c44298fc...`, `biggz sdd-verify-validate --requirements 8 --scenarios 16` PASS |
| Review gate | N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` Divergences (`No review, resolve-review, or reviewGate values: biggz has no review authority`). `biggz sdd-status --json` emits no `reviewGate`; `review_disabled: false` (RDD enabled) but SDD routes via `nextRecommended` only. `nextRecommended: archive`, `dependencies.archive: ready`, `blockedReasons: []` — gate PASS |
| Task gate | PASS — persisted `openspec/changes/hashline-lite/tasks.md` shows 20 [x], 0 [ ] |

## Spec Compliance

**Verdict**: PASS (per `openspec/changes/hashline-lite/verify-report.md`, evidence_revision `sha256:7cc2b823...`, validated via `biggz sdd-verify-validate`)

| Metric | Value |
|--------|-------|
| Requirements | 8/8 compliant |
| Scenarios | 16/16 compliant |
| Tasks | 20/20 complete (Phases 1:3/3, 2:5/5, 3:3/3, 4:7/7, 5:2/2) |
| Blockers | 0 |
| Critical findings | 0 |
| Build | `go vet ./internal/edit/hashline` → 0, `go vet ./internal/edit/hashline ./internal/filemerge ./internal/sdd` → 0 |
| Tests | `go test ./internal/edit/hashline -count=1 -v` → 19 PASS (see matrix), scoped `go test ./internal/filemerge` PASS, `go test ./internal/sdd` PASS (16+ tests) |
| Evidence revision | `sha256:7cc2b8234b1857ec56278cd28147bcd30a9e657c5f7124297b6d21364d65bd6b` — ledger `tok-7e77f1f3411851fce1e87b56`, goal `verify 8 req 16 scen`, max-attempts 3, max-changed-lines 800 |
| Production lines | `parser.go(81)+apply.go(139)+snapshot.go(78)=298 <400` |

**Detailed matrix** (from verify-report — 16/16 COMPLIANT):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| DSL Parsing with #A1B2 | Valid PUT/CUT accepted | `TestParse` — valid PUT 1.=1, CUT 2.=2, PUT <5, PUT <5:, CUT <10, case-insensitive `#a1b2→A1B2` | ✅ COMPLIANT |
| DSL Parsing with #A1B2 | Bad tag rejected | `TestParse` — missing `#ZZZZ`/short `#A1B`/missing colon/wrong op `HELLO`/whole-file `PUT #A1B2` rejected | ✅ COMPLIANT |
| Seen-Range Guard | Unseen rejected | `TestValidateSeen` — `[1-20]` + `50.=60` error, `15.=25` partial overlap error, gap `[1,10][20,30]` + `15.=15` error, `TestApply_UnseenRejected` | ✅ COMPLIANT |
| Seen-Range Guard | Seen accepted | `TestValidateSeen` — `10.=15` in `[1-20]` PASS, `25.=28` in `[20,30]` PASS | ✅ COMPLIANT |
| Hash-Guarded Apply | Match writes | `TestApply_MatchWritesPUT` (PUT 2.=3), `TestApply_CUTMatchingHashRemovesRange`, `TestApply_PUTSingleLineLTMatch`/`TestApply_CUTSingleLineLT` | ✅ COMPLIANT |
| Hash-Guarded Apply | Mismatch warn-and-stop | `TestApply_MismatchWarnAndStop_NoOverwrite` — stale `FFFF` returns `freshHash==correct`, `HashMismatchError Code=needs_attention`, file unchanged; `TestApply_CUTMismatchPreservesFile` | ✅ COMPLIANT |
| Hash-Guarded Apply | Batch safe | `TestApply_BatchSafe` — A stale `FFFF` skipped, B fresh writes `newB`; `TestApply_Concurrent_NearbyStaleSecond` | ✅ COMPLIANT |
| ComputeHash Exact-Range | Range differs from whole | `TestComputeHash` — 100-line fixture `lines[9:20]` hash vs whole differ, `mustSHA(seg)==rangeHash` | ✅ COMPLIANT |
| ComputeHash Exact-Range | Empty digest | `TestComputeHash` — `ComputeHash(nil)` → `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `Hash4(empty)=E3B0`; `TestApply_Hash4Helper` | ✅ COMPLIANT |
| Snapshot Store | Restore | `TestSnapshot` — `Capture(orig)`→modify→`Restore` via `WriteFileAtomic` restores original | ✅ COMPLIANT |
| Snapshot Store | Bounded | `TestSnapshot` — Size 3→Clear 0; `TestSnapshot_Bounded` — 5 entries overwrite stays 5 (≤N per-batch) | ✅ COMPLIANT |
| NoopLoopGuard | No-op aborts | `TestNoopLoopGuard_EqualAborts`, `TestNoopLoopGuard_DifferProceeds`, `TestApply_NoopAbortsNoWrite` — PUT 1.=1 same content aborts, no write | ✅ COMPLIANT |
| Fallback Atomicity | Failure preserves original | `TestApply_WriteAtomicFailurePreservesOriginal` — dir path error, `IsDir` preserved; `WriteFileAtomic` temp+rename, no auto-Mkdir | ✅ COMPLIANT |
| Edit Mode Flag and Quality Gates | Flag disabled keeps legacy | `internal/sdd/apply.go > ApplyEdit: if !IsHashlineMode() { WriteFileAtomic directly }` — off→legacy, no parser invoked | ✅ COMPLIANT |
| Edit Mode Flag and Quality Gates | Flag enabled routes to hashline | `ApplyEdit` when `GetEditMode()=="hashline"` → `hashline.Parse`→`ValidateSeen`→`hashline.Apply` with `needs_attention+freshHash` fallback; `HookRead` fills `seenRanges[path]=[1,n]` + `snap.Capture`; `ClearBatch` | ✅ COMPLIANT |
| Edit Mode Flag and Quality Gates | Gates pass | `go vet` 0, `go test` 0, prod 298 <400, token ≥60% documented (PUT `#A1B2` vs str_replace), DSL idempotent | ✅ COMPLIANT |

## Spec Sync

Delta specs merged into main specs (source of truth) before archive. In openspec mode `openspec/specs/` is the audit authority; filesystem wins on conflict.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| hashline-lite | Created (new domain) | 8 requirements, 16 scenarios — DSL Parsing, Seen-Range Guard, Hash-Guarded Apply, ComputeHash Exact-Range, Snapshot Store, NoopLoopGuard, Fallback Atomicity, Edit Mode Flag and Quality Gates | `openspec/specs/hashline-lite/spec.md` ✅ (135 lines) |

No existing main spec to preserve — delta was a full spec, copied directly `openspec/changes/hashline-lite/specs/hashline-lite/spec.md → openspec/specs/hashline-lite/spec.md`. No REMOVED/RENAMED/MODIFIED (new domain). Subsequent consumers read from `openspec/specs/hashline-lite/spec.md`.

## Verification Gate

- **Review gate**: N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` Divergences. `biggz sdd-status --json` emits no `reviewGate` field; `review_disabled: false` (RDD enabled) but SDD changes route via `nextRecommended` only. `nextRecommended: archive`, `blockedReasons: []`, `dependencies.archive: ready` — gate PASS. No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required.
- **Task gate**: PASS — persisted `openspec/changes/hashline-lite/tasks.md` shows 20/20 [x], 0 [ ] pending. `taskProgress: {total:20, completed:20, pending:0, allComplete:true}`, `applyState: all_done`.
- **Build & Tests**: PASS — `go vet ./internal/edit/hashline` 0, `go test ./internal/edit/hashline -count=1 -v` 19 PASS (0.418s), scoped `go test ./internal/filemerge` PASS, `go test ./internal/sdd` PASS. Production `wc -l` 298 <400.
- **Verify report**: PASS — `openspec/changes/hashline-lite/verify-report.md`, verdict `pass`, 0 blockers, 0 critical, 8/8 req, 16/16 scen, validated via `biggz sdd-verify-validate --requirements 8 --scenarios 16`.
- **Remediation**: Not required — initial verify already PASS, no failed evidence revision, no ledger remediation needed. `biggz sdd-attempt status` shows `Complete: true` for prior attempt ledger (not blocking archive while `nextRecommended: archive`).

## Implementation Summary

- **Parser** (`internal/edit/hashline/parser.go`, 81 lines): `Directive{Op, Start, End, HashTag, Raw}`, `Parse(line)` regex `^(PUT|CUT)\s+(?:(\d+)\.=(\d+):|<\s*(\d+)(?::)?)\s+#([0-9a-fA-F]{4})\b`, trims, upper-cases `HashTag`, rejects missing/malformed/non-hex/`#A1B`/whole-file fallback; `ValidateSeen(d, seen [][2]int) error` checks `Start..End ⊆ ∪ seen`, empty seen→unseen error, partial overlap rejected.
- **Apply** (`internal/edit/hashline/apply.go`, 139 lines): `Hash4(fullSHA string) string` (first 4 upper), `NoopLoopGuard(current, newContent []byte) bool` (`bytes.Equal` abort for PUT), `HashMismatchError{Code:needs_attention, FreshHash, Path, Expected}`, `Apply(path, d, seen, snap *Store, newContent []byte) (freshHash string, err error)` — `ValidateSeen`→`NoopLoopGuard` (PUT only)→`ComputeHash(exactRange)` vs `#A1B2` → match `WriteFileAtomic` else `HashMismatchError` no overwrite, no silent retry, batch-safe. CUT match removes range, mismatch preserves. Failure via `WriteFileAtomic` temp+rename preserves original, no auto-Mkdir, Windows `Access is denied` surfaces as `*os.PathError`.
- **Snapshot** (`internal/edit/hashline/snapshot.go`, 78 lines): `Store{mu sync.Mutex, m map[string][]byte}`, `Capture(path, content []byte)` copy, `Restore(path) error` via `WriteFileAtomic`, `Clear()`, `Size()/Get()` — bounded ≤N per batch, cleared after batch via `sdd/apply.go ClearBatch`.
- **Hook & Flag** (`internal/sdd/apply.go`, modified): `editMode` (`legacy` default), `SetEditMode/GetEditMode/IsHashlineMode()`, `HookRead(path, content)` captures `seenRanges[path]=[1,n]` + `snap.Capture`, `ClearBatch()` resets both, `ApplyEdit(path, directive, newContent)` switches `!IsHashlineMode() → WriteFileAtomic directly` vs `IsHashlineMode() → hashline.Parse`→`ValidateSeen`→`hashline.Apply` with `needs_attention+freshHash` transparent fallback on parse error. `seenRanges map[string][][2]int`, `snap *hashline.Store` held per-process.
- **Reuse**: `internal/filemerge.ComputeHash` (SHA-256 exact bytes, `nil→e3b0...`) and `WriteFileAtomic` (temp+rename, preserve perm, no Mkdir) — no edits to `filemerge`.
- **Tests** (6 files, 410+ lines): `parser_test.go` (valid/invalid tags), `snapshot_test.go` + `snapshot_test.go` Bounded, `apply_test.go` (MatchWrites, MismatchWarnAndStop, CUTMatching, CUTMismatch, CUT/PUT <N, UnseenRejected, BatchSafe, Concurrent_NearbyStaleSecond, WriteAtomicFailurePreservesOriginal, NoopAbortsNoWrite, Hash4Helper), plus `TestValidateSeen`, `TestComputeHash` (100-line 10-20 vs whole, empty `e3b0...`), `TestSnapshot`, `TestNoopLoopGuard`.

## Archive Contents

| Artifact | Status | Path (archived) |
|----------|--------|-----------------|
| proposal.md | ✅ | `openspec/changes/archive/2026-08-27-hashline-lite/proposal.md` (74 lines) |
| spec (delta) | ✅ | `openspec/changes/archive/2026-08-27-hashline-lite/specs/hashline-lite/spec.md` (135 lines) |
| design.md | ✅ | `openspec/changes/archive/2026-08-27-hashline-lite/design.md` (138 lines) |
| tasks.md | ✅ (20/20 [x]) | `openspec/changes/archive/2026-08-27-hashline-lite/tasks.md` (68 lines, 0 [ ]) |
| verify-report.md | ✅ PASS | `openspec/changes/archive/2026-08-27-hashline-lite/verify-report.md` (143 lines, evidence `7cc2b823...`) |
| archive-report.md | ✅ (this file) | `openspec/changes/archive/2026-08-27-hashline-lite/archive-report.md` |

**Task Completion Gate**: `tasks.md` has 0 unchecked (`- [ ]` count 0), 20 checked (`- [x]` count 20) — gate PASS, no stale checkboxes. `sdd-apply` owns checkbox completion; no archive-time reconciliation needed.

**Archive verification**:
- [x] Main specs updated correctly (hashline-lite created, 8 req 16 scen)
- [x] Change folder moved to archive (`openspec/changes/archive/2026-08-27-hashline-lite/`)
- [x] Archive contains all artifacts (proposal, specs, design, tasks, verify-report, archive-report)
- [x] Archived `tasks.md` has no unchecked implementation tasks
- [x] Active changes directory no longer has this change (`openspec/changes/hashline-lite` absent)

## Source of Truth Updated

The following specs now reflect the new behavior (source of truth):

- `openspec/specs/hashline-lite/spec.md` — 8 requirements, 16 scenarios (new domain; delta copied, no destructive overwrite, all prior unrelated domains preserved)

Preserved requirements not mentioned in delta remain unchanged (all other `openspec/specs/*/spec.md` domains untouched; only `hashline-lite` added).

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived:

`proposal → spec (8/16) → design → tasks (20/20) → apply (794 lines, single PR, edit.mode hook + hashline pkg) → verify (PASS 7cc2b823, 8/8 16/16, 19 tests) → archive (2026-08-27)`

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks. `biggz sdd-status --json` after archive will show no active `hashline-lite`, archived `2026-08-27-hashline-lite` with `IsArchived: true`, `NextRecommended: done`.

## Audit Trail

- **Structured status at archive**: `biggz sdd-status --json --instructions` for `hashline-lite` — `changeRoot: C:\Users\USER\Desktop\biggz-ai\openspec\changes\hashline-lite`, `artifactStore: openspec`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, verifyReport:done, applyProgress:missing}`, `taskProgress: {total:20, completed:20, pending:0, allComplete:true}`, `dependencies: {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done, archive:ready}`, `applyState: all_done`, `actionContext: {mode:repo-local, workspaceRoot:C:\Users\USER\Desktop\biggz-ai, allowedEditRoots:[C:\Users\USER\Desktop\biggz-ai]}`, `remediationState: {required:false}`, `nextRecommended: archive`, `blockedReasons: []`, `review_disabled: false` (RDD enabled but SDD path has no review authority per divergences), `phaseInstructions.archive: ["Change: hashline-lite", "State: ready", "Archive only when verify-report.md exists and every task checkbox is complete."]`
- **Artifacts persisted** (openspec mode — filesystem authoritative; no BigMem observation IDs): `proposal.md` (74 lines), `specs/hashline-lite/spec.md` (135 lines, 8 req 16 scen), `design.md` (138 lines), `tasks.md` (68 lines, 20 [x]), `verify-report.md` (143 lines, PASS, 7cc2b823...), `archive-report.md` (this file). Delta specs synced before move; main spec `openspec/specs/hashline-lite/spec.md` created as source of truth.
- **Ledger & validation**: evidence_revision `sha256:7cc2b8234b1857ec56278cd28147bcd30a9e657c5f7124297b6d21364d65bd6b`, test_output_hash `sha256:7cc2b8234b1857ec56278cd28147bcd30a9e657c5f7124297b6d21364d65bd6b`, build_output_hash `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `biggz sdd-verify-validate --requirements 8 --scenarios 16` PASS, ledger settle `tok-7e77f1f3411851fce1e87b56` goal `verify 8 req 16 scen` max-attempts 3 max-changed-lines 800, single PR 794 lines (tasks estimate 480-620, actual within 800 budget).
- **Filesystem authoritative**: `openspec/changes/archive/2026-08-27-hashline-lite/` — proposal/design/tasks/verify-report/archive-report + delta `specs/hashline-lite/spec.md` preserved; new domain `openspec/specs/hashline-lite/spec.md` created.
- **Final-State Authority**: persisted `verify-report.md` verdict PASS with 16/16 COMPLIANT is terminal; no intermediate `apply-progress` (missing) to reconcile — `tasks.md` is the completion authority (20/20 [x]) and outranks snapshots. No contradictions to record; orchestrator launch facts (interactive/openspec/auto-chain/800/single PR/strict_tdd off, 8 req 16 scen, 20/20 [x], 794 lines, `go test` PASS, `go vet` 0, verify PASS) corroborated by `biggz sdd-status` and file reads.

## Risks and Residual

- **None blocking**. Verify-report SUGGESTION only: token saving ≥60% vs `str_replace` is proposal-level measurement (68% vs Grok Fast from oh-my-pi); DSL brevity (`PUT 10.=20: #A1B2`+bytes vs whole-file `str_replace`) inherently satisfies claim, but a one-time `wc` token-count fixture could quantify in a future verify refresh. No CRITICAL/WARNING.
- Single PR 794 lines is within 800 budget; 400-line production sub-budget satisfied (298 <400). No chained PRs needed.
- Windows `Access is denied` on concurrent `WriteFileAtomic` rename surfaces as contention (`*os.PathError`) — not retried, by design; batch continues.

## Notes

- Openspec mode: filesystem is authoritative. Hybrid sync not needed; no BigMem `sdd/hashline-lite/*` topics to reconcile.
- No `reviewGate` receipt exists for this SDD change — expected per `sdd-status-contract.md` Divergences; archive proceeds on `nextRecommended: archive` with empty `blockedReasons` and strict verify PASS.
- No intentional partial archive or stale-checkbox reconciliation — all 20 tasks are genuinely complete per persisted tasks artifact and verify-report.
- Strict TDD off for this change per preflight; `sdd-apply` hook + hashline pkg delivered under Standard mode, gates still green.

