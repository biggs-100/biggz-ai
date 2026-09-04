# Apply Progress: Fix BigMem Status Bypass

**Change**: fix-bigmem-status-bypass
**Mode**: Standard (Strict TDD false; threat-matrix RED tests written and run per work unit)
**Strategy**: auto-chain, stacked-to-main, single PR (all 10 tasks, Phases 1-4)
**Ledger**: work-unit APPLY-bypass-single-pr, token tok-ee5d76d291e03215d120a8ee, max-lines 400

## Skill Resolution

- Read `internal/assets/biggz/biggz-orchestrator-workflow.md` (SDD workflow, dispatcher, gates, ledger, recall) — evidenced.
- Read `internal/assets/biggz/biggz-orchestrator-delegation.md` (routing ladder, delegation rules, edit authority, loss-less prompts) — evidenced.
- No chained-pr registry skill needed (single PR, no split).

## Completed Tasks

- [x] 1.1 `internal/bigmem/topic_prefix.go` — `TopicRow` + `ListByTopicPrefixCtx` (key-only `LIKE ?`, `deleted_at IS NULL`, project/scope `COLLATE NOCASE`, `ORDER BY topic_key`, no cap). Served by `idx_obs_topic_lookup`.
- [x] 1.2 `internal/bigmem/topic_prefix_test.go` — predicates, deleted excluded, personal excluded (case-insensitive), project match case-insensitive, `project=""` bypass, cancelled ctx.
- [x] 2.1 `collectBigMemChangesWithArchiveCtx` via `bigmem.Open` + sweep + `GetCtx` hydrate visible-only (pattern parse on keys first, content only for survivors).
- [x] 2.2 Deleted `openBigMemDB`/`queryBigMemRows`/`scanBigMemTopics`/`isPersonalScope`/`isProjectMismatch` + `database/sql`, `modernc.org/sqlite` imports; `rg "sql\.Open|db\.Query|modernc"` on `engram_status.go` empty.
- [x] 2.3 Absent-DB logged-warning fallback `(nil,nil,nil)`; query/open/resolve errors logged + wrapped `bigmem sdd-status <op>: %w`, never silent.
- [x] 3.1 `StatusCtx`/`StatusWithOptionsCtx`/`applyStoreRoutingCtx`/collect/read/derive `*Ctx` variants with `bigmem.WithTimeout` (5s, caller deadline wins); old funcs delegate via `Background()`. Engram-store collector errors now propagate instead of `return nil,nil,nil`.
- [x] 3.2 Both `IsSessionSummaryBlocked` sites (`deriveChangeStatusCtx`, `deriveChangeStatusWithForcedStoreCtx`) take caller ctx; `rg context.Background status.go` shows wrappers only.
- [x] 3.3 Cancelled ctx fast-fails: `StatusWithOptionsCtx` entry check + collector/Store `WithTimeout` chain; `TestStatusCtxCancel` + `TestCollectBigMemChanges_CancelledCtxFailsFast` assert `errors.Is(err, Canceled|DeadlineExceeded)`.
- [x] 4.1 `engram_status_test.go` extended: personal excluded, project override disables filter, 100-row/2-visible hydration parity (task counts + artifact states), corrupt-DB wrapped error, cancelled-ctx fast fail, `TestStatusCtxCancel`.
- [x] 4.2 E2E green + greps empty (see evidence table).

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/bigmem/topic_prefix.go` | Created (79 lines) | `TopicRow` + `ListByTopicPrefixCtx` key-only sweep |
| `internal/bigmem/topic_prefix_test.go` | Created (178 lines) | 6 predicate/bypass/cancel tests |
| `internal/sdd/engram_status.go` | Modified (+75/-100) | Store-backed collector, raw SQL deleted, visible errors |
| `internal/sdd/status.go` | Modified (+89/-26) | `*Ctx` overload chain, ctx threading, engram error propagation |
| `internal/sdd/engram_status_test.go` | Extended (+228) | 6 parity/integration tests incl. 100-row hydration |
| `openspec/changes/fix-bigmem-status-bypass/tasks.md` | Ticked | All 10 boxes `- [ ]` → `- [x]` |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/sdd/ -run 'TestCollectBigMem' -count=1 -v` — PASS (7/7 incl. 5 new); `go test ./internal/bigmem/ -run TestListByTopicPrefix -count=1` — PASS (6/6); `go test ./internal/sdd/ -run 'TestStatusCtxCancel' -count=1 -v` — PASS |
| Full e2e command and exact result | `go test ./internal/sdd/... ./internal/bigmem/... -count=1 -timeout 180s` — ok sdd 19.7s, ok bigmem 17.7s |
| Runtime harness command and exact result | `go run ./cmd/biggz sdd-status --json` — exit 0, renders active change `fix-bigmem-status-bypass` (schema biggz-ai.sdd-status/v2) through the new `StatusWithOptionsCtx` path |
| Grep acceptance checks | `rg "sql\.Open\|db\.Query\|modernc" internal/sdd/engram_status.go` — empty; `rg context.Background internal/sdd/status.go` — wrappers only; `engram_status.go` — wrapper only (line 363) |
| Rollback boundary | Revert `internal/sdd/engram_status.go` + `status.go` to restore raw-SQL collector; delete `internal/bigmem/topic_prefix*.go`; tests are additive-only. No migration, no Store schema change, no artifact format change |

