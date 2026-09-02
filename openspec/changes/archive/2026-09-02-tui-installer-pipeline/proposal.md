# Proposal: TUI Installer Pipeline

## Intent

Replace `install.Run` from wizard `internal/tui/screens/install.go` (241L Idle→Done) calling monolith `internal/install/install.go` (2689L) with `StagePlan` (`Prepare`→`Apply`) + `ProgressEvent` + `state.json` merge + bar. Lean vs gentle-ai 5491L.

## Scope

### In Scope
- `internal/pipeline` ~150L: `Orchestrator`, `Step`, `StagePlan` (Prepare/Apply), `ExecutionResult`, `ProgressEvent`, `RollbackPolicy`
- Split `internal/install/install.go` (2689L) into steps via pipeline
- `internal/tui/screens/installing.go`: bar 30 chars `█`/`░`, lossless channel, `isSyncSupported`
- `~/.biggz-ai/state.json` merge `--agent` (preserve unknown, atomic)
- CLI `--dry-run --agent --yes` → `Prepare` preview

### Out of Scope
- BigMem, lenses, ledger/SDD, providers/MCPs, Rust/desktop

## Capabilities

### New Capabilities
- `installer-pipeline`: lean `StagePlan` Prepare/Apply + `Orchestrator` + `ProgressEvent` + `RollbackPolicy` (~150L)

### Modified Capabilities
- `tui`: Running spinner → `installing.go` progress screen
- `agent-install`: monolith `install.go` 2689L → reversible steps
- `state-persistence`: pipeline merge (atomic, preserve unknown)
- `cli`: `--dry-run/--agent/--yes` → `Prepare`

## Approach

5 PRs stacked to `main` (<400 each):

1. PR1 core — `StagePlan`/`Orchestrator`/`Step`/`ProgressEvent` + tests
2. PR2 steps — split 2689L → `Prepare`/`Apply`/`Rollback`
3. PR3 state — `~/.biggz-ai/state.json` merge via `filemerge`
4. PR4 TUI — `installing.go` bar 30 chars, lossless chan
5. PR5 CLI — flags + `doInstall` → bar

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/pipeline/*` | New | ~150L |
| `internal/install/*` | Modified | Split 2689L → `steps/` |
| `internal/tui/screens/installing.go` | New | Bar 30 chars, channel |
| `internal/tui/screens/install.go` | Modified | `doInstall` → `Orchestrator` |
| `cmd/biggz/install.go` | Modified | `--dry-run --agent --yes` |
| `openspec/specs/installer-pipeline/spec.md` | New | Pipeline spec |
| `openspec/specs/tui/spec.md` | Modified | Delta bar |
| `openspec/specs/agent-install/spec.md` | Modified | Delta steps |
| `openspec/specs/state-persistence/spec.md` | Modified | Delta merge |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| CSI garble | Low | `isSyncSupported()` + `BIGGZ_NO_ANIMATION` |
| Rollback failure | Med | `RollbackPolicy`/Step idempotent, injection tests |
| state.json corruption | Low | Atomic write + lock, merge not overwrite |
| God-file split | Med | Preserve `install.Run` contract, <400L slices |

## Rollback Plan

`git revert 5..1` (1 commit/PR). No migration — `state.json` additive; delete if needed. `BIGGZ_NO_ANIMATION=1` stays. Verify `go vet` + `go test`.

## Dependencies

- `bubbles`, `lipgloss`, `internal/assets` (`assets.FS`), `internal/plugin.AgentAdapter`

## Success Criteria

- [ ] `Prepare`→`Apply` via `Orchestrator`/`Step` executes
- [ ] `RollbackPolicy` reverts partial `Apply` on failure
- [ ] Progress channel lossless 0→100 (no drops)
- [ ] `state.json` at `~/.biggz-ai/state.json` merges `--agent`, preserves unknown fields
- [ ] TUI bar 0–100, 30 chars `█`/`░` streams from channel
- [ ] `--dry-run` = Prepare only, zero writes (TempDir)
- [ ] `isSyncSupported` + `BIGGZ_NO_ANIMATION` preserved
- [ ] `go vet` + `go test` pass; each PR <400L
