# Tasks: fix-bigmem-recall-recency

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 180–220 (1 new + 6 modified) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `Recent()` wrapper | PR1 | `go test ./internal/bigmem -run TestRecent -count=1` | `biggz recall --json --limit 5` | Del `recall.go` |
| 2 | CLI `recall` + `recent` | PR1 | `go test ./cmd/biggz -run TestRecall -count=1` | `biggz recall --help` | Revert 2 CLI files |
| 3 | Guardrail+gate+install | PR1 | `grep -F "For recency use" internal/assets/biggz/bigmem-protocol.md` | `biggz install --dry-run` | Revert 3 asset files |
| 4 | Docs rank vs recency | PR1 | `grep -F "ORDER BY" docs/architecture.md` | `cat docs/architecture.md` | Revert docs |

## Dependency Graph

```
1.1→1.2/1.3→2.1→2.2; 1.1→3.1→3.2→3.3; 1.1→4.1→4.2(all)
RED: limit50+promptBypass in 1.2; worktree lineage in 3.2
```

## Phase 1: Core Helper (Work Unit 1)

- [x] 1.1 Create `internal/bigmem/recall.go` `Recent(opts) Search("",opts)` preserve 1801 `updated_at DESC` vs 1844 `rank`. Dep: none. Verify: `go vet ./internal/bigmem`.
- [x] 1.2 RED→GREEN `internal/bigmem/recall_test.go` seed 2026-08-27/2026-09-01 fresh-first, limit 100→≤50. Dep: 1.1. Verify: `go test ./internal/bigmem -run TestRecent -count=1`.
- [x] 1.3 Filter + invariant tests `type/project/scope` + grep 1801/1844. Dep: 1.1. Verify: `go test ./internal/bigmem -run TestRecent -count=1`.

## Phase 2: CLI (Work Unit 2)

- [x] 2.1 Add `biggz recall` in `cmd/biggz/main.go` + `recent` alias in `cli_bigmem.go` shared handler, flags `--type --project --scope --limit --json --all --match-mode` cap50 help note. Dep: 1.1. Verify: `go vet ./cmd/biggz`.
- [x] 2.2 Tests `TestRecallAlias/Flags/Help` (REQ-RR1-CLI). Dep: 2.1. Verify: `go test ./cmd/biggz -run TestRecall -count=1`.

## Phase 3: Gate Hardening (Work Unit 3)

- [x] 3.1 Inject `bigmem-protocol.md` literal `For recency use `+"`"+`bigmem search --query "" ORDER BY updated_at DESC`+"`"+` or `+"`"+`biggz recall`+"`"+`; never use FTS term search for 'latest'.`. Dep: none. Verify: `grep -F "For recency use" internal/assets/biggz/bigmem-protocol.md`.
- [x] 3.2 Harden `biggz-orchestrator-workflow.md` Session Boot Recall: mandate `biggz_mem_context(5)`/`Recent`/`Search("")` + `git log -15` + `sdd-status --json` fallback, ban FTS. Dep: 3.1. Verify: `go test ./internal/assets/biggz -count=1`.
- [x] 3.3 Update `internal/install/install.go` `DeployBigMemProtocol` marker idempotent. Dep: 3.1. Verify: `go test ./internal/install -run TestDeploy -count=1`.

## Phase 4: Docs & Verify (Work Unit 4)

- [x] 4.1 Update `docs/architecture.md` rank vs recency table (Query/ORDER BY/When/Example). Dep: 1.1. Verify: `grep -F "ORDER BY" docs/architecture.md`.
- [x] 4.2 Final: `go test ./... -count=1 -timeout 180s` + `go vet ./...` + grep guardrail + `biggz recall --json` smoke. Dep: all. Verify: all green.

## Files

New: `internal/bigmem/recall.go`; Mod: `cmd/biggz/main.go`, `cli_bigmem.go`, `bigmem-protocol.md`, `biggz-orchestrator-workflow.md`, `install.go`, `docs/architecture.md`
