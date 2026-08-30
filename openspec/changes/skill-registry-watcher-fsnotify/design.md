# Design: skill-registry-watcher-fsnotify

## Technical Approach

Port `gentle-pi/skill-registry.ts:500-590` to `internal/skillregistry/watcher.go`. Reuse `Fingerprint`, `Refresh`, `ProviderPriority`, `providerDir` without mutating scan/cache schema. `Start(cwd,ctx)` resolves existing dirs, creates `fsnotify.Watcher`, `WalkDir+Add` subdirs, coalesces `SKILL.md` events with single 500ms `Timer`, fingerprint-gates `Refresh`, falls back to 30s `Ticker` when all watches fail, and gates watcher-only via env/flag. `Close()` is `sync.Once` idempotent; `ctx` cancel triggers `Close`. Doctor reports poll vs healthy. Stacked-to-main, <250 lines.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| Watcher substrate | `fsnotify` vs poll-only vs hybrid | Poll-only simple, 30s latency; fsnotify sub-second; hybrid best UX | `fsnotify` primary + 30s poll fallback; `fsnotify/fsnotify` in `go.mod` |
| Debounce | `Timer.Reset` vs `time.After` vs external lib | `After` leaks goroutines; lib unnecessary | Single `time.Timer` 500ms, reset on `SKILL.md`, non-`SKILL.md` ignored |
| Recursive | `Walk+Add` vs root-only | `fsnotify` non-recursive on Linux; root-only misses nested | `WalkDir` per `uniqueExistingDirs` + `Add`; `Create` IsDir → `Walk+Add` subtree |
| Lifecycle | Struct + `Once`/`Mutex` vs global | Global untestable, leak-prone | `type Watcher struct` with `watcher/timer/ticker/activeWatches/Once`; `Start→*Watcher`, `ctx.Done→Close` |
| Gate scope | Watcher-only vs watcher+`Refresh` | Gating `Refresh` breaks contract | `shouldSkipWatcher` checks `BIGGZ_NO_SKILL_REGISTRY`, alias `GENTLE_PI_NO_SKILL_REGISTRY`, `--no-skills`/`-ns`; `Start` early-return, `Refresh` untouched |

## Data Flow

```
Start(cwd,ctx) → shouldSkipWatcher?─yes→ idle (Refresh still works)
        │no
uniqueExistingDirs(7 providers, existing only)
        │
NewWatcher fail OR Add all fail → 30s Ticker poll Fingerprint→Refresh on change
        │ any Add ok
Events/Errors/ctx/timer/ticker loop → isSkillMD?→ Timer.Reset(500ms)
        │ timer fires → Fingerprint==lastFP? Cached no-op : Refresh+lastFP=fp
Close/ctx cancel → Once: watcher.Close + timer.Stop(drain) + ticker.Stop
Doctor: polling→WARN, watching→PASS, gated→PASS
```

## File Changes

| File | Action | Description | Est. |
|------|--------|-------------|------|
| `internal/skillregistry/watcher.go` | Create | Watcher, `Start`/`Close`, `shouldSkipWatcher`, `uniqueExistingDirs`, recursive Add, debounce+poll loop, `IsPolling/IsWatching` | ~180 |
| `internal/skillregistry/registry.go` | Modify | Add `uniqueExistingDirs` if not exported; no `Fingerprint`/`Refresh` change | ~20 |
| `internal/doctor/skillregistry.go` | Create | `SkillRegistryCheck`: polling→WARN, watching→PASS, gated→PASS | ~60 |
| `internal/doctor/runner.go` | Modify | Register check | 2 |
| `go.mod` | Modify | Add `fsnotify/fsnotify` | 5 |
| `cmd/biggz/*` | Modify | Wire `--no-skills`/`-ns` to gate if not via `os.Args` direct read | ~10 |

## Interfaces / Contracts

```go
const WatchDebounceMS = 500 * time.Millisecond
const PollInterval = 30 * time.Second

type Watcher struct { /* watcher *fsnotify.Watcher; timer *time.Timer; ticker *time.Ticker; active map[string]struct{}; mu sync.Mutex; once sync.Once; root string; lastFP string */ }
func Start(projectRoot string, ctx context.Context) (*Watcher, error) // (nil,nil) when gated
func (w *Watcher) Close() error // idempotent
func (w *Watcher) IsPolling() bool
func (w *Watcher) IsWatching() bool
func shouldSkipWatcher() bool // BIGGZ_/GENTLE_PI_ env ==1 or --no-skills/-ns in os.Args
func uniqueExistingDirs(projectRoot string) []string // ProviderPriority→providerDir, dedupe, Stat
// Reused: Fingerprint(root) string; Refresh(root,bool) (*Result,error); ProviderPriority [7]string
// Doctor: const SkillRegistryCheckID CheckID = "skill-registry"
```

Loop owns Timer; `Stop` drains channel:
```go
if !timer.Stop() { select{case <-timer.C: default:} }
timer.Reset(WatchDebounceMS)
// on Create IsDir → watchRecursive(path); on timer.C → fp:=Fingerprint(root); if fp!=lastFP {Refresh(root,false); lastFP=fp}
```

Errors: `NewWatcher`/`Add` logged `slog.Debug`, no panic; `len(active)==0` → start Ticker; `Refresh` errors logged only.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Gate: env/alias/flags watcher-only (Refresh still works) | Table `t.Setenv` + fake `os.Args` |
| Unit | Dirs: existing returned, missing skipped, deduped 7 providers; recursive Add + Create add | `t.TempDir`, `HOME` override |
| Unit | Debounce: burst 3×300ms→1 Refresh, 2×600ms→2, non-SKILL.md ignored; Fingerprint Cached no-op | Fake events or real `fsnotify` + 50ms timer override, capture `Refresh`/`lastFP` |
| Unit | Fallback: all Add fail → 30s Ticker polls on change only; Lifecycle Close idempotent, ctx cancel | Inject failing watcher, mock Fingerprint, double `Close` |
| Integration | Doctor WARN when polling, PASS when watching, no ticker when any watcher ok | Inject state func via `NewSkillRegistryCheckWithCustom` |
| Gate | `go vet ./...` | CI |

## Threat Matrix

N/A — read-only `fsnotify` on `SKILL.md`; no shell/subprocess/VCS/PR/executable-doc boundary.

| Boundary | Applicability | Design response | Planned RED tests |
|----------|---------------|-----------------|-------------------|
| Documentation-like paths | N/A — only `SKILL.md`, no exec | `isSkillMD` filter | — |
| Git repository selection | N/A — `projectRoot` arg, not git | Use caller `projectRoot` | — |
| Commit state | N/A | — | — |
| Push state | N/A | — | — |
| PR commands | N/A | — | — |

## Migration / Rollout

No migration. Additive; revert via `git revert` + `go mod tidy`. Single PR stacked-to-main.

## Open Questions

- [ ] `Start` Takes explicit `cwd` vs internal `os.Getwd` fallback?
- [ ] Doctor: new `skillregistry.go` vs extend `PlatformCheck`? New file preferred.
- [ ] Pin `fsnotify` at `v1.7` vs latest stable?
