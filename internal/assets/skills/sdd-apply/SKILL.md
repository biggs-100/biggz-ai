---
name: sdd-apply
description: Implement SDD tasks from specs and design. Execute code changes.
trigger: orchestrator launches apply for one or more change tasks.
---

# SDD Apply

Implement tasks from the task plan. Write actual code.

## Activation Contract

1. Read specs, design, and tasks.
2. Read existing code — match patterns.
3. Implement tasks in order.
4. Mark tasks complete.
5. Persist apply-progress.
6. Return summary with evidence.

## Output

- Code changes (files created/modified)
- `apply-progress` with Work Unit Evidence table
