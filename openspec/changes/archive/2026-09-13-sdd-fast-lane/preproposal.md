schema: biggz-ai.sdd-preproposal/v1
revision: 1
exploration_outcome: done
exploration_ref: openspec/changes/sdd-fast-lane/exploration.md
research_request: none
research_classes: []
admission_outcome: not-selected
evidence_outcome: unselected
openspec_ref: openspec/changes/sdd-fast-lane/preproposal.md
engram_ref: sdd/sdd-fast-lane/preproposal
decision: confirmed
proposal_ready: true

# Pre-proposal gate state — sdd-fast-lane

## Confirmed product decisions (maintainer, 2026-09-13)

1. **Merged artifact** — `plan.md`: one document carrying intent, scope,
   requirements with GIVEN/WHEN/THEN scenarios, approach and the task
   checklist. The dispatcher aliases it into the proposal/specs/design/tasks
   slots with per-slot precedence (a real artifact always wins), and the plan
   MUST use the canonical `### Requirement:` / `#### Scenario:` headings so
   verify can count them.
2. **Where the lane lives** — inside the existing `sdd-ff` meta-command: no new
   command, no new overlay entry, no AGENTS.md table entry. `sdd-ff` already
   collapses the planning phases without skipping them, and its thresholds match
   the router's.
3. **Router role** — `sdd-route` stays advisory with no persistence: the lane is
   inferred from the artifact present, the router remains guidance for the
   orchestrator, and nothing new reads or stores its verdict.

## Research lane: waived (recorded, not silently skipped)

Research was **not selected** for this change. Reason: every design question was
answered from in-repo code with `path:line` evidence during explore (the
dispatcher chokepoint, the alias precedent, verify's count admission, the
router's advisory status), and the change touches only `internal/sdd` plus
skill/prompt text. There is no external contract or third-party behaviour to
verify. If the design phase surfaces an external dependency, this record is
superseded and the lane is reopened.

State contract: hybrid store — this file and BigMem
`sdd/sdd-fast-lane/preproposal` MUST stay byte-identical, same revision.
