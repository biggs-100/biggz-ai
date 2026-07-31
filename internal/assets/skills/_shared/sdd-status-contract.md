# SDD Native Status Contract (biggz-ai)

Authoritative contract for resolving SDD change status in biggz-ai. The
orchestrator routes ONLY by the derived structured fields below; never infer
routing from free text.

## Native Dispatcher Projection

`biggz sdd-status` runs from the repo root and scans `openspec/changes/`
(no arguments, no JSON flags). It emits a human-readable projection:

```
RDD status: disabled (unmanaged)              # only when review is disabled
Active changes:
  ✅ <name> — [phase]
  ⬜ <name> — [phase]
No active changes.                             # when the list is empty

Recent archived:
  • <name> (phase)
```

Phase values emitted by the engine (derived from artifact existence):

| Phase string | Meaning |
|---|---|
| `explore/proposal` | `proposal.md` missing |
| `spec` | proposal exists, specs missing |
| `design` | specs exist, `design.md` missing |
| `tasks` | design exists, `tasks.md` missing |
| `apply (N/M tasks)` | tasks exist, `N` of `M` checklist items marked `[x]` |
| `verify` | all tasks done, `verify-report.md` missing |
| `archive-ready` | all artifacts present |

The engine computes this from per-change booleans/counters:
`name`, `has_proposal`, `has_specs`, `has_design`, `has_tasks`,
`tasks_total`, `tasks_done`, `has_apply`, `has_verify`, `is_archived`.
Archives are `openspec/changes/archive/<name>/`; the engine shows the last 3.

`biggz sdd-continue <change>` (from the repo root) confirms the same phase
chain for one change: `proposal → spec → design → tasks → apply → verify →
archive`, printed as `Change: <name>`, `Next phase: <phase>`,
`Description: <phase description>`. Exit 1 with `change "<name>" not found`
if the change does not exist.

The dispatcher reads ONLY OpenSpec file artifacts under `openspec/changes/`.
It cannot observe BigMem-backed changes and emits no structured JSON. Treat
the dispatcher as authoritative ONLY when the session artifact store is
`openspec` or `hybrid`. When the store is `BigMem`, do NOT invoke it at all —
resolve status entirely from BigMem using the manual schema below.

## Derived Structured Status (what prompts consume)

The orchestrator derives this structured status and forwards it to sub-agents.
Schema name: `biggz-ai.sdd-status/v1`.

| Field | Type | Meaning |
|---|---|---|
| `change_name` | string | change directory name |
| `schemaName` | string | `biggz-ai.sdd-status/v1` |
| `planningHome` | string | `openspec/` root relative to the workspace |
| `changeRoot` | string | `openspec/changes/<change_name>/` (file store) or BigMem namespace |
| `artifactPaths` | map | artifact → path (file store) or topic key `sdd/{change}/{type}` (BigMem) |
| `contextFiles` | list | paths/topic keys to read before acting |
| `applyState` | string | `not_started` \| `ready` \| `blocked` \| `all_done` |
| dependency states | map | per artifact: `missing` \| `exists` \| `complete` \| `blocked` |
| `blockedReasons` | list | non-empty ⇒ stop; never proceed to apply/archive/terminal work |
| `actionContext` | object | `{mode: workspace-planning \| change-local, workspaceRoot, allowedEditRoots}` |
| `reviewGate` | object | `{result: allow \| missing \| pending \| scope-changed \| invalidated \| escalated \| delivery: disabled/unmanaged}` |
| `nextRecommended` | string | `sdd-new` \| `propose` \| `spec` \| `design` \| `tasks` \| `apply` \| `verify` \| `review` \| `archive` \| `resolve-blockers` \| `done` |
| task progress | map | `{total, completed, pending, allComplete}` |

### applyState derivation

| Value | Rule |
|---|---|
| `not_started` | `tasks.md` exists, `tasks_done == 0` |
| `ready` | tasks exist and `0 < tasks_done < tasks_total`, or all done but `verify-report.md` missing |
| `all_done` | `tasks_done == tasks_total` AND `verify-report.md` exists |
| `blocked` | any dependency missing for the current phase, or `blockedReasons` non-empty |

### actionContext values

| `mode` | Meaning | `allowedEditRoots` |
|---|---|---|
| `change-local` | implementation edits allowed inside the change's edit roots | non-empty; edit ONLY under these roots |
| `workspace-planning` | planning context only; apply/verify/archive must NOT run | empty ⇒ STOP before any implementation edit |

### nextRecommended derivation (priority order)

