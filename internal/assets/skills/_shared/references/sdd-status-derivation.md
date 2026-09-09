# SDD Status Derivation Reference

Companion to `../sdd-status-contract.md` (the authority for routing). Read this file
only when reconstructing status manually, when a field's derivation is unclear,
or when the binary is unavailable / the store is BigMem-backed.

## Derived Structured Status (what prompts consume)

`biggz sdd-status --cwd <root> --json` derives the structured status natively
in Go (ported from gentle-ai's `sdd-status --json --instructions` derivation
authority) and emits every active change plus the last 3 archived:
`{"active": [...], "archived": [...], "review_disabled": ...}`.
Schema name: `biggz-ai.sdd-status/v2`.

Derived fields emitted per change (camelCase):

| Field | Type | Meaning |
|---|---|---|
| `schemaName` | string | `biggz-ai.sdd-status/v2` |
| `schemaVersion` | int | `2` |
| `changeName` / `Name` | string | change directory name (legacy `Name` key) |
| `changeRoot` | string | `openspec/changes/<change_name>/` |
| `planningHome` | object | `{mode: repo-local, path: openspec/ root}` |
| `artifactStore` | string | `openspec` \| `engram` \| `none` |
| `artifactPaths` | object | artifact â†’ path list (proposal, specs, design, tasks, applyProgress, verifyReport) |
| `contextFiles` | object | same as `artifactPaths` â€” read these before acting |
| `artifacts` | map | artifact â†’ `missing` \| `partial` \| `done` |
| `taskProgress` | object | `{total, completed, pending, allComplete}` |
| `dependencies` | object | per phase: `blocked` \| `ready` \| `all_done` |
| `applyState` | string | `blocked` \| `ready` \| `all_done` |
| `actionContext` | object | `{mode: repo-local, workspaceRoot, allowedEditRoots}` |
| `relationships` | object | `{dependsOn, supersedes, amends, conflictsWith, sameDomainActiveChanges}` |
| `remediationState` | object | `{required, complete, failedEvidenceRevision, reason}` |
| `reviewOffer` | object | optional fresh offer `{available, invocation}` |
| `consent` | object | optional `biggz-ai.sdd-integration.consent/v1` envelope |
| `nextRecommended` | string | see routing below |
| `blockedReasons` | list | non-empty â‡’ stop; never proceed to apply/archive/terminal work |
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

1. `dependencies.apply == ready` â†’ `apply`
2. `dependencies.verify == ready` â†’ `verify`
3. apply `all_done` with a current verify report that is not `all_done` â†’ `remediate` when `remediationState.required`; otherwise fall through (biggz has no review authority, so there is no `resolve-review` value)
4. `dependencies.verify == all_done` and apply `all_done` â†’ `archive`
5. proposal not `all_done` â†’ `propose`
6. specs not `all_done` â†’ `spec`
7. design not `all_done` â†’ `design`
8. tasks not `all_done` â†’ `tasks`
9. otherwise â†’ `resolve-blockers`
10. archived â†’ `done`

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
  skill-documentation compatibility, but status derivation NEVER reads it â€”
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
| research | `sdd/{change-name}/research` (`biggz-ai.sdd-research/v1`) |
| preproposal | `sdd/{change-name}/preproposal` (`biggz-ai.sdd-preproposal/v1`) |
| review artifacts | `sdd/{change-name}/review/{transaction,ledger,receipt,gate-context}` |
| DAG state | `sdd/{change-name}/state` (legacy; never read by the native derivation) |

Field derivation is identical to the native projection:

| Field | Source |
|---|---|
| `change_name` | observed topic key |
| `phase` | first missing artifact in proposal â†’ spec â†’ design â†’ tasks â†’ apply â†’ verify â†’ archive |
| `state` | `pending` \| `in_progress` \| `completed` \| `blocked` |
| `tasks_total` / `tasks_done` | count `- [ ]` / `- [x]` lines in the tasks artifact |
| `artifact_states` | phase â†’ `missing` \| `exists` \| `complete` |
| `nextRecommended` | same priority chain as the native derivation |
| `blockedReasons` | missing required artifacts for the current phase |
| `actionContext` | `workspace-planning` unless the change has an explicit edit-root decision |
| `reviewGate` | from `sdd/{change-name}/review/gate-context` + `receipt` topics |

Archive detection: an `archive-report` exists and the change is no longer
active â†’ `phase: archive`, `state: completed`, `nextRecommended: done`.
