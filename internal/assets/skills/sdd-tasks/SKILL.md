---
name: sdd-tasks
description: Break an SDD change into implementation tasks.
trigger: orchestrator launches task planning for a change.
---

# SDD Tasks

Break down a design into concrete implementation tasks.

## Activation Contract

1. Read design — understand architecture and file changes.
2. Define tasks by dependency order.
3. Include test tasks alongside implementation.
4. Estimate review workload and recommend PR strategy.

## Output

- `openspec/changes/{change}/tasks.md`
- Work units with test evidence and rollback boundaries
