---
name: sdd-tasks
description: Break an SDD change into implementation tasks with dependencies, test evidence, and work units.
trigger: orchestrator launches task planning for a change.
---

# SDD Tasks

Break down the design into concrete, ordered implementation tasks. Each task must be specific, verifiable, and have a clear test evidence requirement.

## Activation Contract

1. Read design — understand architecture, file changes, and testing strategy.
2. Define tasks ordered by dependency.
3. Include test tasks alongside implementation tasks.
4. Estimate review workload and recommend PR strategy.
5. Each task must be independently verifiable.
6. Persist task artifact.

## Hard Rules

- Every task must be verifiable — a clear definition of "done" (test passes, file created, etc.).
- Implementation and test tasks for the same unit must be paired or adjacent in the task list.
- Review workload guard: if total estimate > 400 lines across files, recommend splitting into chained PRs.
- Never create a task that spans more than one logical concern (single responsibility per task).
- Every task must list the files it touches (create/modify/delete).
- Dependencies must be explicit (`depends_on: [TASK-1, TASK-3]`).

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Small change | 1-3 files, <100 lines | Single task, single PR |
| Medium change | 4-10 files, 100-400 lines | 3-8 tasks, single PR if well-structured |
| Large change | >10 files or >400 lines | Tasks grouped into work units, recommend chained PRs |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`. Apply **Section E (Review Workload Guard)**.
2. **Load design** — read `openspec/changes/{change-name}/design.md`. Extract file changes and testing strategy.
3. **Estimate workload** — count new files, modified files, test files. Estimate total lines.
4. **Define tasks** — create `openspec/changes/{change-name}/tasks.md`:
   ```yaml
   ---
   title: "{change-name} — Tasks"
   total_estimate: N files, N lines
   review_workload_estimate: N lines across N files
   recommendation: single-pr | split-pr | chained-prs
   ---
   ## TASK-1: {title}
   - **Description**: {what to do}
   - **Files**: `path/to/file.go` (create/modify/delete)
   - **Depends on**: none | TASK-N
   - **Test evidence**: {test name or assertion that proves completion}
   - **Definition of done**: {observable condition}

   ## TASK-2: {title}
   - **Description**: {what to do}
   - **Files**: `path/to/file.go` (create/modify/delete)
   - **Depends on**: TASK-1
   - **Test evidence**: {test name or assertion}
   - **Definition of done**: {observable condition}
   ```
5. **Phase grouping** (for large changes):
   - **Phase 1** (foundation): data structures, interfaces, base types
   - **Phase 2** (implementation): business logic, handlers, wiring
   - **Phase 3** (integration): wiring, startup, configuration
   - **Phase 4** (verification): tests, benchmarks, docs
6. **Persist** — write tasks file and Engram entry. Update `_meta.yaml` with `phase: tasks`.
7. **Recommend next phase** — apply.

## Output Contract

```yaml
status: success | blocked
executive_summary: "Defined 6 tasks across 2 phases. Total: 5 new files, 2 modified. ~350 lines."
artifacts:
  - path: openspec/changes/{change-name}/tasks.md
    type: task-plan
    summary: "Task checklist with dependencies and test evidence"
next_recommended: apply
risks:
  - description: "350 lines approaching review threshold — consider splitting if tests add significant size"
    severity: low
skill_resolution: auto
```

## References

- `../_shared/sdd-phase-common.md`
- `openspec/changes/{change-name}/design.md`
- `openspec/changes/{change-name}/tasks.md`
- `openspec/changes/{change-name}/_meta.yaml`
