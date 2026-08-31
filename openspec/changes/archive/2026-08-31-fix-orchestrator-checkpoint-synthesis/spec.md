# Spec — fix-orchestrator-checkpoint-synthesis

Concatenated pointer for `openspec` artifact store. Canonical deltas live in `specs/{domain}/spec.md`.

## Domains

- `orchestrator` → `specs/orchestrator/spec.md` (REQ-001, REQ-002, REQ-003, REQ-005, REQ-007)
- `pi-integration` → `specs/pi-integration/spec.md` (REQ-001, REQ-004, REQ-005, REQ-006, REQ-008)

All 8 invariants verified via Given/When/Then. Strict source is `currentTurnMarkdown ≤120s + HasSynthesis(4 markers)` only; history fallback removed from block path; kept only for `BIGGZ_ADVISE=1` thin `concern: synthesis is thin`. Pending dual-write and child/recall/preflight bypasses preserved.

## Coverage Summary

| REQ | Summary | Domain | Scenarios |
|-----|---------|--------|-----------|
| REQ-001 | Block strict currentTurn only | orchestrator, pi-integration | 2 |
| REQ-002 | Allow fresh synthesis ≤120s | orchestrator | 2 |
| REQ-003 | Preflight bypass | orchestrator | 1 |
| REQ-004 | Child bypass | pi-integration | 1 |
| REQ-005 | Session Recall bypass | orchestrator, pi-integration | 2 |
| REQ-006 | Advise thin concern only | pi-integration | 2 |
| REQ-007 | Pending dual-write | orchestrator | 2 |
| REQ-008 | Tests rewrite 5 → block | pi-integration | 2 |

See domain deltas for full Given/When/Then scenarios.
