# Review Specification

## Purpose

Review covers content-addressed storage, GitCommonDir resolution, flock locking, immutable evidence chains, and candidate capture taxonomy ported from gentle parity.

## Requirements

### Requirement: Store GitCommonDir with Content-Addressed Events

The system MUST resolve store via `GitCommonDir` (`internal/review/store.go` → `git rev-parse --git-common-dir`) to `v1/events/<sha256>` with `publishImmutable`. Flat `.git/biggz/review-transactions/<lineage>` MUST NOT be created anew. Legacy flat MUST remain readable (dual-read).

#### Scenario: Worktree writes to common dir

- GIVEN worktree where `.git` points to `/git/worktrees/wt1`
- WHEN `Open(repo,lineageID)`+`Append`
- THEN file MUST be under `git-common-dir/biggz/review-transactions/<lineage>/v1/events/<sha256>`

#### Scenario: Legacy flat readable

- GIVEN chain under legacy flat
- WHEN `LoadChain` called
- THEN MUST return identical `ValidatedChain` via dual-read

### Requirement: Flock-based File Lock

The system MUST lock `internal/review/lock.go` via `flock(LOCK_EX|LOCK_NB)` on `GitCommonDir/biggz/review-transactions/<lineage>/.lock`. `O_EXCL` MUST NOT be primary. Stale detection (PID+mtime>5m) MUST remain.

#### Scenario: Concurrent serialize via flock

- GIVEN two processes `Finalize` same lineage
- WHEN both `Acquire`
- THEN one MUST get `BusyError` until `flock` releases

### Requirement: PublishImmutable Evidence Chain

The system MUST publish (`model/hash.go`, `internal/review/snapshot.go`, `internal/review/receipt.go`, `internal/review/hash.go`) via `publishImmutable` where `Snapshot.Hash=domainHash("biggz-ai.review-snapshot/v1\x00"+writeLengthPrefixed(baseTree,candidate,paths))`, `EvidenceHash=domainHash("biggz-ai.review-evidence/v1\x00"... )`, and receipt binds `FixDeltaHashForSnapshot` (`internal/review/finalize.go`). Re-materialization MUST be hash-identical.

#### Scenario: Snapshot length-prefix

- GIVEN snapshot baseTree/candidate
- WHEN `computeSnapshotHash` called
- THEN MUST equal gentle vector via `writeLengthPrefixed`+domain

#### Scenario: Idempotent publish

- GIVEN payload at `v1/events/<sha256>`
- WHEN `publishImmutable` same bytes again
- THEN MUST be no-op and chain valid

### Requirement: Candidate Capture Taxonomy and Binary Marker

The system MUST classify candidate capture failures via `wrapRuntimeCandidateUnavailable` — any unavailable runtime candidate (missing lineage, empty candidate tree, or `Binary files differ` git marker) MUST be wrapped as a typed candidate-unavailable error before surfacing. `Binary files differ` output from `git diff` MUST be detected as binary content, treated as a capture taxonomy case (not a generic diff error), and MUST produce a typed wrapped error that review admission can distinguish from transport failures.

#### Scenario: Missing candidate wrapped as unavailable

- GIVEN a capture binding whose target commit has no candidate tree
- WHEN capture resolves the candidate
- THEN it MUST return an error wrapped via `wrapRuntimeCandidateUnavailable` and callers MUST be able to assert the unavailable cause

#### Scenario: Binary files differ marker typed

- GIVEN `git diff --numstat` emits `Binary files a/foo.bin and b/foo.bin differ`
- WHEN `DeriveOriginalChangedLines` / candidate capture parses the diff
- THEN it MUST detect the marker as binary change and return a typed `wrapRuntimeCandidateUnavailable` error (not a parse failure string)

#### Scenario: Unavailable distinguished from transport error

- GIVEN a capture failure due to candidate unavailability
- WHEN a caller checks with `errors.As` for the unavailable taxonomy
- THEN a transport-layer error (e.g., `stdout truncated`) MUST NOT match that type

#### Scenario: Successful capture not wrapped

- GIVEN a valid candidate tree and text diff
- WHEN capture runs
- THEN it MUST not wrap the result with the unavailable taxonomy and MUST return the normal preflight artifact

### Requirement: Provider Contract Offline SHA256 Pin Verification

The system MUST provide `scripts/check-provider-contract.mjs` and `internal/contracts/verify.go` `VerifyProviderContract(lockPath, root) error` that offline-verify `contracts/review-integration/v1/**` and `v2/**` files against `contracts/review-integration/provider-contract.lock.json` via SHA256 hex. `1`-byte drift, unlisted, or missing file MUST cause drift error (`drift <path>` or `unlisted <path>`) and exit `1`; exact pins MUST pass with `check passed N files`. No network fetch (`no fetch`), and manifest covers 44 files (42 v1 + 2 v2). `v2` schema/fixture are ported from `v1` with `v2` identifiers and MUST validate.

#### Scenario: Exact pins pass offline with no fetch

- GIVEN `provider-contract.lock.json` matches SHA256 of every file under `v1` and `v2`
- WHEN `node scripts/check-provider-contract.mjs` or `VerifyProviderContract` runs offline (no network)
- THEN it MUST exit `0` / return nil and report `check passed 44 files`

#### Scenario: One-byte drift fails

- GIVEN one byte is appended to `contracts/review-integration/v1/fixtures/contract.fixture.json`
- WHEN verification runs
- THEN it MUST exit `1` / return `drift <path>` error and print `offline only`

#### Scenario: Unlisted file fails

- GIVEN a new file is added under `contracts/review-integration/v1/` without updating the lock
- WHEN verification runs
- THEN it MUST report `unlisted <path>` and fail

### Requirement: Package Manifest Offline Verification

The system MUST provide `scripts/verify-package-files.mjs` that offline-verifies the sorted relative walk of `contracts/review-integration/v1/**` + `v2/**` against the lock keys (`provider-contract.lock.json`) without reading file contents beyond existence. Unlisted walked file or missing listed key MUST cause `unlisted`/`missing` error and exit `1`; exact match MUST pass `verify passed N files`.

#### Scenario: Exact manifest passes

- GIVEN walked files exactly match lock keys (44 files)
- WHEN `node scripts/verify-package-files.mjs` runs
- THEN it MUST exit `0` with `verify passed 44 files`

#### Scenario: Unlisted file in manifest check fails

- GIVEN a walked file is not in the lock set
- WHEN the script runs
- THEN it MUST report `unlisted <rel>` and exit `1`

#### Scenario: Missing listed key fails

- GIVEN a lock key has no corresponding file on disk
- WHEN the script runs
- THEN it MUST report `missing <rel>` and exit `1`

### Requirement: CI Skill-Lint and Provider-Contract Jobs

The system MUST modify `.github/workflows/ci.yml` to add jobs `skill-lint` (Node 20, `node scripts/check-skill-lint.mjs`) and `provider-contract` (Node 20 + Go stable, `node scripts/check-provider-contract.mjs` and `node scripts/verify-package-files.mjs`) that run after `format` (`needs: format`).

#### Scenario: CI contains skill-lint job after format

- GIVEN `.github/workflows/ci.yml` is parsed
- WHEN inspecting jobs
- THEN `skill-lint` MUST exist with `needs: format` and `run: node scripts/check-skill-lint.mjs`

#### Scenario: CI contains provider-contract job with both checks

- GIVEN the workflow file
- WHEN inspecting `provider-contract` job
- THEN it MUST run both `check-provider-contract.mjs` and `verify-package-files.mjs` with `needs: format`

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
