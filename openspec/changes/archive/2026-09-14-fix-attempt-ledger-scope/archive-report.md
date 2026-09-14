# Archive Report — fix-attempt-ledger-scope

```yaml
schema: biggz-ai.archive-report/v1
change: fix-attempt-ledger-scope
archived_at: "2026-09-14 (UTC; local clock 2026-09-13 23:29 UTC-5 — consistent with the sync-report timestamp 2026-09-14T04:24:15Z)"
archived_to: openspec/changes/archive/2026-09-14-fix-attempt-ledger-scope/
store: openspec (file-backed; hybrid semantics — filesystem wins)
verification: pass_with_warnings (admitted by biggz sdd-verify-validate)
evidence_revision: sha256:3fc55d07630e90063d2b6d4c73ab854680ed11194ad89221a147608906086ea7
committed: false
delivery: pending — size:exception vs split decision with the maintainer
```

## Change

`fix-attempt-ledger-scope` — lets SDD work units (slices) advance inside one ledger generation without a
reset, and stops a `passed` settle from closing the whole generation while the change still has pending
tasks. Scope: `internal/sddattempt` (ledger), `internal/sdd` (status routing), `cmd/biggz` (CLI
projection), assets (orchestrator workflow / status contract / verify prompts), plus the runtime spec
delta.

## Archive date & final path

- **Archive date**: 2026-09-14 (UTC, per house convention; the local wall clock read 2026-09-13 23:29 at
  UTC-5, consistent with the sync-report UTC timestamp).
- **Final path**: `openspec/changes/archive/2026-09-14-fix-attempt-ledger-scope/`.
- The source directory no longer exists under `openspec/changes/` — the active directory contains only
  `archive/`.
- No `.biggz-instance` existed in this change (verified pre-move: stat not found), so none required
  rename-preservation. No peer archive artifacts were touched.

## Artifact inventory (11 entries — the 10 pre-existing artifacts plus this report)

| Artifact | Status | Notes |
|----------|--------|-------|
| `_meta.yaml` | present | name/created/phase/description |
| `preproposal.md` | present | pre-planning |
| `exploration.md` | present | domain exploration |
| `proposal.md` | present | approved scope |
| `design.md` | present | technical design |
| `specs/runtime/spec.md` | present | delta: 11 requirements / 26 scenarios |
| `tasks.md` | present | 19/19 checked, 0 unchecked (persisted artifact — Task Completion Gate PASS) |
| `apply-progress.md` | present | implementation log (intermediate snapshot) |
| `verify-report.md` | present | final re-verification: PASS WITH WARNINGS |
| `sync-report.md` | present | delta → canonical application record |
| `archive-report.md` | present | this report |

Not produced: `review-subject.json` — no RDD review ran for this change (review gate unmanaged; see
"Delivery status"). Not authored by this phase; noted for the audit trail.

## Delta application at archive (idempotent re-run)

The delta application was re-run before the move to prove idempotence, via a temporary in-module Go
runner calling `sdd.Sync("fix-attempt-ledger-scope", "<workspace>", "<prompt with allow-destructive>")`
(there is no CLI command for sync). The `allow-destructive` marker cleared the large MODIFIED guard
(maintainer-approved; `largeMutationThreshold = 20`, `internal/sdd/openspec-deltas.go:13`).

| Metric | Before re-run | After re-run |
|--------|---------------|--------------|
| Canonical requirements | 21 | 21 |
| Canonical scenarios | 57 | 57 |
| Canonical spec sha256 | `ccee3cfef2368bcfce76c5f8b30574e65e5ae133dec4506e39afdcadfed9d302` | identical |

`SyncResult: applied` on both runs; **zero net change** — the ADDED blocks matched byte-identically and
the MODIFIED replacement was in place, exactly as designed (`applyAddedDelta` leaves an existing
identical block untouched). The temporary runner (`tools/tmpsyncrun/`) was deleted afterwards;
`git status --short` confirms nothing was left behind (`tools/` contains only the pre-existing
`nosourcegrep`). **Sync did not commit anything** — `git log -1` remained `f6ee068b` before and after.

## Final counts

| Surface | Requirements | Scenarios |
|---------|--------------|-----------|
| Delta (`specs/runtime/spec.md`) | 11 (10 ADDED + 1 MODIFIED) | 26 (22 ADDED + 4 MODIFIED) |
| Canonical (`openspec/specs/runtime/spec.md`) | 21 | 57 |

