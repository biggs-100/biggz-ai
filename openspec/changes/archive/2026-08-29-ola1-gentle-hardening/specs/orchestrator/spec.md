# Delta for orchestrator

## ADDED Requirements

### Requirement: Task-Scoped Repository-Relative Path Validation

The system MUST implement `internal/orchestrator/surfaces.go` `isTaskScopedRepositoryRelativePath(value string) bool` that normalizes `\` to `/`, rejects empty, absolute (`/` or `C:`), `~`-prefixed, whitespace-containing, or `..`-segment paths, strips leading `./` segments, and rejects if the first path segment contains glob characters `*?[]{}`. Only narrow repository-relative paths pass.

#### Scenario: Rejects parent traversal

- GIVEN value `../x`
- WHEN `isTaskScopedRepositoryRelativePath` is called
- THEN it MUST return `false`

#### Scenario: Rejects absolute and home paths

- GIVEN value `/etc/passwd` or `~/x`
- WHEN validated
- THEN it MUST return `false`

#### Scenario: Rejects glob in first segment

- GIVEN value `*.go` or `a[0]/b` or `a{b}/c`
- WHEN validated
- THEN it MUST return `false`

#### Scenario: Rejects whitespace paths

- GIVEN value `a b/c`
- WHEN validated
- THEN it MUST return `false`

#### Scenario: Accepts scoped and dot-normalized paths

- GIVEN value `src/pkg/file.go` or `./src/file.go`
- WHEN validated
- THEN it MUST return `true` (after `./` stripped and first-segment check)

### Requirement: Bounded Writer Dispatch Surface Guard

The system MUST implement `internal/sdd/status.go` `ShouldEnforceScopedSurfaces(fileCount int) bool` returning `fileCount >= 4` (3 allow, 4 enforce) and `ValidateBoundedWriterSurfaces` / `internal/orchestrator/surfaces.go` `rejectUnscopedBoundedWriterDispatch` that when `fileCount >= 4` checks each allowed surface via `isTaskScopedRepositoryRelativePath` and, on any unscoped entry, returns `ScopedSurfaceRejection{Block:true, Reason:WRITER_EDIT_SURFACE_REJECTION}` instructing parent to derive narrow surfaces and relaunch.

#### Scenario: FileCount 3 allows without per-path check

- GIVEN `fileCount` is `3` and surfaces contain `../x`
- WHEN `ValidateBoundedWriterSurfaces` / `rejectUnscopedBoundedWriterDispatch` is called
- THEN it MUST allow (return nil) without per-path enforcement

#### Scenario: FileCount 4 enforces per-path and rejects unscoped

- GIVEN `fileCount` is `4` and one surface is `../x` or `*.go`
- WHEN guard is called
- THEN it MUST return a rejection with `WRITER_EDIT_SURFACE_REJECTION` and `Block true`

#### Scenario: FileCount 4 passes when all surfaces scoped

- GIVEN `fileCount` is `4` and surfaces are `["src/a.go", "internal/b.go"]`
- WHEN guard is called
- THEN it MUST return nil (allow)
