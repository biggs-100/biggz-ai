---
name: sdd-verify
description: Verify SDD implementation against specs, design, and tasks. Run tests, validate requirements, check design coherence, and produce verify report.
trigger: orchestrator launches verification after apply.
---

# SDD Verify

Prove that the implementation satisfies the spec requirements, follows the design, and completes all tasks. Produces a verify report with pass/fail evidence per requirement.

## Activation Contract

1. Run all tests — report pass/fail counts.
2. Validate every requirement (REQ-N) against the implementation.
3. Check design decisions against actual code.
4. Verify every task is marked done with evidence.
5. Run linters and static analysis if configured.
6. Use `biggz sdd-verify-validate` for structured validation.
7. Produce verify report.

## Hard Rules

- Every REQ-N from the spec must have an explicit verification result (pass/fail/skip).
- Verification must be observable — not "I checked the code" but "test X passes" or "output Y matches".
- Never declare verification complete without running the test suite.
- If `biggz sdd-verify-validate` is available, use it as the final validation gate.
- If verification fails, be specific about what failed and why.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| All pass | All tests pass, all requirements verified | Verify report = pass, recommend archive |
| Partial fail | Some tests fail or requirements unmet | Report specifics, recommend fixes |
| Test gap | Requirement has no automated test | Flag as verification gap, manual test evidence acceptable |
| Validation tool | `biggz sdd-verify-validate` available | Run it as final gate |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Load all artifacts** — read spec (requirements), design (decisions), tasks (completion), apply-progress (evidence).
3. **Run test suite** — execute `go test ./...` (or project test command). Capture output: pass count, fail count, duration.
4. **Verify each requirement** — for each REQ-N in the spec:
   - Find the corresponding test(s) from apply-progress or tasks.
   - If test exists and passes: mark REQ as PASS with evidence.
   - If test exists and fails: mark REQ as FAIL with error details.
   - If no test exists: mark REQ as SKIP with reason.
5. **Check design coherence** — verify architecture decisions from design.md are reflected in the code. Check interfaces match, file changes match.
6. **Run validation tool** — if `biggz sdd-verify-validate` is installed, run:
   ```
   biggz sdd-verify-validate --input <verify-report> --requirements N --scenarios N
   ```
7. **Write verify report** — create `openspec/changes/{change-name}/verify-report.md`:
   ```yaml
   ---
   phase: verify
   tests_run: N
   tests_passed: N
   tests_failed: N
   requirements_total: N
   requirements_passed: N
   requirements_failed: N
   requirements_skipped: N
   design_checks: pass | fail | warn
   validation_tool: pass | fail | not-run
   ---
   ## Test Results
   {summary of test run}

   ## Requirement Verification
   | REQ | Status | Evidence |
   |-----|--------|----------|
   | REQ-1 | PASS | TestCreateUser passes ✅ |
   | REQ-2 | PASS | TestAuthenticateUser passes ✅ |
   | REQ-3 | FAIL | TestAuthorizeAdmin fails — wrong permission check |

   ## Design Coherence
   - Decision AD-1 (middleware chain): implemented as designed ✅
   - Decision AD-2 (error wrapping): error wrapping not found ❌

   ## Recommendations
   - Fix REQ-3 authorization check
   - Add error wrapping per AD-2
   ```
8. **Persist** — write verify report and Engram entry. Update `_meta.yaml` with `phase: verify`.
9. **Recommend next phase** — if all pass: archive. If failures: fix and re-verify.

## Output Contract

```yaml
status: pass | fail | partial
executive_summary: "5/5 tests pass, 5/5 requirements verified. Validation tool: pass. Recommend archive."
artifacts:
  - path: openspec/changes/{change-name}/verify-report.md
    type: verify-report
    summary: "Pass/fail evidence per requirement + test results + design coherence"
next_recommended: archive | apply
risks:
  - description: "Verification gaps for REQ-5 — manual testing only"
    severity: low
skill_resolution: auto
```

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-verify.md`
- `openspec/changes/{change-name}/spec.md`
- `openspec/changes/{change-name}/design.md`
- `openspec/changes/{change-name}/apply-progress.md`
- `openspec/changes/{change-name}/verify-report.md`
