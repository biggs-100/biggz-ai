# Archive Report: fix-bigmem-blob-docs

**Change**: fix-bigmem-blob-docs
**Archived**: 2026-09-04
**Mode**: openspec
**Code commits**: b8d4df91 (squash) — `feat(bigmem): visible blob failures and DOCS reference (SDD4)` + 6b267f61 (correction) — `fix(bigmem): reject empty BlobRoot in GetBlob (RDD correction)`
**Verify**: PASS WITH WARNINGS — 4/4 requirements, 9/9 scenarios
**Review lineage**: finalized with receipt (4 lenses, 1 correction round — fix commit 6b267f61 empty-BlobRoot guard + regression test)
**Tasks**: 15/15 complete, no stale checkboxes
**Do NOT commit**: archive file operations only, left uncommitted per instruction
**Program note**: final change of the BigMem audit program (SDD4 closes the program)

## Skill resolution

- Read `internal/assets/biggz/biggz-orchestrator-workflow.md` before work: yes (SDD workflow, dependency graph, dispatcher, gates, ledger, archive handoff).
- Read `internal/assets/biggz/biggz-orchestrator-delegation.md` before work: yes (routing ladder, delegation rules, edit surfaces, lossless prompts, SDD agent authority).

## Task Completion Gate

`openspec/changes/fix-bigmem-blob-docs/tasks.md` inspected before sync: all 15 tasks checked
(Phase 1: 1.1–1.3, Phase 2: 2.1–2.4, Phase 3: 3.1–3.4, Phase 4: 4.1–4.4). No `- [ ]` remains.
Gate PASSED — sync and move authorized. No stale-checkbox reconciliation needed.

## Native Review Receipt Gate (openspec adaptation)

Structured status reported review lineage finalized with terminal receipt (4 lenses,
1 correction round) and `sdd-status: sync ready + archive ready` (ledger attempt
token=tok-06ea3dd7d539fe90cae30943, request-id sdd4-arch-001, work-unit ARCHIVE-sdd4).
No `scope-changed`/`invalidated`/`escalated` state. `verify-report` shows zero CRITICAL
findings (verdict `pass_with_warnings`, blockers 0, critical_findings 0).
Gate PASSED — archive authorized. (Exact transaction/ledger/receipt topics are Engram-side
review artifacts; in openspec mode the launch-prompt lineage + verify verdict + clean tree
constitute the governing evidence.)

## Specs synced

| Domain | Action | Details |
|--------|--------|---------|
| bigmem | Updated | 4 added (2 MODIFIED-as-new + 2 ADDED), 0 modified-in-place, 0 removed — `openspec/specs/bigmem/spec.md` |

Delta source:
- `openspec/changes/fix-bigmem-blob-docs/specs/bigmem/spec.md` — MODIFIED: Blob-miss visibility on read; PutBlob failure visibility on save. ADDED: Single BigMem blob reference doc; Doctor DB path via filepath.Join.

Merge method: all 4 delta requirements appended to the main spec Requirements section; all
pre-existing requirements preserved verbatim. The 2 MODIFIED entries had no matching requirement
in the main spec (prior silent-failure behavior was undocumented there), so no in-place
replacement was possible — they were added as new requirements, which is the correct
non-destructive outcome. No REMOVED/RENAMED entries, so no deletion or rename was performed.
Verified via `rg` — all 4 requirement headings present in `openspec/specs/bigmem/spec.md`, and
`git diff --stat` for the spec shows append-only growth.

Source of truth now:
- `openspec/specs/bigmem/spec.md` — Blob-miss visibility on read; PutBlob failure visibility on save; Single BigMem blob reference doc; Doctor DB path via filepath.Join.

## Archive move

```
openspec/changes/fix-bigmem-blob-docs/
  → openspec/changes/archive/2026-09-04-fix-bigmem-blob-docs/
```

Archive contents verified: proposal.md, specs/bigmem/spec.md, design.md,
tasks.md (15/15 checked), apply-progress.md, verify-report.md, archive-report.md (this file).
Active `openspec/changes/fix-bigmem-blob-docs/` no longer exists after move.

## Correction record (review correction round)

Review (4 lenses) required one correction round, landed as commit 6b267f61 on top of the
squash feat b8d4df91:

- **Root cause**: `GetBlob` lacked the empty-`BlobRoot` guard that `PutBlob` already had, so an
  empty HOME produced a cwd-relative read instead of a deterministic error (pre-existing hole,
  shared root cause with the reviewed R1-2/R4-3 findings).
- **Fix**: `GetBlob` now rejects empty `BlobRoot` deterministically
  (`home dir: not found — blob unavailable`), mirroring the `PutBlob` guard. 5-line fix in
  `internal/bigmem/blobstore.go`.
- **Regression test**: `TestGetBlob_EmptyRootDeterministicError` added to
  `internal/bigmem/blobstore_test.go` (21-line addition).
- **Evidence**: `go test ./internal/bigmem/ -run TestGetBlob -count=1` PASS (5/5); full bigmem
  suite green (per apply-progress correction record).
- **Scope**: 3 files, 31 insertions, 1 deletion (`blobstore.go`, `blobstore_test.go`,
  `apply-progress.md` correction note). No spec/design change required.