- MODIFIED: `### Requirement: Interrupted Refund Capped at 2×` — replaced in place; cap semantics now
  bind to the live objective generation (was: change-wide `2×MaxAttempts` total).
- ADDED: Successor Advance, Repeated Work Unit Refusal, Scope-Change Guard, Reset Preserves the Audit,
  Reset Opens a Fresh Budget Epoch, Generation Attribution, Lifetime Accounting, Pre-Change Ledger
  Compatibility, Remediation Admission, No New CLI Surface.
- Every untouched pre-existing requirement survives byte-verbatim in the canonical spec; spec
  header/purpose unchanged; canonical file valid UTF-8 without BOM.

## Final verification (admitted)

- **Verdict**: PASS WITH WARNINGS — admitted by `biggz sdd-verify-validate`.
- **Evidence revision** (admitted, equals the settled hash of the full-suite output):
  `sha256:3fc55d07630e90063d2b6d4c73ab854680ed11194ad89221a147608906086ea7`.
- **Coverage of the delta**: 11/11 requirements and 26/26 scenarios demonstrated with execution
  evidence; **0 unproven**. Authoritative suite: `go test ./... -count=1 -timeout 900s` → exit 0, 60
  packages ok, no FAIL/panic/DATA RACE.
- The first verification's W2 (the `internal/sdd/status.go` stale-decision extension lacked a test) is
  **CLOSED** by `TestStaleDecisionCompletedLedgerRoutesUnstranded` in the final run.

## Ledger final shape

The change exercised its own advance path in the real flow:

- **7 attempts across 4 objective generations with ZERO resets**.
- Three real successor advances: `apply-work` → `verify` → `budget-recovery` → `verify-fresh-epoch`.

## Final-state facts (orchestrator handoff — these postdate the artifacts above)

- The planned 19 tasks are complete, AND a **fourth corrective slice** was applied afterwards (not in
  `tasks.md`; recorded in the BigMem `apply-progress-corrective` observation and the final
  verify-report).
- Two corrections made after the first verify, since verified:
  (a) the passive `sdd-attempt status` projection now reports `work_unit_complete` with a
  successor-naming exit for a legitimately complete ledger (`corrupt_authority` reserved for anomalous
  completion); (b) `ResetResult.AttemptsReset` → `AttemptsPreserved` (`json:"attempts_preserved"`) and
  the CLI now prints `Previous attempts preserved: %d`.
- **Hard regression found by the orchestrator and FIXED before archive**: because `Reset` preserves the
  attempt chain, the budget guard `len(live) >= 2*MaxAttempts` stayed permanently true and the ledger
  became unrecoverable (neither `reset` nor a successor could proceed). `Reset` now opens a fresh budget
  epoch (generation advance recorded in `RuntimeReset.ToGeneration`, `omitempty`); the exhausted cap no
  longer gates a re-opened objective or a successor; the exhausted-budget exits name a remedy that
  works. The first verification had mis-graded this as a warning; it was escalated to a blocker and
  fixed.
- The **spec delta was amended** to declare that behaviour: `### Requirement: Reset Opens a Fresh Budget
  Epoch` (4 scenarios). Final delta counts: 11 requirements (10 ADDED + 1 MODIFIED), 26 scenarios
  (22 ADDED + 4 MODIFIED) — as recorded above.

### State reconciliation notes

- `verify-report.md` and `apply-progress.md` are intermediate snapshots: their "done" claims hold, and
  the residual warnings below are carried from the final report.
- BigMem mirror staleness: the BigMem-side copies of intermediate artifacts reflect earlier snapshots
  (e.g. `sdd/fix-attempt-ledger-scope/tasks` was saved 2026-09-13T22:06:13Z with chain strategy
  "pending" and unchecked boxes). The filesystem artifacts in this archive are the final authority; the
  BigMem-side `archive-report` observation is the closure signal that flips `sdd-status` classification.
  This is a ranking resolution (persisted FS artifact + launch-prompt facts win), recorded here because
  the two sources diverge.
- Pre-move, `sdd-status` showed the filesystem-derived entry (`sync: all_done`, `nextRecommended:
  archive`); immediately post-move, before the BigMem mirror, the stale BigMem-derived snapshot
  briefly surfaced the change as active (0/19 tasks, `nextRecommended: apply`). The BigMem mirror of
  this report — the documented closure signal (`engram_status.go:517-519`) — flips the change to
  `archived`, removing it from the active list.

