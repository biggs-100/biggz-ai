# Archive Report: add-odd-lane

**Change**: add-odd-lane
**Archived**: 2026-09-20
**Store**: openspec (file-backed)
**Archived to**: `openspec/changes/archive/2026-09-20-add-odd-lane/`
**Branch**: `docs/archive-add-odd-lane` (changes uncommitted; the orchestrator owns delivery)

## Final State

| Item | Value |
|------|-------|
| Verify verdict at close | `pass_with_warnings` — 0 CRITICAL, 0 FAILING/UNTESTED |
| Requirements | 6/6 implemented |
| Scenarios | 16/16 evaluated — 9 executable-verified, 1 PARTIAL (OR-003-S1, WARNING-1), 6 textual-only |
| Tasks | 18/18 complete (persisted `tasks.md`, every task `- [x]`) |
| `evidence_revision` | `sha256:54e13851c68b36c09e8e316f5abfe0592d6e9adf926913a08abf394913859380` |
| Ledger | Untouched by this phase — no `sdd-attempt` acquire/begin/finish/settle was run |
| Review gate | No `reviewGate` present (change unreviewed/unmanaged); a fresh `reviewOffer` is present in native status and, per the status contract, never authorizes, blocks, or governs archive/delivery |
| Pre-phase blocked reason | `sync blocked: destructive change (REMOVED or large MODIFIED) for domain "orchestrator" without explicit approval; add allow-destructive to prompt` — satisfied by the explicit `allow-destructive` authorization carried in this phase's launch prompt |

## Spec Sync (destructive merge authorized)

Delta specs applied to main specs with `ParseDeltaSpec`/`ApplyDeltas` semantics (exact requirement-name matching; untouched requirements preserved byte-for-byte):

| Domain | Added | Modified | Removed | Result |
|--------|-------|----------|---------|--------|
| `odd-lane` | 4 | 0 | 0 | New `openspec/specs/odd-lane/spec.md`: `# odd-lane Specification`, `## Purpose`, `## Requirements`, then REQ-ODD-001..REQ-ODD-004 with their scenarios copied verbatim from the delta |
| `orchestrator` | 0 | 1 | 0 | `### Requirement: REQ-OR-003 — Route Field in Status and Continue` replaced in place, same position in file order: baseline prose + 4 scenarios removed, delta prose + 5 scenarios in; exactly one `REQ-OR-003` heading remains |
| `sdd-status` | 1 | 0 | 0 | `### Requirement: REQ-SS-ODD-001 — Read-Only ODD Documents Projection` appended with its 3 scenarios |
| **Total** | **5** | **1** | **0** | |

### Merge verification evidence

| Check | Command | Result |
|-------|---------|--------|
| One `REQ-OR-003` heading | `rg -c '^### Requirement: REQ-OR-003' openspec/specs/orchestrator/spec.md` | `1` |
| `REQ-OR-004` intact | `rg -c '^### Requirement: REQ-OR-004' openspec/specs/orchestrator/spec.md` | `1` |
| `odd-lane` has exactly 4 requirements | `rg -n '^### Requirement:' openspec/specs/odd-lane/spec.md` | REQ-ODD-001/002/003/004 — 4 matches |
| `REQ-SS-ODD-001` appears once | `rg -c '^### Requirement: REQ-SS-ODD-001' openspec/specs/sdd-status/spec.md` | `1` |
| Baseline required route values gone from REQ-OR-003 | `sed -n '/REQ-OR-003/,/REQ-OR-004/p' … \| rg -n 'THEN .route. MUST be .direct-inline.\|THEN .route. MUST be .delegated-direct.'` | 0 matches (exit 1) |
| No duplicated requirement names | `rg -h '^### Requirement:' <3 specs> \| sort \| uniq -d` | empty |
| Delta blocks byte-equal to merged blocks | Python byte compare of extracted blocks | `odd-lane: True`, `sdd-status: True`, `orchestrator REQ-OR-003 == delta: True` |
| Everything outside the edited block unchanged | Python byte compare vs `git show HEAD:<path>` | orchestrator outside-block: `True`; sdd-status prefix == HEAD + one blank line: `True` |
| Diff scope | `git diff --stat` | `orchestrator/spec.md` +20/−7; `sdd-status/spec.md` +24/−0 |