1. No active change matches → `sdd-new`
2. `proposal.md` missing → `propose`
3. specs missing → `spec`
4. `design.md` missing → `design`
5. `tasks.md` missing → `tasks`
6. tasks exist, not all complete → `apply`
7. all tasks complete, `verify-report.md` missing → `verify`
8. verified, review binding/`reviewGate.result` not `allow` → `review`
9. `reviewGate.result == allow` (or delivery `disabled/unmanaged`) → `archive`
10. archived → `done`

`blockedReasons` non-empty overrides every value above: report the reasons and
STOP. Never proceed to apply, archive, or terminal work while it is non-empty.

### reviewGate derivation

Native review authority comes from the biggz review CLI:

- `biggz review list` — lineages and their last operation/state.
- `biggz review status <lineage> --json` — `lineage_id`, `head_hash`,
  `event_count`, `chain_valid`, `receipt` (present ⇒ approved),
  `integrity_verdict`, `budget_counters`. No `next_transition` is returned;
  route only from the returned state.
- `biggz review gate pre-pr|pre-push <lineage> --json [--dry-run]` — `gate`,
  `passed`, `reasons`, `dry_run`; exit 1 when not passed.
- `biggz review bind-sdd <change> <lineage> <revision>` — records the
  governing approved lineage in the change's runtime ledger
  (`.biggz/sdd-runtime/`); archive requires this binding.

Derive `reviewGate.result`: `allow` when an approved receipt exists and the
binding matches the change; `missing`/`pending` when no binding or no receipt;
`invalidated` when `chain_valid` is false; `escalated` when the change is
`Complete` or a successor lineage supersedes the binding. When RDD reports
`disabled (unmanaged)` and no review governs the change, use
`reviewGate.delivery: disabled/unmanaged`.

## Routing Rules

- Route ONLY by `nextRecommended` and dependency states; never infer from free
  text.
- `blockedReasons` non-empty → stop; report the reasons.
- `nextRecommended: verify` → verification/remediation may run only to refresh
  evidence.
- `nextRecommended: resolve-blockers` → report `blockedReasons` and stop.
- `nextRecommended` planning token (`propose`, `spec`, `design`, `tasks`) →
  launch the corresponding planning phase.
- `actionContext.mode: workspace-planning` with no `allowedEditRoots` → never
  launch apply/verify/archive work that would infer repo-local ownership.
- If status cannot be resolved safely, return `status: blocked` with the
  missing information.

## Manual Status Schema (BigMem fallback)

Used when the binary is unavailable OR the session artifact store is `BigMem`
(the dispatcher cannot see BigMem-backed changes).

Resolve artifacts with `biggz_mem_search` + `biggz_mem_get_observation` on the
change's topic keys (prefix `sdd/{change-name}/`):

| Artifact | Topic key |
|---|---|
| init context | `sdd-init/{project}` |
| proposal | `sdd/{change-name}/proposal` |
| spec | `sdd/{change-name}/spec` |
| design | `sdd/{change-name}/design` |
| tasks | `sdd/{change-name}/tasks` |
| apply progress | `sdd/{change-name}/apply-progress` |
| verify report | `sdd/{change-name}/verify-report` |
| archive report | `sdd/{change-name}/archive-report` |
| review artifacts | `sdd/{change-name}/review/{transaction,ledger,receipt,gate-context}` |
| DAG state | `sdd/{change-name}/state` |

Field derivation is identical to the native projection:

| Field | Source |
|---|---|
| `change_name` | observed topic key |
| `phase` | first missing artifact in proposal → spec → design → tasks → apply → verify → archive |
| `state` | `pending` \| `in_progress` \| `completed` \| `blocked` |
| `tasks_total` / `tasks_done` | count `- [ ]` / `- [x]` lines in the tasks artifact |
| `artifact_states` | phase → `missing` \| `exists` \| `complete` |
| `nextRecommended` | same priority chain as the native derivation |
| `blockedReasons` | missing required artifacts for the current phase |
| `actionContext` | `workspace-planning` unless the change has an explicit edit-root decision |
| `reviewGate` | from `sdd/{change-name}/review/gate-context` + `receipt` topics |

Archive detection: an `archive-report` exists and the change is no longer
active → `phase: archive`, `state: completed`, `nextRecommended: done`.

## Binary Availability

- `biggz sdd-status` / `biggz sdd-continue <change>` — run from the repo root
  (both read `openspec/` in the current directory; neither supports `--cwd`).
- `biggz sdd-attempt status <change>` — runtime ledger: revision, next action,
  active attempt, decision-required, complete, binding lineage/revision.
- `biggz sdd-verify-validate --input <path> [--requirements N] [--scenarios N]`
  — strict verify-report admission.
- If `biggz` is unavailable, use the Manual Status Schema above.
