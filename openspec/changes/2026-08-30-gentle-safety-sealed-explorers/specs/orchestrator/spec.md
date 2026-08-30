# Delta for orchestrator

## ADDED Requirements

### Requirement: Sealed Explorer Scout Fallback

The system MUST implement sealed-explorer semantics via `internal/orchestrator/surfaces.go` (`isTaskScopedRepositoryRelativePath`, `readAllowedEditSurfaceEntries`/`hasTaskScopedAllowedEditSurfaces`, `rejectUnscopedBoundedWriterDispatch`) and `internal/sdd/status.go` `ShouldEnforceScopedSurfaces`/`ValidateBoundedWriterSurfaces`: when a bounded writer agent (`worker`|`gentle-ai-worker`) is dispatched and surfaces are absent, unscoped, or fileCount enforces but no valid `## Allowed edit surfaces` block exists, the parent MUST NOT launch the writer as bounded editor; it MUST relaunch as `scout` read-only (no `write`/`edit` tool calls, read-only exploration, log fallback, no human block/prompt). With at least one valid scoped surface and `fileCount>=4` passing, writer MUST proceed.

#### Scenario: Writer without surfaces becomes scout read-only

- GIVEN `input{agent:worker, task:"explore repo", context:""}` with no `## Allowed edit surfaces` heading
- WHEN `rejectUnscopedBoundedWriterDispatch`/`ValidateBoundedWriterSurfaces` (fileCount 4) is called
- THEN it MUST return `Block` with `WRITER_EDIT_SURFACE_REJECTION` and caller MUST relaunch as scout with only read tools; no file write MUST occur

#### Scenario: Writer with valid surfaces passes

- GIVEN `task:"## Allowed edit surfaces\n- internal/orchestrator/surfaces.go\n- docs/guide.md\n"`
- WHEN guard is called
- THEN it MUST return `nil` and writer MAY use `write`/`edit` limited to those surfaces

#### Scenario: Scout fallback is logged without human block

- GIVEN scout relaunch after rejection
- WHEN fallback occurs
- THEN system MUST log `scout_fallback` with reason `WRITER_EDIT_SURFACE_REJECTION` and MUST NOT emit `ask_user_question`/human notification

#### Scenario: Non-writer agent never becomes scout

- GIVEN `input{agent:researcher, task:"no surfaces"}`
- WHEN guard is called
- THEN it MUST return `nil` regardless of surfaces

### Requirement: Task-Scoped Surface Validation and Surface Consistency

The system MUST implement `isTaskScopedRepositoryRelativePath` (normalize `\→/`, reject empty/absolute `C:` or `/` or `~`, reject whitespace/`\t`/`\n`, strip leading `./+`, reject any `..` segment, reject if first segment contains `*?[]{}`) and `hasTaskScopedAllowedEditSurfaces`/`readAllowedEditSurfaceEntries` that extracts the block after `## Allowed edit surfaces` (case-insensitive, up to next `#{1,2}` heading), parses bullet/numbered entries (strip `` ` ``), requires ≥1 entry per heading, validates each via `isTaskScoped...`, dedups/sorts and requires all headings to agree on identical set; `ShouldEnforceScopedSurfaces(fileCount)` MUST return `fileCount>=4` (`3→false`, `4→true`).

#### Scenario: Rejects traversal, absolute, glob first-segment, whitespace

- GIVEN `../x`, `/etc/passwd`, `~/x`, `*.go`, `a[0]/b`, `a b/c`, `""`, `.`
- WHEN `isTaskScopedRepositoryRelativePath` is called
- THEN it MUST return `false`

#### Scenario: Accepts dot-normalized and deep glob

- GIVEN `./src/file.go`, `internal/orchestrator/surfaces.go`, `internal/foo*.go` (glob in second segment)
- WHEN validated
- THEN it MUST return `true`

#### Scenario: Heading parsing requires valid scoped entries

- GIVEN `good="## Allowed edit surfaces\n- \`internal/a.go\`\n"` versus `bad="## Allowed edit surfaces\n- \`../x\`\n"` versus `missing="no heading"`
- WHEN `hasTaskScopedAllowedEditSurfaces` is called
- THEN `good` MUST return `true`; `bad` and `missing` MUST return `false`

#### Scenario: FileCount threshold 3 allows, 4 enforces

- GIVEN `fileCount=3` with `../x` versus `fileCount=4` with same
- WHEN `ValidateBoundedWriterSurfaces` is called
- THEN `3` MUST return `nil`; `4` MUST return `Block` with `WRITER_EDIT_SURFACE_REJECTION`

### Requirement: Sealed Orchestration Logging

The system MUST log sealed decisions (path validation failures, scout fallback) with `agent`, `fileCount`, and offending surface where applicable, at debug/info level without blocking human flow.

#### Scenario: Invalid surface logs offending entry

- GIVEN writer surfaces `["../x"]` at `fileCount=4`
- WHEN rejected
- THEN log MUST contain the offending surface `../x` and `Block true`
