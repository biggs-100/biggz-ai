# Archive Report: fix-bigmem-status-bypass

**Change**: fix-bigmem-status-bypass
**Archived**: 2026-09-04 (UTC; container local 2026-09-03)
**Mode**: openspec (filesystem source of truth; no Engram observation IDs)
**Status**: success
**Next recommended**: done

## Skill Resolution

- Read `internal/assets/biggz/biggz-orchestrator-workflow.md` (SDD workflow, dispatcher, gates, ledger, recall) — evidenced.
- Read `internal/assets/biggz/biggz-orchestrator-delegation.md` (routing ladder, delegation rules, edit authority, lossless prompts) — evidenced.
- No chained-pr registry skill needed (single change, no PR split at archive time).

## Executive Summary

SDD change `fix-bigmem-status-bypass` is fully planned, implemented, verified, review-corrected, spec-synced, and archived. All 10 tasks complete, verify-report valid (6/6 requirements, 9/9 scenarios, PASS WITH WARNINGS, 0 CRITICAL), review lineage completed through 4 lenses plus 1 correction round (fix commit `663d47e0`) and burned per lifecycle with gate `allowed:true, delivery:burned/unmanaged`, delta spec (6 ADDED requirements) merged into `openspec/specs/sdd-status/spec.md`, and the change folder moved to `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/`. No commit was created (per instruction).

## Artifacts

| Artifact | Path (final, post-move) | State |
|----------|-------------------------|-------|
| proposal | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/proposal.md` | done |
| delta spec | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/specs/sdd-status/spec.md` | done (6 ADDED) |
| design | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/design.md` | done |
| tasks | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/tasks.md` | done (10/10, no unchecked) |
| apply-progress | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/apply-progress.md` | done (incl. RDD correction section) |
| verify-report | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/verify-report.md` | done (PASS WITH WARNINGS, valid) |
| archive-report | `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/archive-report.md` (this file) | done |
| main spec (source of truth) | `openspec/specs/sdd-status/spec.md` | updated (+6 requirements) |

## Task Completion Gate (passed)

- `openspec/changes/fix-bigmem-status-bypass/tasks.md` inspected before any sync/move: `grep "- [ ]"` returns `NO_UNCHECKED`; `sdd-status --json` reports `taskProgress: total 10, completed 10, pending 0, allComplete true`.
- No stale-checkbox reconciliation was needed; `sdd-apply` owns completion and the persisted artifact already reflects final state.

## Native Review Receipt Gate (passed with burned delivery)

Authoritative native evidence (rank 1 per Final-State Authority), read at archive time:

- `review inspect fix-bigmem-status-bypass`: lineage head `4ef13abe0963`, 9 events — `start_review` (commit `4907878f`, 947 changed lines, budget 200, lenses risk/resilience/readability/reliability, tier high) → `in_review` → 4× `lens_result` (readability, reliability, resilience, risk) → `resume` → `complete_review` (receipt `receipts\5e8b1335ad3d8bd563269e11d852df0db31cddce53baaa0e1b0f5c38e2c1c8a1.json`, hash `sha256:debd8c…`) → `burn_review` (same receipt hash). Chain valid.
- `review gate post-apply fix-bigmem-status-bypass --json`: `passed:false, allowed:true, delivery:"burned/unmanaged", reason:"review burned: receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"`.
- `review status --lineage fix-bigmem-status-bypass`: `Receipt: valid (hash: sha256:dc46aba12…)`, `Fix Rounds: 0/1`, `Scoped Valids: 0/1`, `Next Transition: correction (budget remaining: 200)` — post-burn residual counters, not pending work.
- `sdd-status --json`: `artifactStore: openspec`, `dependencies: {proposal,specs,design,tasks,apply,verify: all_done, sync: ready, archive: ready}`, `nextRecommended: sync` (pre-sync) with `archive: ready`; `sdd-continue fix-bigmem-status-bypass` → `Next phase: archive`.
- `actionContext.mode: repo-local` (not `workspace-planning`); `allowedEditRoots` contains workspace root — archive move permitted.

