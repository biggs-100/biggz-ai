# Apply Progress — fix-bigmem-store-ctx · PR1 (nucleo WithTimeout + 5 core Ctx)

**Change**: fix-bigmem-store-ctx
**Work unit**: PR1-nucleo-WithTimeout-5core (request-id pr1-core-001, token tok-6991867fbab9cffdb34bc9c8, outcome=progress)
**Mode**: Standard (strict_tdd=false)
**Delivery**: auto-chain · stacked-to-main · PR1 slice (~147 changed lines, under 200-line budget)
**Date (UTC)**: 2026-09-04

## Scope (PR1 ONLY — Phase 1, tasks 1.1–1.3)

- [x] 1.1 Add `WithTimeout` helper in `internal/bigmem/bigmem.go` (5s default, caller deadline wins) (CTX-3)
- [x] 1.2 Add `SaveCtx/GetCtx/SearchCtx/UpdateCtx/DeleteCtx` in `internal/bigmem/bigmem.go` via `WithTimeout` (CTX-1)
- [x] 1.3 Wire `QueryContext/ExecContext/QueryRowContext/BeginTx` + wrapped `ctx.Err()` in 5 core methods, no plain Query/Exec (CTX-4)
- [ ] 2.1–2.3 (PR2: full.go + 8-wrapper parity) — NOT touched
- [ ] 3.1–3.3 (PR3: consumers) — NOT touched
- [ ] 4.1–4.3 (tests) — NOT touched (throwaway verification only, removed)

## What was done

- Added `context` import, `defaultBigmemTimeout = 5s`, and `WithTimeout(ctx)`:
  caller deadline wins (`WithCancel` child, default never extends); otherwise 5s default; nil ctx → Background.
- `SaveCtx` holds Save logic: `BeginTx(ctx)`, all `tx.*` → `*Context` variants,
  checkpoint/busy_timeout PRAGMAs → `ExecContext`, pre/commit `ctx.Err()` checks with
  `fmt.Errorf("bigmem save…: %w", ctx.Err())`. `Save` is now a `Background()` wrapper.
- `GetCtx` (`QueryRowContext`), `SearchCtx` (all 5 `Query` → `QueryContext`,
  checkpoint → `ExecContext`), `UpdateCtx` (`GetCtx`+`SaveCtx`, never legacy per D3),
  `DeleteCtx` (`ExecContext`) — each with `WithTimeout` + visible `ctx.Err()` errors.
  Legacy `Get/Search/Update/Delete` are thin `Background()` wrappers (5 of 8; rest in PR2).
