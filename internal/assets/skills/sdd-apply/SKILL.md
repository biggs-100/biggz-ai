---
name: sdd-apply
description: Implement SDD tasks from specs, design, and task plan. Write code, run tests, and produce apply-progress report. Trigger: orchestrator launches apply for one or more change tasks.
license: MIT
metadata:
  author: biggz-ai
  version: '1.0'
---

# SDD Apply

Implement pending tasks from the task plan. Write code, match existing patterns, run tests, and produce an apply-progress report.

## Activation Contract

1. Read specs, design, and tasks to understand what to build.
2. Read existing code — match code style, patterns, and conventions.
3. Implement tasks in dependency order.
4. Run tests after each task (`go test ./...` or equivalent).
5. Mark tasks complete with evidence.
6. Persist apply-progress.
7. Return summary with task completion evidence.

## Hard Rules

- Read existing code before writing — match naming, file layout, error handling patterns.
- Run tests after EVERY task. Never batch test runs at the end.
- Never modify files outside the task's file list without explicit design amendment.
- If implementation reveals a design flaw, do NOT silently deviate — flag it.
- Each task completion must include specific evidence (test output, file content proof).
- All new public symbols must have documentation.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Design flaw | Implementation reveals spec gap or design error | Block, flag to user, suggest design revision |
| Test failure | Tests fail after task implementation | Fix before marking task done |
| Pattern mismatch | Existing code diverges from task expectations | Follow existing code patterns, note deviation |
| New unreachable | Implementation uncovers edge case not in spec | Document in apply-progress as discovered edge case |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Load artifacts** — read all of: `spec.md` (requirements), `design.md` (architecture), `tasks.md` (task list).
3. **Read existing code** — examine the files listed in the task. Understand naming, imports, error patterns, and test style.
4. **Implement tasks in order** — for each task:
   a. Create/modify files as specified in the task.
   b. Follow existing code conventions (naming, imports, error handling, test style).
   c. Run `go test ./...` (or the project's test command).
   d. If tests pass and task is complete, record evidence.
   e. If tests fail, fix until they pass before proceeding.
5. **Write apply-progress** — create/update `openspec/changes/{change-name}/apply-progress.md`:
   ```yaml
   ---
   phase: apply
   tasks_completed: 3/6
   tasks_blocked: 0
   ---
   ## Work Unit Evidence
   | Task | Status | Evidence | Files Touched |
   |------|--------|----------|--------------|
   | TASK-1 | done | `go test ./...` passes, type defined | `path/to/new.go` |
   | TASK-2 | done | TestCreateUser passes | `path/to/handler.go` |
   | TASK-3 | done | All db tests pass | `path/to/store.go` |
   | TASK-4 | pending | — | `path/to/router.go` |
   ```
6. **Commit suggestion** — if the user wants to commit, suggest work-unit-based commits matching task boundaries.
7. **Persist** — save apply-progress to file and Engram. Update `_meta.yaml` with `phase: apply`.
8. **Recommend next phase** — when all tasks done: verify. Otherwise: continue apply.

## Output Contract

```yaml
status: in-progress | done | blocked
executive_summary: "Completed 4 of 6 tasks. Tests passing. 2 tasks remaining."
artifacts:
  - path: openspec/changes/{change-name}/apply-progress.md
    type: apply-progress
    summary: "Task completion status with evidence per task"
next_recommended: apply | verify
risks:
  - description: "Remaining tasks touch shared router — may conflict with other in-flight changes"
    severity: medium
skill_resolution: auto
```

## PR Auto-Generation

When apply completes and all tasks are done, the orchestrator MAY create a PR:

```bash
biggz pr create <change-name> --title "feat: implemented X" --label "type:feature"
```

This:
1. Creates a branch named `type/change-name` (auto-detected from change name)
2. Stages and commits changes with a conventional commit message
3. Pushes to origin
4. Creates a PR with file list, test plan checklist, and `Closes #` placeholder
5. Adds specified labels

Branch type auto-detection:

| Change prefix | Branch type |
|---|---|
| fix, bug, hotfix | `fix/` |
| docs, doc, readme | `docs/` |
| refactor, cleanup | `refactor/` |
| chore, ci, build, test | `chore/` |
| (everything else) | `feat/` |

## References

- `../branch-pr/SKILL.md`
- `../_shared/sdd-phase-common.md`
- `openspec/changes/{change-name}/spec.md`
- `openspec/changes/{change-name}/design.md`
- `openspec/changes/{change-name}/tasks.md`
- `openspec/changes/{change-name}/apply-progress.md`
