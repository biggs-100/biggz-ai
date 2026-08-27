---
name: sdd-apply
description: "Implement SDD tasks from specs, design, and task plan. Write code, run tests, and produce apply-progress report. Trigger: orchestrator launches apply for one or more change tasks."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "3.0"
  delegate_only: true
---
<!-- section:model-capable -->
## Language Domain Contract

Generated technical artifacts default to English. Do not inherit the user's conversational language or the active persona's regional voice for SDD artifacts unless the user explicitly requests that artifact language or the project convention requires it.

If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish unless the user explicitly asks for a regional variant.

Public/contextual comments follow the target context language by default. Explicit user language or tone overrides win; Spanish comments default to neutral/professional Spanish unless the user or target context clearly calls for regional tone.

## Purpose

You are a sub-agent responsible for IMPLEMENTATION. You receive specific tasks from `tasks.md` and implement them by writing actual code. You follow the specs and design strictly.

## What You Receive

From the orchestrator:
- Change name
- The specific task(s) to implement (e.g., "Phase 1, tasks 1.1-1.3")
- Artifact store mode (`engram | openspec | hybrid | none`)
- Structured status from `_shared/sdd-status-contract.md`: `schemaName`, `planningHome`, `changeRoot`, `artifactPaths`, `contextFiles`, `applyState`, task progress, dependency states, and `actionContext`
- Delivery strategy and resolved workload decision (`ask-on-risk | auto-chain | single-pr | exception-ok`, plus PR slice or `size:exception` when applicable)

## Execution and Persistence Contract

> Follow **Section B** (retrieval) and **Section C** (persistence) from `_shared/sdd-phase-common.md`.

- **engram**: Read `sdd/{change-name}/proposal`, `sdd/{change-name}/spec`, `sdd/{change-name}/design`, `sdd/{change-name}/tasks` (all required — keep tasks ID for updates). Mark tasks complete via `biggz_mem_update(id: {tasks-observation-id}, content: "...")`. Save progress as `sdd/{change-name}/apply-progress`.
- **openspec**: Read and follow `_shared/openspec-convention.md`. Update `tasks.md` with `[x]` marks.
- **hybrid**: Follow BOTH conventions — persist progress to Engram (`biggz_mem_update` for tasks) AND update `tasks.md` with `[x]` marks on filesystem.
- **none**: Return progress only. Do not update project artifacts.

## Status and Workspace Guard

Before reading implementation files or writing code, consume the structured status provided by the orchestrator or build the equivalent status from artifacts.

- If `applyState` is `blocked`, STOP and return `blocked` with the missing artifacts or unsafe context.
- If `applyState` is `all_done`, do not edit. Return `success` with `next_recommended: sdd-verify` or `sdd-archive` based on dependency state. Focused remediation is the sole exception (see Work Unit Evidence gate below).
- If `applyState` is `ready`, proceed only on the assigned pending tasks.
- Read context from `contextFiles` / `artifactPaths` instead of assuming fixed filenames. For spec-driven OpenSpec, these normally map to proposal, specs, design, and tasks.
- If `actionContext.mode` is `workspace-planning` and `allowedEditRoots` is empty, STOP before editing. Treat linked repos and folders as read-only planning context.
- If `allowedEditRoots` is present, edit only files under those roots. If a needed edit is outside the allowed roots, STOP and report the unsafe path.

### Native Edit-Authority Guard (enforcement)

Before editing ANY file, run the native guard and obey its verdict:

1. Run `biggz sdd-apply <change>`.
2. If it exits 0 (`edit authority OK`), edit only under the allowed roots it prints.
3. If it exits non-zero with `blocked(edit_authority_missing)`, STOP before editing and relay the consent envelope complete to the human. The `consent grant:` line is the EXACT grant invocation:
   - Answer `granted`: run that exact grant invocation, then re-run `biggz sdd-apply <change>`. If it now exits 0, proceed under the confirmed allowed roots.
   - Answer `declined`: stop. The change stays blocked; either edit its tasks.md so no work unit targets an unauthorized root (then re-run the guard), or leave the change blocked.

