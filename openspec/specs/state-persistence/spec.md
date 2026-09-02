# State Persistence Specification

## Purpose

The State Persistence domain provides JSON-based persistence of agent installation state to `~/.biggz-ai/state.json`. It defines the schema, read/write lifecycle, and merge strategy for reconciling external state updates with local state.

## Requirements

### Requirement: InstallState Schema

The system MUST define an `InstallState` struct with fields: AgentID (string), Components (map[string]ComponentStatus), Skills (map[string]SkillStatus), LastSync (time.Time), and PendingSync (int). ComponentStatus MUST include Installed (bool) and Version (string). SkillStatus MUST include Deployed (bool) and Hash (string).

#### Scenario: Happy path — valid state round-trip

- GIVEN an InstallState with all fields populated
- WHEN it is serialized to JSON and deserialized back
- THEN all fields MUST round-trip with identical values
- AND time.Time MUST survive JSON round-trip via RFC 3339

#### Scenario: Extra unknown fields in JSON

- GIVEN a JSON file with fields not in the InstallState struct
- WHEN it is deserialized
- THEN unknown fields MUST be preserved if the merge strategy is used
- OR silently ignored if reading into the struct directly
- AND the system MUST NOT fail to deserialize

### Requirement: State File Read

The system MUST provide a `ReadState(homeDir string) (*InstallState, error)` function. If the state file does not exist, the function MUST return a default InstallState (AgentID: "", Components: empty map, Skills: empty map, zero time) and nil error.

#### Scenario: Happy path — file exists

- GIVEN a valid state.json at `~/.biggz-ai/state.json`
- WHEN ReadState is called
- THEN it MUST return the parsed InstallState
- AND the AgentID MUST match the file contents

#### Scenario: File does not exist

- GIVEN no state.json at the expected path
- WHEN ReadState is called
- THEN it MUST return a default InstallState
- AND the error MUST be nil

#### Scenario: Malformed JSON

- GIVEN a state.json with invalid JSON content
- WHEN ReadState is called
- THEN it MUST return nil and a non-nil error
- AND the error MUST describe the parsing failure

### Requirement: State File Write

The system MUST provide a `WriteState(homeDir string, state *InstallState) error` function. The function MUST create `~/.biggz-ai/` directory if it does not exist.

#### Scenario: Happy path — write and read back

- GIVEN a populated InstallState
- WHEN WriteState is called, then ReadState is called
- THEN the read-back state MUST match the written state

#### Scenario: Nil state

- GIVEN nil as the state parameter
- WHEN WriteState is called
- THEN it MUST return an error
- AND the file MUST NOT be created or modified

### Requirement: Merge Strategy

The system MUST provide a `MergeState(base, incoming *InstallState) *InstallState` function. Merge MUST overwrite fields from incoming onto base. Unknown fields in incoming that don't exist in the schema MUST be preserved during merge.

#### Scenario: Happy path — incoming overwrites

- GIVEN a base state with AgentID "opencode" and an incoming with AgentID "claude"
- WHEN MergeState is called
- THEN the result's AgentID MUST be "claude"
- AND fields not in incoming MUST retain base values

#### Scenario: Nil incoming

- GIVEN a valid base state and nil incoming
- WHEN MergeState is called
- THEN it MUST return a copy of base (no mutation)
- AND no error MUST be returned

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
