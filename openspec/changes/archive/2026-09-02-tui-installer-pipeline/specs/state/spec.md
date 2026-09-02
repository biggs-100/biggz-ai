# State Specification

## Purpose

Pipeline-aware persistence for `~/.biggz-ai/state.json` with atomic merge, unknown-field preservation, and `--agent` routing. Extends base state-persistence with filemerge atomic writes and dry-run support.

## Requirements

### Requirement: REQ-STATE-001 — Schema and Round-Trip

The system MUST define `InstallState` with `AgentID`, `Components`, `Skills`, `LastSync`, `PendingSync` and MUST support JSON round-trip preserving RFC3339 times. Unknown fields MUST be preserved via raw-map merge path.

#### Scenario: Round-trip preserves all fields

- GIVEN InstallState with all fields populated
- WHEN serialized then deserialized
- THEN all fields MUST be identical

#### Scenario: Unknown fields preserved on merge

- GIVEN existing JSON with `{"AgentID":"opencode","custom":"keep"}`
- WHEN merged with `AgentID=="pi"`
- THEN `custom` MUST remain `"keep"`

### Requirement: REQ-STATE-002 — Atomic File Lifecycle

The system MUST provide `ReadState`/`WriteState` with `filemerge.WriteFileAtomic` (temp+rename) and file lock at `~/.biggz-ai/state.json`. `ReadState` MUST return default when missing and error on malformed JSON. `WriteState` MUST create `~/.biggz-ai/` dir if absent.

#### Scenario: Write then read back

- GIVEN populated InstallState
- WHEN WriteState then ReadState
- THEN states MUST be equal

#### Scenario: Malformed JSON error

- GIVEN invalid JSON content
- WHEN ReadState called
- THEN it MUST return non-nil error

### Requirement: REQ-STATE-003 — Pipeline Step Atomic Merge for --agent

Pipeline state step MUST atomically merge `--agent` selection: `Prepare` validates, `Apply` reads existing via raw map, overwrites `AgentID` and agent-specific Components/Skills, preserves unknown keys, and writes via temp+rename under lock. `--dry-run` MUST not write.

#### Scenario: Merge --agent preserves other agents

- GIVEN state with `AgentID opencode` and custom key
- WHEN Apply with `--agent pi`
- THEN AgentID MUST be `pi` and custom key MUST remain

#### Scenario: Dry-run zero writes

- GIVEN `--dry-run` and no file
- WHEN Prepare executes
- THEN preview MUST list planned AgentID and no file MUST be created

#### Scenario: Atomic no partial

- GIVEN Apply writing
- WHEN inspected mid-write
- THEN target path MUST be either old content or new content, never partial

### Requirement: REQ-STATE-004 — Concurrent Safety

Concurrent pipeline state writes MUST serialize via lock and remain uncorrupted.

#### Scenario: Two concurrent writes serialized

- GIVEN two goroutines calling Apply concurrently
- WHEN both complete
- THEN file MUST be valid JSON and one AgentID MUST win without corruption
