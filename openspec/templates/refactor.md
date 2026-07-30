# Refactor Spec Template

## REQ-001: Motivation

**Title**: Why refactor
**Description**: [Current pain points — technical debt, performance, readability]

| Pain point | Impact |
|------------|--------|
| [Issue] | [Impact description] |

## REQ-002: Scope

**Title**: What changes
**Description**: [Which files, packages, or components are affected]

### Files to modify

| File | Current approach | New approach |
|------|-----------------|--------------|
| `path/to/file.go` | [current] | [new] |

### Files to delete

- `path/to/obsolete.go`

### Files to create

- `path/to/new.go`

## REQ-003: Behavioral Preservation

**Title**: No behavior change
**Description**: The refactor MUST NOT change external behavior

GIVEN the same inputs as before the refactor
WHEN the code executes
THEN the outputs are identical to the pre-refactor behavior

## REQ-004: Migration Path

**Title**: Gradual migration

GIVEN existing consumers of the old API
WHEN the refactor is applied
THEN [migration strategy — deprecation, backward compat, codemod]

## REQ-005: Verification

**Title**: How to verify the refactor

GIVEN the existing test suite
WHEN all tests pass with the new implementation
THEN the refactor is complete

| Test suite | Command |
|------------|---------|
| Unit tests | `go test ./...` |
| Integration | `go test ./... -run Integration` |

GIVEN benchmark results before the refactor
WHEN benchmarks are run after the refactor
THEN performance is [same/better/worse — define acceptable threshold]
