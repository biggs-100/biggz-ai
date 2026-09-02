# Delta for state-persistence

## Purpose

Extend state persistence with pipeline-aware atomic merge for `~/.biggz-ai/state.json` handling `--agent` flag, preserving unknown fields, and using `filemerge` atomic writes.

## ADDED Requirements

### Requirement: REQ-STATE-PIPE-001 — state.json Path and Atomic Merge

The system MUST persist install state at `~/.biggz-ai/state.json` via a pipeline `state` step. Writes MUST be atomic via `filemerge.WriteFileAtomic` (temp+rename) under file lock. The step MUST merge `--agent` selection into state: overwrite `AgentID` and update `Components`/`Skills` for that agent, leaving other agents untouched.

#### Scenario: Atomic write on Apply

- GIVEN state step `Apply` with `AgentID=="pi"`
- WHEN it executes
- THEN `~/.biggz-ai/state.json` MUST exist containing `"AgentID":"pi"`
- AND write MUST be via temp+rename with no partial file visible

#### Scenario: Subsequent --agent overwrites AgentID

- GIVEN existing state `AgentID=="opencode"`
- WHEN state step Apply with `--agent pi` executes
- THEN resulting `AgentID` MUST be `"pi"`

#### Scenario: Concurrent writes serialized via lock

- GIVEN two concurrent `WriteState` calls
- WHEN both Apply
- THEN one MUST acquire lock, write atomically, and the other MUST retry/serialize without corruption

### Requirement: REQ-STATE-PIPE-002 — Preserve Unknown Fields and Merge Strategy

`ReadState`/`MergeState` MUST preserve unknown JSON fields not in `InstallState` struct. Pipeline state step MUST read existing file with raw map, merge known fields, and write back preserving unknown keys. If file does not exist, it MUST create default state.

#### Scenario: Unknown fields preserved on merge

- GIVEN existing `state.json` with `{"AgentID":"opencode","custom_key":"keep"}`
- WHEN merge with incoming `AgentID=="claude"` executes
- THEN output MUST contain `"custom_key":"keep"` unchanged

#### Scenario: Unknown fields survive round-trip

- GIVEN file with unknown nested object
- WHEN `ReadState` then `WriteState` with different AgentID
- THEN unknown object MUST remain byte-identical

#### Scenario: Missing file creates default then merges

- GIVEN no `state.json` at `~/.biggz-ai/state.json`
- WHEN state step Apply runs
- THEN file MUST be created with default + incoming `AgentID`

### Requirement: REQ-STATE-PIPE-003 — Dry-Run Zero Writes for State

When pipeline `Prepare` runs in `--dry-run` mode, state step MUST report planned merge without writing. `Apply` MUST be skipped for state step in dry-run. Preview MUST include target path and merged `AgentID`.

#### Scenario: Dry-run reports without file creation

- GIVEN `--dry-run` and no existing `state.json`
- WHEN `Prepare` executes
- THEN preview MUST state `write ~/.biggz-ai/state.json with AgentID X`
- AND file MUST NOT exist after Prepare

#### Scenario: Dry-run with existing file preserves it

- GIVEN existing `state.json` with `AgentID=="opencode"`
- WHEN dry-run Prepare with `--agent pi`
- THEN preview MUST show `AgentID: opencode -> pi`
- AND file content MUST remain `opencode` after Prepare
