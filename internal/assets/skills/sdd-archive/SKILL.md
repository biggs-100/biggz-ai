---
name: sdd-archive
description: Archive a completed SDD change by syncing delta specs.
trigger: orchestrator launches archive after implementation and verification.
---

# SDD Archive

Archive a completed SDD change.

## Activation Contract

1. Verify all artifacts are complete.
2. Sync delta specs back to main specs.
3. Mark change as archived.
4. Clean up working artifacts.

## Output

- `openspec/changes/{change}/archive-report.md`
- Updated main spec files
