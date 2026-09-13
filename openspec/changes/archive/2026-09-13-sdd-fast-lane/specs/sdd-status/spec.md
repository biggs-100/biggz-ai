# Delta for sdd-status

## ADDED Requirements

### Requirement: Plan-Only Fast Lane Alias

The system MUST treat a change whose only planning artifact is `plan.md` as ready for apply, by aliasing `plan.md` into the proposal, specs, design, and tasks slots with per-slot precedence: a real artifact always wins over the alias. Task checkboxes MUST be read from the plan itself (`internal/sdd/status.go:638`). The alias MUST hold in both derivation paths — `resolveArtifactPaths` (`internal/sdd/status.go:994`) and `bigmemArtifactPaths` (`internal/sdd/engram_status.go:168`) — or the change diverges between stores. The plan MUST use the canonical `### Requirement:` / `#### Scenario:` headings so verify admission totals match (`internal/sdd/verify.go:188,197,201`). Writing a real artifact MUST graduate the change to the full pipeline in place — no migration, no new field. The lane MUST NOT relax any terminal gate: existing blocked reasons keep applying (`internal/sdd/status.go:905`).

#### Scenario: Plan-only change reaches apply

- GIVEN a change with `plan.md` and no proposal, specs, design, or tasks
- WHEN status derives
- THEN all four slots MUST resolve to the plan and apply MUST be ready

#### Scenario: Real artifact wins per slot

- GIVEN `plan.md` plus a real `design.md`
- WHEN slots resolve
- THEN design MUST resolve to `design.md` and the other slots to `plan.md`

#### Scenario: Checklist read from plan

- GIVEN `plan.md` containing task checkboxes
- WHEN task progress derives
- THEN progress MUST count the checkboxes of the plan file

#### Scenario: Cross-store parity

- GIVEN the same plan-only change visible in filesystem and BigMem
- WHEN both derivations run
- THEN both MUST report the same lane-ready artifact set

#### Scenario: Canonical headings admitted

- GIVEN a plan carrying `### Requirement:` / `#### Scenario:` blocks
- WHEN `biggz sdd-verify-validate` admits the verify envelope
- THEN declared totals MUST match the plan counts

#### Scenario: In-place graduation

- GIVEN a lane change where `tasks.md` is later written
- WHEN status derives again
- THEN tasks MUST resolve to `tasks.md` with no migration required

#### Scenario: Gates not relaxed

- GIVEN a lane change with an outstanding gate obligation
- WHEN terminal gates evaluate
- THEN the same blocked reasons as the full pipeline MUST apply
