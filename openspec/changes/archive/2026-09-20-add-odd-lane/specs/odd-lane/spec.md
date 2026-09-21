# Delta for odd-lane

## ADDED Requirements

### Requirement: REQ-ODD-001 — Seven-Step ODD Protocol

Organic (non-SDD) work MUST execute these steps in order: (1) Authorize — establish whether the request authorizes a change; (2) Explore the existing code proportionately; (3) Resolve uncertainty — scoped research, one focused question per real product decision, at most one assumption challenge; (4) Classify — substantial when exploration yields 2+ meaningful implementation steps or progress worth recovering; (5) Track before the first write; (6) Implement task by task with observed proof; (7) Close with the verified outcome, failed/pending checks, and next step. Steps MUST NOT be reordered, and the protocol MUST NOT require an SDD phase attempt.

#### Scenario: Ordered protocol for substantial authorized work

- GIVEN an authorized implementation request yielding 2+ meaningful steps
- WHEN the orchestrator runs the ODD lane
- THEN Authorize, Explore, Resolve uncertainty, Classify, Track, Implement, Close MUST occur in that order
- AND tracking MUST precede the first source write

#### Scenario: Unresolved uncertainty pauses before classify

- GIVEN a real product decision is unresolved after exploration
- WHEN the protocol reaches Resolve uncertainty
- THEN the orchestrator MUST ask one focused question and wait
- AND MUST NOT classify or write before the answer

### Requirement: REQ-ODD-002 — Task Document Contract

Substantial ODD work MUST create exactly one document `odd/tasks/<slug>.md` at the repository root (sibling of `openspec/`), before the first source write. The document MUST carry required sections `Objective`, `Tasks` (checkbox list with stable IDs and acceptance criteria), `Evidence` (observed proof per task, including commit identity), and `Next`. It MUST be the single source of truth for that work: no BigMem or Engram mirror, never archived, never deleted, and never rewritten to erase completed progress.

#### Scenario: Document exists before first write

- GIVEN substantial work classified with 3 tasks
- WHEN the first source file is about to be edited
- THEN `odd/tasks/<slug>.md` MUST already exist with the objective and 3 task IDs
- AND the orchestrator MUST report the document path and task count in one line

#### Scenario: No mirror and no archive

- GIVEN `odd/tasks/<slug>.md` completed and closed
- WHEN the repository and memory stores are inspected
- THEN `odd/tasks/<slug>.md` MUST still exist at the repository root
- AND no BigMem/Engram topic mirroring it MUST exist

### Requirement: REQ-ODD-003 — Read-Only Work Leaves No Task Artifacts

Read-only work — explanation, investigation, review, and proposal-only requests — MUST end with no durable task artifacts. The orchestrator MUST NOT create or update `odd/tasks/*.md` for read-only work, MUST NOT invent task IDs, and MUST NOT mark progress for work it did not perform.

#### Scenario: Explanation request leaves odd/ absent

- GIVEN a read-only question about existing code
- WHEN the ODD lane completes
- THEN no `odd/tasks/*.md` MUST be created
- AND any pre-existing document MUST remain untouched

#### Scenario: Small understood work stays untracked

- GIVEN a small, understood single-step change
- WHEN the lane classifies it
- THEN it MUST stay small without a durable task document
- AND the close report MUST still state the verified outcome

### Requirement: REQ-ODD-004 — ODD Creates No SDD Lifecycle

ODD work MUST NOT create SDD artifacts (`proposal.md`, `spec.md`, `design.md`, `tasks.md`), phase attempts, or synthetic SDD runs, and MUST NOT modify `openspec/changes/` or `openspec/specs/`. ODD MUST be entered without any SDD grant; SDD remains opted-in only per `REQ-OR-004`.

#### Scenario: ODD work leaves openspec clean

- GIVEN an ODD task implemented and closed
- WHEN `openspec/changes/` is inspected
- THEN no new or modified entries MUST exist there
- AND `biggz sdd-status --json` MUST NOT register an SDD change for that work

#### Scenario: SDD not synthesized

- GIVEN an ODD feature with 5 completed tasks
- WHEN the close report is emitted
- THEN it MUST NOT name an SDD phase, gate, or artifact
