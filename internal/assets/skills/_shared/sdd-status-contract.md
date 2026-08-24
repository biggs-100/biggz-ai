# SDD Native Status Contract (biggz-ai)

Authoritative contract for resolving SDD change status in biggz-ai. The
orchestrator routes ONLY by the derived structured fields below; never infer
routing from free text.

## Native Dispatcher Projection

`biggz sdd-status` runs from the repo root and scans `openspec/changes/`
(`--cwd <dir>` selects a different root). It emits a human-readable
projection; `--json` emits the structured envelope below, and
`--instructions` additionally renders the per-phase instruction blocks.

```
RDD status: disabled (unmanaged)              # only when review is disabled
Active changes:
  ✅ <name> — [phase]
  ⬜ <name> — [phase]
No active changes.                             # when the list is empty

Recent archived:
  • <name> (phase)
```

Phase values emitted by the engine are nextRecommended-aware: planning
routes render as `[next: propose|spec|design|tasks]`, runtime phases as
`[next: apply|verify|remediate|archive]`, archived as `(done)`, and
`resolve-blockers` lists the blocked reasons inline. The historical
file-probe chain (`explore/proposal` → `spec` → `design` → `tasks` →
`apply (N/M tasks)` → `verify` → `archive-ready`) remains the fallback when
no derivation ran.

### Legacy read-compat fields

The per-change booleans/counters `Name`, `HasProposal`, `HasSpecs`,
`HasDesign`, `HasTasks`, `TasksTotal`, `TasksDone`, `HasApply`, `HasVerify`,
`IsArchived` (PascalCase wire names; documented historically as
`has_proposal`, `tasks_total`, `tasks_done`, `has_verify`, `is_archived`)
plus `granted_roots`, `edit_authority_blocked`, `missing_roots`, `consent`
remain emitted for read-compatibility with existing consumers. They are
file-probe approximations: `HasSpecs` is true for any non-empty `specs/`
directory and the legacy task counters recognize only `- [` checkbox
prefixes. **Do not route on them** — route ONLY on the derived fields
below, which are the authority.

Archives are `openspec/changes/archive/<name>/`; the engine shows the last 3.

`biggz sdd-continue <change>` (from the repo root) confirms the same phase
chain for one change: `proposal → spec → design → tasks → apply → verify →
archive`, printed as `Change: <name>`, `Next phase: <phase>`,
`Description: <phase description>`. Exit 1 with `change "<name>" not found`
if the change does not exist.

