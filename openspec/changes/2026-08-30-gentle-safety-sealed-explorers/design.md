# Design: gentle-safety-sealed-explorers — Safety + Sealed Explorers

## Technical Approach

Two stacked PRs mirroring `gentle-ai.ts:280-720` verbatim. PR1 centralizes 6/8/5 safety in `internal/policy/guardrails.go` → 3 surfaces (Pi gate, OpenCode `safety.ts`, `review/gate.go`). PR2 reuses `surfaces.go`+`status.go` for `## Allowed edit surfaces` with `fileCount>=4→scout`. Covers 9 req / 26 scenarios. `Go 1.25`, `strict_tdd false`.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|----------|--------|--------------|-----------|
| Source of truth | `guardrails.go` owns regex+config; JS mirrors verbatim | Duplicate per surface, RPC service | Go testable; runtimes isolated (Go/JS) so copy is explicit |
| Config merge | global→project shallow-copy; project `AutonomousMode` wins; malformed→`safeGuardrailsConfig{false,{}}` | Overwrite, fail-open | Keeps global defaults; prevents mutation; matches oracle |
| Env fast-path | `GENTLE_PI_AUTONOMOUS_MODE=1` → `{true,{}}` no I/O | Always read files | Deterministic autonomous override; spec requires no I/O |
| 3-surface parity | `DENIED[6]/SENSITIVE[8]/GUARDED[5]` literal copy | Shared lib | No shared process across Pi/OpenCode/Go; verbatim keeps cardinality audited |
| Scout fallback | `reject→scout` read-only, log `scout_fallback`, no human block | `ask_user_question` | Gentle invariant: never block human on missing surfaces |

## Data Flow

### Safety (identical on 3 surfaces)

```
ToolCall{tool,input,command}
 ├─ IsDenied(command) ──true──▶ Block (DENIED[6]: rm -rf /|~|$HOME|.., reset --hard, clean -fd, push --force/-f incl. git -C, chmod -R 777, chown -R)
 ├─ EvaluateSensitivePathTool(tool,input) ──match──▶ Block (SENSITIVE[8])
 └─ ClassifyGuardedCommand(command,cfg) ──▶ block|confirm|allow|not-guarded
      IsDenied→block; else GUARDED[5] (gitPush/gitRebase/branchDeleteForce/npmPublish/piRemove):
        !AutonomousMode→confirm | AutonomousMode+override→override | else default (allow/confirm/confirm/block/confirm)
```

### Config merge

```
LoadRuntimeGuardrailsConfig(cwd)
 if GENTLE_PI_AUTONOMOUS_MODE==1 → {true,{}} no I/O
 else global=Parse(~/.pi/gentle-ai/runtime-guardrails.json); project=Parse(cwd/.pi/gentle-ai/runtime-guardrails.json)
      merged=copy(global); merged.AutonomousMode=project.AutonomousMode; merge maps; malformed→safeGuardrailsConfig
```

### Sealed explorers

```
Dispatch{agent,task,context,fileCount} → ShouldEnforceScopedSurfaces(fileCount>=4)
 hasTaskScopedAllowedEditSurfaces: find ## Allowed edit surfaces (ci) → next #{1,2}; parse bullets (strip `); ≥1; validate isTaskScoped each; dedup/sort; all headings agree
 isTaskScoped: \→/, reject empty/absolute/~, whitespace, strip ./+, reject .., reject first-segment *?[]{}
 agent∉{worker,gentle-ai-worker}→nil; has surfaces→nil; else Block WRITER_EDIT_SURFACE_REJECTION→relaunch scout (read-only, logged)
```

## Interfaces / Contracts

```go
// internal/policy/guardrails.go
func IsDenied(command string) bool
func ClassifyGuardedCommand(command string, cfg RuntimeGuardrailsConfig) string // block|confirm|allow|not-guarded
func ParseGuardrailsConfigFile(raw string) (*RuntimeGuardrailsConfig, bool)    // malformed→(nil,false)
func LoadRuntimeGuardrailsConfig(cwd string, configHome ...string) RuntimeGuardrailsConfig
func EvaluateSensitivePathTool(toolName string, input any) *ToolCallDecision   // nil or Block
type RuntimeGuardrailsConfig struct { AutonomousMode bool; GuardedCommands map[string]string }

