# Tasks: sdd-fast-lane

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~220–310 (code ≈40–50, tests ≈180–260) |
| 400-line budget risk | Low |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | alias in both resolvers + lane/parity/CLI tests | PR 1 | `go test ./internal/sdd/ -run 'Lane\|Parity' -count=1` | `biggz sdd-status --json --cwd <plan-only fixture>` | revert alias hunks; no migration |
| 2 | text surfaces + `sdd-ff.md` lane mode | PR 1 | `go test ./internal/assets/ ./cmd/biggz/ -count=1` | rebuilt CLI `biggz sdd-status --json` on plan-only fixture | revert prompt/doc hunks; alias unaffected |

## Phase 1: RED — Lane Core

- [x] 1.1 RED: plan-only fixture via real dispatcher → apply→all_done→verify→archive ready (REQ-ALIAS, `internal/sdd/status_lane_test.go`, `seedDeriveChange`, ~60)
- [x] 1.2 RED: per-slot precedence + in-place graduation: real `design.md`/`tasks.md` win, others alias plan (REQ-ALIAS, same file, ~30)
- [x] 1.3 RED: plan checklist drives progress (`status.go:638,:1106`) (REQ-ALIAS, same file, ~25)
- [x] 1.4 RED: canonical `Requirement`/`Scenario` headings → verify admission totals match plan counts (REQ-ALIAS, `verify.go:188,197,201`, ~30)
- [x] 1.5 Verify RED fails: `go test ./internal/sdd/ -run Lane -count=1`

## Phase 2: RED — Parity, Legacy, Dogfood

- [x] 2.1 RED: BigMem/FS parity — same plan-only via `seedBigMemChange` → identical artifact set + apply ready (REQ-ALIAS, same file, ~40)
- [x] 2.2 RED: legacy four-artifact change + terminal gates intact (same blocked reasons) (REQ-ALIAS, same file, ~20)
- [x] 2.3 RED: CLI dogfood — `runSDDStatusCLIArgs` over plan-only fixture → apply ready JSON (REQ-ALIAS, `cmd/biggz/sdd_status_cli_test.go`, ~35)
- [x] 2.4 Verify RED: `go test ./internal/sdd/ ./cmd/biggz/ -count=1` (legacy green)

## Phase 3: GREEN — Alias (Both Resolvers)

- [x] 3.1 GREEN: alias in `resolveArtifactPaths`: real first, `plan.md` fallback; Specs → `[plan]`; apply/verify never alias (REQ-ALIAS, `internal/sdd/status.go:994`, ~12)
- [x] 3.2 GREEN: extend `spec↔specs` alias with `plan` fallback in `bigmemArtifactState`; `bigmemArtifactPaths` adds `plan` topic (REQ-ALIAS, `internal/sdd/engram_status.go:146,:168`, ~8)
- [x] 3.3 GREEN: `bigmemSlotContent(m, suffix)` (real → `plan` → "") at `:221`,`:258`,`:312` (REQ-ALIAS, same file, ~7)
- [x] 3.4 Verify GREEN: `go test ./internal/sdd/ -count=1`

## Phase 4: GREEN — Text Surfaces + sdd-ff

- [x] 4.1 GREEN: lane-aware read lists `:19`/`:25`: "or the merged `plan.md` in the fast lane" (REQ-LANE, `internal/sdd/instructions.go`, ~2)
- [x] 4.2 GREEN: apply ready = planning set resolved (real or alias), not all-done (REQ-LANE, `internal/assets/skills/_shared/sdd-status-contract.md:223`, ~1)
- [x] 4.3 GREEN: `:338` "Never skip phases" → "Never skip gates" (REQ-LANE, `internal/assets/biggz/biggz-orchestrator-workflow.md`, ~1)
- [x] 4.4 GREEN: `sdd-ff.md` lane mode — `plan.md` scaffold (canonical headings + checklist, Total>0); depth follows artifacts; gates kept; no new command (REQ-LANE, `internal/assets/prompts/sdd/sdd-ff.md`, ~15)
- [x] 4.5 Verify: `go test ./internal/assets/ ./internal/sdd/ -count=1`; grep `:338` lacks "Never skip phases"

## Phase 5: Apply-Time Checks

- [x] 5.1 `deriveRoute` (`status.go:1216`) reports `organic` flags on plan-only; consumers tolerate; no verdict persisted (REQ-LANE, `internal/sdd/status.go`, ~0)
- [x] 5.2 Legacy renderers (`sdd-continue`/status) checked over plan-only change (REQ-LANE, `internal/sdd`, ~0)
- [x] 5.3 Engram branch of `resolveArtifactPaths` tolerates `bigmem:sdd/{name}/plan` via `firstPath` (REQ-ALIAS, `internal/sdd/status.go:994`, ~0)
- [x] 5.4 Rebuild CLI (`go build ./cmd/biggz`) — assets embedded, before dogfood (REQ-LANE, `cmd/biggz`, ~0)
- [x] 5.5 Dogfood: `biggz sdd-status --json --cwd <plan-only fixture>` → apply ready (REQ-ALIAS, ~0)
- [x] 5.6 Final: `go test ./... -count=1 -timeout 900s` green (REQ-ALIAS, ~0)
