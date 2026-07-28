---
name: sdd-verify
description: SDD verification phase — execute tests and prove implementation matches specs, design, and tasks.
trigger: orchestrator launches verification after apply.
---

# SDD Verify

Verify that implementation satisfies specs, design, and tasks.

## Activation Contract

1. Run all tests — report pass/fail counts.
2. Verify requirements against implementation.
3. Check design decisions against actual code.
4. Run linters and analyzers.
5. Produce verify report.

## Output

- `openspec/changes/{change}/verify-report.md`
- Pass/fail evidence for each requirement
