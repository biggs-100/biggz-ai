# Design: TUI Installer Pipeline

## Context

`internal/install/install.go` (2689L) is called synchronously from `internal/tui/screens/install.go` (241L `Idle→Done` via `doInstall→tea.Msg`). It deploys skills, overlay, MCP, pi extensions and markers without preview, streaming, or rollback. Goal: lean port of gentle-ai's 5491L pipeline to ~150L — `StagePlan` Prepare/Apply + `Orchestrator` + lossless `ProgressEvent` + `RollbackPolicy` + `~/.biggz-ai/state.json` atomic merge. Out-of-scope: BigMem, ledger/SDD, providers.

Preserve guards (`tui.go:isSyncSupported`/`tuiAnimationsDisabled`, `filemerge.WriteFileAtomic`) and `lipgloss`/`FakeAgent.SetTempDir` patterns.

## Goals

- `StagePlan` Prepare (read-only, zero writes outside TempDir) → Apply (sequential) via `Orchestrator.Run`; keep `install.Run` facade.
- `ProgressEvent{Step,Percent 0..100,Message}` over buffered chan (>=16), monotonic lossless, closed by Apply, no leak.
- `RollbackPolicy` (`RollbackOnFailure`/`NoRollback`), idempotent reverse rollback, aggregated errors.
- `--dry-run` → Prepare only + preview; `--agent`/`--yes` via `plugin.AgentAdapter`; suppress CSI 2026 when `BIGGZ_NO_ANIMATION=1`/`TERM=dumb`.
- `installing.go` 30-char bar `█`/`░` (`Percent*30/100`) via `lipgloss`, consumed by `tea.Cmd`.

## Non-Goals

- BigMem, lenses, ledger/SDD, MCP beyond `biggz-mcp`, Rust/desktop.
- New palette (Rose Pine in `styles.go` stays).
- Breaking migration — `state.json` merge is additive.

## Architecture

```
CLI cmd/biggz/install.go (--dry-run --agent --yes)
        │ build StagePlan, Policy
        ▼
  Orchestrator.Run(ctx, StagePlan)
   ├─ Prepare() → PlanPreview (ordered Names, zero writes)
   └─ Apply(ctx, ProgressChan) → ExecutionResult{Success, Steps[]StepResult, Error}
        │ sequential
        ▼
  Step[0..N] (internal/install/steps/*.go)
   Name/Prepare/Apply(ch ProgressChan)/Rollback — via WriteFileAtomic + Adapter paths
        ▼
  ~/.biggz-ai/state.json (raw-map merge + lock) · ~/.biggz/skills/ · agent dirs

TUI: install.go ──tea.Cmd──► Orchestrator.Run ──ProgressChan(buf32)──► installing.go
                               Update(ProgressEvent) → View(): 30×█/░ + Step/Message, syncOutput guarded
```

**`internal/pipeline` ~150L:**

```go
type ProgressEvent struct{ Step string; Percent int; Message string }
type ProgressChan chan ProgressEvent
type Step interface{ Name()string; Prepare(ctx)error; Apply(ctx, ProgressChan)error; Rollback(ctx)error }
type StagePlan interface{ Prepare(ctx)(*PlanPreview,error); Apply(ctx,ProgressChan)(*ExecutionResult,error) }
type Orchestrator struct{ Policy RollbackPolicy }
func (o *Orchestrator) Run(ctx context.Context, p StagePlan)(*ExecutionResult,error)
```

`Apply` owns `close(ch)`; `Orchestrator` blocks Apply if Prepare failed and wraps `fmt.Errorf("%s: %w", step.Name(), err)`.

**Steps** (`internal/install/steps/`): `skills.go` (DeploySkillsToBiggzDir/AgentDir), `overlay.go` (`MergeJSONC`), `state.go` (raw-map + `WriteFileAtomic`+lock), `pi_extensions.go` (pi-guarded). Wrap existing `Deploy*` helpers.