## Archive Move

`mv openspec/changes/add-odd-lane openspec/changes/archive/2026-09-20-add-odd-lane` (whole folder, no content edits).

- File set before/after: identical (10 files + 3 `specs/` domain dirs): `_meta.yaml`, `proposal.md`, `design.md`, `tasks.md`, `apply-progress.md`, `verify-report.md`, `state.yaml`, `specs/{odd-lane,orchestrator,sdd-status}/spec.md`.
- `openspec/changes/add-odd-lane/` is now absent; active changes directory is clean.
- `state.yaml` was moved intact and never edited — it still holds its persisted `pending_question` (`biggz-ai.pending-question/v1`) block.
- No `.biggz-instance` existed in the change folder; nothing extra was preserved or created.
- Post-archive hygiene (branch/worktree cleanup): **skipped** — this is a non-interactive sub-agent run, and the launch constraints prohibit branch operations. No branches or worktrees were deleted.

## Carried-Forward Warnings (verbatim from `verify-report.md`)

**WARNING-1 — OR-003-S1 `nextRecommended` clause is unsatisfiable as written (pre-existing, not introduced here)**
> The delta requires `nextRecommended` MUST be empty for organic direct work, but `resolveNextRecommended` (`internal/sdd/status.go:1215-1222` + `internal/sdd/status_helpers2.go:67-107`) never returns `""` for an active change, and change-less organic work has no `active` entry to carry `route`/`subroute` at all (design D4: "change-less organic subroute is unobservable by design"). Fresh harness: organic fixture → `route: organic`, `subroute: direct-inline`, `nextRecommended: spec`; empty change dir → `nextRecommended: propose`; change-less → `active: null`. The clause is inherited verbatim from the baseline spec (`openspec/specs/orchestrator/spec.md:672-676`); this change did not touch `deriveRoute`/`resolveNextRecommended`. Row OR-003-S1 is ⚠️ PARTIAL. Recommend a follow-up spec-wording fix (drop or restate the clause) — no code defect.

- **Follow-up**: a future change must drop or restate the `nextRecommended MUST be empty` conjunct in `REQ-OR-003` scenario 1. It was merged verbatim here per the explicit authorization and MUST NOT be silently rewritten during archive.

**WARNING-2 — Verification Map gap list under-reports textual-only rows (documentation accuracy, no code impact)**
> The declared note is accurate for ODD-004-S2, ODD-002-S2 and ODD-003-S1/S2, but ODD-001-S2 and ODD-002-S1 also have no executable proof; the only related test (`TestOddLaneContract`) asserts the shipped doc/skill text, not runtime document creation or the focused question. Add them to the map's gap list if the prose-only contract is accepted.

- **Follow-up**: any future verification map must list **ODD-001-S2** and **ODD-002-S1** as textual-only alongside ODD-004-S2 / ODD-002-S2 / ODD-003-S1/S2. Documentation only; the archived `verify-report.md` already records the corrected, expanded list.

## Delivery Facts

| Slice | PR | Merge commit |
|-------|----|--------------|
| 1 — ODD contract surfaces | #155 | `b58cae6a` |
| 2a — Read-only ODD array | #156 | `3a66a8b4` |
| 2b — Status/CLI ODD surface | #157 | `0cc43302` |
| 3 — Declared subroute | #158 | `99743506` |

Master head at archive time: **`99743506`** (merge of PR #158).

## Rollback Boundary

Reverting the archive delivery commit removes the new `openspec/specs/odd-lane/` file, restores the two modified spec files (`openspec/specs/orchestrator/spec.md`, `openspec/specs/sdd-status/spec.md`) to their pre-sync contents, and deletes the `openspec/changes/archive/2026-09-20-add-odd-lane/` folder. The four already-merged delivery PRs (#155–#158) are unaffected by reverting the archive commit.

## Cycle Summary

- Sync: 5 requirements added, 1 modified, 0 removed across 3 domains.
- Archive: folder moved with all artifacts; task completion 18/18 confirmed from the persisted artifact before the move.
- Updated sources of truth: `openspec/specs/odd-lane/spec.md` (new), `openspec/specs/orchestrator/spec.md` (REQ-OR-003), `openspec/specs/sdd-status/spec.md` (REQ-SS-ODD-001).
- SDD cycle: **complete**.
