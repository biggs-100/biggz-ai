# Delta for Review

## ADDED Requirements

### Requirement: RDD CAS with gen-%010d.json, LOCK and expectedRevision

The system MUST store clone/worktree overrides as `gen-%010d.json` under `<gitDir>/biggz/rdd-mode` with `LOCK` flock, schema `biggz-ai.rdd-status/v1`, fields `generation/previous_revision/mode/recorded_at/revision` where `revision` is SHA-256 over all fields except `revision`, generation is zero-padded 10 digits capped at `999999999`, and `expectedRevision` MUST be enforced via `ErrRDDModeRevisionMismatch`.

#### Scenario: CAS write with matching revision holds LOCK

- GIVEN head `gen-0000000003.json` rev `abc` with flock on `LOCK`
- WHEN `SetCloneLocalRDDMode` with `expectedRevision="abc"` and `disabled`
- THEN `gen-0000000004.json` MUST be created with `generation=4`, `previous_revision="abc"` and LOCK held across read→compute→publish

#### Scenario: Mismatch fails closed

- GIVEN head rev `head-rev`
- WHEN write with `expectedRevision="stale-rev"`
- THEN MUST fail with `ErrRDDModeRevisionMismatch` `expected "stale-rev" but head is "head-rev"` and create no file

#### Scenario: Corrupt head repaired in-place

- GIVEN head `gen-0000000005.json` corrupt
- WHEN disable requested
- THEN MUST overwrite slot `5` in-place (not chain) with valid revision

#### Scenario: Immutable publish rejects overwrite

- GIVEN `gen-0000000004.json` exists with B1
- WHEN `publishImmutable` writes same slot with B2
- THEN MUST fail (O_CREATE|O_EXCL) and B1 stays unchanged

#### Scenario: RDDStatus exposes Revision token

- GIVEN head rev `tok123`
- WHEN `RDDStatus` called
- THEN report MUST return `Revision="tok123"`

### Requirement: RDD Reach and Pre-Relocation Mirror

The system MUST define `RDDModeReach` `""` unreported/`machine`/`this_build`, publish clone writes to both relocated root `<commonDir>/biggz/rdd-mode` and mirror `<commonDir>/gentle-ai/.../rdd-mode` with identical bytes at same slot (relocated first), and return `RDDModePartialApplyError` on half-applied failure.

#### Scenario: ReachMachine when both succeed

- GIVEN mirror reachable and writable
- WHEN clone disable succeeds at both paths
- THEN `Reach="machine"` and both files identical at same slot

#### Scenario: ReachThisBuild when mirror unavailable

- GIVEN mirror missing or unwritable
- WHEN clone disable succeeds at relocated only
- THEN `Reach="this_build"` and relocated file exists

#### Scenario: ReachUnreported on reads

- GIVEN any persisted state
- WHEN `RDDStatus`/`ResolveRDDMode` called
- THEN `Reach=""` and MUST NOT probe mirror

#### Scenario: PartialApplyError on mirror failure

- GIVEN relocated succeeds, mirror reachable but publish fails
- WHEN write completes
- THEN MUST return `RDDModePartialApplyError` wrapping cause and not report as applied

### Requirement: Source-Aware RDDDisabledError

The system MUST carry `RDDDisabledError{Source,Operation}` with `Source` `default|global|clone_local|worktree` and `Operation` `start|mutate`, print exact enable command per Source, and append frozen wording only for `mutate`.

#### Scenario: Global source prints single command

- GIVEN source `global` or `default`
- WHEN `Error()` called
- THEN MUST contain `biggz rdd enable --scope=global` without `then`

#### Scenario: Clone source prints chained command

- GIVEN source `clone`/`clone_local`
- WHEN `Error()` called
- THEN MUST contain `biggz rdd enable --scope=global then biggz rdd enable --scope=clone`

#### Scenario: Mutate appends frozen wording

- GIVEN `Operation=mutate`
- WHEN `Error()` called
- THEN MUST contain `the review is frozen, not discarded` and `to continue it from where it stopped`

#### Scenario: Start omits frozen wording

- GIVEN `Operation=start`
- WHEN `Error()` called
- THEN MUST NOT contain `frozen, not discarded`

#### Scenario: Authorize propagates Source

- GIVEN `RDDStatus` source `worktree` disabled
- WHEN `AuthorizeRDDOperation(Mutate)` called
- THEN MUST return `RDDDisabledError{Source=worktree,Operation=mutate}`

#### Scenario: Read never blocked

- GIVEN mode `disabled`
- WHEN `AuthorizeRDDOperation(Read)` called
- THEN MUST return nil