## Deviations from Design

None — implementation matches design (key-only sweep + `GetCtx` hydration, `*Ctx` overloads with `Background()` delegation, `bigmem.WithTimeout` 5s, `project=""` override convention, `COLLATE NOCASE` personal exclusion). One intentional behavior fix inside scope: engram-store routing now propagates collector errors instead of `return nil,nil,nil` (required by spec Req5 "never silent").

## Issues Found

- Pre-existing workspace dirt (`openspec/changes/fix-bigmem-store-ctx/` deletions, `openspec/specs/bigmem/spec.md` modification) was left untouched; it is not part of this change.
- Review budget: changed lines total ~775 (prod ~369 incl. deletions, tests ~406) vs 400-line budget and 300–360 forecast. Overrun comes from required parity/integration tests (tasks 1.2, 4.1) and mechanical `*Ctx` wrapper chain (task 3.1). Production diff proper is +243/-126 with net +117. Recommend parent confirm `size:exception` or split per the suggested units (Unit 1 `topic_prefix*.go`, Unit 2 sdd files) before review.

## Remaining Tasks

None. 10/10 tasks complete. Ready for verify.

## Workload / PR Boundary

- Mode: single PR (per orchestrator directive; auto-chain stacked-to-main, no split requested)
- Current work unit: APPLY-bypass-single-pr (Phases 1-4, all tasks)
- Boundary: `topic_prefix.go` additive first → collector rewrite → `*Ctx` threading → parity/integration tests + e2e
- Estimated review budget impact: ~775 changed lines (add+del), exceeds 400 budget — `size:exception` confirmation recommended; clean split available at Unit 1/Unit 2 boundary if reviewer prefers

## Correction RDD-correction-hybrid-hydration (2026-09-04)

- Ledger: work-unit RDD-correction-hybrid-hydration, token tok-662e625f5e761ba90b87665c, request-id sdd2-fix-001, max-lines 200, review budget 80.
- R1-hydration-drop (`engram_status.go` hydration loop): `GetCtx` failure for a visible row now logs + returns `bigmem sdd-status hydrate <id>: %w` instead of log+continue partial success (Req5).
- R4-hybrid-error-swallow (`status.go` hybrid routing): BigMem collector errors now log + propagate as `bigmem sdd-status hybrid collect: %w` instead of silent success; absent-DB `(nil,nil,nil)` fallback with its explicit warning still falls through to filesystem (Req5).
- R3-none-silent (`engram_status.go` empty-store guard): logs `[sdd-status] artifact store none, skipping BigMem collection, falling back to filesystem-only` before `(nil,nil,nil)` (Req5).
- R1-fallback-stale (`status.go` engram empty-DB fallback): filesystem re-collect failure now logs `[sdd-status] bigmem engram fallback re-collect failed, returning degraded BigMem-derived status` (Req5).
- R3-explore-parity (`engram_status.go` mergeTopic): `explore` excluded from seen set alongside `state`, restoring legacy header intent (Req6 parity).
- Tests added (`engram_status_test.go` +81): `TestCollectBigMemChanges_HydrationErrorFails` (revision_count text-corruption forces List-ok/Get-fail, asserts wrapped `bigmem sdd-status`), `TestStatusWithOptions_HybridPropagatesBigMemError` (corrupt DB + fs change asserts hybrid propagates, no silent success), `TestCollectBigMemChanges_ExploreExcludedFromSeen` (explore/state-only invisible, proposal+explore visible).
- Evidence: `go build ./...` exit 0; `go test ./internal/sdd/ ./internal/bigmem/ -count=1` ok sdd 16.3s, ok bigmem 9.7s; new tests 3/3 PASS.
- Diff: 3 files, 89 insertions(+), 3 deletions(-); prod code 11 changed lines (within 80 review budget), total within 200 ledger max-lines. No commit.
