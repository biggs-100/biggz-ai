# Delta for bigmem — bigmem-rescue-ownership

## ADDED Requirements

### Requirement: REQ-RO1 — Atomic Per-Write Session Adoption (resolveWriteProjectTx)

The system MUST provide `resolveWriteProjectTx(sessionID, requestedProject)` that atomically claims an orphan session when no foreign owner exists.

#### Scenario: Orphan adopted atomically in same TX
- GIVEN session `S` exists with `project IS NULL OR trim(project)=''`
- AND `foreignRecordOwnerTx(S)` finds zero rows with `project != requestedProject`
- WHEN `resolveWriteProjectTx(S, "projA")` runs inside the caller's `BEGIN IMMEDIATE` TX
- THEN `sessions.project` for `S` MUST become `projA` in same TX and the caller MUST succeed

#### Scenario: Already-owned session is no-op
- GIVEN session `S` has `project="projA"`
- WHEN `resolveWriteProjectTx(S, "projA")` is called
- THEN no UPDATE MUST occur and the call MUST succeed

### Requirement: REQ-RO2 — Ambiguous Ownership Rejection

The system MUST reject adoption when foreign observations exist and MUST return `ErrProjectOwnershipAmbiguous` with a rescue hint.

#### Scenario: Foreign project blocks adoption
- GIVEN session `S` has `project IS NULL`
- AND an observation exists with `session_id=S AND project="other" AND project != "projA"`
- WHEN `resolveWriteProjectTx(S, "projA")` is invoked
- THEN it MUST return `ErrProjectOwnershipAmbiguous` and NOT update `sessions.project`

#### Scenario: Error carries rescue hint
- GIVEN `ErrProjectOwnershipAmbiguous` is returned for session `S` and project `projA`
- WHEN the error is formatted
- THEN it MUST contain `biggz bigmem rescue-ownership --project projA --session S`

### Requirement: REQ-RO3 — Bulk Rescue with Plan (RescueNullProjectOwnership)

The system MUST provide `RescueNullProjectOwnership(project)` that bulk-adopts all orphan sessions via a two-phase Plan.

#### Scenario: Bulk adopts N orphans
- GIVEN N sessions exist with `project IS NULL OR trim(project)=''`
- AND none have foreign owners for project `projA`
- WHEN `RescueNullProjectOwnership("projA")` executes
- THEN N sessions MUST be updated to `projA`, each via per-session TX with `adoptSessionOwnershipTx`, and result MUST report `adopted=N`

#### Scenario: Plan dry-run matches apply
- GIVEN `RescueNullProjectOwnership.Plan("projA")` is called (or `--dry-run`)
- WHEN the subsequent `RescueNullProjectOwnership("projA")` is applied
- THEN Plan counts/IDs and `ambiguous` list MUST match the apply result; `unknown` project values MUST NOT be rescuable

### Requirement: REQ-RO4 — Save Integration

The system MUST integrate ownership resolution into `Save` before dedup, serialized under `Store.mu` and `BEGIN IMMEDIATE`.

#### Scenario: Save resolves before dedup in single TX
- GIVEN `Save` is called with `sessionID=S` and requested project `projA`
- WHEN `Save` executes
- THEN it MUST call `resolveWriteProjectTx` before FTS dedup, inside a single `BEGIN IMMEDIATE` TX holding `Store.mu`, and succeed after adoption

#### Scenario: Concurrent saves remain serialized
- GIVEN two concurrent `Save` calls for the same orphan session `S` with `projA`
- WHEN both enter `Store.mu` + `BEGIN IMMEDIATE`
- THEN exactly one adoption MUST win, the other MUST see already-owned and proceed without error

### Requirement: REQ-RO5 — CLI rescue-ownership

The system MUST provide `biggz bigmem rescue-ownership --project X [--session Y] [--dry-run] [--json]` for operator-initiated rescue.

#### Scenario: Bulk rescue via CLI
- GIVEN N orphan sessions exist
- WHEN `biggz bigmem rescue-ownership --project projA` runs
- THEN output MUST report adopted count N (or JSON with `adopted`, `skipped`, `ambiguous`)

#### Scenario: Scoped and dry-run modes
- GIVEN orphan session `S` exists
- WHEN `biggz bigmem rescue-ownership --project projA --session S --dry-run --json` runs
- THEN no DB mutation MUST occur, stdout MUST be valid JSON with matching Plan, and `--session S` MUST limit scope to `S` only
