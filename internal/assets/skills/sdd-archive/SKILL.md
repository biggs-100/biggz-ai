---
name: sdd-archive
description: Archive a completed SDD change by syncing delta specs, moving to archive, and producing archive report.
trigger: orchestrator launches archive after implementation and verification.
---

# SDD Archive

Finalize a completed change. Sync delta specs back into main domain specs, move the change to the archive, and produce an archive report.

## Activation Contract

1. Verify all artifacts are complete and verification passed.
2. Sync delta specs back to main spec files.
3. Move change artifacts to archive location.
4. Mark change as archived.
5. Clean up working artifacts (optional).
6. Persist archive report.

## Hard Rules

- Never archive a change with failing verification or incomplete tasks — block and demand verification.
- Delta spec merge must be lossless — preserve history of what was added.
- The archive must be readable and navigable for future reference.
- The original change directory under `openspec/changes/{change-name}` is the source of truth; archive is a copy.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Verification failed | verify-report shows failures | Block — do not proceed |
| Has delta spec | `spec.md` in change dir has `spec_type: delta` | Merge into domain spec in `openspec/specs/` |
| Has full spec | `spec.md` in change dir has `spec_type: full` | No merge needed — already at `openspec/specs/` |
| No verify report | `verify-report.md` missing | Block — run verify first |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Verify completeness** — read `_meta.yaml` for phase. If phase != `verify`, block. Read `verify-report.md` — if status is not `pass`, block.
3. **Load all artifacts** — gather `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `apply-progress.md`, `verify-report.md`.
4. **Sync delta specs** — if spec was a delta spec:
   - Read `openspec/changes/{change-name}/spec.md`.
   - Append ADDED requirements to the main domain spec at `openspec/specs/{domain}/spec.md`.
   - Add `[from: {change-name}]` annotation to each added requirement.
5. **Create archive** — copy change directory to `openspec/archive/{change-name}-{date}/`. Include all artifacts.
6. **Write archive report** — create `openspec/changes/{change-name}/archive-report.md`:
   ```yaml
   ---
   phase: archive
   archived_at: {timestamp}
   archived_to: openspec/archive/{change-name}-{date}
   specs_merged: true | false
   total_requirements_added: N
   ---
   ## Archive Summary
   - **Change**: {change-name}
   - **Description**: {from proposal}
   - **Duration**: {time from create to archive}
   - **Tasks completed**: N/N
   - **Tests**: N passed, 0 failed
   - **Specs merged**: {which specs and how many requirements}

   ## Artifacts
   - `openspec/archive/{change-name}-{date}/proposal.md`
   - `openspec/archive/{change-name}-{date}/spec.md`
   - `openspec/archive/{change-name}-{date}/design.md`
   - `openspec/archive/{change-name}-{date}/tasks.md`
   - `openspec/archive/{change-name}-{date}/apply-progress.md`
   - `openspec/archive/{change-name}-{date}/verify-report.md`

   ## Lessons
   - {any retrospective notes or guidance for future changes}
   ```
7. **Update metadata** — update `_meta.yaml` with `phase: archive` and `archived: true`.
8. **Clean up** — optionally remove the change directory from `openspec/changes/` (user decision).
9. **Persist** — write archive report to file and Engram.
10. **Recommend next step** — done (or start a new change).

## Output Contract

```yaml
status: success | blocked
executive_summary: "Archived change 'add-auth'. 3 delta requirements merged into auth spec."
artifacts:
  - path: openspec/changes/{change-name}/archive-report.md
    type: archive-report
    summary: "What was archived, what was merged, lessons"
  - path: openspec/archive/{change-name}-{date}/
    type: archive-directory
    summary: "Full artifact copy"
next_recommended: done
risks: []
skill_resolution: auto
```

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-archive.md`
- `openspec/changes/{change-name}/verify-report.md`
- `openspec/specs/`
- `openspec/archive/`
