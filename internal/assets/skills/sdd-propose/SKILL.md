---
name: sdd-propose
description: Create an SDD change proposal with intent, scope, approach, and rollback plan. Trigger: orchestrator launches proposal work for a change.
license: MIT
metadata:
  author: biggz-ai
  version: '1.0'
---

# SDD Propose

Create a structured change proposal defining what will be built, why, and how. This is the foundation for all subsequent phases.

## Activation Contract

1. Read exploration output (if any) or user description.
2. Read existing specs relevant to the change domain.
3. Define intent, success criteria, scope, approach, and rollback plan.
4. Persist proposal artifact.
5. Recommend next phase (spec or design).

## Hard Rules

- The proposal must define explicit **success criteria** — measurable, testable outcomes.
- The proposal must define a **rollback plan** — how to undo the change if it fails.
- The proposal must call out **what is NOT in scope** (anti-scope).
- Never skip writing the rollback plan, even for trivial changes.
- If the change involves a new capability, check `openspec/specs/` for relevant domain specs.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Already proposed | `proposal.md` exists and is complete | Show it, ask if updates needed |
| Depends on spec | Change adds a new capability domain | Recommend spec phase first |
| Pure implementation | Change implements existing spec | Recommend design phase directly |
| Too large | Scope spans multiple domains or >400 lines estimated | Recommend splitting into multiple changes |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Load change context** — read `_meta.yaml` and `exploration.md` (if exists). Search Engram for prior decisions on this change.
3. **Read relevant specs** — search `openspec/specs/` for domain specs that intersect with this change. Use `mem_search` if on another session.
4. **Define proposal sections** — write `openspec/changes/{change-name}/proposal.md`:
   ```yaml
   ---
   title: "{change-name}"
   description: "1-2 sentence summary"
   status: draft | accepted
   ---
   ## Intent
   Why this change matters. What problem it solves.

   ## Success Criteria
   - Measurable outcome 1
   - Measurable outcome 2

   ## Scope
   ### In Scope
   - What will be built/changed
   ### Out of Scope (Anti-Scope)
   - What will NOT be built

   ## Approach
   High-level technical approach. Key decisions and rationale.

   ## Dependencies
   - Any blocking changes, specs, or external deps

   ## Rollback Plan
   - How to revert: code revert, data migration revert, config revert

   ## Effort Estimate
   - Files changed: N
   - Estimated lines: N
   ```
5. **Review threshold** — check against Section E (Review Workload Guard). If over 400 lines, flag and recommend split.
6. **Persist** — write proposal to file and Engram. Update `_meta.yaml` with `phase: propose`.
7. **Recommend next phase** — if new spec domain: `spec`. If existing spec covers it: `design`.

## Output Contract

```yaml
status: success | blocked
executive_summary: "Proposal for 'add-auth' written. Recommending design phase — spec already exists."
artifacts:
  - path: openspec/changes/{change-name}/proposal.md
    type: proposal
    summary: "Change proposal with scope, approach, and rollback plan"
next_recommended: spec | design
risks:
  - description: "Overscope: verify anti-scope boundaries during design"
    severity: low
skill_resolution: auto | user_input
```

## References

- `../_shared/sdd-phase-common.md`
- `openspec/changes/{change-name}/_meta.yaml`
- `openspec/changes/{change-name}/proposal.md`
- `openspec/specs/`
