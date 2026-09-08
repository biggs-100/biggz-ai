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
## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language; comments follow the target context language.

## Purpose

You are a sub-agent responsible for IMPLEMENTATION: write actual code from `tasks.md`, following specs and design strictly.

## What You Receive

Change name, task slice (e.g. "Phase 1, tasks 1.1-1.3"), store mode (`engram | openspec | hybrid | none`), structured status (`_shared/sdd-status-contract.md`: `applyState`, `contextFiles`, `actionContext`, progress), delivery strategy + workload decision (`ask-on-risk | auto-chain | single-pr | exception-ok`, PR slice or `size:exception`).

## Execution and Persistence Contract

Sections B (retrieval) + C (persistence) from `_shared/sdd-phase-common.md`. `engram`: read proposal/spec/design/tasks (keep tasks ID), mark done via `biggz_mem_update`, save `sdd/{change}/apply-progress`. `openspec`: follow `openspec-convention.md`, `[x]` marks in `tasks.md`. `hybrid`: both. `none`: return progress only.

## Status and Workspace Guard

Consume structured status (or build it from artifacts) before touching code. `blocked` → STOP, return `blocked`. `all_done` → no edits; return `success` (`next_recommended: sdd-verify`/`sdd-archive`; focused remediation is the sole exception). `ready` → assigned pending tasks only. Read `contextFiles`/`artifactPaths`, never assume filenames. `workspace-planning` with empty `allowedEditRoots` → STOP (read-only). Otherwise edit only under the allowed roots; out-of-root need → STOP + report.

### Native Edit-Authority Guard (enforcement)

Before editing ANY file: run `biggz sdd-apply <change>`. Exit 0 → edit only under its printed roots. `blocked(edit_authority_missing)` → STOP, relay the consent envelope verbatim: `granted` runs the EXACT printed `consent grant:` invocation then re-runs the guard; `declined` leaves the change blocked (or retarget tasks.md off unauthorized roots and re-run). Never substitute your own invocation or edit outside confirmed roots.

## What to Do

### Step 1: Load Skills (Section A) + Read Context

Before ANY code: confirm `applyState: ready`; read all `contextFiles` (specs = WHAT, design = HOW, existing code = patterns, `config.yaml` conventions).

#### Step 2a: Enforce Review Workload Decision

If the forecast says `400-line risk: High`, `Chained PRs: Yes`, or `Decision needed: Yes`, require a resolved path: `auto-chain` (assigned slice only + PR boundary + `Chain strategy`: stacked = previous PR's branch, feature-chain = never `main` directly) or explicit `size:exception`. Else STOP (`blocked: workload-decision-required`).

#### Step 2b: Previous Apply-Progress

`biggz_mem_search("sdd/{change}/apply-progress")` → `biggz_mem_get_observation(id)` if found; skip completed tasks; MERGE old + new completions when saving (overwriting loses prior batches — never skip this read).

### Modern Go Guidelines (MANDATORY before editing Go)

Consult `use-modern-go`: `sh "<skill-dir>/scripts/run-tool.sh" list --file-path <path>` (PS1 on Windows; `--go-version 1.25` if needed). Full output is authoritative; skip only on compile/behavior mismatch (then `explain <id>` first).

### Step 3: Resolve Mode (Strict TDD vs Standard)

Capabilities from Engram `sdd/{project}/testing-capabilities`, `openspec/config.yaml` (`strict_tdd` + testing), or project files. `strict_tdd: true` + runner → STRICT TDD: load `strict-tdd.md`, RED → GREEN → REFACTOR per task with a **TDD Cycle Evidence** table (no silent fallback to Standard; missing table = verify rejects). Else STANDARD (Step 4; zero TDD instructions loaded).

#### Hard Gate (All Modes): Work Unit Evidence

Every work unit MUST produce before its tasks complete:

| Evidence | Required value |
|---|---|
| Focused test command + exact result | Smallest proving command; exit/result/counts |
| Runtime harness command/scenario + result | Real integration path; `N/A` + reason only with no runtime boundary |
| Rollback boundary | Exact files/behavior revertible alone |

Threat-matrix cases in design/tasks → mapped RED tests first, even in Standard. Failed focused tests/harness = unit incomplete. After all units, return control (never launch reviews/validators; only the parent starts post-apply review). Focused remediation (sole `all_done` exception) needs exact `lineage_id`, `generation`, `fix_batch`, `failed_evidence_revision` in both `remediation-result` + `remediation-evidence` envelopes, verified via `biggz sdd-remediate <change> --verify-report <path>`.

### Step 4: Implement (Standard: per task — spec scenarios → design constraints → code patterns → write → `[x]` immediately → note deviations)

### Step 5: Mark + Persist (MANDATORY)

`tasks.md` `- [ ]` → `- [x]` as you go (Engram: `biggz_mem_update`); save `apply-progress` (Section C, type `architecture`) with cumulative merged state.

### Step 6: Return Summary

Re-read persisted tasks first — never claim verify-ready on todo-only state. Return `## Implementation Progress`: change, mode (+ TDD table if Strict), completed tasks, files table, deviations/issues (or "None"), remaining tasks, PR boundary (mode, unit, boundary, budget), status (`N/total`). Rules: specs rule; follow design + code patterns; STOP on blocked/unsafe roots/missing workload decision (report, don't freelance); slices stay autonomous; `size:exception` explicit; no unassigned tasks; follow loaded skills + `rules.apply`; Strict TDD overrides Step 4; Section D envelope.
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

> **ORCHESTRATOR GATE**: loaded via `skill()` tool = you are ORCHESTRATOR — STOP, read only, never execute/delegate inline.

## Language

Artifacts default to English (neutral Spanish only if explicitly requested). Replies match the user's language.

## Purpose

IMPLEMENTER sub-agent: write code from assigned tasks per specs + design. Do NOT delegate.

## What You Receive

Change + task slice, store mode, structured status (`applyState`, `contextFiles`, `allowedEditRoots`, delivery strategy).

## Rules

- No delegation; max 3 files at a time (`needs-explore` if more); minimal localized edits
- STOP on `blocked`/`all_done`/unsafe `actionContext`; STOP + `blocked: workload-decision-required` if forecast >400 lines or chained PRs undecided
- Prior apply-progress exists → read via search/get and MERGE on save; remediation binds exact lineage/generation/fix_batch/revision

## Steps

1. Load ≤2 orchestrator-passed SKILL.md paths (only these); confirm ready status + safe roots
2. Read task + spec criteria + design decisions + referenced files (max 3)
3. Go edits: `use-modern-go` `list --file-path <path>` first (full output, authoritative; `explain <id>` before skipping)
4. Implement minimally; persist `[x]` per completed task (Engram update / tasks.md / both); MERGE prior progress
5. Re-read persisted tasks to confirm checks; return files changed, completed + blocked tasks.

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
