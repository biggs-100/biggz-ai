# Review Candidate View Specification

## Purpose

Review candidate view is the RO materialization sub-domain of review — same contract as top-level `candidate-view` but nested under `review` for parity with `lib/review-candidate-view.ts`.

## Requirements

### Requirement: Read-Only Candidate View with SHA256 Manifest and Symlink Guard

The system MUST materialize candidate view read-only (`0444` files, `0555` dirs in `internal/review/candidate_view.go`), emit `digestChangedPathManifest` as `sha256:<hex>` over canonical JSON, run `git --raw -z` with `GIT_LITERAL_PATHSPECS=1`, classify renames/modeOnly/typeChanged, block `../../etc/passwd` via `isWithin` after symlink resolve, and skip chmod when `GOOS=windows`.

#### Scenario: RO permissions
- GIVEN view materializes on non-Windows
- WHEN `stat` checks files/dirs
- THEN files MUST be `0444` and dirs `0555`

#### Scenario: SHA256 manifest
- GIVEN canonical JSON manifest
- WHEN `digestChangedPathManifest` computes
- THEN output MUST be `sha256:<hex>` of `sha256(JSON)`

#### Scenario: Raw -z handling
- GIVEN rename, modeOnly, typeChanged entries
- WHEN `git --raw -z` with `GIT_LITERAL_PATHSPECS=1` parses
- THEN each MUST be classified correctly, NUL-delimited

#### Scenario: Traversal blocked, Windows skip
- GIVEN path `../../etc/passwd` or symlink escaping repo
- WHEN validated
- THEN MUST be blocked; on `GOOS=windows` chmod MUST be skipped
