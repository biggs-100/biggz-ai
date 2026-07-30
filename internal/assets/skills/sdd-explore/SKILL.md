---
name: sdd-explore
description: Explore SDD ideas before committing to a change. Investigate codebase, compare approaches, and provide go/no-go recommendation. Trigger: orchestrator launches exploration or requirement clarification.
license: MIT
metadata:
  author: biggz-ai
  version: '1.0'
---

# SDD Explore

Lightweight pre-proposal phase to investigate a change idea, compare approaches, and clarify scope before committing to a full proposal.

## Activation Contract

1. Understand user intent and the problem being solved.
2. Investigate the codebase for relevant code and patterns.
3. Identify and compare viable approaches.
4. Document scope boundaries, risks, and unknowns.
5. Provide a clear go/no-go recommendation.

## Hard Rules

- Always explore at least 2 approaches when the change has architectural impact.
- Always investigate existing code patterns before proposing new ones.
- Do NOT accept "no exploration needed" without evidence — verify codebase context.
- Limit exploration depth: spend no more time than the change complexity warrants.
- If existing code makes the change trivial, recommend fast-forward (`sdd-ff`).

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Trivial change | One obvious approach, no architectural impact | Recommend skip to sdd-ff |
| Two+ viable approaches | Architectural choice required | Write comparison table with tradeoffs |
| No clear path | Undefined requirements or conflicting constraints | Ask clarifying questions, return to user |
| Existing solution | Codebase already solves the problem | Flag as duplicate, recommend no-go |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Read change metadata** — open `openspec/changes/{change-name}/_meta.yaml` for description and context.
3. **Investigate codebase** — search for existing implementations related to the change domain. Look at tests, interfaces, and usage patterns.
4. **Identify approaches** — list at least 2 viable approaches (can be 1 if trivial). For each: describe approach, list pros/cons, estimate complexity.
5. **Write exploration document** — create `openspec/changes/{change-name}/exploration.md`:
   ```yaml
   ---
   investigation: summary of what was found
   approaches:
     - name: "approach A"
       pros: [...]
       cons: [...]
       complexity: low | medium | high
     - name: "approach B"
       pros: [...]
       cons: [...]
       complexity: low | medium | high
   recommendation: approach | no-go | need-clarification
   scope_boundaries: "what is in scope and what is out"
   risks: [...]
   ---
   ```
6. **Update metadata** — update `_meta.yaml` with `phase: explore` and `recommendation`.
7. **Recommend next step** — if go: update metadata phase to `propose` and recommend sdd-propose. If no-go: explain why and archive the change directory.
8. **Persist** — save exploration findings to Engram.

## Output Contract

```yaml
status: success | blocked | need-clarification
executive_summary: "Explored auth middleware options. Two approaches compared. Recommending middleware-as-handlers."
artifacts:
  - path: openspec/changes/{change-name}/exploration.md
    type: exploration-report
    summary: "Approach comparison with recommendation"
next_recommended: propose | ff | none
risks:
  - description: "Chosen approach may not scale beyond current requirements"
    severity: low
  - description: "If no-go: change directory should be cleaned up"
    severity: low
skill_resolution: user_input
```

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-explore.md`
- `openspec/changes/{change-name}/_meta.yaml`
- `openspec/changes/{change-name}/exploration.md`
