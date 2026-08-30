# Tasks: skill-registry-watcher-fsnotify

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~267 (180+20+60+2+5) |
| 400-line budget risk | Low |
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
| 1 | Full watcher: fsnotify debounce + fingerprint gate + poll fallback + doctor | PR 1 | `go test ./internal/skillregistry -run TestWatcher && go vet ./...` | Edit `SKILL.md` → regen ≤500ms; `biggz doctor --json` PASS/WARN | `git revert <commit> && go mod tidy` — one commit |

## Phase 1: Foundation

- [x] 1.1 Add `fsnotify/fsnotify` to `go.mod` (`go get`+`tidy`, verify `go list -m`)
- [x] 1.2 Extract `uniqueExistingDirs(root) []string` in `registry.go` — dedupe 7 `ProviderPriority`, `Stat` filter, skip missing
- [x] 1.3 Scaffold `watcher.go`: `WatchDebounceMS=500ms`, `PollInterval=30s`, `Watcher` struct, `isSkillMD` filter

## Phase 2: Core Implementation

- [x] 2.1 Implement `shouldSkipWatcher()` — `BIGGZ_NO_SKILL_REGISTRY=1`, alias `GENTLE_PI_`, `--no-skills`/`-ns` in `os.Args`; gate watcher only
- [x] 2.2 Implement `watchRecursive`+`Start(root,ctx)` — `uniqueExistingDirs`→`NewWatcher`→`WalkDir+Add`, gated→`(nil,nil)`
- [x] 2.3 Implement loop: `Events/Errors/ctx/timer/ticker` select, `isSkillMD` filter, single `Timer` reset, drain on `Stop`
- [x] 2.4 On `timer.C`: `fp:=Fingerprint` vs `lastFP`; `Cached`→no-op else `Refresh`+update `lastFP`
- [x] 2.5 Fallback: `len(active)==0`→30s `Ticker` poll `Fingerprint` only on change; `IsPolling/IsWatching`; `Create IsDir→watchRecursive`
- [x] 2.6 Implement `Close()` — `sync.Once` idempotent, `watcher.Close`, `timer.Stop`+drain, `ticker.Stop`, `ctx` triggers `Close`

## Phase 3: Integration

- [x] 3.1 Create `internal/doctor/skillregistry.go` — `SkillRegistryCheckID="skill-registry"`, polling→WARN else PASS
- [x] 3.2 Register check in `internal/doctor/runner.go`
- [x] 3.3 Verify `--no-skills`/`-ns` wiring in `cmd/biggz` if not via `os.Args` direct

## Phase 4: Testing / Verification

- [x] 4.1 Gate tests: env `BIGGZ_`/`GENTLE_PI_` and `--no-skills`/`-ns` no watcher/ticker but `Refresh` still works
- [x] 4.2 Dirs tests: existing returned, missing skipped, deduped 7 providers, recursive `Add`, `Create` adds subdir
- [x] 4.3 Debounce tests: burst 3×300ms→1 `Refresh`, 2×600ms→2, non-`SKILL.md` ignored
- [x] 4.4 Fingerprint tests: changed→`Regenerated`, `Cached`→no write
- [x] 4.5 Fallback tests: all `Add` fail→ticker polls on change only, partial→no ticker, doctor WARN/PASS
- [x] 4.6 Lifecycle tests: double `Close` no panic, `ctx` cancel cleans, `go vet`+`go test ./internal/skillregistry` PASS

## Phase 5: Cleanup

- [x] 5.1 `go vet ./...` + `go test ./... -count=1 -timeout 180s`; verify `git diff --stat` <400 lines
- [x] 5.2 Keep only `slog.Debug` for watcher errors; document `Start` `nil,nil` gated case
