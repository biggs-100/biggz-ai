# Archive Report: fix-route-nextrecommended-clause

| Field | Value |
|-------|-------|
| Change | `fix-route-nextrecommended-clause` |
| Domain | `orchestrator` (REQ-OR-003) |
| Archived to | `openspec/changes/archive/2026-09-20-fix-route-nextrecommended-clause/` |
| Archive date | 2026-09-20 (ISO) |
| Delivery branch | `fix/route-nextrecommended-clause` based on `master` @ `d0b539b4` |
| Store mode | `openspec` |
| Final status | **Archived — cycle complete, delivery pending** (issue + PR + merge owned by the orchestrator) |

## Final State

- **Single delta, already synced.** `openspec/specs/orchestrator/spec.md` REQ-OR-003 now carries the routing-inertness invariant instead of the unsatisfiable `nextRecommended MUST be empty` clause. The sync ran under explicit human `allow-destructive` authorization (REQ-OR-003 block ≈39 lines > `largeMutationThreshold = 20`, `internal/sdd/openspec-deltas.go:13`).
- **Idempotence confirmed before archiving.** The spec of record's REQ-OR-003 block is byte-identical to the delta's requirement block; only the delta's `# Delta for orchestrator` / `## MODIFIED Requirements` headers differ. No second merge was performed, and none is needed.
- **Verify snapshot.** `verify-report.md` records `pass_with_warnings`: 1/1 requirements, 5/5 scenarios (4 COMPLIANT, 1 PARTIAL), 0 blockers, 0 critical findings. That snapshot predates the sync and the tasks restructure; its "pending sync/dispatcher" claims are historical (see below).
- **Implementation.** `internal/sdd/subroute_test.go` +35 lines (`TestSubrouteRoutingInert`); tracked diff `1 file changed, 35 insertions(+)`. Zero production deltas — `internal/sdd/status.go` and the routing surface (`route.go`, `continue.go`, `gates.go`, `status_helpers2.go`) are untouched.
- **Tasks complete.** Persisted `tasks.md` shows 7/7 checkboxes checked, 0 unchecked. Native dispatcher at archive time: `taskProgress {total: 7, completed: 7, pending: 0, allComplete: true}`, `dependencies {apply: all_done, verify: all_done, sync: all_done, archive: ready}`, `nextRecommended: archive`, `blockedReasons` absent, review mode disabled/unmanaged (no `reviewGate`).

## Already-Applied Sync (1-Line Change)

The delta's `## MODIFIED Requirements` replaced exactly one bullet in `openspec/specs/orchestrator/spec.md` (line ~678); all four preserved scenarios are byte-identical.

**Before:**
```markdown
- AND `nextRecommended` MUST be empty (no SDD next)
```

**After:**
```markdown
- AND `nextRecommended` MUST equal the value reported without the declared subroute — declaring a subroute MUST NOT influence routing (no SDD next derived from it)
```

## Idempotence Check

Commands and result:

```text
$ awk '/^### Requirement: REQ-OR-003/{f=1} f{print} /^### Requirement: REQ-OR-004/{if(f) exit}' openspec/specs/orchestrator/spec.md \
    | grep -v '^### Requirement: REQ-OR-004' > /tmp/main-req003-clean.txt
$ sed -n '/^### Requirement: REQ-OR-003/,$p' openspec/changes/fix-route-nextrecommended-clause/specs/orchestrator/spec.md > /tmp/delta-req003.txt
$ diff -u /tmp/main-req003-clean.txt /tmp/delta-req003.txt
diff-exit=0   (no differences — requirement blocks byte-identical)
```

Residual check that the old clause is gone from the spec of record (task 3.3):

```text
$ rg -n 'nextRecommended.*MUST be empty' openspec/specs/orchestrator/
orchestrator-rg-exit=1   (no matches)
```

The delta's only remaining distinction is structural: its `# Delta for orchestrator` title and `## MODIFIED Requirements` section header. Its intent is fully materialized in the spec of record.

## Ledger Evidence Revisions

Both work units settled `passed` before archiving (no acquire/settle performed by this phase):

| Work unit | Result | Evidence revision |
|-----------|--------|-------------------|
| `apply-fix-route-nextrecommended-clause` | `passed` | `sha256:47ece8338aa66a6d08e90d525f80c6c19607d36c642388c7da71fd29005006de` |
| `verify-fix-route-nextrecommended-clause` | `passed` | `sha256:160a2b0945c7ada6ee58f47caeecbdb5a7b4d9a80424e09b0fab71832b4eb942` |

The verify evidence revision matches `verify-report.md`'s leading envelope (`evidence_revision`, `test_output_hash`).

## Tasks Restructure After Verify (WARNING-2 Resolution)

After verification passed, the orchestrator restructured `tasks.md`: steps 3.x–4.x (gated sync, issue, archive, PR) had been written as task checkboxes. Because verify readiness requires `taskProgress.AllComplete` (`internal/sdd/status.go:1192-1197`), those unchecked boxes pinned the native dispatcher at `nextRecommended: apply` while the implementation unit was already done. Verification recorded this as WARNING-2.

