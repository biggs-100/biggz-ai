---
name: sdd-spec
description: Write SDD delta specs with functional requirements and GIVEN/WHEN/THEN scenarios.
trigger: orchestrator launches spec work for a change.
---

# SDD Spec

Translate the proposal into structured specifications with numbered functional requirements and GIVEN/WHEN/THEN acceptance scenarios.

## Activation Contract

1. Read proposal to understand scope and success criteria.
2. Check for existing spec files in the change domain.
3. If domain spec exists, write delta spec (additions only).
4. If no domain spec exists, write full spec.
5. Each requirement must have at least one GIVEN/WHEN/THEN scenario.
6. Persist spec artifact.

## Hard Rules

- Every functional requirement must have a unique ID (`REQ-N`).
- Every requirement must have at least one GIVEN/WHEN/THEN scenario.
- Scenarios must be testable — no subjective or ambiguous assertions.
- Write delta specs when extending a domain; write full specs for new domains.
- Delta specs clearly mark added requirements with `[ADDED]` prefix.
- Requirements must be verifiable independently from each other.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| New domain | No spec exists at `openspec/specs/{domain}/spec.md` | Write full spec |
| Extension | Spec exists and change adds capabilities | Write delta spec additions |
| No scenarios | Existing spec has no GIVEN/WHEN/THEN | Migrate existing requirements to scenario format |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Load proposal** — read `openspec/changes/{change-name}/proposal.md`. Extract scope and success criteria.
3. **Search existing specs** — check `openspec/specs/` for the relevant domain. Use glob `openspec/specs/{domain}/*.md`.
4. **Determine spec type** — full vs. delta based on whether domain spec exists.
5. **Write spec file** — create `openspec/changes/{change-name}/spec.md` (for delta) or `openspec/specs/{domain}/spec.md` (for full):
   ```yaml
   ---
   title: "{domain} Spec"
   status: draft | approved
   spec_type: full | delta
   source_change: "{change-name}"
   ---
   ## Requirements
   ### REQ-1: {title}
   - **Description**: {what the system must do}
   - **Priority**: must | should | could
   - **Scenarios**:
     - **Scenario 1.1**: {title}
       - GIVEN {context}
       - WHEN {action}
       - THEN {expected outcome}
     - **Scenario 1.2**: {title}
       - GIVEN {context}
       - WHEN {action}
       - THEN {expected outcome}
   ```
6. **Review scenarios** — verify every scenario is testable. If a scenario describes behavior that cannot be observed from outside the system, restructure it.
7. **Persist** — write spec file. Update `_meta.yaml` with `phase: spec`. Save to Engram.
8. **Recommend next phase** — design.

## Output Contract

```yaml
status: success | blocked
executive_summary: "Wrote 5 requirements with 8 GIVEN/WHEN/THEN scenarios for auth domain."
artifacts:
  - path: openspec/changes/{change-name}/spec.md
    type: delta-spec
    summary: "N requirements with numbered scenarios"
  - path: openspec/specs/{domain}/spec.md
    type: full-spec
    summary: "Domain spec with all requirements"
next_recommended: design
risks:
  - description: "Scenarios may miss edge cases — review during design"
    severity: low
skill_resolution: auto
```

## References

- `../_shared/sdd-phase-common.md`
- `openspec/changes/{change-name}/proposal.md`
- `openspec/changes/{change-name}/spec.md`
- `openspec/specs/{domain}/spec.md`
