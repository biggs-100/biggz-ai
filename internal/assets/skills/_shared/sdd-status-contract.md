# SDD Manual Status Schema Contract

When the native binary is unavailable or the backend is BigMem, the
orchestrator resolves status manually using this schema.

## Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `change_name` | string | SDD change name from directory |
| `phase` | string | Current phase: propose, spec, design, tasks, apply, verify, archive |
| `state` | string | pending, in_progress, completed, blocked |
| `tasks_total` | int | Total tasks (from tasks.md checklist) |
| `tasks_done` | int | Completed tasks (marked [x]) |
| `artifact_states` | map | Phase → state mapping for each artifact |

## Artifact Resolution

Check existence of these files under `openspec/changes/<name>/`:

| Phase | Artifact | State |
|-------|----------|-------|
| propose | `proposal.md` | exists → completed |
| spec | `specs/**/spec.md` | exists → completed |
| design | `design.md` | exists → completed |
| tasks | `tasks.md` | exists → completed |
| apply | `apply-progress.md` | Check [x]/[ ] ratio |
| verify | `verify-report.md` | exists → completed |
| archive | in `openspec/changes/archive/` | moved → completed |

## Next Phase Resolution

Priority order:
1. Missing `proposal.md` → propose
2. Missing `specs/**/spec.md` → spec
3. Missing `design.md` → design
4. Missing `tasks.md` → tasks
5. Tasks exist but not done → apply
6. Missing `verify-report.md` → verify
7. All exist → archive
