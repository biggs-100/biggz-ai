# Delta for sdd-status

## ADDED Requirements

### Requirement: REQ-SS-ODD-001 — Read-Only ODD Documents Projection

`biggz sdd-status` MUST expose an ODD-documents array with one entry per `odd/tasks/*.md`: `path` (repo-relative), `taskProgress` (`total` and `completed` counted from `- [ ]`/`- [x]` checklist items), and `lastTouched` (file modification timestamp). The array MUST appear in BOTH `--json` output and human output. The array MUST NOT appear in `active`, MUST NOT influence `nextRecommended`, MUST NOT add or remove `blockedReasons`, and MUST NOT introduce, gate, or block any phase, gatekeeper, workload guard, edit authority, or RDD check. Missing, unreadable, or malformed documents MUST be skipped without error, never fatal.

#### Scenario: Documents listed in both outputs

- GIVEN `odd/tasks/a.md` with 3 of 5 checkboxes done and `odd/tasks/b.md` with 0 of 2
- WHEN `biggz sdd-status --json` and human output are produced
- THEN the array MUST list both paths with `taskProgress` `{total:5,completed:3}` and `{total:2,completed:0}`
- AND each entry MUST carry a `lastTouched` timestamp

#### Scenario: Observability-only isolation

- GIVEN a completed ODD document while no SDD change is active
- WHEN `biggz sdd-status --json` derives
- THEN `active` MUST be empty, `nextRecommended` MUST be unchanged, and `blockedReasons` MUST be empty

#### Scenario: Malformed document cannot block SDD transition

- GIVEN `odd/tasks/broken.md` is unreadable or malformed and a change has its proposal done, so `nextRecommended` resolves to `spec`
- WHEN status derives and `biggz sdd-continue <change>` runs
- THEN `nextRecommended` MUST remain `spec` with `blockedReasons` empty
- AND the entry MUST be skipped without error, and `sdd-propose`/`sdd-spec` MUST NOT be blocked, gated, or warned about the document