## Residual warnings

- **W1 — Review budget overage**: ~2,610 authored lines (working tree, see Delivery status) against the
  400-line PR review budget — 6.5×.
- **W2 — Delta spec size**: `specs/runtime/spec.md` = 1,011 words against the sdd-spec skill's ~650-word
  guide (1.56×).
- **Non-blocking suggestions from the final verification**:
  - S1 — `begin`/`acquire` guard asymmetry (`begin` can still drift the 4 fields on an open objective;
    pinned by the frozen `budget_refund_test.go:163`).
  - S2 — `omitempty` vs `omitzero` for the new ints (documented intentional deviation; identical bytes
    for ints).
  - S3 — `RuntimeStatus.CumulativeChangedLines` now publishes the real value (external JSON consumers
    will see it change from the previous always-0).
  - S4 — canonical refund-cap wording — **RESOLVED by the sync re-run**: the canonical spec now carries
    the generation-scoped wording (the delta MODIFIED replaced the old "total" text).

## Delivery status

- **Nothing is committed.** `git log --oneline -1` is still `f6ee068b` (no new commit — sync did not
  commit, archive did not commit).
- Working tree at close: 12 tracked files modified — the 11 change files at +945/−131
  (`cmd/biggz/cli_sdd.go`, `cmd/biggz/sdd_attempt_grant_cli_test.go`,
  `internal/assets/biggz/biggz-orchestrator-workflow.md`, `internal/assets/opencode/commands/sdd-verify.md`,
  `internal/assets/prompts/sdd/sdd-verify.md`, `internal/assets/skills/_shared/sdd-status-contract.md`,
  `internal/sdd/remediation_derive_test.go`, `internal/sdd/status.go`,
  `internal/sddattempt/acquire_settle_test.go`, `internal/sddattempt/cas_store_test.go`,
  `internal/sddattempt/sddattempt.go`) plus the canonical spec `openspec/specs/runtime/spec.md`
  (+187/−8) → 12 files, +1132/−139 overall. Four new test files untracked (`advance_test.go` 691,
  `cas_contract_test.go` 349, `legacy_compat_test.go` 193, `budget_recovery_test.go` 301 lines).
  ~2,610 authored lines in total.
- **Delivery decision pending with the maintainer**: the 400-line review budget cannot hold this change
  (6.5×), so `size:exception` versus a split by layers is the maintainer's call BEFORE any PR is
  opened. No review receipt exists for this change (review gate unmanaged; no `review-subject.json`
  produced).

## Post-archive hygiene (branch / worktree)

Non-interactive run — **nothing was deleted**:

- Current branch: `master`, tracking `origin/master`, not gone. 0 local branches in `[gone]` state
  (the merged feature branches were already pruned in an earlier session).
- Read-only `git fetch --prune --dry-run` observed 3 stale remote-tracking refs that a real prune would
  remove (`origin/feat/sdd-fast-lane`, `origin/feat/wire-pi-review-relay-cli`,
  `origin/feat/wire-pi-review-relay-sdd`). Left untouched per non-interactive policy — no
  `branch -d`/`-D`, no worktree pruning.
- Worktrees: a single worktree (this repo, branch `master`); nothing prunable.

## Archive verification checklist

- [x] Delta applied / re-applied with no net change; canonical spec updated (21/57) — sync before move.
- [x] Change folder moved to `openspec/changes/archive/2026-09-14-fix-attempt-ledger-scope/` with all
      artifacts.
- [x] Active directory clean (only `archive/` remains under `openspec/changes/`).
- [x] Tasks complete in the persisted artifact (19/19; Task Completion Gate PASS; 0 unchecked).
- [x] No CRITICAL verify issues (0 blockers, 0 critical findings in the final report).
- [x] `git log` unchanged — no commit created by sync, the move, or this report.
- [x] Hygiene: non-interactive — delete nothing; observations recorded above.
- [x] BigMem mirror of this report saved under `sdd/fix-attempt-ledger-scope/archive-report`
      (closure signal; flipped `sdd-status` classification from active to archived).
- [x] `.biggz-instance`: none existed; nothing deleted anywhere.
