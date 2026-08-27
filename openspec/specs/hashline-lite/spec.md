# hashline-lite Specification

## Purpose

Lite DSL for `sdd-apply`: PUT/CUT with `#A1B2` hash, seen/hash guard, snapshot, NoopLoopGuard, atomic fallback. Saves ≥60% tokens vs `str_replace`.

## Requirements

### Requirement: DSL Parsing with #A1B2

System MUST parse `PUT N.=M:`, `PUT <N`, `CUT N.=M:` / `CUT <N` with `#A1B2` 4-hex xxhash (SHA-256→4-hex MAY). MUST reject missing/malformed/non-hex tag. Whole-file fallback MUST NOT exist.

#### Scenario: Valid PUT/CUT accepted

- GIVEN `PUT 10.=20: #A1B2` or `CUT 5.=8: #A1B2` with correct hash
- WHEN parsed
- THEN MUST be accepted with start, end, hash extracted

#### Scenario: Bad tag rejected

- GIVEN `PUT 10.=20:` without tag or `#ZZZZ`
- WHEN parsed
- THEN MUST be rejected

### Requirement: Seen-Range Guard

System MUST track seen ranges from read hook and MUST reject any `N`/`N.=M` not seen. Unseen MUST NOT be applied.

#### Scenario: Unseen rejected

- GIVEN seen `[1-20]` and `PUT 50.=60: #A1B2`
- WHEN validated
- THEN MUST be rejected with unseen-range error

#### Scenario: Seen accepted

- GIVEN seen `[1-20]` and `PUT 10.=15: #A1B2` valid
- WHEN validated
- THEN MUST pass

### Requirement: Hash-Guarded Apply

System MUST compare `expectedHash` (`#A1B2`) vs `ComputeHash(exactRange)` before write. On match PUT MUST write range and CUT MUST remove range via `WriteFileAtomic`. On mismatch MUST NOT overwrite, MUST return `needs_attention`+`freshHash`, leave file unchanged, NOT abort batch, NOT retry silently.

#### Scenario: Match writes

- GIVEN on-disk hash `A1B2` and directive `A1B2`
- WHEN applied
- THEN file MUST be updated atomically

#### Scenario: Mismatch warn-and-stop

- GIVEN directive `h1` but on-disk is `h2`
- WHEN validated
- THEN MUST return `needs_attention` with `freshHash=h2` and file unchanged

#### Scenario: Batch safe

- GIVEN batch file A stale, file B fresh
- WHEN A mismatches
- THEN A MUST be skipped and B MUST still be applied

### Requirement: ComputeHash Exact-Range

System MUST hash only exact bytes, not whole file. MUST produce 4-hex `#A1B2`; empty/nil MUST yield `e3b0c442...`. Range 10-20 of 100-line fixture MUST differ from whole-file hash.

#### Scenario: Range differs from whole

- GIVEN 100-line fixture, range 10-20
- WHEN `ComputeHash(range)` vs `ComputeHash(whole)`
- THEN MUST differ and range hash MUST equal direct hash of those bytes

#### Scenario: Empty digest

- GIVEN `nil` or empty input
- WHEN `ComputeHash` called
- THEN MUST return `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`

### Requirement: Snapshot Store

System MUST keep bounded snapshot store from read hook, MUST allow restore to last seen content, MUST bound growth and clear after batch.

#### Scenario: Restore

- GIVEN file read and snapshot stored
- WHEN `Restore(path)` called
- THEN file MUST be restored to snapshot bytes

#### Scenario: Bounded

- GIVEN batch of N files
- WHEN snapshots created
- THEN store MUST hold ≤N entries

### Requirement: NoopLoopGuard

System MUST abort no-op where `newContent == currentRangeContent`; no write MUST occur.

#### Scenario: No-op aborts

- GIVEN `newContent` equals current bytes
- WHEN NoopLoopGuard checks
- THEN MUST abort with no write

### Requirement: Fallback Atomicity

Writes MUST use `WriteFileAtomic` temp+rename. On failure original MUST be preserved. Parent dirs MUST NOT be auto-created. Windows `Access is denied` MUST surface as contention.

#### Scenario: Failure preserves original

- GIVEN parent missing or rename fails
- WHEN `WriteFileAtomic` called
- THEN MUST return error and original unchanged

### Requirement: Edit Mode Flag and Quality Gates

System MUST expose `edit.mode=hashline` opt-in; without flag legacy MUST remain. Code MUST be <400 lines in `internal/edit/hashline`, pass `go vet ./...` and `go test ./... -count=1 -timeout 180s`. Token saving ≥60% vs `str_replace` MUST be measured.

#### Scenario: Flag disabled keeps legacy

- GIVEN `edit.mode` not set
- WHEN `sdd-apply` runs
- THEN hashline-lite MUST NOT activate

#### Scenario: Flag enabled routes to hashline

- GIVEN `edit.mode=hashline`
- WHEN valid PUT/CUT directive submitted
- THEN MUST route through hashline-lite parser/apply

#### Scenario: Gates pass

- GIVEN change applied
- WHEN `go vet` and `go test ./... -count=1 -timeout 180s` run
- THEN both MUST pass