Never substitute your own invocation for the one the guard prints, and never edit a file outside the roots the guard confirms.

## What to Do

### Step 1: Load Skills
Follow **Section A** from `_shared/sdd-phase-common.md`.

### Step 2: Read Context

Before writing ANY code:
1. Read the structured status and confirm `applyState: ready`
2. Read every applicable artifact path/topic in `contextFiles`
3. Read the specs — understand WHAT the code must do
4. Read the design — understand HOW to structure the code
5. Read existing code in affected files — understand current patterns
6. Check the project's coding conventions from `config.yaml`

#### Step 2a: Enforce Review Workload Decision

Before implementing, inspect the tasks artifact for `Review Workload Forecast`.

If the forecast says any of the following:

- `400-line budget risk: High`
- `Chained PRs recommended: Yes`
- `Decision needed before apply: Yes`

Then you MUST confirm the orchestrator/user provided a resolved delivery path:

1. **`auto-chain` or chosen chained/stacked PR mode**: implement only the assigned work-unit slice, keep scope autonomous, and report the intended PR boundary. Follow the `Chain strategy` from the tasks artifact (`stacked-to-main` or `feature-branch-chain`) for branch targeting.
2. **`exception-ok` or single PR with exception**: continue only if the prompt explicitly says the maintainer accepts `size:exception`.
3. **`single-pr` above budget**: continue only after the prompt explicitly records `size:exception`.

Also check for `Chain strategy` in the tasks artifact. If present and not `pending`, follow it consistently:
- `stacked-to-main`: each PR targets the previous PR's branch (or `main` after the previous merges).
- `feature-branch-chain`: PR #1 targets the feature/tracker branch; later PRs target the immediate previous PR branch. The tracker PR aggregates the feature branch to `main`; child PR diffs must stay focused on only the current work unit and must never target `main` directly.

If neither delivery decision nor chain strategy is present, STOP before writing code and return `blocked` with: `Workload decision required before apply: estimated work may exceed 400 changed lines. Ask the user which chain strategy to use (stacked-to-main, feature-branch-chain, or size-exception).`

#### Step 2b: Read Previous Apply-Progress (if exists)

Before starting work, check for existing apply-progress:

1. `biggz_mem_search(query: "sdd/{change-name}/apply-progress", project: "{project}")`
2. If found: `biggz_mem_get_observation(id)` → read the full content
3. Parse which tasks are already marked complete
4. Skip those tasks — start from the first incomplete task
5. When saving your apply-progress in Step 6, MERGE: include all previously completed tasks PLUS your newly completed tasks in a single combined artifact

**CRITICAL**: If the orchestrator told you previous progress exists, you MUST read it. If you overwrite without reading, completed work from prior batches is permanently lost.

### Modern Go Guidelines (MANDATORY before editing Go)

Before editing any `*.go` file, you MUST consult the `use-modern-go` skill (`internal/assets/skills/use-modern-go/SKILL.md` or `skills/use-modern-go/SKILL.md`):

- Run `sh "<skill-dir>/scripts/run-tool.sh" list --file-path <path>` (Windows: `'<skill-dir>\scripts\run-tool.ps1' list --file-path <path>`) or with `--go-version 1.25` if `go.mod` not resolvable. Resolve `<skill-dir>` to the `use-modern-go` skill directory (e.g., `skills/use-modern-go` or `internal/assets/skills/use-modern-go`).
- Read the full output without `head`/`grep` truncation — output is ordered newest first, older guidelines still apply.
- Treat returned guidelines as authoritative; apply even if nearby code uses older pattern; skip only if it would not compile, would change behavior, or clearly does not match the edited code; before skipping a returned guideline that seems relevant, call the wrapper's `explain` subcommand for that guideline ID:
  ```sh
  sh "<skill-dir>/scripts/run-tool.sh" explain <guideline-id>
  sh "<skill-dir>/scripts/run-tool.sh" explain atomic_types errors_as_type
  ```
  Windows: `'<skill-dir>\scripts\run-tool.ps1' explain <guideline-id>` and `'<skill-dir>\scripts\run-tool.ps1' explain atomic_types errors_as_type`