- Zero plain `Query/Exec/QueryRow/Begin` remain in the SaveCtx (L1442–1700) and
  Get/Search/Update/Delete (L1699–2090) ranges. Out-of-scope plain calls
  (rescue/sync/relations/doctor paths) intentionally untouched for PR2/PR3.

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/bigmem/ -run 'TestSaveCtx\|TestSearchCtx' -count=1` → ok (no tests to run — Phase 4 tests are PR3 scope) |
| Throwaway wiring proof (removed after run) | `go test ./internal/bigmem/ -run 'TestZZPR1CtxWiring' -count=1 -v` → PASS (cancelled ctx → `context.Canceled` on all 5 Ctx; Save/Get/Search parity; WithTimeout default vs override) |
| Runtime harness command and exact result | `go test ./internal/bigmem/ -count=1 -timeout 180s` → ok (7.1s); `go build ./internal/bigmem/` → exit 0; `go vet ./internal/bigmem/` → clean |
| Static wiring proof | `rg QueryContext\|BeginTx` hits all 5 Ctx; `rg 'func (s *Store) (Save\|Get\|Search\|Update\|Delete)\('` shows 5 legacy wrappers intact (signatures unchanged) |
| Rollback boundary | Revert `internal/bigmem/bigmem.go` only (single file, 128+/19-, no schema/data change) |

## Deviations from Design

- Partial wrapper conversion (5 core now, 3 extended in PR2 task 2.2) — required to stay
  under the 200-line PR1 budget and keep the slice autonomous; full 8-wrapper parity lands in PR2.
- `UpdateCtx` calls `GetCtx`+`SaveCtx` each applying `WithTimeout` (nested timeouts collapse
  to the earliest deadline) instead of a single shared timeout scope — matches D3 and CTX-3
  override semantics; no behavior change for callers.

## Issues Found

None. `full.go`, consumers, and permanent tests untouched per instructions.

## Workload / PR Boundary

- Mode: chained PR slice (auto-chain, stacked-to-main)
- Current work unit: PR1-nucleo-WithTimeout-5core
- Boundary: starts at `WithTimeout` helper, ends at 5 core Ctx + 5 wrappers in `bigmem.go`
- Review budget impact: 147 changed lines (128 insertions, 19 deletions) — within max-lines 200
- Next: PR2 (full.go extended Ctx + 8-wrapper parity + plain-call sweep), then PR3 (consumers + tests)

## Status

3/11 tasks complete (Phase 1 done). Ready for next batch (PR2). Do NOT verify/archive yet.

---

# Apply Progress — fix-bigmem-store-ctx · PR2 (extended Ctx + 8-wrapper parity)

**Change**: fix-bigmem-store-ctx
**Work unit**: PR2-extended-wrappers-full (request-id pr2-ext-005, token tok-e7a369cbd59a7937f4f8075f, outcome=progress)
**Mode**: Standard (strict_tdd=false)
**Delivery**: auto-chain · stacked-to-main · PR2 slice (~113 changed lines in `full.go`, under 200-line budget)
**Date (UTC)**: 2026-09-04

## Scope (PR2 ONLY — Phase 2, tasks 2.1–2.3)

- [x] 2.1 Add `SessionContextCtx/TimelineCtx/SavePromptCtx` in `internal/bigmem/full.go` with same pattern (CTX-2)
- [x] 2.2 Convert 8 legacy methods to `Background()` wrappers delegating to Ctx twins (CTX-4)
- [x] 2.3 Verify no plain `Query/Exec/QueryRow/Begin` on Store paths inside the 8 `*Ctx` bodies (CTX-4)
- [ ] 3.1–3.3 (PR3: consumers) — NOT touched
- [ ] 4.1–4.3 (PR3: permanent tests) — NOT touched (throwaway verification only, removed)

PR1 section above preserved as-is (WithTimeout + 5 core Ctx in `bigmem.go`, re-verified present via rg before starting; NOT reimplemented).

## What was done

- Added `context` import to `internal/bigmem/full.go` (only new import).
- `SessionContextCtx`: `WithTimeout` + pre-check `ctx.Err()` → `ExecContext` DDL (legacy-ignored errors preserved for non-ctx failures) → `QueryContext` + wrapped `ctx.Err()` mapping + `rows.Err()` check. `SessionContext` is now a `Background()` wrapper.
- `SavePromptCtx`: `WithTimeout` + pre-check → `mu.Lock` (same scope as legacy) → `ExecContext` DDL + `ExecContext` INSERT with `ctx.Err()` mapping; keeps stripPrivateTags/truncate and `return p, err` parity on non-ctx INSERT errors. `SavePrompt` is now a wrapper.
- `TimelineCtx`: `WithTimeout` + pre-check → all 4 driver calls (`QueryRowContext` focus lookup, `QueryContext` before/focus/after + plain-path) with `ctx.Err()` mapping; legacy `if err == nil` skip-blocks preserved for non-ctx errors so happy-path results match; added `rows.Err()` guards that surface only ctx cancellation plus a post-focus-branch `ctx.Err()` check. `Timeline` is now a wrapper.
- No `bigmem.go` logic change (5 PR1 core Ctx + 5 wrappers intact); no consumer or permanent-test changes.

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/bigmem/ -run 'TestZZPR2CtxWiring' -count=1 -v` → PASS (cancelled ctx → `context.Canceled` on all 3 extended Ctx; SavePrompt/SessionContext/Timeline parity; file removed after run) |
| Runtime harness command and exact result | `go test ./internal/bigmem/ -count=1 -timeout 180s` → ok (7.7s); `go build ./internal/bigmem/` → exit 0; `go vet ./internal/bigmem/` → clean |
| Static wiring proof | `rg SessionContextCtx\|TimelineCtx\|SavePromptCtx` → 3 Ctx + 3 wrappers in `full.go`; `rg 'func (s *Store) (Save\|Get\|Search\|Update\|Delete\|SessionContext\|Timeline\|SavePrompt)\('` → all 8 legacy wrappers; `rg 's\.db\.(Query\|Exec\|QueryRow\|Begin)\('` → zero hits inside all 8 Ctx spans (remaining hits are out-of-scope methods: branches/sessions-DDL/doctor/relations/sync) |
| Rollback boundary | Revert `internal/bigmem/full.go` only (single file, 101+/12-, no schema/data change; `bigmem.go` PR1 hunk untouched) |

