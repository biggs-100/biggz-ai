---
name: sdd-ff
description: "Fast-forward through SDD phases — generate all planning artifacts in sequence and skip to implementation for well-understood changes. Trigger: sdd ff, fast forward, skip phases"
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: biggz-ai
  version: '1.0'
  delegate_only: true
---

# SDD Fast-Forward

Generate all planning artifacts (proposal, spec, design, tasks) from a brief description in one pass, then proceed directly to implementation. Designed for changes that are well-understood and don't benefit from individual phase review cycles.

Two depths, picked by the artifacts the change ends up with:

- **Fast lane** — one merged `plan.md` stands in for proposal, spec, design, and tasks.
- **Full fast-forward** — the four planning artifacts, for changes that still want per-artifact traceability.

## Activation Contract

1. User must explicitly acknowledge skipping individual review phases.
2. Generate complete spec, design, and tasks from a brief description.
3. Proceed to apply phase immediately.
4. Verification and archive are still required — no exception.

## Hard Rules

- User MUST explicitly confirm they accept skipping phase-level review. A single "yes, proceed" is sufficient — do not require repeated confirmation or make the user jump through hoops.
- All four planning artifacts (proposal, spec, design, tasks) MUST be generated and written to disk — no phase is truly skipped, just collapsed into one step.
- In the fast lane, the single `plan.md` replaces the four planning artifacts; phase depth follows the artifact set present, and no gate is skipped.
- Verification is NEVER skipped — even fast-forward changes must pass full verification.
- If the change is large (>400 lines estimated), refuse fast-forward and recommend full SDD workflow.
- If the change introduces architectural impact (new public interface, new domain, new external dependency), refuse fast-forward and recommend full SDD workflow.
- If user seems uncertain about the approach, route to sdd-explore instead.

## Fast Lane

When the change is small, well-understood, and a single merged artifact is enough, generate ONE `plan.md` instead of the four planning artifacts. Phase depth follows the artifact set present: a change whose only planning artifact is `plan.md` proceeds to apply, verify, and archive.

`plan.md` scaffold (canonical headings and checklist are contractual):

```markdown
### Requirement: <capability>
#### Scenario: <observable behavior>
- [ ] <implementation task>
```

- Every requirement MUST carry at least one `#### Scenario:` block: verify admission compares the verify-envelope totals against these exact headings.
- The checklist MUST contain at least one `- [ ]` item — task progress is counted from the plan file itself.
- Writing a real planning artifact later graduates the change in place: no migration, no new field.
- Gates are NEVER skipped in the lane: the RDD delivery receipt, `biggz sdd-verify-validate`, the PR workload guard, the session-summary guard, and edit authority all apply exactly as in the full pipeline.
- No new command, overlay, or AGENTS.md entry: the lane runs inside this same `sdd-ff`.

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
2. **Evaluate complexity** — from the description, estimate: files to change, new abstractions needed, architectural impact, test volume. If >400 lines or architectural impact, refuse with explanation and suggest full SDD. Decide the depth: fast lane (single `plan.md`, see Fast Lane) when the change is small and well-understood; full fast-forward otherwise.
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
   - Fast lane only: skip steps 5-8 and write the single `plan.md` scaffold instead.
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
skill_resolution: fallback-path
```

Fast lane: the artifact list is the single `openspec/changes/{change-name}/plan.md` (`type: auto-plan`, merged proposal/spec/design/tasks) and `executive_summary` reports the lane.

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-ff.md`
- `../sdd-propose/SKILL.md`
- `../sdd-spec/SKILL.md`
- `../sdd-design/SKILL.md`
- `../sdd-tasks/SKILL.md`
- `../sdd-apply/SKILL.md`
- `../sdd-verify/SKILL.md`
