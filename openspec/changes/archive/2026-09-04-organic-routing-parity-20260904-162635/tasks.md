# Tasks: Organic Routing Parity

## Task 1: Rewrite Work Routing Ladder

**File:** `internal/assets/biggz/biggz-orchestrator-delegation.md`

**What to do:**
- Replace current `Work Routing Ladder` section (Inline Direct / Simple Delegation / SDD) with the 3-route table from design.md.
- Add explicit rules: `SIZE NEVER SELECTS SDD`, `Per-action delegation does not change route`, `Direct/delegated create zero SDD artifacts`.
- Add route selection thresholds: 1-3 files → direct, 4+ → delegated, ambiguous + request → SDD.

**Test evidence:**
- [ ] `cat internal/assets/biggz/biggz-orchestrator-delegation.md | grep -c "Direct inline"` returns ≥1
- [ ] `cat internal/assets/biggz/biggz-orchestrator-delegation.md | grep -c "SIZE NEVER"` returns ≥1
- [ ] `cat internal/assets/biggz/biggz-orchestrator-delegation.md | grep -c "zero SDD artifacts"` returns ≥1

**Work unit:** 1 file, ~30 lines changed. Prompt-only, no Go.

---

## Task 2: Add Public States to Workflow

**File:** `internal/assets/biggz/biggz-orchestrator-workflow.md`

**What to do:**
- Add `## Public Implementation States` section with Working/Checking/Ready/Needs your decision table.
- Replace synthesis lifecycle `◆ phase·status·next` references with state strings where appropriate.
- Keep synthesis markers (`## Sub-agent Result`, `**What was done:**`, etc.) — they are record-keeping, not states.

**Test evidence:**
- [ ] `cat internal/assets/biggz/biggz-orchestrator-workflow.md | grep -c "Working"` returns ≥1
- [ ] `cat internal/assets/biggz/biggz-orchestrator-workflow.md | grep -c "Ready"` returns ≥1
- [ ] `cat internal/assets/biggz/biggz-orchestrator-workflow.md | grep -c "Needs your decision"` returns ≥1

**Work unit:** 1 file, ~25 lines added. Prompt-only, no Go.

---

## Task 3: Add Route Field to ChangeStatus

**File:** `internal/sdd/status.go`

**What to do:**
- Add `Route string` field to `ChangeStatus` struct (after `BlockedReasons`, before `PhaseInstructions`).
- Add `deriveRoute(cs *ChangeStatus) string` function that returns `"sdd"` if any SDD artifact exists, `"organic"` otherwise, `""` if archived.
- Wire `cs.Route = deriveRoute(cs)` in `deriveChangeStatusWithForcedStoreCtx` (after line ~537, after `cs.BlockedReasons`).

**Test evidence:**
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `./biggz.exe sdd-status --json` for active SDD change shows `"route":"sdd"`
- [ ] `./biggz.exe sdd-status --json` for organic work (no SDD artifacts) shows `"route":"organic"`

**Work unit:** 1 file, ~25 lines added (1 struct field + 1 function + 1 wiring call).

---

## Task 4: Add Route Context to sdd-continue

**File:** `cmd/biggz/cli_sdd.go`

**What to do:**
- After `sdd-continue` prints `NextRecommended`, also print `Route: <route>` using `deriveRoute` from status.go.
- For organic work, print `No SDD next — work completed via direct/delegated route.`
- For SDD work, print `Next: <phase>`.

**Test evidence:**
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `./biggz.exe sdd-continue <change>` for SDD change shows `Route: sdd`
- [ ] `./biggz.exe sdd-continue <change>` for organic work shows `Route: organic`

**Work unit:** 1 file, ~15 lines added.

---

## Task 5: Build and Smoke Test

**What to do:**
- `go build ./...` — full build
- `go vet ./...` — static analysis
- Manual smoke: `./biggz.exe sdd-status --json` for active change
- Manual smoke: `./biggz.exe sdd-status --json` with no active changes

**Test evidence:**
- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `sdd-status --json` output includes `"route"` field
- [ ] No regressions in existing SDD status output

**Work unit:** Verification only, no code changes.

---

## Task 6: Update Orchestrator Test (if exists)

**File:** `internal/assets/biggz/orchestrator_test.go`

**What to do:**
- If test checks template invariants (markers, aliases), add Route-related assertions.
- If no test exists for routing, skip.

**Test evidence:**
- [ ] `go test ./internal/assets/biggz/...` passes (if test exists)

**Work unit:** Conditional, 0-10 lines.

---

## Execution Order

1. Task 1 (delegation.md) — prompt-only, no risk
2. Task 2 (workflow.md) — prompt-only, no risk
3. Task 3 (status.go) — Go change, additive field
4. Task 4 (cli_sdd.go) — Go change, additive output
5. Task 5 (build + smoke) — verification
6. Task 6 (test update) — conditional

Tasks 1-2 can run in parallel. Tasks 3-4 can run in parallel after 1-2. Task 5 gates on 3-4. Task 6 is optional.

## Total Scope

- **Files changed:** 4-5 (2 .md + 2 .go + optional test)
- **Lines added:** ~95 (30 + 25 + 25 + 15 + optional)
- **Lines removed:** ~15 (old routing ladder text)
- **Net:** +80 lines, predominantly prompt guidance
- **Risk:** Low (additive fields, prompt-only routing, no FSM changes)
- **Revert cost:** trivial (git revert 4 commits)
