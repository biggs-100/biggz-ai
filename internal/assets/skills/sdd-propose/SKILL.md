---
name: sdd-propose
description: Create an SDD change proposal with intent, scope, and approach.
trigger: orchestrator launches proposal work for a change.
---

# SDD Propose

Create a structured change proposal.

## Activation Contract

1. Read specs relevant to the change domain.
2. Define intent, scope, approach, rollback plan.
3. Persist proposal artifact.
4. Recommend next phase (spec or design).

## Output

- `openspec/changes/{change}/proposal.md`
- Or Engram `sdd/{change-name}/proposal`
