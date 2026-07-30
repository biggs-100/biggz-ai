# Strict TDD Verify Module — TDD Compliance + Assertion Audit

Load this module ONLY when Strict TDD Mode is active. In Standard Mode,
this file is NEVER read, NEVER processed, NEVER consumes tokens.

## Philosophy

When Strict TDD Mode is active, verification goes beyond "does the code work?"
to "was the code built correctly?" We verify both the OUTCOME and the PROCESS.

---

## 1. TDD Compliance Check

Read the `apply-progress` artifact. For EACH task, validate:

### RED Phase
- [ ] RED column contains a test file path
- [ ] That test file EXISTS in the codebase (CRITICAL if missing)
- [ ] The test references production code (not a tautology)

### GREEN Phase
- [ ] GREEN column contains an implementation file path
- [ ] The test file PASSES when executed (CRITICAL if fails)
- [ ] Run: `{test_command} {test_file}` — must exit 0

### TRIANGULATE Phase
- [ ] TRIANGULATE column reports ≥2 test cases per behavior
- [ ] Verify by inspecting test file for at least 2 distinct cases
- [ ] ⚠️ WARNING if only 1 case exists (exception: purely structural tasks)

### SAFETY NET Phase
- [ ] If any modified file was covered by existing tests
- [ ] Safety net must not be "N/A" when test files exist for modified paths
- [ ] ⚠️ WARNING if safety net is missing for modified paths with test coverage

### Overall
- [ ] TDD Cycle Evidence table EXISTS in apply-progress
- [ ] No task has ❌ FAILED status
- ⚠️ If no TDD Cycle Evidence table found → **CRITICAL** — apply did not follow TDD

---

## 2. Test Layer Validation

Classify ALL test files in the diff by testing layer:

| Layer | Files | Count |
|-------|-------|-------|
| Unit | `user_test.go` | 3 |
| Integration | `api_test.go` | 1 |
| E2E | `e2e_test.go` | 0 |

**Cross-reference with capabilities:**
- If capabilities declare integration tools but all tests are unit → ⚠️ WARNING
- If capabilities declare E2E tools but no E2E tests for critical flows → ⚠️ WARNING

---

## 3. Coverage Check

When a coverage tool is available in cached capabilities:

```
{coverage_command} {changed_files}
```

**Thresholds (changed files only):**
| Coverage | Rating |
|----------|--------|
| ≥ 95% | ✅ Excellent |
| ≥ 80% | ✅ Acceptable |
| < 80% | ⚠️ Low — review untested paths |
| < 50% | ❌ CRITICAL — major gaps |

Include the coverage percentage per changed file in the verify report.

---

## 4. Assertion Quality Audit (MANDATORY)

Scan ALL test files for banned patterns:

### CRITICAL Findings
| Pattern | How to Detect |
|---------|---------------|
| Tautology | `assert.Equal(1, 1)`, `expect(true).toBe(true)` |
| No production code call | Test that doesn't call any production function |
| Ghost loop | Loop that generates tests but iterates zero times |
| CSS class assertion | `expect(el.className).toBe(...)` |

### WARNING Findings
| Pattern | How to Detect |
|---------|---------------|
| Empty collection without companion | `expect(arr).toHaveLength(0)` with no setup |
| Type-only assertion | `expect(obj).toBeInstanceOf(Type)` as only assertion |
| Smoke-test only | "Renders without crash" as only test |
| Mock-heavy | More mocks than assertions |
| No triangulation | Only 1 test case per behavior |

### Evidence Table

| Finding | Location | Severity | Status |
|---------|----------|----------|--------|
| Tautology | `user_test.go:42` | CRITICAL | ❌ |
| Smoke-only | `page_test.go:15` | WARNING | ⚠️ |

---

## 5. Quality Metrics

Run quality tools on changed files only:

- **Linter**: `{linter_command} {changed_files}` — ⚠️ WARNING if warnings
- **Type checker**: `{type_checker_command}` — ❌ CRITICAL if errors

---

## 6. Full Suite Execution

Run the complete test suite:

```
{test_command}
```

- All tests MUST pass (exit 0)
- Pre-existing failures are reported but do NOT block the verify
- New failures (tests that passed in baseline but fail now) are ❌ CRITICAL

---

## 7. Verify Report TDD Section

Include this section in the verify report YAML envelope and body:

```yaml
tdd_compliance: pass|warnings|fail
tdd_assertion_quality: pass|warnings|fail
tdd_coverage: "85%"
tdd_tasks_compliant: 5/5
```

Body section:
```
## TDD Compliance
- Tasks compliant: 5/5
- Assertion quality: pass
- Coverage: 85% (acceptable)

## TDD Issues
- None
```
