# CLI Command Spec Template

## REQ-001: Command Interface

**Title**: `biggz [command]`
**Description**: [What the command does]

### Usage

```
biggz [command] <positional-args> [--flags]
```

### Arguments

| Position | Name | Required | Description |
|----------|------|----------|-------------|
| 1 | [arg] | yes/no | [description] |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--[flag]` | string/int/bool | [default] | [description] |

### Scenario: Basic usage

GIVEN the user runs `biggz [command] [args]`
WHEN the command executes
THEN it [expected behavior]

### Scenario: With flags

GIVEN the user runs `biggz [command] --[flag]=[value]`
WHEN the command executes
THEN it [expected behavior with flag]

### Scenario: Invalid input

GIVEN the user runs `biggz [command]` without required args
WHEN the command executes
THEN it prints usage information and exits with code 1

## REQ-002: Output Format

**Title**: Output format for [command]
**Description**: [How the command outputs data]

GIVEN the command completes successfully
WHEN output is printed
THEN it follows format [description]

GIVEN the `--json` flag is passed
WHEN the command completes
THEN output is valid JSON

## REQ-003: Error Handling

**Title**: Error behavior for [command]

GIVEN the command encounters [error condition]
WHEN it executes
THEN it prints a descriptive error message to stderr
AND exits with a non-zero code
