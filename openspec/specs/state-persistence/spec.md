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
