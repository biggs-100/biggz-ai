---
name: odd
description: "Trigger: ODD, organic direct development, non-SDD substantial task, odd/tasks document, durable direct-work evidence. Define the ODD lane artifact contract."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Load when substantial non-SDD (organic) work is selected, or when creating, updating, or closing an `odd/tasks/<slug>.md` document.

ODD is NOT an SDD phase. It creates no proposal/spec/design/tasks artifacts, no phase attempts, no synthetic SDD runs, and no BigMem/Engram mirror. SDD stays opted-in only.

## Hard Rules

- Run the protocol in order: Authorize → Explore → Resolve uncertainty → Classify → Track before the first write → Implement → Close. Never reorder; never require an SDD phase attempt.
- Substantial work gets exactly ONE document: `odd/tasks/<slug>.md` at the repository root (sibling of `openspec/`), created BEFORE the first source write.
- `odd/tasks/<slug>.md` is the single source of truth: no BigMem/Engram mirror, never archived, never deleted, never rewritten to erase completed progress.
- Read-only work (explanation, investigation, review, proposal-only) leaves no durable task artifacts: no document, no invented task IDs, no progress marks for work not performed.
- Never create or modify `openspec/changes/` or `openspec/specs/`; never invoke `biggz odd-*` (no such command exists — the orchestrator writes the document by convention).
- Substantial means 2+ meaningful implementation steps, or progress worth recovering. Small understood single-step work stays untracked; its close report still states the verified outcome.

## Document Contract

Required sections of `odd/tasks/<slug>.md`:

- `## Objective` — the outcome the work must deliver, in one or two sentences.
- `## Tasks` — checkbox list with stable IDs and acceptance criteria (`- [ ] 1.1 ...`).
- `## Evidence` — observed proof per task: exact command, result, and commit identity.
- `## Next` — verified outcome, failed/pending checks, and the next step.

## Execution Steps

1. Authorize: confirm the request authorizes a change; read-only requests stop tracking here.
2. Explore: read the existing code proportionately to the request.
3. Resolve uncertainty: scoped research; one focused question per real product decision, then wait; at most one assumption challenge.
4. Classify: substantial → track; small understood → proceed untracked; read-only → no artifacts.
5. Track before the first write: create `odd/tasks/<slug>.md` with objective and stable task IDs; report path and task count in one line.
6. Implement task by task: smallest change per task, tick each box as it completes, record observed proof.
7. Close: state the verified outcome, failed/pending checks, and next step; never name an SDD phase, gate, or artifact.

## References

Routing protocol and read-only guard: `internal/assets/biggz/biggz-orchestrator-delegation.md` (ODD Lane section).