Gate reading: `burn_review` is not in the blocking set (`missing/pending/malformed/scope-changed/invalidated/escalated`); gate `allowed:true` plus `sdd-continue: archive` and `archive: ready` authorize close. Delivery at close is `burned/unmanaged` via ordinary repository policy (receipt ephemeral and burned after finalize), not a persisted allow receipt.

### Recorded contradiction (launch prompt vs native authority)

- Launch prompt (rank 3, most recent account): "review lineage fix-bigmem-status-bypass finalized with persisted receipt (4 lenses, 1 correction round fix commit 663d47e0)".
- Native review authority (rank 1, wins): at archive time the receipt is **burned, not persisted** — `complete_review` persisted receipt `sha256:debd8c…` at `2026-09-03T22:01:41-05:00`, then `burn_review` at the same second burned it (ephemeral lifecycle); `review status --lineage fix-bigmem-status-bypass-663d47e0` returns `Event Count: 0, Receipt: none` because that suffixed lineage never existed (correct lineage is unsuffixed `fix-bigmem-status-bypass`).
- Resolution: report the native final state (burned/unmanaged, allowed true). The "4 lenses + 1 correction round (`663d47e0`)" portion of the prompt is corroborated by native events and commit history and is retained; only the "persisted receipt at close" portion is stale and is NOT repeated as a current fact.

## Verify Final State (per hierarchy, not snapshot echo)

- `verify-report` (intermediate snapshot, rank 4, written `2026-09-03 21:53 -0500`): `verdict: pass`, `requirements 6/6`, `scenarios 9/9`, `blockers 0`, `critical 0`, `test_command: go test ./internal/sdd/ ./internal/bigmem/ -count=1 -timeout 180s` exit 0 (`ok sdd 14.766s, ok bigmem 8.568s`), `build: go vet` exit 0. validated at archive time via `sdd-verify-validate --requirements 6 --scenarios 9` → `Verify report is valid.` WARNING (budget/dirt only, no CRITICAL): ~775 changed lines vs 400 budget, single-PR per directive with clean Unit1/Unit2 split available, recommend `size:exception`; pre-existing dirt note (see below).
- Post-snapshot work (rank 3 prompt facts + rank 2 persisted `apply-progress` correction section + repo evidence): review found CRITICAL `R1-hydration-drop` (GetCtx per-row failure logged+continued with partial success); correction commit `663d47e0` (`2026-09-03 22:01 -0500`, after verify-report) fixed it plus R4-hybrid-error-swallow, R3-none-silent, R1-fallback-stale, R3-explore-parity, adding 3 tests (`TestCollectBigMemChanges_HydrationErrorFails`, `TestStatusWithOptions_HybridPropagatesBigMemError`, `TestCollectBigMemChanges_ExploreExcludedFromSeen`, +81 lines, 3 files 89+/3-). Correction evidence per `apply-progress`: `go build ./...` exit 0; `go test ./internal/sdd/ ./internal/bigmem/ -count=1` ok sdd 16.3s, ok bigmem 9.7s, 3/3 new PASS.
- Archive-time re-run (final numbers carried from highest-ranked live evidence): focused correction tests 3/3 PASS; full `go test ./internal/sdd/ ./internal/bigmem/ -count=1 -timeout 180s` → `ok sdd 15.129s, ok bigmem 8.587s` (exit 0). No fresh independent `verify-report` was written after the correction; the persisted `verify-report` therefore covers the pre-correction tree and the correction is covered by `apply-progress` evidence plus this archive-time run. This is recorded as history, not as a fresh verification verdict.
- CRITICAL gate: no CRITICAL in `verify-report`; the one review-time CRITICAL was corrected in `663d47e0` and the review completed (`complete_review` → `burn_review`, gate `allowed:true`). Archive is not blocked.
- Budget/dirt warnings at close: review budget overrun stands (intentional single-PR, `size:exception` recommendation recorded, no approval artifact required at archive); the "pre-existing workspace dirt" note in `verify-report` (`fix-bigmem-store-ctx` deletions, `bigmem/spec.md` modification, `archive/2026-09-04-fix-bigmem-store-ctx/`) is stale — at archive time `git status --porcelain` is clean for tracked work and `git log` shows `289a43ef docs(sdd): archive SDD1 fix-bigmem-store-ctx` already landed, so there is no residual dirt attributable to this change. Stale snapshot claims are attributed to their time and not repeated as current facts.