## Deviations from Design

- None — implementation matches design (D1–D4). `UpdateCtx` nested-timeout note from PR1 still applies, unchanged.
- `TimelineCtx` adds `rows.Err()`/post-branch `ctx.Err()` guards that surface ONLY ctx cancellation (non-ctx iteration behavior identical to legacy skip/continue pattern) — strict superset for CTX-4 driver-cancellation visibility, happy-path parity verified by throwaway test.

## Issues Found

None. Consumers and permanent tests untouched per instructions.

## Workload / PR Boundary

- Mode: chained PR slice (auto-chain, stacked-to-main)
- Current work unit: PR2-extended-wrappers-full
- Boundary: starts at `context` import in `full.go`, ends at 3 extended Ctx + 3 wrappers + plain-call sweep
- Review budget impact: 113 changed lines in `full.go` (101 insertions, 12 deletions) — within max-lines 200; `bigmem.go` 147-line PR1 hunk carried in working tree, not re-counted
- Next: PR3 (3-consumer migration + permanent tests + full verification)

## Status

6/11 tasks complete (Phases 1–2 done). Ready for next batch (PR3). Do NOT verify/archive yet.

---

# Apply Progress — fix-bigmem-store-ctx · PR3 (consumers + permanent tests, FINAL SLICE)

**Change**: fix-bigmem-store-ctx
**Work unit**: PR3-consumers-tests (request-id pr3-cons-001, token tok-817884a326b059af88b56f31, outcome=progress→verify)
**Mode**: Standard (strict_tdd=false)
**Delivery**: auto-chain · stacked-to-main · PR3 FINAL slice (~270 changed lines, under 300-line budget)
**Date (UTC)**: 2026-09-04
**Skills**: internal/assets/biggz/biggz-orchestrator-workflow.md + biggz-orchestrator-delegation.md (both read before work)

## Scope (PR3 ONLY — Phase 3 tasks 3.1–3.3 + Phase 4 tasks 4.1–4.3)

- [x] 3.1 Migrated `internal/sdd/session_guard.go` to `SearchCtx/SessionContextCtx/SaveCtx` with inbound ctx, kept `select ctx.Done` pre-check (CTX-5)
- [x] 3.2 Migrated `cmd/biggz-mcp/main.go` handlers to `*Ctx` with `Background()` phase 1 per D5 (CTX-5)
- [x] 3.3 Migrated `internal/doctor/bigmem.go` Remedy to `SearchCtx` probe, kept PRAGMA wiring + pre-check (CTX-5)
- [x] 4.1 Added unit table tests: cancelled ctx errors for all 8, parity, default vs override (CTX-1..CTX-4)
- [x] 4.2 Added integration tests on temp DB: `SearchCtx` under WAL contention, round-trip, pre-check fast-fail (CTX-4/5)
- [x] 4.3 Ran e2e: `rg '*Ctx'` per consumer file, `go build ./...`, `go test ./internal/bigmem/` green (CTX-5)

PR1 section (WithTimeout + 5 core Ctx in `bigmem.go`) and PR2 section (3 extended Ctx + wrappers in `full.go`) above preserved as-is; both re-verified present via rg before starting (WithTimeout, 8 *Ctx) and NOT reimplemented.

## What was done

