# Tasks: Fix BigMem Status Bypass

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 300–360 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (Unit 1 → Unit 2 stack-ready if review asks) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Key-only sweep `ListByTopicPrefixCtx` + tests | PR 1 | `go test ./internal/bigmem/ -run TestListByTopicPrefix -count=1` | N/A (library API, no CLI path yet) | Delete `internal/bigmem/topic_prefix*.go` |
| 2 | Store-backed collector + `*Ctx` threading + parity tests | PR 2 (base: PR 1) | `go test ./internal/sdd/ -run TestCollectBigMem -count=1` | `go run ./cmd/biggz sdd-status --json` on seeded store | Revert `internal/sdd/engram_status.go`, `status.go` only |

## Phase 1: Foundation — key-only sweep

- [x] 1.1 Create `internal/bigmem/topic_prefix.go` with `TopicRow` + `ListByTopicPrefixCtx` (key-only `LIKE ?`, `deleted_at IS NULL`, project/scope `COLLATE NOCASE`, no cap). Test: `go vet ./internal/bigmem/`
- [x] 1.2 Create `internal/bigmem/topic_prefix_test.go` (predicates, personal excluded, case-insensitive match, `project=""` bypass). Test: `go test ./internal/bigmem/ -run TestListByTopicPrefix -count=1`

## Phase 2: Core — Store-backed collector

- [x] 2.1 Add `collectBigMemChangesWithArchiveCtx` in `internal/sdd/engram_status.go` via `bigmem.Open` + sweep + `GetCtx` hydrate visible-only. Test: `go test ./internal/sdd/ -run TestCollectBigMemParity -count=1`
- [x] 2.2 Delete `openBigMemDB`/`queryBigMemRows`/`scanBigMemTopics` + `database/sql`, `modernc.org/sqlite` imports. Test: `rg -n "sql\.Open|db\.Query|modernc" internal/sdd/engram_status.go` empty
- [x] 2.3 Add absent-DB logged warning fallback + `fmt.Errorf("bigmem sdd-status <op>: %w")` on query errors, no `(nil,nil,nil)`. Test: `go test ./internal/sdd/ -run TestCollectBigMemCorruptDB -count=1`

## Phase 3: Integration — ctx threading

- [x] 3.1 Add `StatusCtx`/`StatusWithOptionsCtx`/routing/derive `*Ctx` variants in `internal/sdd/status.go` (`bigmem.WithTimeout` 5s). Test: `go build ./...`
- [x] 3.2 Replace `context.Background` at both `IsSessionSummaryBlocked` sites (~453/~723) with caller ctx; old funcs delegate via `Background()`. Test: `rg -n "context\.Background" internal/sdd/status.go` shows only wrappers
- [x] 3.3 Verify cancelled ctx fast-fails with `Canceled`/`DeadlineExceeded`. Test: `go test ./internal/sdd/ -run TestStatusCtxCancel -count=1`

## Phase 4: Verification — parity + e2e

- [x] 4.1 Extend `internal/sdd/engram_status_test.go`: personal excluded, project match/override, 100-row/2-visible hydrates 2. Test: `go test ./internal/sdd/ -run TestCollectBigMem -count=1 -v`
- [x] 4.2 Run e2e `go test ./internal/sdd/... ./internal/bigmem/... -count=1` + greps for `sql.Open`, `db.Query`, hot-spot `Background`. Test: all green, greps empty
