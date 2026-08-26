# Delta for filemerge

## ADDED Requirements

### Requirement: Hashline Content-Hash Guarded Edits

The system MUST provide `ComputeHash(content []byte) string` and `ApplyWithHash(path string, expectedHash string, newContent []byte) error` in `internal/filemerge/hashline.go`. `ComputeHash` MUST hash the exact target range (not the whole file) using a stable content hash (e.g., SHA-256 hex). `ApplyWithHash` MUST validate the current on-disk hash against `expectedHash` before writing; on mismatch it MUST NOT overwrite, MUST re-read the file, and MUST return a `needs_attention` error carrying the fresh hash. Callers in `internal/review/correction.go` MUST compute and store the hash at read time and validate at write time. The system MUST support a `force` flag that bypasses hash validation when explicitly set. The design follows warn-and-stop (avisar-y-parar), not silent retry.

#### Scenario: Hash matches — apply succeeds

- GIVEN file at path with range hash `abc123`
- WHEN `ApplyWithHash(path, "abc123", newContent)` is called and on-disk hash is still `abc123`
- THEN the edit MUST be applied atomically and return no error

#### Scenario: Hash mismatch — warn and stop with fresh hash

- GIVEN `ApplyWithHash(path, "abc123", newContent)` but on-disk hash is now `def456`
- WHEN validation runs
- THEN it MUST return an error with `code: needs_attention` and `freshHash: def456`
- AND the file MUST remain unchanged and the whole batch MUST NOT abort

#### Scenario: Concurrent nearby edits trigger mismatch

- GIVEN two subagents read the same file range with hash `h1`
- WHEN agent A writes first and changes hash to `h2`, then agent B calls `ApplyWithHash` with `h1`
- THEN agent B MUST receive `needs_attention` with `freshHash: h2`

#### Scenario: Force flag bypasses validation

- GIVEN `ApplyWithHash` called with `force: true` and stale hash
- WHEN validation would otherwise fail
- THEN the system MUST overwrite regardless of hash mismatch

#### Scenario: Exact-range hashing only

- GIVEN file with 100 lines and target range lines 10-20
- WHEN `ComputeHash` is called on that range vs whole file
- THEN hash MUST differ from whole-file hash and reflect only the range
