# Delta for sdd

## MODIFIED Requirements

### Requirement: Sync Execution Contract

The system MUST provide agent `sdd-sync`, skill `internal/assets/skills/sdd-sync/SKILL.md`, prompt `sdd-sync.md`, and implementation `internal/sdd/openspec-deltas.go` + `internal/sdd/sync.go` porting ADDED/MODIFIED/REMOVED from `lib/openspec-deltas.ts` 1:1 without auto-commit, child subagents, or archive move.

The write path MUST additionally enforce three rules. (1) When `openspec/specs/{domain}/spec.md` is absent and the delta file contains `### Requirement:` blocks in full-spec shape, sync MUST create the living spec as a byte-identical copy of that delta file and report `applied`. (2) When a delta file has non-empty content but no `### Requirement:` blocks, sync MUST fail closed with `blocked`, naming the offending file and the remedy, and MUST NOT create or overwrite any living spec. (3) Sync MUST never write empty content over a non-empty source, and a write that produced no content MUST NOT be reported as `applied`.
(Previously: the 1:1 port had no rule for full-spec-shaped new domains, so zero parsed deltas wrote empty content under `applied`.)

#### Scenario: Sync executor without archive move

- GIVEN `openspec/changes/{change}/specs/sdd/spec.md` contains valid deltas
- WHEN `sdd-sync` executes on `openspec` store
- THEN `openspec/specs/sdd/spec.md` MUST reflect deltas and `openspec/changes/{change}/` MUST still exist

#### Scenario: No commit created

- GIVEN sync completed successfully
- WHEN git log is inspected
- THEN no new commit with `sdd-sync` auto-commit MUST exist

#### Scenario: Full-spec new domain copies verbatim

- GIVEN a change whose new-domain delta file is full-spec shaped (`# ... Specification`, `## Purpose`, `### Requirement:` blocks) and its living spec is absent
- WHEN sync applies the change
- THEN `openspec/specs/{domain}/spec.md` MUST exist as a byte-identical copy of the delta file and MUST NOT be empty
- AND the result MUST be `applied`

#### Scenario: Verbatim copy is scoped to absent targets

- GIVEN a domain whose non-empty living spec already exists and a full-spec-shaped delta file
- WHEN sync processes that domain
- THEN the living spec MUST NOT be replaced by a verbatim copy of the delta file

#### Scenario: Content without requirement blocks fails closed

- GIVEN a delta file with non-empty content but no `### Requirement:` blocks
- WHEN sync applies the change
- THEN the result MUST be `blocked` and the message MUST name the offending delta file and the remedy
- AND no living spec MUST be created or overwritten

#### Scenario: Full-spec-only change remains skipped

- GIVEN a change whose only delta file is full-spec shaped
- WHEN sync resolves the change
- THEN the result MUST be `not-applicable` with a message naming the change
- AND zero files under `openspec/specs/` MUST be created or modified

#### Scenario: MODIFIED applies in place

- GIVEN an existing living spec and a delta file with `## MODIFIED Requirements` naming an existing requirement
- WHEN sync applies the change
- THEN the result MUST be `applied` and the living spec MUST contain the updated requirement text
- AND the pre-existing spec header and unmodified requirements MUST be preserved

#### Scenario: No empty write for any shape

- GIVEN a delta file with non-empty content that yields no writable output, including when the change-level gate is bypassed
- WHEN the sync write path runs
- THEN it MUST NOT write empty content over a non-empty source
- AND a write that produced no content MUST NOT be reported as `applied`
