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

## Native Engine

Native `biggz-ai.sdd-status/v2` is the sole status contract. A request for `v1`
or another prior contract fails read-only with one instruction: start a fresh
implementation state and rerun `biggz sdd-status --contract biggz-ai.sdd-status/v2`.
When status recommends `propose`, the orchestrator-owned pre-proposal gate
separately requires confirmed decisions, valid evidence references, and matching
hybrid state; selected research must be `done`.

- When the session artifact store is `openspec` or `hybrid` and the `biggz` binary
  is available, prefer `biggz sdd-status [change] --cwd <repo> --json --instructions`
  for read-only status and `biggz sdd-continue [change] --cwd <repo>` for
  dispatcher output. When the store is `engram`, do not invoke those OpenSpec
  dispatcher commands.
- The native dispatcher reads only OpenSpec file artifacts and emits
  `artifactStore: openspec`; it cannot observe Engram-backed changes. Treat
  dispatcher status as authoritative only when the selected artifact store is
  `openspec` or `hybrid`. When the selected store is `engram`, resolve artifact
  status from Engram (`biggz_mem_search` + `biggz_mem_get_observation` on the
  change topic keys) using the manual schema below.
- Runtime-attempt authority is different from artifact dispatch: normal
  runtime-bearing OpenSpec and Engram continuations MUST bracket external
  execution with `biggz sdd-attempt acquire|settle --cwd <repo> --change <change>`.
  Their bounded result contains only `proceed`, `blocked`, or `complete` plus an
  opaque continuation token when required, and MAY carry `settle_obligation` on a
  `proceed`. The Git-common-dir immutable chain remains the sole authority for
  ordinals, cumulative attempt/line budgets, runtime evidence, and ordinary SDD
  failed-evidence remediation. Full `status|begin|finish|reset` payloads MUST NOT
  be embedded in the SDD v2 status document.
- A phase actor launched by a parent that already holds a `proceed` acquire for
  that exact work unit authenticates as that same attempt with the returned
  `--token`; it MUST NOT acquire again blind.
- When `sdd-attempt status` carries a `biggz-ai.sdd-integration.consent/v1`
  consent block, the ledger is ASKING, not reporting. Treat it as a Lossless
  Blocking Prompt: relay the complete envelope in order, preserve answer tokens
  and invocations, and never answer on their behalf. In a non-interactive
  runtime, emit the complete envelope and STOP.
- For `openspec` and `hybrid` stores, treat native status JSON as authoritative
  over prompt inference or manually reconstructed state.
- When `blockedReasons` is non-empty, do not proceed to terminal, archive, or
  apply work. Return or report `blockedReasons` and stop unless `nextRecommended`
  is `verify`, in which case verification may run only to remediate or refresh
  evidence for the blockers. When `nextRecommended` is `resolve-blockers`,
  always report `blockedReasons` and stop. When `nextRecommended` is a planning
  token (`propose`, `spec`, `design`, or `tasks`), launch the corresponding
  planning phase — missing planning artifacts are the expected output of those
  phases, not genuine blockers.
- `nextRecommended` is a bounded machine token for routing, not human prose.
  Route only by `nextRecommended` and dependency states. Human-readable
  explanation belongs in `blockedReasons`.
- If the binary is unavailable, fall back to this prompt contract and the
  manual status schema below. Manual fallback status MUST stay shape-compatible
  with native `biggz-ai.sdd-status/v2` JSON even when values are reconstructed
  manually.

## Status Schema

Return status as markdown with these fields, or as equivalent JSON when the
host supports it. This is the exact frozen external `StatusV2Projection`, not
the extensible internal aggregate:

```yaml
schemaName: biggz-ai.sdd-status
schemaVersion: 2
changeName: <change-name-or-null>
artifactStore: openspec | engram | none
planningHome:
  mode: repo-local
  path: <absolute path to openspec>
changeRoot: <absolute path to openspec/changes/<change> or null>
artifactPaths:
  proposal: [<absolute path>]
  specs: [<absolute paths>]
  design: [<absolute path>]
  tasks: [<absolute path>]
  applyProgress: [<absolute path>]
  verifyReport: [<absolute path>]
contextFiles:
  proposal: [<absolute readable files>]
  specs: [<absolute readable files>]
  design: [<absolute readable files>]
  tasks: [<absolute readable files>]
  applyProgress: [<absolute readable files>]
  verifyReport: [<absolute readable files>]
artifacts:
  proposal: missing | done | partial
  specs: missing | done | partial
  design: missing | done | partial
  tasks: missing | done | partial
  applyProgress: missing | done | partial
  verifyReport: missing | done | partial
taskProgress:
  total: 0
  completed: 0
  pending: 0
  allComplete: false
dependencies:
  proposal: blocked | ready | all_done
  specs: blocked | ready | all_done
  design: blocked | ready | all_done
  tasks: blocked | ready | all_done
  apply: blocked | ready | all_done
  verify: blocked | ready | all_done
  archive: blocked | ready | all_done
applyState: blocked | all_done | ready
actionContext:
  mode: repo-local
  workspaceRoot: <absolute path>
  allowedEditRoots: [<absolute paths>]
relationships:
  dependsOn: []
  supersedes: []
  amends: []
  conflictsWith: []
  sameDomainActiveChanges: []
remediationState:
  required: false
  complete: false
  failedEvidenceRevision: ""
  reason: ""
reviewOffer:
  available: true
  invocation: <fresh review start command>
consent: <optional exact biggz-ai.sdd-integration.consent/v1 envelope>
phaseInstructions:
  apply: [<instruction strings>]
  verify: [<instruction strings>]
  remediate: [<instruction strings>]
  archive: [<instruction strings>]
nextRecommended: propose | spec | design | tasks | apply | verify | remediate | archive | sdd-new | select-change | resolve-blockers
blockedReasons: []
```