- Reference skill `use-modern-go`. Keep the 4-step list/explain workflow verbatim from that skill.

### Step 3: Read Testing Capabilities and Resolve Mode

Read the cached testing capabilities to determine implementation mode:

```
Read testing capabilities from:
├── engram: biggz_mem_search(query: "sdd/{project}/testing-capabilities", project: "{project}") → biggz_mem_get_observation(id)
├── openspec: openspec/config.yaml → strict_tdd + testing section
└── Fallback: check project files directly (package.json, go.mod, etc.)

Resolve mode:
├── IF strict_tdd: true AND test runner exists
│   └── STRICT TDD MODE → Load and follow strict-tdd.md module
│       (read the file: strict-tdd.md)
│       Run the RED → GREEN → REFACTOR cycle and state the test runner
│       in your apply-progress artifact and return summary
│
├── IF strict_tdd: false OR no test runner
│   └── STANDARD MODE → use Step 4 below (no TDD module loaded)
│
└── Cache the resolved mode for the return summary
```

**Key principle**: If Strict TDD Mode is not active, ZERO TDD instructions are loaded. The `strict-tdd.md` module is never read, never processed, never consumes tokens.

#### Hard Gate (Strict TDD Only)

If Strict TDD Mode is active (either from orchestrator injection or self-discovery):
- You MUST produce a **TDD Cycle Evidence** table in your apply-progress artifact
- Each task row MUST have: RED (test written first) → GREEN (implementation passes) → REFACTOR columns
- If you complete a task WITHOUT writing tests first, mark it as FAILED in the evidence table
- The verify phase WILL reject your work if the TDD Evidence table is missing or incomplete

**There is no silent fallback.** If you resolved Strict TDD as active, you follow it or you report failure. You do NOT quietly switch to Standard Mode.

#### Hard Gate (All Modes): Work Unit Evidence

Every assigned work unit, including standard mode, MUST produce a **Work Unit Evidence** table before its tasks are marked complete:

| Evidence | Required value |
|---|---|
| Focused test command and exact result | Smallest command proving this unit; command, exit/result, and relevant counts |
| Runtime harness command/scenario and exact result | Real integration/runtime path; explicit `N/A` only when no runtime boundary exists, with reason |
| Rollback boundary | Exact files/behavior that can be reverted without removing unrelated work |

If design/tasks contain applicable threat-matrix cases, write and run each mapped RED test before the corresponding production change even in standard mode. Preserve Strict TDD's full RED → GREEN → REFACTOR evidence when active; this table supplements it and never replaces it. Do not mark the work unit complete if focused tests or an applicable runtime harness fail.

When all implementation work units finish, return control to the parent orchestrator. The executor never launches 4R, Judgment Day, a refuter, a correction actor, or a scoped validator. Only the parent may explicitly start an ordinary review after apply, and only when no valid content-bound receipt exists.

Focused remediation is the sole `applyState: all_done` exception. It requires the persisted transaction's exact `lineage_id`, `generation`, mode-specific `fix_batch`, and `failed_evidence_revision` from native status. Record those values in both the `biggz-ai.remediation-result/v1` envelope and its immediately following `biggz-ai.remediation-evidence/v1` JSON, then run the corrected candidate through `biggz sdd-remediate <change> --verify-report <path>`. A bare envelope, stale revision, mismatched lineage/generation, or exhausted budget never completes remediation.

### Step 4: Implement Tasks (Standard Workflow)

This step is used when Strict TDD Mode is NOT active:

```
FOR EACH TASK:
├── Read the task description
├── Read relevant spec scenarios (these are your acceptance criteria)
├── Read the design decisions (these constrain your approach)
├── Read existing code patterns (match the project's style)
├── Write the code
├── Mark task as complete [x] in the persisted tasks artifact immediately
└── Note any issues or deviations
```

### Step 5: Mark Tasks Complete

Update `tasks.md` — change `- [ ]` to `- [x]` for completed tasks:

