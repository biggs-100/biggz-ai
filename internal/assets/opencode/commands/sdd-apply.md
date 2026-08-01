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

POST-APPLY REVIEW ROUTING (NEGOTIATED CONTRACT):
After apply returns, rerun native SDD status. If `nextRecommended: review`, the parent orchestrator begins native review routing by querying the change's bound lineage (from `biggz sdd-attempt status <change>` binding fields) with:

`biggz review status <lineage> --contract biggz-ai.review-integration/v1 --next-transition`

Route ONLY from the returned `next_transition` envelope — never from prose, raw status fields, or eligibility:

| envelope | what the orchestrator does |
|---|---|
| `collect` (inputs.capture) | satisfy the named capture input with its exact capture operation: run `biggz review capture-result --preflight` with the input's `lineage`/`target`/`lens`/`order`/`expected_revision` (and `repository_context` when issued) to derive `subject_hash`, run the reviewer, capture with the same binding, then query STATUS again |
| `execute` | invoke the exact operation with the ordered argument tokens unchanged: `finalize <lineage>`, or `resume <lineage> --correction-lines <budget_remaining>` (the offered forecast is the max allowed; a lower one may be executed). Then query STATUS again |
| `stop` | stop and surface `reason_code` without running a lifecycle operation. `ready_for_gates` means the receipt exists: run the lifecycle gates (`biggz review gate <kind> <lineage>`) when the lifecycle demands, validating the same receipt |

CONSENT RELAY: when a start needs consent, `biggz review start --subject <file> --contract biggz-ai.review-integration/v1` prints the typed consent envelope; every choice carries the exact follow-up invocation for that answer (original flags echoed, frozen candidate lineage pinned). Relay the envelope complete to the human, run EXACTLY the one named invocation (`--consent granted|declined`, scoped to that candidate), then query STATUS again. Never answer on behalf of the human. The parent never substitutes direct START, and the apply executor never launches review.

Reuse a valid receipt; later commit/push/PR/release events only validate it.