`reviewOffer` is optional and appears only after strict independent verification
passes while review mode is enabled. It is a fresh mode-only offer with exactly
`available` and `invocation`; it carries no lineage, receipt, binding, successor,
gate, transaction, or previous review result. Disabled review mode is structural
absence. Repeated status reads may present the same fresh offer and no offered,
declined, burned, or historical authority changes archive readiness.

`phaseInstructions` is optional and appears only when instructions are
requested. It carries execution-phase keys (`apply`, `verify`, `remediate`,
`archive`); planning-phase instructions (`propose`, `spec`, `design`,
`tasks`) are surfaced in dispatcher markdown. `consent` is structurally absent
everywhere except an OpenSpec-backed native status that reports
`blocked(edit_authority_missing)`; manual fallback MUST NOT reconstruct it.
Empty path fields MUST be arrays, not null. `changeName` and `changeRoot` are
nullable; all other non-optional sections should be present in fallback output
so consumers can parse native and manual status the same way.

## Apply State

- `blocked`: Required apply artifacts are missing, task selection is ambiguous, or action context makes edits unsafe.
- `all_done`: Tasks artifact exists and every implementation task is checked `[x]`.
- `ready`: Tasks artifact exists, at least one implementation task remains unchecked, and edit scope is safe.

## Dependency States

- `proposal`, `specs`, `design`, and `tasks` report whether prerequisite artifacts are blocked, ready, or all done.
- `apply` is `ready` only when specs, design, and tasks are available and task progress is not all done.
- `verify` is `ready` only when every implementation task is complete and required planning/apply evidence is available. Review presence, absence, or non-allow state is informational: it never routes status to `review`, suppresses test/build execution, or blocks verification. Apply-progress and focused work-unit checks support implementation evidence but never replace the independent final SDD verification.
- Verify routing parses only the strict leading `biggz-ai.verify-result/v1` envelope. It compares measured requirement/scenario totals with actual specs and requires current test/build commands, zero passing exit codes, and output hashes. Human prose never controls readiness.
- Failed evidence may route to `remediate` only through ordinary SDD failed-evidence accounting for the same failed evidence revision. Remediation completion requires concrete focused-test, runtime-harness (or justified N/A), and rollback evidence; a bare envelope never passes.
- `archive` is `ready` only when tasks are complete and strict SDD verification passes. A `reviewOffer` never authorizes, blocks, or governs archive or delivery.
- A passing remediation settlement requires a fresh verification report before archive. The historical failed report is preserved and never erased, no PASS is fabricated, and archive stays blocked until a current passing report exists.
- Before a runtime-bearing continuation, call compact `sdd-attempt acquire` with `<acquire-id>` and launch only for `state: proceed`; retain its opaque token and call compact `sdd-attempt settle` after the external run with a distinct `<settle-id>`. Reuse each operation's own request ID only for its idempotent replay. `blocked` or `complete` stops the launch, and settle's three states alone control whether another bounded acquire is allowed. When acquire returns `settle_obligation`, RELAY IT TO THE HUMAN VERBATIM BEFORE LAUNCHING THE WORK UNIT, and carry it into the settle. It is never a block — the token is real and the launch proceeds. Reset remains an explicit maintainer scope decision and never occurs automatically.
- Planning and apply phases never auto-launch ordinary 4R or Judgment Day. Only after independent SDD verification passes may status present the optional review offer. Pre-commit, pre-push, pre-PR, and release follow ordinary repository policy; review outcomes never create a delivery gate or a new review budget.

## Action Context Guard

The orchestrator MUST carry `actionContext` into any phase launch.

- If manually reconstructed context cannot prove edit ownership or allowed edit roots, stop before editing.
- If `allowedEditRoots` is present, only edit files within those roots.
- If a command cannot prove a file is inside the authoritative workspace or allowed edit roots, stop and ask for clarification.

