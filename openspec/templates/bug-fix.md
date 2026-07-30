# Bug Fix Spec Template

## REQ-001: Bug Description

**Title**: [Brief description of the bug]
**Affected**: [Component/feature]
**Severity**: critical | high | medium | low
**Reported by**: [Issue # or reporter]

### Current behavior

[What happens now — include error messages, stack traces, screenshots]

### Expected behavior

[What should happen instead]

### Scenario: Reproduce the bug

GIVEN [preconditions]
WHEN [steps to reproduce]
THEN [current incorrect behavior]

### Scenario: After fix

GIVEN [same preconditions]
WHEN [same steps]
THEN [correct behavior]

## REQ-002: Root Cause

**Title**: Root cause analysis
**Description**: [What caused the bug at the code level]

GIVEN the root cause is [description]
WHEN the fix is applied
THEN the root cause is eliminated without breaking existing behavior

## REQ-003: Regression Prevention

**Title**: Test coverage for fix
**Description**: [What test ensures this doesn't regress]

GIVEN [test scenario]
WHEN [test action]
THEN [test assertion]

## REQ-004: Edge Cases

**Title**: Edge cases for [scenario]

| Edge case | Expected behavior |
|-----------|------------------|
| Empty input | [behavior] |
| Null value | [behavior] |
| Maximum value | [behavior] |
| Concurrent access | [behavior] |