**State merge:** `ReadState` → `map[string]any` raw → merge incoming `AgentID`/`Components`/`Skills` → `WriteFileAtomic` (sha256+rename+`SyncDir`). Unknown keys preserved.

**TUI wiring:** `doInstall` → `tea.Cmd`. `installing.go` creates `ch:=make(ProgressChan,32)`, runs `Orchestrator.Run` in goroutine, forwards via `waitProgress(ch)` until `!ok` → `Done/Failed`. Reuses `tui.go:41 isSyncSupported`/`tuiAnimationsDisabled`+`tickCmd`; `View` strips markers when disabled.

## Alternatives Considered

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Generic pipeline vs inline refactor | Inline simpler but re-creates god-file, no dry-run/rollback reuse | **Generic pipeline** — lean, testable |
| Buffered channel vs callback | Callback couples TUI, blocks if slow, no close | **Buffered channel** — lossless, idiomatic `tea.Cmd` |
| Explicit `Percent` vs auto fraction | Auto hides file-level progress | **Explicit Percent** — step controls granularity |
| Raw-map merge vs typed struct | Struct drops unknown fields | **Raw-map merge** — preserves keys, matches `MergeJSONC` |
| 30-char `█/░` bar vs spinner | Spinner not progress | **█/░ bar** with `ProgressFilled/Empty`, `IsPrettyEnabled` fallback |

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| CSI garble on dumb terminals | Corruption | `isSyncSupported()` strip + guards; `syncOutput` idempotent (`tui.go:63`) |
| Rollback hides root cause | Lost error | Aggregate `apply %s: %w; rollback %s: %v`; idempotent double-Rollback test |
| `state.json` corruption | Data loss | `WriteFileAtomic` + lock + raw-map merge (not overwrite) |
| Split over budget | Review risk | 5 PRs <400L, facade preserved |
| Channel leak/drops | Hang | Cap >=16, `defer close(ch)`, burst 20-event test |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit pipeline | Prepare blocks Apply, success/fail, Rollback reverse+aggregate, close/leak | Fake Step inject error, burst 20, double Rollback |
| Unit steps | Idempotent Apply (mtime), partial cleanup, Prepare zero writes | `FakeAgent.SetTempDir`, `os.Lstat` check |
| Unit state | Atomic merge, unknown preserved, concurrent, dry-run no write | `custom_key` round-trip, `WaitGroup` writers |
| Unit TUI | Bar 0/50/100% = 0/15/30 `█`, monotonic, env guards | `View()` char count, `isSyncSupported==false` when `TERM=dumb`, `tickCmd==nil` when `BIGGZ_NO_ANIMATION=1` |
| Integration | `Run` facade → Orchestrator → steps in TempDir | `Run(ctx,FakeAgent{TempDir})` e2e, only temp written, `go vet` |
| E2E manual | `--dry-run --agent pi --yes` preview vs real | Temp HOME, diff `state.json`, bar streams |

## Rollout

5 PRs stacked to `main` (<400L, `go vet`+`go test`):

1. **PR1 core** — `internal/pipeline` + tests.
2. **PR2 steps** — split 2689L → `steps/` (skills/overlay/pi), `Run` delegates.
3. **PR3 state** — `state` step (`WriteFileAtomic` + raw-map merge + lock).
4. **PR4 TUI** — `installing.go` (30-char bar, lossless chan, guards) into `install.go`.
5. **PR5 CLI** — `cmd/biggz/install.go` flags → Prepare preview vs Apply.

Rollback: `git revert 5..1` (one commit/PR). No migration — additive `state.json`; delete to reset. `BIGGZ_NO_ANIMATION=1` stays.

## Open Questions

- [ ] Lock for `state.json` — reuse `review` lock or `filemerge`-local `flock`?
- [ ] Step count 3 vs 4 (pi separate) to keep each <100L?