## Edit Authority Consent

A change whose tasks.md work units target paths outside `allowedEditRoots` never reports apply ready. Native status reports `applyState: blocked` and `blockedReasons` carries a `blocked(edit_authority_missing)` reason naming each unauthorized edit root and both exits: edit tasks.md so every work unit stays inside the authorized edit roots, or grant this change edit authority for the named edit roots.

- Detection is conservative prose inspection: backticked path-like tokens inside markdown checkbox lines that resolve to a path in a Git repository outside the authorized roots. A different repository is named by its Git root; a same-repository target is narrowed to its containing edit root. A context reference can raise a false consent question; the consequence is a question, never silent authority.
- An OpenSpec-backed native status that reports `blocked(edit_authority_missing)` also carries the typed `biggz-ai.sdd-integration.consent/v1` envelope as the optional `consent` block: headline, reason, `value`, the missing roots as evidence, exactly two choices with answer tokens `granted` and `declined` (each with label, effect, and an exact invocation), and an off-path note.
- Answer flow: the orchestrator relays the COMPLETE envelope losslessly as a blocking prompt. Only on the human's explicit `granted` answer does the agent execute the envelope's named grant invocation, verbatim and exactly once, then re-enter through native status. The agent NEVER runs the grant unprompted and NEVER answers on the human's behalf.
- Decline stays blocked: the agent runs the envelope's decline invocation, nothing is persisted, the change stays `blocked(edit_authority_missing)`, and the reason names both exits.

## Status Output

Every command that acts on a change MUST show status before launching an executor or performing archive work:

- Active change selection and schemaName.
- Artifact statuses and paths/topics used as context.
- Task progress and unchecked task list when tasks exist.
- Next recommended action.
- `blockedReasons` when `nextRecommended` is not `verify`, plus any edit-root blockers.

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
| `artifactPaths` | object | artifact → path list (proposal, specs, design, tasks, applyProgress, verifyReport) |
| `contextFiles` | object | same as `artifactPaths` — read these before acting |
| `artifacts` | map | artifact → `missing` \| `partial` \| `done` |
| `taskProgress` | object | `{total, completed, pending, allComplete}` |
| `dependencies` | object | per phase: `blocked` \| `ready` \| `all_done` |
| `applyState` | string | `blocked` \| `ready` \| `all_done` |
| `actionContext` | object | `{mode: repo-local, workspaceRoot, allowedEditRoots}` |
| `relationships` | object | `{dependsOn, supersedes, amends, conflictsWith, sameDomainActiveChanges}` |
| `remediationState` | object | `{required, complete, failedEvidenceRevision, reason}` |
| `reviewOffer` | object | optional fresh offer `{available, invocation}` |
| `consent` | object | optional `biggz-ai.sdd-integration.consent/v1` envelope |
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
| research | `sdd/{change-name}/research` (`biggz-ai.sdd-research/v1`) |
| preproposal | `sdd/{change-name}/preproposal` (`biggz-ai.sdd-preproposal/v1`) |
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

## Research and Pre-Proposal Gate

Selected research is mandatory. The orchestrator MUST offer `sdd-research`
after `sdd-explore` and treat selection as mandatory. Before every `propose`,
invoke `sdd-propose` only when selected research is `done` or research is
unselected, product decisions are `confirmed`, evidence references are valid,
and the selected artifact-store state is ready. Unresolved choices require one
lossless grouped prompt with all context, options, consequences, allowed
answers, and exact tokens; it MUST persist the pending state before prompting,
then STOP without invoking `sdd-propose`. The proposer receives a confirmed
pre-proposal handoff and MUST NOT interview or infer consent. Native
`biggz-ai.sdd-status/v2` is the sole contract; `v1` is retired. See
`skills/_shared/research-lifecycle.md` for the full contract
(`biggz-ai.sdd-research/v1` and `biggz-ai.sdd-preproposal/v1`).

## Binary Availability

- `biggz sdd-status [--cwd <dir>] [--json] [--instructions] [--contract <contract>]` — scans
  `openspec/` (current directory, or the `--cwd` root); `--json` emits the
  derived envelope, `--instructions` adds `phaseInstructions`, `--contract`
  selects the status contract (default `biggz-ai.sdd-status/v2`; `v1` fails
  with fresh-v2 rerun instruction).
- `biggz sdd-continue <change>` — legacy phase-chain projection for one
  change.
- `biggz sdd-attempt status <change>` — runtime ledger: revision, next action,
  active attempt, decision-required, complete, binding lineage/revision.
- `biggz sdd-verify-validate --input <path> [--requirements N] [--scenarios N]`
  — strict verify-report admission.
- If `biggz` is unavailable, use the Manual Status Schema above.