The dispatcher (`biggz sdd-status --json --instructions`) is now authoritative
for both `openspec` and `BigMem` via the native hybrid merge
(`internal/sdd/engram_status.go` port of gentle-ai's `resolveEngramStatus`):
it scans `openspec/changes/` on the filesystem and merges in BigMem
observations (`sdd/{change}/{proposal,spec,design,tasks,apply-progress,verify-report,archive-report}`)
with filesystem winning on name conflict. Invoke the dispatcher for
`openspec`, `BigMem`, and `hybrid` alike and treat its JSON as the single
authority; the manual BigMem schema below is now the fallback only when
the binary is unavailable.

## Derived Structured Status (what prompts consume)

`biggz sdd-status --cwd <root> --json` derives the structured status natively
in Go (ported from gentle-ai's `sdd-status --json --instructions` derivation
authority) and emits every active change plus the last 3 archived:
`{"active": [...], "archived": [...], "review_disabled": ...}`.
Schema name: `biggz-ai.sdd-status/v1`.

Derived fields emitted per change (camelCase):

| Field | Type | Meaning |
|---|---|---|
| `schemaName` | string | `biggz-ai.sdd-status/v1` |
| `schemaVersion` | int | `1` |
| `changeName` / `Name` | string | change directory name (legacy `Name` key) |
| `changeRoot` | string | `openspec/changes/<change_name>/` |
| `planningHome` | object | `{mode: repo-local, path: openspec/ root}` |
| `artifactPaths` | object | artifact → path list (proposal, specs, design, tasks, applyProgress, verifyReport) |
| `contextFiles` | object | same as `artifactPaths` — read these before acting |
| `artifacts` | map | artifact → `missing` \| `partial` \| `done` |
| `taskProgress` | object | `{total, completed, pending, allComplete}` |
| `dependencies` | object | per phase: `blocked` \| `ready` \| `all_done` |
| `applyState` | string | `blocked` \| `ready` \| `all_done` |
| `actionContext` | object | `{mode: repo-local, workspaceRoot, allowedEditRoots}` |
| `remediationState` | object | `{required, complete, failedEvidenceRevision, reason}` |
| `nextRecommended` | string | see routing below |
| `blockedReasons` | list | non-empty ⇒ stop; never proceed to apply/archive/terminal work |
| `phaseInstructions` | object | `--instructions` only; `{apply, verify, remediate, archive}` lists |

### artifact state derivation

| Value | Rule |
|---|---|
| `missing` | artifact path absent (specs: no `spec.md` found under `specs/`) |
| `partial` | exists but trimmed content empty (specs: any found `spec.md` empty) |
| `done` | non-empty content (specs: every found `spec.md` non-empty) |

`tasks.md` checkboxes count with the unified pattern
`^\s*(?:[-*]|\d+[.)])\s+\[([ xX])\]` (same pattern edit-authority detection
uses). `allComplete` is true iff total > 0 and pending == 0.

Spec counts are derived from `specs/**/spec.md` headings:
`### Requirement: ...` or `### REQ-<n>: ...` count as requirements,
`#### Scenario: ...` as scenarios. The verify report's totals must match.

### applyState derivation

| Value | Rule |
|---|---|
| `blocked` | proposal/specs/design/tasks not all done, or tasks list empty, or blocked by edit authority |
| `ready` | planning done, `0 < tasks_total` and pending > 0 |
| `all_done` | planning done and every checkbox complete |

`blocked` when any dependency missing for the current phase, or
`blockedReasons` non-empty.

### actionContext values

`mode: repo-local` with `workspaceRoot` (the openspec parent) and
`allowedEditRoots` = `[workspaceRoot] + granted_roots` (the per-change
granted edit authority). Apply edits are authorized only inside those roots.

### nextRecommended derivation (priority order)

1. `dependencies.apply == ready` → `apply`
2. `dependencies.verify == ready` → `verify`
3. apply `all_done` with a current verify report that is not `all_done` → `remediate` when `remediationState.required`; otherwise fall through (biggz has no review authority, so there is no `resolve-review` value)
4. `dependencies.verify == all_done` and apply `all_done` → `archive`
5. proposal not `all_done` → `propose`
6. specs not `all_done` → `spec`
7. design not `all_done` → `design`
8. tasks not `all_done` → `tasks`
9. otherwise → `resolve-blockers`
10. archived → `done`

`blockedReasons` non-empty overrides every value above: report the reasons
and STOP. Never proceed to apply, archive, or terminal work while it is
non-empty. Blocked reasons split into expected planning reasons (missing or
partial `proposal.md` / `specs/**/spec.md` / `design.md` / `tasks.md`),
which are hidden for planning routes and shown otherwise, and genuine
reasons (`tasks.md has no markdown task checkboxes.`,
`blocked(edit_authority_missing): ...`, and the remediation reason), which
are always shown.

### remediationState derivation

Unmanaged only (biggz has no review authority): when apply is `all_done`
and the current verify report fails evaluation,
`required: true` with `failedEvidenceRevision` (the report's
`evidence_revision`) and reason `verify evidence requires unmanaged
remediation for <rev>: <verify reason>; receipt-driven review is disabled,
so this correction is bounded by the native runtime attempt budget alone`.
Correction is bounded by the native runtime attempt ledger alone: when the
ledger's last attempt passed with `--remediates-evidence-revision` matching
the failed revision, the state clears, `dependencies.verify` becomes
`ready`, and next becomes `verify`.

## Routing Rules

- Route ONLY by `nextRecommended` and dependency states; never infer from free
  text.
- `blockedReasons` non-empty → stop; report the reasons.
- `nextRecommended: verify` → verification/remediation may run only to refresh
  evidence.
- `nextRecommended: resolve-blockers` → report `blockedReasons` and stop.
- `nextRecommended` planning token (`propose`, `spec`, `design`, `tasks`) →
  launch the corresponding planning phase.
- `nextRecommended: remediate` → run the bounded correction declared by
  `remediationState` (bind to `failedEvidenceRevision`, finish with
  `--remediates-evidence-revision`).
- `actionContext.mode: repo-local` with `allowedEditRoots` → apply edits are
  authorized only inside those roots; never launch apply/verify/archive work
  that would infer ownership outside them.
- If status cannot be resolved safely, return `status: blocked` with the
  missing information.

## Divergences from gentle-ai

- **No `select-change` value**: biggz lists EVERY change in the envelope
  (active + archived) with its own derived status, so there is no ambiguity
  point and no `select-change` `nextRecommended`; consumers pick by change
  name from `active`/`archived`.
- **No `sdd-new` value**: an empty changes directory yields an empty
  `active` list, not a status object.
- **No `review`, `resolve-review`, or `reviewGate` values**: biggz has no
  review authority on the SDD path; apply-done-with-failed-verify routes to
  `remediate` (unmanaged, bounded by the runtime attempt ledger) and the
  resolve-review exit is skipped entirely.
- **No stale-evidence machinery**: a totals mismatch against the current
  spec counts is simply a failing verify evaluation (`does not match actual
  requirement/scenario count`), not a separate "stale" classification.
- **`state.yaml` is deprecated**: `sdd-new` still writes it for
  skill-documentation compatibility, but status derivation NEVER reads it —
  every derived state comes from the file artifacts themselves.

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
| DAG state | `sdd/{change-name}/state` (legacy; never read by the native derivation) |

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

- `biggz sdd-status [--cwd <dir>] [--json] [--instructions]` — scans
  `openspec/` (current directory, or the `--cwd` root); `--json` emits the
  derived envelope, `--instructions` adds `phaseInstructions`.
- `biggz sdd-continue <change>` — legacy phase-chain projection for one
  change.
- `biggz sdd-attempt status <change>` — runtime ledger: revision, next action,
  active attempt, decision-required, complete, binding lineage/revision.
- `biggz sdd-verify-validate --input <path> [--requirements N] [--scenarios N]`
  — strict verify-report admission.
- If `biggz` is unavailable, use the Manual Status Schema above.
