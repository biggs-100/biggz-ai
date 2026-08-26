# FileMerge Specification

## Purpose

The filemerge domain provides JSONC merging and atomic file writing utilities used by install and sync workflows.

## Requirements

### Requirement: WriteFileAtomic

The system MUST provide `WriteFileAtomic(path string, content []byte, perm os.FileMode) (WriteResult, error)`. `WriteResult` MUST contain `Changed bool` and `Created bool`. The function MUST read the existing file (if any) and compare its bytes to `content`. If identical, MUST skip the write and return `Changed: false`. If the file does not exist, MUST create it atomically via temp+rename and return `Created: true`. If the file exists and content differs, MUST overwrite atomically and return `Changed: true`. Parent directories MUST NOT be auto-created — callers are responsible for `MkdirAll`. On write failure, the original file MUST be preserved.

#### Scenario: Content unchanged — skip

- GIVEN a file at path with content "hello"
- WHEN `WriteFileAtomic(path, []byte("hello"), 0644)` is called
- THEN the file content MUST remain unchanged
- AND the returned `WriteResult` MUST have `Changed: false, Created: false`

#### Scenario: New file — create

- GIVEN no file exists at path
- WHEN `WriteFileAtomic(path, []byte("content"), 0644)` is called
- THEN a new file MUST be created with "content"
- AND the returned `WriteResult` MUST have `Created: true, Changed: false`

#### Scenario: Content differs — overwrite

- GIVEN a file at path with "old content"
- WHEN `WriteFileAtomic(path, []byte("new content"), 0644)` is called
- THEN the file MUST contain "new content"
- AND the returned `WriteResult` MUST have `Changed: true, Created: false`

#### Scenario: Non-existent parent directory

- GIVEN a path whose parent directory does not exist
- WHEN `WriteFileAtomic` is called
- THEN the function MUST return an error
- AND no file MUST be created

### Requirement: MergeJSONC

The system MUST provide `MergeJSONC(existing, overlay []byte) ([]byte, error)` that deep-merges two JSONC byte slices. JSONC comments (`//` and `/* */`) and trailing commas MUST be stripped from both inputs before parsing. The function MUST correctly preserve `//` and `/*` sequences inside JSON string literals. Nested map values from overlay MUST be merged recursively into existing. Flat (non-map) overlay values MUST replace existing values at the same key. Arrays in overlay MUST replace existing arrays entirely — no element-wise merge. If a value in overlay contains `"__replace__": true`, that value MUST replace the target entirely (the `__replace__` key itself is stripped from output). If overlay is empty, the function MUST return existing unchanged. If either input is invalid JSON after stripping, the function MUST return an error.

#### Scenario: Flat key merge

- GIVEN `existing = {"name": "test", "version": 1}` and `overlay = {"enabled": true}`
- WHEN `MergeJSONC` is called
- THEN the result MUST contain all three keys with correct values

#### Scenario: Overlay replaces flat key

- GIVEN `existing = {"name": "original", "color": "red"}` and `overlay = {"name": "replaced"}`
- WHEN `MergeJSONC` is called
- THEN the result MUST have `name: "replaced"` and `color: "red"` preserved

#### Scenario: Deep merge of nested objects

- GIVEN `existing = {"settings": {"theme": "dark", "font": 12}}` and `overlay = {"settings": {"theme": "light"}}`
- WHEN `MergeJSONC` is called
- THEN the result MUST have `settings.theme: "light"` and `settings.font: 12` preserved from existing

#### Scenario: `__replace__` sentinel

- GIVEN `existing = {"nested": {"a": 1, "b": 2}}` and `overlay = {"nested": {"__replace__": true, "c": 3}}`
- WHEN `MergeJSONC` is called
- THEN the result MUST have `nested = {"c": 3}` (a and b removed, `__replace__` key stripped)

#### Scenario: Array replacement

- GIVEN `existing = {"items": [1, 2, 3]}` and `overlay = {"items": [4, 5]}`
- WHEN `MergeJSONC` is called
- THEN the result MUST have `items = [4, 5]` (entire array replaced)

#### Scenario: Invalid JSON returns error

- GIVEN invalid JSON for existing or overlay
- WHEN `MergeJSONC` is called
- THEN an error MUST be returned

#### Scenario: Comments and trailing commas stripped

- GIVEN input with `//` line comments, `/* */` block comments, and trailing commas inside strings
- WHEN `MergeJSONC` processes the input
- THEN comments and trailing commas MUST be removed, string content preserved, and output MUST be valid JSON

#### Scenario: Empty overlay

- GIVEN `existing = {"a": 1}` and `overlay = []byte{}` or nil
- WHEN `MergeJSONC` is called
- THEN the result MUST equal the existing input unchanged

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
