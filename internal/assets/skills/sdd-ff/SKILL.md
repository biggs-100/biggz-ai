---
name: sdd-ff
description: Fast-forward through SDD phases — generate all planning artifacts in sequence and skip to implementation for well-understood changes.
trigger: sdd ff, fast forward, skip phases
---

# SDD Fast-Forward

Generate all planning artifacts (proposal, spec, design, tasks) from a brief description in one pass, then proceed directly to implementation. Designed for changes that are well-understood and don't benefit from individual phase review cycles.

## Activation Contract

1. User must explicitly acknowledge skipping individual review phases.
2. Generate complete spec, design, and tasks from a brief description.
3. Proceed to apply phase immediately.
4. Verification and archive are still required — no exception.

## Hard Rules

- User MUST explicitly confirm they accept skipping phase-level review. A single "yes, proceed" is sufficient — do not require repeated confirmation or make the user jump through hoops.
- All four planning artifacts (proposal, spec, design, tasks) MUST be generated and written to disk — no phase is truly skipped, just collapsed into one step.
- Verification is NEVER skipped — even fast-forward changes must pass full verification.
- If the change is large (>400 lines estimated), refuse fast-forward and recommend full SDD workflow.
- If the change introduces architectural impact (new public interface, new domain, new external dependency), refuse fast-forward and recommend full SDD workflow.
- If user seems uncertain about the approach, route to sdd-explore instead.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Trivial change | 1-3 files, no new abstractions | Allow fast-forward |
| Architectural change | New interface, new exported type, new domain, new external dep | Refuse — require full SDD |
| Large change | >400 lines estimated across all files | Refuse — require full SDD |
| User uncertain | User says "not sure", "maybe", or asks exploratory questions | Route to explore instead |
| Spec already exists | Change domain has existing specs at openspec/specs/ | Generate delta spec, reference existing |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Evaluate complexity** — from the description, estimate: files to change, new abstractions needed, architectural impact, test volume. If >400 lines or architectural impact, refuse with explanation and suggest full SDD.
3. **Check existing specs** — search `openspec/specs/` for domain specs related to the description. If found, note them for reference during generation.
4. **Confirm with user** — present a brief complexity assessment and ask: "This will generate spec, design, and tasks from your description, then proceed to implementation. Verification is still required. Proceed?" If no, route to sdd-explore.
5. **Generate proposal** — write `openspec/changes/{change-name}/proposal.md`:
   - Intent and success criteria from the description.
   - Scope (in/out) inferred from description.
   - High-level approach.
   - Rollback plan (code revert).
6. **Generate spec** — write `openspec/changes/{change-name}/spec.md`:
   - Functional requirements derived from success criteria.
   - GIVEN/WHEN/THEN scenario per requirement.
   - If existing domain spec found, write delta spec referencing it.
7. **Generate design** — write `openspec/changes/{change-name}/design.md`:
   - Architecture decisions (minimal — only what's needed).
   - Data flow description.
   - File change list (exhaustive).
   - Testing strategy per requirement.
   - Threat matrix (lightweight).
8. **Generate tasks** — write `openspec/changes/{change-name}/tasks.md`:
   - Ordered task list by dependency.
   - Test evidence requirement per task.
   - Review workload estimate.
9. **Update metadata** — set `phase: apply` and `generated_by: sdd-ff` in `_meta.yaml`.
10. **Proceed to apply** — delegate to sdd-apply skill starting from TASK-1.
11. **Persist** — save all four generated artifacts to Engram for cross-session traceability.

## Output Contract

```yaml
status: success | refused | blocked
executive_summary: "Fast-forward approved. Generated 4 planning artifacts (proposal, spec, design, tasks). Starting apply."
artifacts:
  - path: openspec/changes/{change-name}/proposal.md
    type: auto-proposal
    summary: "Auto-generated proposal from description"
  - path: openspec/changes/{change-name}/spec.md
    type: auto-spec
    summary: "Auto-generated requirements with scenarios"
  - path: openspec/changes/{change-name}/design.md
    type: auto-design
    summary: "Auto-generated architecture, file changes"
  - path: openspec/changes/{change-name}/tasks.md
    type: auto-tasks
    summary: "Auto-generated ordered task list"
next_recommended: apply
risks:
  - description: "Auto-generated artifacts may miss edge cases — rely on verify phase to catch them"
    severity: medium
skill_resolution: user_input
```

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-ff.md`
- `../sdd-propose/SKILL.md`
- `../sdd-spec/SKILL.md`
- `../sdd-design/SKILL.md`
- `../sdd-tasks/SKILL.md`
- `../sdd-apply/SKILL.md`
- `../sdd-verify/SKILL.md`