## Specs Synced

Domain `sdd-status` — main spec existed, delta contained only ADDED requirements:

| Domain | Action | Details |
|--------|--------|---------|
| sdd-status | Updated `openspec/specs/sdd-status/spec.md` | 6 added, 0 modified, 0 removed, 0 renamed; all pre-existing requirements preserved |

Appended requirements (names verbatim):

1. `Requirement: BigMem Status via Store Ctx API` (2 scenarios: Store-sourced collection; Absent DB falls back explicitly)
2. `Requirement: SQL-Side Visibility Filtering` (1 scenario: Predicates in SQL)
3. `Requirement: Minimal Hydration` (1 scenario: Visible-only hydration)
4. `Requirement: Caller Context With Timeout` (2 scenarios: Cancellation fails fast; No Background at hot spots)
5. `Requirement: Visible BigMem Failures` (1 scenario: Query error surfaces)
6. `Requirement: Project Visibility Parity` (2 scenarios: Personal excluded; Project match and override)

No REMOVED/RENAMED destructive operations; no collision (`sameDomainActiveChanges: null`); no legacy-flat or RENAMED guard content. Merge is purely additive and non-destructive.

Code commits covered (already on `master`, not created by archive):

- `f633ea59` feat(bigmem): key-only topic-prefix sweep (Unit1)
- `4907878f` feat(sdd): store-backed status collector with ctx (Unit2)
- `663d47e0` fix(sdd): visible failures for status collector (RDD correction)

Archive created no commits per instruction.

## Archive Move

- Source: `openspec/changes/fix-bigmem-status-bypass/` (proposal, specs/, design, tasks, apply-progress, verify-report, archive-report)
- Destination: `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/` (UTC date per session header; container local 2026-09-03)
- Method: filesystem move after spec sync; active changes directory no longer contains `fix-bigmem-status-bypass`.
- `openspec/config.yaml` contains no `rules.archive`; default archive policy applied.

## Verification Checklist

- [x] Main spec updated correctly (6 ADDED appended, others preserved)
- [x] Change folder moved to `openspec/changes/archive/2026-09-04-fix-bigmem-status-bypass/`
- [x] Archive contains proposal, specs/, design, tasks, apply-progress, verify-report, archive-report
- [x] Archived `tasks.md` has no unchecked implementation tasks (10/10)
- [x] Active `openspec/changes/` no longer has `fix-bigmem-status-bypass`
- [x] `sdd-verify-validate` passes; no CRITICAL in verify-report; review CRITICAL corrected in `663d47e0`
- [x] Review gate `allowed:true, delivery:burned/unmanaged`; `sdd-continue` → archive
- [x] No commit created; no staged files (`git diff --cached` empty at close of archive file ops — archive moves are unstaged filesystem changes by design)

## Risks / Open Questions

- Review delivery at close is `burned/unmanaged` (ephemeral receipt lifecycle), not a persisted allow receipt — downstream consumers must rely on ordinary repository policy (commits `f633ea59/4907878f/663d47e0` + green tests), not on a live receipt lookup.
- No post-correction independent `verify-report` exists; correction coverage rests on `apply-progress` evidence plus the archive-time test re-run (both green). A future strict audit may ask for a fresh `sdd-verify` pass, but current status (`archive: ready`, `sdd-continue: archive`) does not require it.
- Review budget overrun (~775 vs 400, intentional single-PR) carries the standing `size:exception` recommendation for the review/merge path; archive does not adjudicate it.
- None blocking: tasks 10/10, verify valid PASS WITH WARNINGS, gate allowed, specs synced, folder archived.

## Detailed Report (first 300 chars)

`fix-bigmem-status-bypass` archived: 10/10 tasks, verify 6/6 req 9/9 PASS WITH WARNINGS (0 CRITICAL), review 4 lenses + correction 663d47e0 completed then burned (gate allowed:true burned/unmanaged), 6 ADDED reqs merged to openspec/specs/sdd-status/spec.md, folder → archive/2026-09-04-fix-bigmem-status-bypass/, no commit.