They are now listed as gated steps **without checkboxes**; a disclosure block in `tasks.md` explains the restructure, its reason, and the precedent (`archive/2026-09-16-fix-checkpoint-ask-context/tasks.md`). No content was lost and no checkbox was added or removed during archive.

Dispatcher route before → after:

| | Before restructure | After restructure |
|---|--------------------|-------------------|
| `taskProgress` | 7/13 (6 pending: 3.1–3.3, 4.1–4.3) | 7/7, `allComplete: true` |
| `dependencies.verify` | `blocked` | `all_done` |
| `nextRecommended` | `apply` | `archive` |

The verify report's `7/13` snapshot is preserved as an accurate record of the pre-restructure moment and was not rewritten.

## Carried-Forward Warnings (recorded verbatim, not fixed here)

1. **The guard is a behavior lock, not red-to-green.** `TestSubrouteRoutingInert` cannot prove the old wording wrong; the unsatisfiability argument rests on code reading (`resolveNextRecommended` never returns `""`; change-less organic work exposes no `route`/`subroute`) plus `TestSubrouteChangeLessWorkspace`.
2. **Scenario coverage honesty.** S1's equality is derivation-level (not asserted through CLI/JSON), and S4's SDD `nextRecommended` conjunct is TEXTUAL-ONLY.

Verify snapshot warnings since resolved (kept for history): WARNING-1 (pending gated sync — the sync has since run) and WARNING-2 (dispatcher cannot route to verify — the restructure has since fixed it). SUGGESTION 2 (executable S4 assertion) remains open as future work, out of scope.

## Delivery Facts

- Branch `fix/route-nextrecommended-clause` exists with the working-tree changes (`internal/sdd/subroute_test.go`, `openspec/specs/orchestrator/spec.md`); **nothing is committed yet**.
- **The orchestrator still owes: the `bug` issue (step 4.1), the PR with `Closes #<N>` + exactly one `type:*` label (step 4.3), and the merge.** Under the accepted `size:exception` precedent (#112/#123/#159), the PR includes the SDD trail.
- Archive moved the whole change folder with no content edits: `_meta.yaml`, `state.yaml`, `proposal.md`, `design.md`, `tasks.md`, `apply-progress.md`, `verify-report.md`, `specs/orchestrator/spec.md`. The source folder is absent. No `.biggz-instance` was present in this change folder.
- Post-archive branch/worktree hygiene: branch already exists and is current; no `[gone]` branch/worktree pruning performed. The orchestrator owns delivery.

## Rollback Boundary

Reverting the delivery commit (never executed by this phase; the rollback is the orchestrator's boundary):

- The guard test `TestSubrouteRoutingInert` disappears.
- `openspec/specs/orchestrator/spec.md` returns to the unsatisfiable clause `- AND nextRecommended MUST be empty (no SDD next)` — the documented status quo ante.
- The archived folder disappears (it lives only in the delivery commit), returning the delta to the active changes area.

No runtime behavior changes in either direction: zero production deltas.

## Evidence Commands Run

| Command | Result |
|---------|--------|
| `diff -u` REQ-OR-003 block, spec-of-record vs delta | exit 0 — byte-identical |
| `rg -n 'nextRecommended.*MUST be empty' openspec/specs/orchestrator/` | exit 1 — no matches |
| `git diff --stat -- internal/sdd/subroute_test.go` | `1 file changed, 35 insertions(+)` |
| `git diff --stat -- internal/sdd/status.go internal/sdd/route.go internal/sdd/continue.go internal/sdd/gates.go` | empty — routing surface untouched |
| `biggz sdd-status --json` | `nextRecommended: archive`; `taskProgress 7/7 allComplete: true`; `dependencies.archive: ready`; no `blockedReasons`; review disabled/unmanaged |
| `grep '^\s*- \[ \]' tasks.md` | no unchecked boxes |
| `mv openspec/changes/fix-route-nextrecommended-clause openspec/changes/archive/2026-09-20-fix-route-nextrecommended-clause` | source folder absent; 8 artifacts present at destination |

## Artifact Checklist

- [x] `proposal.md`, `design.md`, `_meta.yaml`, `state.yaml` archived
- [x] `tasks.md` archived — 7/7 implementation tasks complete; gated steps disclosed
- [x] `apply-progress.md` archived
- [x] `verify-report.md` archived (`pass_with_warnings`, 0 critical, 0 blockers)
- [x] Delta `specs/orchestrator/spec.md` archived (synced, idempotence-proofed)
- [x] Spec of record updated: `openspec/specs/orchestrator/spec.md` (1 changed bullet, 4 scenarios preserved)
- [x] Source change folder absent
- [x] `.biggz-instance` retained (none existed)
- [x] Archive is an audit trail — never delete or modify

**Cycle complete.** The change is archived; delivery (issue, PR, merge) remains with the orchestrator.
