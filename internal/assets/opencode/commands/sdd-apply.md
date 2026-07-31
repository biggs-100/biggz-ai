---
description: Implement SDD tasks — writes code following specs and design
agent: biggz-orchestrator
subtask: true
---

You are the `biggz-orchestrator`, not an SDD executor. This command is allowed to launch the hidden `sdd-apply` sub-agent only after the orchestration gates below pass.

CONTEXT:

- Working directory: before doing anything else, run `git rev-parse --show-toplevel 2>/dev/null || pwd` with your bash tool and use the returned path as the authoritative workspace. In OpenCode Desktop (Electron) the parse-time interpolation resolves to the app data directory, not the project.
- Current project: the `basename` of the detected workspace above.

HARD GATES:

1. SDD Session Preflight must already be complete for this session. It must include execution mode, artifact store, chained PR strategy, and review budget. If missing, ask the exact orchestrator preflight prompt and STOP. Do not run apply in the same turn.
2. `sdd-init` must already exist or be run after preflight, per the orchestrator init guard.
3. Resolve the active change using the status contract. If `$ARGUMENTS` is missing or ambiguous, ask the user to choose and STOP. Do not guess.
4. Produce structured status before acting and use it to confirm the active change has spec, design, and tasks artifacts in the selected artifact store.
5. Review workload guard must have passed. If task forecast exceeds the session review budget or needs a chained-PR decision, ASK and STOP unless the preflight strategy already resolves it.
6. actionContext must allow implementation edits. If status reports `workspace-planning` with no allowed edit roots, STOP before launching apply.

DEPENDENCY CHECK:

- If spec, design, or tasks are missing, do NOT implement.
- Tell the user this is not ready for apply and suggest `/sdd-new <change>` or `/sdd-ff <change>`.

TASK:
If all gates pass, launch the hidden `sdd-apply` sub-agent with:

- The resolved artifact store from session preflight; do not hardcode BigMem.
- The structured status: schemaName, planningHome/changeRoot, artifactPaths/contextFiles, task progress, applyState, dependency states, and actionContext.
- References to the spec, design, tasks, and any apply-progress artifacts.
- The resolved delivery/chained PR strategy and review budget.
- Strict TDD instructions if `sdd-init` detected strict TDD.

Return a structured orchestration result with: status, executive_summary, artifacts, next_recommended, risks, and skill_resolution.

POST-APPLY REVIEW ROUTING:
After apply returns, rerun native SDD status. If `nextRecommended: review`, the parent orchestrator begins native review routing with `biggz review list` followed by `biggz review status <lineage> --json` for the change's bound lineage (from `biggz sdd-attempt status <change>` binding fields). Read only the returned state and route only from it: for an in-review lineage, complete the exact operation the state names (`biggz review resume <lineage>` or `biggz review start --subject <file>` / `biggz review validate <lineage>`); for an approved receipt, run the terminal gate `biggz review gate pre-pr|pre-push <lineage> --json`; for `stop`, stop without running a lifecycle operation. The parent never substitutes direct START, and the apply executor never launches review.

### Authority-First Terminal Procedure

Use only the native biggz review facade; it appends and reads back native authority (`.git/biggz/review-transactions/`) before materializing existing compatibility artifacts.

| Order | Operation | Required result | Terminal mirrors |
|---|---|---|---|
| 01 | `biggz review list`; `biggz review status <lineage> --json` (lineage resolved from the change's runtime binding) | one provider-owned lineage returned with `chain_valid: true` and a receipt or valid `integrity_verdict`; `receipt` present means approved | blocked |
| 02 | provider-returned transition (`biggz review resume <lineage>` / `biggz review start --subject <file>` / `biggz review validate <lineage>`) | exact `execute` operation/arguments completed or `collect` inputs satisfied; `stop` halts without a lifecycle operation | blocked |
| 03 | repeat 01–02 | `biggz review gate pre-pr|pre-push <lineage> --json` returns `passed: true` against the same receipt; exit 1 with reasons blocks the terminal gate | blocked |
| 04 | `reconcile-terminal-mirrors` (`biggz review bind-sdd <change> <lineage> <revision>`; refresh BigMem `sdd/{change}/review/gate-context`) | existing mirrors reconciled to the native store | allowed |

After ambiguous output, query STATUS again; native discovery reports the committed authority and its next transition without another budget. Malformed or ambiguous lineage remains invalid.

Reuse a valid receipt; later commit/push/PR/release events only validate it.
