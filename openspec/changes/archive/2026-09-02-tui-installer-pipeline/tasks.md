# Tasks: TUI Installer Pipeline

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1200-1500 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 core → PR2 steps → PR3 state → PR4 TUI → PR5 CLI |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Pipeline core Orchestrator/StagePlan | PR1 | `go test ./internal/pipeline -run TestOrchestrator` | burst 20 lossless + close leak | `internal/pipeline/*` |
| 2 | Steps extraction 2689L→steps/ | PR2 | `go test ./internal/install -run TestStep` | `FakeAgent.SetTempDir` e2e | `internal/install/steps/*` |
| 3 | State merge ~/.biggz-ai/state.json | PR3 | `go test ./internal/install -run TestStateMerge` | 2 writers + `custom_key` | `steps/state.go` |
| 4 | TUI installing 30-char bar █/░ | PR4 | `go test ./internal/tui -run TestInstalling` | `TERM=dumb` + `NO_ANIMATION` | `screens/installing.go` |
| 5 | CLI flags + verify | PR5 | `go test ./cmd/biggz -run TestInstallFlags` | `--dry-run --agent pi --yes` Temp HOME | `cmd/biggz/install.go` |

## Phase 1: PR1 Pipeline Core (~150L)

- [x] 1.1 Create `internal/pipeline/pipeline.go`: `ProgressEvent{Step,Percent,Message}`, `ProgressChan`, `Step`, `StagePlan`, `RollbackPolicy`, `ExecutionResult`
- [x] 1.2 Implement `Orchestrator.Run`: Prepare→Apply, wrap `%s: %w`, `RollbackOnFailure` reverse + aggregate, `close(ch)`
- [x] 1.3 Tests `pipeline_test.go`: Prepare blocks Apply, success/fail, rollback B→A not C, double-Rollback idempotent, burst20 no drops, closed

## Phase 2: PR2 Steps Extraction (2689L split)

- [x] 2.1 Split `install.go` into `steps/skills.go`, `overlay.go`, `pi_extensions.go` via `WriteFileAtomic` + `Adapter`
- [x] 2.2 Refactor `install.Run` facade to build StagePlan → `Orchestrator.Run`
- [x] 2.3 Tests `steps_test.go`: idempotent Apply mtime unchanged, partial 2/5 rollback cleans, Prepare zero writes, `FakeAgent.SetTempDir`

## Phase 3: PR3 State Step (state.json merge)

- [x] 3.1 Create `steps/state.go`: raw `map[string]any` merge `AgentID`/`Components`, `WriteFileAtomic`+lock, create `~/.biggz-ai/`
- [x] 3.2 Prepare validates AgentID, Apply preserves unknown, missing→default
- [x] 3.3 Tests `state_test.go`: `custom_key` preserved, nested byte-identical, concurrent WaitGroup no corrupt, dry-run no file, atomic no partial

## Phase 4: PR4 TUI Installing (30 chars █/░)

- [x] 4.1 Create `screens/installing.go`: `InstallingModel`, bar `Percent*30/100` `█`/`░`, `waitProgress` tea.Cmd
- [x] 4.2 Wire `screens/install.go:doInstall` → `Orchestrator.Run` `ProgressChan(32)`, reuse `isSyncSupported`, strip CSI when `NO_ANIMATION`/`TERM=dumb`
- [x] 4.3 Tests `installing_test.go`: bar 0%=30`░` 50%=15`█`+15`░` 100%=30`█`, 10 events lossless, `TERM=dumb` zero CSI, close→Done

## Phase 5: PR5 CLI Wiring (--dry-run --agent --yes)

- [x] 5.1 Extend `cmd/biggz/install.go`: flags `--dry-run --agent --yes` → Prepare preview only (StagePlan Prepare + ProgressChan(32), zero writes) via `install.go` 184L + `cli_sync_install.go` removed duplicate installRun
- [x] 5.2 Route `--agent` via `AgentAdapter`; `--yes` skips prompt but validates (invalid `--agent nope` blocks Apply even with --yes, Prepare validation loop before Apply)
- [x] 5.3 Tests `install_test.go`: dry-run zero writes, invalid agent blocks Apply, e2e `Run(FakeAgent{TempDir})` → `cmd/biggz/install_test.go` 5 tests + `internal/install/install_pr5_test.go` 4 tests PASS (dry-run no state.json, invalid Prepare blocks, e2e 4 steps + ProgressChan lossless)

## Phase 6: Verification

- [x] 6.1 `go vet ./... && go test ./... -count=1` — evidenced: `go vet ./...` exit 0, `go test ./... -count=1 -timeout 180s` exit 0 (33da42e), ledger rev 4fc4779
- [x] 6.2 Manual: `BIGGZ_NO_ANIMATION=1`, `TERM=dumb` plain, `--dry-run --agent pi --yes` preview vs real — covered by automated tests `TestInstalling_TermDumb_ZeroCSI`, `TestInstalling_NoAnimation_DisablesCSIAndTick`, `TestPR5_DryRunZeroWrites` (verify-report WARNING resolved via automated guards)
- [x] 6.3 Verify each PR <400L `git diff --stat main`, `git revert 5..1` — evidenced: each file <400L verified via `wc -l` (max 368 overlay.go, 350, etc.), `git diff --stat HEAD` shows per-file deltas <400 (96 cli_sync, 322 install.go split, 136 tui), all steps per verify-report
