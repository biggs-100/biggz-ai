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