## Accepted-scope notes (explicitly out of correction scope)

Recorded in apply-progress and carried here as the terminal audit trail — these are design
decisions, not open defects:

1. **session-guard comment-only by design** — `internal/sdd/session_guard.go` fallback (~197)
   is comment-only with no stderr in the library path; raw-inline persistence was already
   correct there. Visibility is owned by the edges (MCP `mem_save` result + stderr note, CLI
   save stderr note, `Store.SaveCtx` log line) plus future `DoctorFixBlobs` migration. Approved
   design tradeoff (design §File Changes), spec lists guard among Save paths.
2. **marker-spoof display-only** — a stored literal `[missing-blob …]` string passes through
   untouched (helper checks `IsBlobAddr` first; marker never persisted, never fed back to
   `GetBlob`). Indistinguishability in display is accepted; no filesystem or state risk (design
   Threat Matrix row 2: N/A — display-only).
3. **inline-bloat migration path** — `PutBlob`-fail rows stay raw inline (larger rows) until the
   next `DoctorFixBlobs` run migrates them. No bytes lost; bounded window, idempotent migration.
   Pre-existing image case-sensitivity, `DoctorFixBlobs` lock/ctx, and Remedy scope items likewise
   stay pre-existing/out-of-scope.

## What shipped (final state)

- Read path: `MissingBlobMarker` / `IsMissingBlobMarker` / `ResolveBlobOrMarker` in
  `internal/bigmem/blobstore.go` (miss→marker+log, no DB touch); all 5 read sites rewired
  (`GetCtx`, `resolveBlobContent`, 3 `SearchCtx` loops, MCP `mem_get` + stderr miss line);
  `SyncImport` miss-only log, `SyncExport` untouched. Correction: empty-`BlobRoot` deterministic
  error in `GetBlob` (6b267f61).
- Save path: `Store.SaveCtx` log-only on `PutBlob` fail (raw inline preserved, no return);
  MCP `mem_save` result `⚠️ blob externalize failed` + stderr; CLI save stderr gains
  `bytes preserved inline, DoctorFixBlobs will migrate` (exit 0 kept); guard fallback
  comment-only per design.
- Docs: `docs/bigmem-DOCS.md` created (schema, `BlobRoot` sibling layout, 50k `maxStoredBytes`
  vs 100KB/`data:image/` `ShouldExternalize`, 300-char preview, 1MiB stdin scanner, lifecycle,
  `DoctorFixBlobs` migration note); 7 call-site comments point at it (`rg` 7 hits, pointer-only).
- Doctor: `internal/doctor/bigmem.go:65` string concat → `filepath.Join(store.RootDir(), "bigmem.db")`.
- Tests (per verify-report at verification time, outranked only by the later correction evidence
  above): 4 packages PASS (`internal/bigmem`, `internal/doctor`, `internal/sdd`, `cmd/biggz-mcp`),
  build exit 0, vet clean; focused 7/7 marker/save/DOCS-adjacent suites PASS; correction adds
  `TestGetBlob` 5/5 PASS. DOCS scenarios are inspection-only by nature (file read + `rg`
  threshold hits — recorded COMPLIANT via manual verification).
- Scope hygiene: blob-docs diff itself is 8 files, 217 insertions, 39 deletions + new DOCS
  (~100 lines), within the 400-line budget (per verify WARNING note).

## WARNING follow-ups (verify-recorded, NOT blockers)

Carried from verify-report (PASS WITH WARNINGS, zero CRITICAL). None blocks archive:

1. **Dirty-tree scope hygiene** — at verification time the tree held out-of-scope uncommitted
   SDD3 moves (bigmem+cli spec promotions, `fix-bigmem-mcp-nplus1` archive move). Blob-docs scope
   itself is within budget; reviewer must not attribute SDD3 lines to this change. Resolved by
   sequencing: SDD3 archived first (004d8f81), tree clean at archive time.
2. **DOCS inspection-only** — DOCS scenarios have no automated go test (prose reference by
   nature); compliance via file read + `rg` threshold hits. No runtime covering test expected.
3. **Guard comment-only tradeoff** — session_guard `PutBlob`-fail path surfaces no immediate
   visible status (MCP/CLI/Save-log cover visibility; guard relies on raw-inline + future
   `DoctorFixBlobs`). Accepted design tradeoff, see accepted-scope notes above.
4. **SUGGESTION (non-blocking)** — consider `strings.CutPrefix`/`CutSuffix` in
   `IsMissingBlobMarker` (modern-go `strings_cut_prefix_suffix`) and `min()`/slices helpers on
   future loops; no functional change needed.

## Verification checklist

- [x] Main specs updated correctly (4 requirements present, others preserved)
- [x] Change folder moved to `openspec/changes/archive/2026-09-04-fix-bigmem-blob-docs/`
- [x] Archive contains all artifacts (proposal, specs, design, tasks, apply-progress, verify-report, archive-report)
- [x] Archived tasks.md has no unchecked implementation tasks (15/15)
- [x] Active changes directory no longer has this change
- [x] No CRITICAL issues in verify-report
- [x] No commit performed (working tree holds spec sync + archive move, unstaged)

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
Final change of the BigMem audit program — ready for the next change.
