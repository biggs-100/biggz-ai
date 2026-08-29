# Delta for review

## ADDED Requirements

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
