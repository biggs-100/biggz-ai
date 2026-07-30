# Tasks: Review Authority System

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1400–1800 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Foundation) → PR 2 (Core) → PR 3 (Gates + CLI) |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
```

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Model types, FSM 13-state, event store, locking, threat-matrix RED tests | PR 1 | `go test ./model/ ./internal/review/ -run 'TestFSM|TestStoreAppend'` | Temp dir store via `t.TempDir()` | Revert model/ changes; store/ is additive |
| 2 | Receipt chain-binding, authority facade, ledger persistence, budget enforcement | PR 2 | `go test ./internal/review/ -run 'TestReceipt|TestAuthority|TestBudget'` | Create chain via store, verify receipt | Revert receipt/authority/correction/ledger |
| 3 | Gates (pre-PR, pre-push, scope detect), CLI subcommands, config, integration tests | PR 3 | `go test ./internal/review/ ./cmd/biggz/ -run 'TestGate|TestScopeDiff|TestCLI'` | `go run . review list` in biggz repo | Revert gate/cli/config files |

## Phase 1: Foundation

- [x] 1.1 Add LineageID, Role, BudgetCounters fields to `model/review.go`
- [x] 1.2 Expand FSM to 13 states with guard table in `model/fsm.go` (all transitions from spec)
- [x] 1.3 Create `internal/review/store.go` — Append, LoadChain, Validate with SHA-256 naming
- [x] 1.4 Create `.git/biggz/review-transactions/` on first Append call
- [x] 1.5 Add per-lineage file-based LOCK to `internal/review/lock.go`
- [x] 1.6 RED test: Git repo selection — relative path, symlink repo, non-repo cwd

## Phase 2: Core Implementation

- [x] 2.1 Rewrite `internal/review/receipt.go` — chain-bound receipt (genesis+head+count+lineage)
- [x] 2.2 Rewrite `internal/review/authority.go` — store facade with inventory/status/validation
- [x] 2.3 Add budget counter enforcement in `internal/review/correction.go`
- [x] 2.4 Adapt `internal/review/ledger.go` to persist via event store
- [x] 2.5 Adapt `internal/review/review.go` lifecycle to use new FSM + store
- [x] 2.6 RED test: Commit state — staged-only, dirty worktree, clean state scope detection

## Phase 3: Gates & CLI

- [x] 3.1 Rewrite `internal/review/gate.go` — PrePRGate (findings + receipt + dry-run)
- [x] 3.2 Add scope change detection via `git diff-tree` to gate package
- [x] 3.3 Add PrePushGate (scope delta + unacknowledged change detection)
- [x] 3.4 Add `review list/status/gate` subcommands to `cmd/biggz/main.go`
- [x] 3.5 Create `.biggz/config.yaml` sample with gate opt-out config
- [x] 3.6 Wire `--json` and `--dry-run` flags to all gate commands

## Phase 4: Integration & Verification

- [x] 4.1 Unit tests: store Append/LoadChain/Validate — collision, tamper, empty
- [x] 4.2 Unit tests: FSM guard table — all 13×4 pairs, budget edge cases
- [x] 4.3 Unit tests: receipt bind+verify — valid, tampered, empty, clone-proof
- [x] 4.4 Unit tests: scope change detection — stub `git diff-tree` output
- [x] 4.5 Integration: gate pre-PR/pre-push — happy, block, dry-run exits 0
- [x] 4.6 Integration: CLI review list/status — N lineages, JSON+table output
- [x] 4.7 Integration: correction budget exhaustion — max fix rounds → reject
