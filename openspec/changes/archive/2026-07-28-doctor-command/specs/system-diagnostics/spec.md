# System Diagnostics Specification

## Purpose

The system-diagnostics domain defines a health check framework for biggz-ai installations. It provides typed checks, a panic-isolated runner, severity-categorized reports, and atomic remedies.

## Requirements

### Requirement: Check Framework

MUST define CheckID, Status (pass/warn/fail), Result, Check (ID + Run func), and Runner (ordered checks with Run()). The Runner MUST isolate panics per check via recover — one panic MUST NOT abort remaining checks.

#### Scenario: Panic isolation

- GIVEN a Runner with checks [A, B, C] where B panics
- WHEN Run() completes
- THEN Result[0] and Result[2] have their check outputs
- AND Result[1] MUST have Status=fail with the captured panic message

### Requirement: Report Categorization

MUST produce a Report grouping results into CRITICAL (fail), WARNING (warn), and INFO (pass) buckets. Each bucket MUST be independently iterable.

#### Scenario: Severity groups

- GIVEN a Report with 1 fail, 1 warn, 2 passes
- WHEN accessing severity buckets
- THEN CRITICAL MUST contain 1 result, WARNING 1, INFO 2

### Requirement: Atomic Remedies

Checks MAY declare a Remedy (ID + Description + Action func). The system MUST iterate declared remedies for --fix. Each Action MUST complete atomically and return an error on failure.

#### Scenario: Remedy dispatch

- GIVEN a check with a declared Remedy
- WHEN the remedy Action executes
- THEN it MUST complete atomically
- AND return an error if it cannot be applied

### Requirement: SQLite Integrity Check

MUST verify bigmem SQLite integrity via PRAGMA integrity_check. A failure MUST produce a CRITICAL result.

#### Scenario: Database corruption

- GIVEN a bigmem SQLite file with integrity violations
- WHEN the check runs
- THEN Result MUST have Status=fail with severity CRITICAL

### Requirement: Config Directory Check

MUST verify the biggz-ai config directory and required subdirectories exist. Missing directories MUST produce a CRITICAL result.

#### Scenario: Missing subdirectory

- GIVEN a config directory missing the `plugins` subdirectory
- WHEN the check runs
- THEN Result MUST have Status=fail

### Requirement: MCP Binary Presence

MUST verify biggz-mcp exists at the expected location and is executable. Absence MUST produce a CRITICAL result.

#### Scenario: Binary not found

- GIVEN biggz-mcp is absent from the expected path
- WHEN the check runs
- THEN Result MUST have Status=fail
- AND the message MUST include the expected path

### Requirement: Review Store Chain Integrity

MUST verify the review transaction chain via store.Validate(). A broken chain MUST produce a CRITICAL.

#### Scenario: Missing transaction

- GIVEN a review store with a gap in the transaction chain
- WHEN the check runs
- THEN Result MUST have Status=fail

### Requirement: PATH Shadowing Check

MUST scan PATH for duplicate binaries named biggz or biggz-mcp. Duplicates across directories MUST produce a WARNING result.

#### Scenario: Shadowed binary

- GIVEN PATH contains two directories each with biggz-mcp
- WHEN the check runs
- THEN Result MUST have Status=warn
- AND the message MUST list both paths

### Requirement: Disk Space Check

MUST check available disk space on the biggz data partition. Below 500 MB free MUST produce a WARNING result with remaining space in the message.

#### Scenario: Low disk space

- GIVEN the data partition has 200 MB free
- WHEN the check runs
- THEN Result MUST have Status=warn
- AND the message MUST include "200 MB"

### Requirement: Git Availability

MUST verify git is in PATH and the working directory is a git repository. Git absence MUST produce a CRITICAL result; not a git repo MUST produce a WARNING.

#### Scenario: Git not installed

- GIVEN git is not found in PATH
- WHEN the check runs
- THEN Result MUST have Status=fail
- AND severity MUST be CRITICAL

### Requirement: Version Information

MUST report the installed and latest available biggz-ai version. A mismatch MUST produce an INFO result with both versions.

#### Scenario: Version drift

- GIVEN installed v1.0.0 and latest is v1.1.0
- WHEN the check runs
- THEN Result MUST have Status=pass with INFO severity
- AND the message MUST include "v1.0.0" and "v1.1.0"

### Requirement: Backup State Check

MUST verify the last backup timestamp is within 7 days. A backup older than 7 days MUST produce a WARNING result with age in days.

#### Scenario: Stale backup

- GIVEN the last backup was 10 days ago
- WHEN the check runs
- THEN Result MUST have Status=warn
- AND the message MUST include "10 days"