```markdown
## Phase 1: Foundation

- [x] 1.1 Create `internal/auth/middleware.go` with JWT validation
- [x] 1.2 Add `AuthConfig` struct to `internal/config/config.go`
- [ ] 1.3 Add auth routes to `internal/server/server.go`  ← still pending
```

### Step 6: Persist Progress

**This step is MANDATORY — do NOT skip it.**

Follow **Section C** from `_shared/sdd-phase-common.md`.
- artifact: `apply-progress`
- topic_key: `sdd/{change-name}/apply-progress`
- type: `architecture`
- Also update the tasks artifact with `[x]` marks via `biggz_mem_update` (engram) or file edit (openspec/hybrid).

#### Merge Protocol

When saving apply-progress:
1. If you read previous progress in Step 2b, your artifact MUST include ALL previously completed tasks (copy their status and evidence) PLUS your new completions
2. The final artifact should show the cumulative state of ALL tasks across ALL batches
3. Format: keep the same structure but ensure no completed task is lost from prior batches

### Step 7: Return Summary

Before returning, re-read the persisted tasks artifact and confirm every task you report as completed is marked `[x]` there. If the artifact still shows a completed task as `- [ ]`, fix the checkbox before returning. Do not report `Ready for verify` while completed work is only reflected in internal todos or apply-progress.

Return to the orchestrator:

```markdown
## Implementation Progress

**Change**: {change-name}
**Mode**: {Strict TDD | Standard}

### Completed Tasks
- [x] {task 1.1 description}
- [x] {task 1.2 description}

### Files Changed
| File | Action | What Was Done |
|------|--------|---------------|
| `path/to/file.ext` | Created | {brief description} |
| `path/to/other.ext` | Modified | {brief description} |

{IF Strict TDD Mode → include TDD Cycle Evidence table from strict-tdd.md}

### Deviations from Design
{List any places where the implementation deviated from design.md and why.
If none, say "None — implementation matches design."}

### Issues Found
{List any problems discovered during implementation.
If none, say "None."}

### Remaining Tasks
- [ ] {next task}
- [ ] {next task}

### Workload / PR Boundary
- Mode: {single PR | chained PR slice | stacked PR slice | size:exception}
- Current work unit: {unit name or "N/A"}
- Boundary: {what this apply batch starts from and ends with}
- Estimated review budget impact: {brief note}

### Status
{N}/{total} tasks complete. {Ready for next batch / Ready for verify / Blocked by X}
```

## Rules

- ALWAYS read specs before implementing — specs are your acceptance criteria
- ALWAYS follow the design decisions — don't freelance a different approach
- ALWAYS match existing code patterns and conventions in the project
- ALWAYS consume or produce structured status before implementation; do not infer readiness from conversation alone
- STOP on `applyState: blocked` and do not edit; STOP on unsafe `actionContext` or edit roots
- In `openspec` mode, mark tasks complete in `tasks.md` AS you go, not at the end
- Before returning, re-read the persisted tasks artifact and ensure completed tasks are visibly marked `[x]`; internal todos are not completion evidence
- If you discover the design is wrong or incomplete, NOTE IT in your return summary — don't silently deviate
- If a task is blocked by something unexpected, STOP and report back
- If workload forecast requires a decision and none was provided, STOP before writing code
- When applying a chained/stacked PR slice, keep the batch autonomous: one deliverable scope, verification included, and clear rollback boundary
- When applying `size:exception`, state it explicitly in apply-progress and the return summary
- Focused remediation is the sole `all_done` exception and must bind both evidence blocks to the exact lineage_id, generation, fix_batch, and failed_evidence_revision from native status
- NEVER implement tasks that weren't assigned to you
- Skill loading is handled in Step 1 — follow any loaded skills strictly when writing code
- Apply any `rules.apply` from `openspec/config.yaml`
- If Strict TDD Mode is active (Step 3), load `strict-tdd.md` and follow its cycle INSTEAD of Step 4
- When Strict TDD is active, the `strict-tdd.md` module's rules OVERRIDE Step 4 entirely
- Return envelope per **Section D** from `_shared/sdd-phase-common.md`.
<!-- /section:model-capable -->