- `session_guard.go` (6 lines): `SessionContext(5)`→`SessionContextCtx(ctx, 5)`, `Search("", opts)`→`SearchCtx(ctx, "", opts)` in `HasSessionSummary` (inbound ctx, pre-check untouched); `tryMCPSave` takes `ctx` and uses `SaveCtx(ctx, …)` for both observation writes.
- `main.go` (14 lines + `context` import): all 13 legacy call sites → `*Ctx` twins with `context.Background()` (Save×2→SaveCtx, Search→SearchCtx, Get×4→GetCtx, Update→UpdateCtx, Delete→DeleteCtx, SessionContext→SessionContextCtx, SavePrompt→SavePromptCtx, Timeline→TimelineCtx). Phase 1 per D5 — no request ctx exists in the stdio loop; timeout still enforced via WithTimeout.
- `doctor/bigmem.go` (3 lines): Remedy Action adds `SearchCtx(ctx, "", {Limit:1})` probe on the opened store before `DoctorFix` (error ignored, repair proceeds); `Run` PRAGMA wiring + Remedy pre-check untouched.
- `internal/bigmem/ctx_test.go` (new, 259 lines): `TestCtxCancelledTable` (8/8 cancelled→`context.Canceled` via `errors.Is`), `TestCtxParity` (Save/Get/Search/Update/SessionContext/Timeline/SavePrompt Ctx-vs-legacy + DeleteCtx), `TestWithTimeoutDefaultVsOverride` (5s default, caller 1s deadline wins unextended), `TestSearchCtxWALContention` (8 writers×5 + 4 readers×5 concurrent, 40 unique obs round-trip), `TestCtxCancelledFastFail` (cancelled ctx on closed store → `context.Canceled`, proves pre-check short-circuits before SQLite).

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/bigmem/ -run 'TestCtx\|TestWithTimeout' -count=1 -v` → PASS (all 8 cancelled subtests + parity + timeout + fast-fail); `TestSearchCtxWALContention` → PASS after dedupe fix |
| Runtime harness command and exact result | `go test ./internal/bigmem/ -count=1 -timeout 180s` → ok (16.6s, only failure during dev was the new contention test, fixed); `go build ./...` → exit 0; `go vet` on 4 touched pkgs → clean |
| Static wiring proof (CTX-5 grep) | `*Ctx` hits per file: main.go 13, session_guard.go 4, doctor 1; zero legacy `store.(Save\|Get\|Search\|Update\|Delete\|SessionContext\|Timeline\|SavePrompt)(` remain in the 3 consumer files |
| Rollback boundary | Revert `internal/sdd/session_guard.go` + `cmd/biggz-mcp/main.go` + `internal/doctor/bigmem.go` + delete `internal/bigmem/ctx_test.go`; `bigmem.go`/`full.go` PR1+PR2 hunks untouched |

## Deviations from Design

- None — implementation matches D5 exactly (guard inbound ctx + pre-check; main Background phase 1; doctor SearchCtx probe, no invented DoctorCtx).

## Issues Found

- Contention test first run: 40 identical Title/Content observations deduped to 1 by the 15m dedupe window (expected engine behavior, not a ctx bug). Fixed by uniquifying Title/Content per writer op; final count asserts exactly 40.

## Workload / PR Boundary

- Mode: chained PR slice (auto-chain, stacked-to-main) — FINAL slice of PR1→PR2→PR3
- Current work unit: PR3-consumers-tests
- Boundary: starts at consumer imports, ends at permanent tests + full green suite
- Review budget impact: ~270 changed lines (consumers ~21 + ctx_test.go ~250) — within max-lines 300; PR1 (147) + PR2 (113) hunks carried in working tree, not re-counted
- Next: verify (all 11/11 tasks complete)

## Status

11/11 tasks complete (Phases 1–4 done). Ready for verify. No commit per instructions.

---

# Apply Progress — fix-bigmem-store-ctx · RDD correction (CTX-1/CTX-4/CTX-5 gaps, FINAL)

**Change**: fix-bigmem-store-ctx
**Work unit**: RDD-correction-ctx-gaps (request-id rdd-fix-001, token tok-b2d73b8c101249cafa8d1464)
**Mode**: Standard (strict_tdd=false)
**Delivery**: auto-chain · stacked-to-main · correction slice (76 insertions, 4 deletions — within 200-line ledger + review budgets)
**Date (UTC)**: 2026-09-04
**Skills**: internal/assets/biggz/biggz-orchestrator-workflow.md + biggz-orchestrator-delegation.md (both read before work)

## Scope (RDD correction ONLY — 5 deterministic CRITICAL findings, no new features)

- [x] R1 SearchCtx: rows.Err() + ctx.Err() check after FTS loop; QueryContext error mapping on LIKE fallback + topic-key phase-1 (CTX-1/CTX-4)
- [x] R2 SaveCtx phases 1/2: tx Exec/Commit errors map to wrapped ctx.Err() when ctx done, matching phase-3 pattern (CTX-4)
- [x] R3 TimelineCtx focus path: non-ctx driver errors propagate (return err) when ctx live; ctx mapping kept when cancelled (CTX-1/CTX-4)
- [x] R4 UpdateCtx atomicity: existence re-check under write lock before SaveCtx (budget-bounded minimum per task; no locking-helper calls under lock, no deadlock) (CTX-1)
- [x] R5 tryMCPSave: ctx.Err() checks before/after legacy EnsureImplicitSession/SessionEnd with wrapped ctx error (CTX-5; scope kept, no new twins)

PR1/PR2/PR3 sections above preserved as-is; post-squash HEAD db70332e re-verified present via grep before starting (WithTimeout, 8 *Ctx) and NOT reimplemented.

## What was done

- `internal/bigmem/bigmem.go` (+45): topic-key `QueryContext` error now returns `search: %w` (or wrapped `ctx.Err()` when cancelled) instead of silent skip; `rows.Err()` after FTS loop returns wrapped ctx error or driver error instead of silent partial; LIKE fallback `QueryContext` error mapped the same way; phase-1/2 `ExecContext`/`Commit` errors map to `bigmem save exec/commit: %w` when ctx done; `UpdateCtx` re-checks `SELECT id` under `s.mu.Lock` (direct `QueryRowContext`, never `GetCtx`, so no deadlock) and returns `not found` instead of resurrecting a concurrently deleted row. Redundant `if err == nil` guards kept intentionally after early error returns to hold the diff to insertions only.
- `internal/bigmem/full.go` (+20/-4): focus-path `beforeRows`/`afterRows` `QueryContext` errors now `return nil, err` when ctx live (previously swallowed); `beforeRows.Err()`/`afterRows.Err()` rewritten to propagate non-ctx iteration errors and keep wrapped `ctx.Err()` mapping when cancelled.
- `internal/sdd/session_guard.go` (+15): `tryMCPSave` checks `ctx.Err()` before `EnsureImplicitSession`, between the two legacy calls, and after `SessionEnd`, returning `session guard save: %w` when cancelled; legacy errors still returned verbatim when ctx live (scope kept, no new twins).
- `ctx_test.go` untouched: existing cancelled-ctx/parity/fast-fail assertions hold (new branches trigger only on driver errors with live ctx, which tests do not inject).

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/bigmem/ ./internal/sdd/ ./internal/doctor/ -count=1` → ok (bigmem 9.3s, sdd 14.1s, doctor 1.6s) |
| Runtime harness command and exact result | `go build ./...` → exit 0; `go vet ./internal/bigmem/ ./internal/sdd/` → clean; `gofmt -l` on 3 touched files → empty |
| Rollback boundary | Revert `internal/bigmem/bigmem.go` + `internal/bigmem/full.go` + `internal/sdd/session_guard.go` (3 files, 80 changed lines, no schema/data/test change) |

## Deviations from Design

- UpdateCtx uses the budget-bounded minimum (existence re-check under write lock, tiny unlock→SaveCtx window remains) instead of the full single-lock re-read+save refactor, exactly as the task permits when the refactor exceeds budget; the check itself holds the write lock so no concurrent Delete interleaves during the guard, and the window is documented here.
- `if err == nil` wrappers kept after new early error returns (redundant-but-harmless) to keep the correction insertions-only and reviewable; behavior is identical to unindented code.

## Issues Found

None. Out-of-scope items untouched per instructions (MCP Background pattern, WAL checkpoint placement, DeleteObservation legacy API).

## Workload / PR Boundary

- Mode: chained PR slice (auto-chain, stacked-to-main) — correction on top of PR1→PR2→PR3 squash db70332e
- Current work unit: RDD-correction-ctx-gaps
- Boundary: starts at SearchCtx topic-key query, ends at tryMCPSave ctx guards; no consumer/test/schema changes
- Review budget impact: 80 changed lines (76 insertions, 4 deletions) — within max-lines 200 and review budget 200
- Next: verify (11/11 tasks + 5/5 corrections complete)

## Status

11/11 tasks + 5/5 RDD corrections complete. Ready for verify. No commit per instructions.
