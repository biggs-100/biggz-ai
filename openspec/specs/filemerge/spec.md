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
