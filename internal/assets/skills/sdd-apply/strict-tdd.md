# Strict TDD Module — RED → GREEN → TRIANGULATE → REFACTOR

Load this module ONLY when `strict_tdd: true` is resolved. In Standard Mode,
this file is NEVER read, NEVER processed, NEVER consumes tokens.

## Philosophy

TDD is not testing. TDD is **software design driven by tests**.

**The Three Laws:**
1. **You must not write production code** before you have written a failing test.
2. **You must not write more of a test** than is sufficient to fail — and not compiling IS failing.
3. **You must not write more production code** than is sufficient to pass the currently failing test.

---

## TDD Implementation Cycle

### 0. SAFETY NET — Establish Baseline
Before touching any file that is being modified:
- Run existing tests on those files
- Note the baseline: `Tests: N passing, M failing`
- If baseline has pre-existing failures, report them as `[PRE-EXISTING]`
- A task that introduces NEW failures on top of pre-existing ones is STILL a failure

### 1. UNDERSTAND
Read the task, spec, design, and existing code. Plan what behavior you need to add or change.

### 2. RED — Write a Failing Test FIRST
- Write the test BEFORE any production code
- The test MUST reference production code that does NOT exist yet
- Run ONLY this test file — it MUST fail (compilation failure IS failure)
- If it passes without new code → the test is tautological → DELETE it
- If it fails for the wrong reason (e.g., unrelated error) → FIX the test
- **Evidence**: Commit or save the RED state

### 3. GREEN — Make It Pass
- Write the MINIMUM code to make the test pass
- "Fake It" (return a constant) IS valid for the first pass
- Execute the test file — it MUST pass
- Do NOT refactor yet. Do NOT optimize yet.
- **Evidence**: Commit or save the GREEN state

### 4. TRIANGULATE — Prove the Implementation is General
- Add a SECOND test case with DIFFERENT inputs/outputs
- If both tests pass with the same implementation → it's general
- If the second test requires additional code → repeat RED → GREEN
- Minimum 2 test cases per behavior. The only exception: purely structural tasks
   where the behavior is already fully characterized by the type system.
- **Evidence**: Test file with ≥2 cases for the new behavior

### 5. REFACTOR — Improve Without Changing Behavior
- Improve names, extract methods, remove duplication
- Run tests AFTER EVERY refactoring step
- If a test breaks → revert the refactoring, understand why, try again
- **Evidence**: Commit or save the REFACTOR state

### 6. Mark Task Complete
- Update TDD Cycle Evidence table
- Note any deviations and why

---

## TDD Cycle Evidence Table

Every task MUST produce a row in this table within the `apply-progress` artifact.

| Task | RED (test file) | GREEN (impl file) | TRIANGULATE (cases) | REFACTOR (changes) | Status |
|------|----------------|-------------------|--------------------|--------------------|--------|
| T1   | `user_test.go` | `user.go` | 3 cases | Extract validateEmail() | ✅ PASS |

**Rules:**
- RED column: path to test file that was written FIRST
- GREEN column: path to implementation file
- TRIANGULATE column: number of test cases per behavior
- REFACTOR column: what was improved
- Status: ✅ PASS, ⚠️ WARNING, ❌ FAILED

If a task row shows ❌ FAILED because tests were not written first → the
verify phase will reject the apply. No silent fallback.

---

## Choosing the Test Layer

Based on cached testing capabilities, choose accordingly:

| Context | Layer | Tool |
|---------|-------|------|
| Pure logic / utility | Unit | Fastest runner |
| Component rendering | Integration (or Unit with mocks) | Framework test utils |
| Multi-component flow | Integration (or Unit with mocks) | Integration test libs |
| Critical business flow | E2E (or Integration, or Unit) | E2E framework |

General rule: prefer the FASTEST layer that gives sufficient confidence.
If you need more than 7 mocks → the test is at the wrong level.

---

## Test Execution During the Cycle

- During RED/GREEN/REFACTOR: run ONLY the relevant test file (fast iteration)
- Full suite runs happen in `sdd-verify`, not here
- Detect test command from (in priority order):
  1. Cached testing capabilities (`biggz_mem_search`)
  2. `openspec/config.yaml` → `rules.apply.test_command`
  3. Fallback detection from project files

---

## Assertion Quality Rules (MANDATORY)

### Banned Patterns (CRITICAL if found)
1. **Tautologies**: `expect(true).toBe(true)`, `assert.Equal(1, 1)` — tests nothing
2. **Empty collection alone**: `expect(arr).toHaveLength(0)` without setup context
3. **Type-only assertions alone**: `expect(obj).toBeInstanceOf(Type)` without behavior check
4. **Ghost loops**: a `for/describe` loop that generates tests but iterates zero times
5. **Incomplete TDD cycle**: test written AFTER production code (not TDD — flag as ❌)

### Mock Hygiene (WARNING)
- If more mocks than assertions → test is at the wrong level
- 7+ mocks → STOP, restructure the code, don't add more mocks
- Extract-before-mock: extract the logic to a pure function, then test it directly
- Never mock what you don't own (external APIs, databases — use adapters instead)

### Implementation Detail Coupling (CRITICAL)
- CSS class assertions are NEVER valid. Test semantic outcomes.
- Testing private methods directly is a smell — test through the public API
- Testing internal state after an event is a smell — test the observable effect

### Smoke Test Rule
- "Renders without crash" is NOT a valid test on its own
- It must be accompanied by at least one behavioral assertion

### Approval Testing (for refactoring existing code)
- Before touching production code: write approval tests that capture current behavior
- Run approval test → it passes (captures baseline)
- Refactor production code
- Run approval test → it passes (behavior preserved)
- If approval test fails → behavior changed unexpectedly → revert

---

## Violation Handling

| Violation | Consequence |
|-----------|------------|
| No RED phase evidence | ❌ FAILED — task not in TDD |
| Test written after code | ❌ FAILED — not TDD |
| Tautological test | ❌ FAILED — test must fail without new code |
| Mock count > assertion count | ⚠️ WARNING — wrong test level |
| CSS class assertion | ❌ FAILED — tests implementation detail |
| Smoke test without behavior | ❌ FAILED — insufficient coverage |