// JS mirrors (biggz-synthesis-gate.js, safety.ts)
function isDenied(cmd:string): boolean
function classifyGuardedCommand(cmd:string, cfg:RuntimeGuardrailsConfig): string
function evaluateSensitivePathTool(tool:string, input:any): {block:boolean,reason:string}|undefined

// internal/orchestrator/surfaces.go
func IsTaskScopedRepositoryRelativePath(v string) bool
func HasTaskScopedAllowedEditSurfaces(values ...string) bool
func RejectUnscopedBoundedWriterDispatch(input map[string]any) *Rejection
func readAllowedEditSurfaceEntries(section string) []string

// internal/sdd/status.go
func ShouldEnforceScopedSurfaces(fileCount int) bool // >=4
func ValidateBoundedWriterSurfaces(input map[string]any, fileCount int) *ScopedSurfaceRejection
const WRITER_EDIT_SURFACE_REJECTION = "Parent must derive..."
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/policy/guardrails.go` | Modify | Export `IsDenied`, `ClassifyGuardedCommand`, `ParseGuardrailsConfigFile`, `LoadRuntimeGuardrailsConfig`, `EvaluateSensitivePathTool`; verbatim 6/8/5; add `surface+kind` log |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Add DENIED/SENSITIVE/GUARDED triples; deny→block, guarded per mode, sensitive→block |
| `internal/assets/opencode/plugins/safety.ts` | Create | Plugin mirroring guardrails.go; hooks `tool_call`; ~120 lines; follows `model-variants.ts` pattern |
| `internal/review/gate.go` | Modify | Import `policy`; pre-check 3 decisions before publication gates; denied/sensitive→`Allowed=false` |
| `internal/orchestrator/surfaces.go` | Modify | Expose `isTaskScoped…`, `readAllowed…`, `hasTaskScoped…`, `rejectUnscoped…`; integrate scout relaunch |
| `internal/sdd/status.go` | Modify | Keep `ShouldEnforceScopedSurfaces`/`ValidateBoundedWriterSurfaces` (3→false,4→true) |

PR1 ~250, PR2 <150, both <400 stacked-to-main.

## Threat Matrix

| Boundary | Applicable | Design response | Planned RED tests |
|----------|------------|-----------------|-------------------|
| Documentation-like paths | N/A — no execution boundary | — | — |
| Git repository selection | Applicable — `git -C` before `push` | `GIT_PUSH_RE` with global-flags src | `IsDenied("git -C /r push --force")→true`; `IsDenied("git push")→false` |
| Commit state | N/A — no index/staging logic | — | — |
| Push state | Applicable — any `--force`/`-f` including `-uf` is deny | Lookahead `(?=.*--force)` + `(?=.*-[^-]*f)`; denied overrides allow | `Classify("git push --force",{true,{gitPush:allow}})→block` |
| PR commands | N/A — no PR `--head` handling | — | — |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit Go | `IsDenied` 6 families incl. `git -C` force, `EvaluateSensitivePathTool` 8 patterns + `.env` variants + array + `exec→nil`, `ClassifyGuardedCommand` denied-over-allow/defaults/override/`!auto→confirm`, `LoadRuntimeGuardrailsConfig` env/malformed/merge | `go test ./internal/policy -run TestIsDenied` |
| Unit Go | `isTaskScoped` rejects `../` `/` `~` `*.go` `a b` / accepts `./a/b` `foo*.go`; heading parse valid/bad/missing/multi-heading; `Validate` 3→nil 4→Block | `go test ./internal/orchestrator ./internal/sdd` |
| Unit JS | Mirror checks in `biggz-synthesis-gate.test.mjs` + `safety.ts` | Node fixture, no network |
| Integration | 3-surface parity: same `push --force` + `read ~/.ssh` blocks | Harness calling each surface |
| Gate | `go vet`, `go test ./internal/policy ./internal/orchestrator ./internal/sdd -count=1 -timeout 180s` PASS | CI |

## Migration / Rollout

No migration. Rollback: `git revert` PR2→PR1; deletes `safety.ts`.

## Alternatives

- Single PR: rejected — independent rollback, budget.
- Central service: rejected — disjoint runtimes.
- Block human: rejected — scout non-blocking invariant.

## Open Questions

- None blocking; verbatim boundaries already agreed.