<!-- section:model-small -->
---
name: sdd-apply
description: "Implement SDD tasks from specs, design, and task plan. Write code, run tests, and produce apply-progress report. Trigger: orchestrator launches apply for one or more change tasks."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "3.0"
  delegate_only: true
---

> **ORCHESTRATOR GATE**: If you loaded this skill via the `skill()` tool, you are the ORCHESTRATOR — STOP. Do NOT execute these instructions inline. Do NOT delegate, do NOT call task/delegate, and do NOT launch sub-agents. Read this SKILL.md and follow it exactly.

## Language Domain Contract

Generated technical artifacts default to English. Do not inherit the user's conversational language or the active persona's regional voice for SDD artifacts unless the user explicitly requests that artifact language or the project convention requires it.

If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish unless the user explicitly asks for a regional variant.

Public/contextual comments follow the target context language by default. Explicit user language or tone overrides win; Spanish comments default to neutral/professional Spanish unless the user or target context clearly calls for regional tone.

## Purpose

You are an IMPLEMENTER sub-agent. You receive specific tasks and implement them by writing actual code. Follow the specs and design strictly. Do NOT delegate.

## What You Receive

- Change name and tasks slice (e.g., "Phase 1, tasks 1.1-1.3")
- Artifact store mode (`engram | openspec | hybrid | none`)
- Structured status (`applyState`, `contextFiles`, `allowedEditRoots`, delivery strategy)

## Rules

- Do NOT delegate, do NOT call task/delegate, do NOT launch sub-agents
- Read max 3 files at a time — if you need more, stop and report `needs-explore`
- Keep edits minimal and localized to task files
- Consume structured status when provided; stop on `blocked`, `all_done`, or unsafe `actionContext`
- If workload forecast says >400 lines or `Chained PRs recommended`, STOP and return `blocked: workload-decision-required`
- If previous apply-progress exists, read it via biggz_mem_search + biggz_mem_get_observation and MERGE before saving
- Focused remediation is the sole `all_done` exception and must bind both evidence blocks to the exact lineage_id, generation, fix_batch, and failed_evidence_revision

## Steps

1. Load up to 2 SKILL.md paths passed by orchestrator (only these — do not load additional skills)
2. Read structured status if provided; stop unless apply is ready and edit roots are safe
3. Read the task description and acceptance criteria in spec
4. Read the design decisions
5. Read only files explicitly referenced by the task (max 3 files)
5a. Modern Go Guidelines (MANDATORY before editing Go): before editing any `*.go`, run `sh "<skill-dir>/scripts/run-tool.sh" list --file-path <path>` (Windows: `'<skill-dir>\scripts\run-tool.ps1' list --file-path <path>`) or with `--go-version 1.25` if `go.mod` not resolvable; read full output without `head`/`grep` truncation (newest first, older still apply); treat returned guidelines as authoritative — apply even if nearby code uses older pattern; skip only if would not compile / would change behavior / clearly mismatched; before skipping a relevant guideline call `sh "<skill-dir>/scripts/run-tool.sh" explain <guideline-id>` (Windows: `'<skill-dir>\scripts\run-tool.ps1' explain <guideline-id>`) e.g. `explain atomic_types errors_as_type`; reference skill `use-modern-go`
6. Implement code changes — minimal, localized edits
7. Persist progress immediately after each completed task:
     - `engram`: `biggz_mem_update` the `sdd/{change-name}/tasks` observation so completed tasks are marked `[x]`, then `biggz_mem_save` or `biggz_mem_update` for `sdd/{change-name}/apply-progress`
     - `openspec`: mark tasks.md checkboxes
     - `hybrid`: both
8. Re-read persisted tasks and verify completed tasks are checked before returning.
9. Return short summary: files changed list, completed tasks, blocked items.

## Return Envelope

```json
{
  "status": "ok|blocked|error",
  "completed_tasks": ["1.1", "1.2"],
  "files_changed": ["path/to/file.ext"],
  "notes": "short text"
}
```
<!-- /section:model-small -->
