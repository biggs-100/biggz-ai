# Proposal: skill-registry-watcher-fsnotify

## Intent

Registry stale mid-session. `registry.go` refreshes only at `session_start` + manual `biggz skill-registry refresh`; mid-session `SKILL.md` edits invisible. Port gentle-pi `fsnotify` watcher to Go (500ms debounce, fingerprint-gated, 30s poll fallback).

## Scope

### In Scope
- `internal/skillregistry/watcher.go` (`fsnotify/fsnotify`)
- Recursive watch on existing dirs via `uniqueExistingDirs` (7 `ProviderPriority`: `user:opencode/biggz/claude/kilo` + `project:skills/opencode/github`)
- 500ms debounce (`WATCH_DEBOUNCE_MS=500`), fingerprint-gated `Refresh` (no-op if `Cached`)
- Gate `BIGGZ_NO_SKILL_REGISTRY=1` + alias `GENTLE_PI_NO_SKILL_REGISTRY` + `--no-skills`/`-ns` — watcher only
- Poll 30s fallback when `fsnotify` fails; doctor `WARN` when unavailable (no-op cache-hit when watcher works)
- Lifecycle `Start(cwd,ctx)` / `Close()` on shutdown; tests for debounce/no-op/gate/poll

### Out of Scope
- Change `Fingerprint()`/`Refresh()`/cache; disable initial refresh; watch missing dirs; remote skills; per-event TUI toast; reorder priority

## Capabilities

### New Capabilities
- `skill-registry-watcher`: fsnotify watcher, 500ms debounce, existing-dirs recursive, env/flag gate (watcher-only), 30s poll fallback, doctor warn, shutdown close

### Modified Capabilities
- `prompt-skill-resolver`: no scan change; watcher reuses `ProviderPriority`+`Fingerprint` as trigger

## Approach

Port `gentle-pi/skill-registry.ts:500-590` to Go: resolve existing dirs → `fsnotify.NewWatcher()` walk+`Add` subdirs (Linux) → single `Timer` 500ms on `SKILL.md` events → `Refresh` if fingerprint changed → `shouldSkipWatcher()` checks env/flag → if watch fails for all dirs start 30s ticker polling `Fingerprint()`; clean `activeWatchers` on `Close()`. One PR stacked-to-main, <250 lines.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/skillregistry/watcher.go` | New | watcher, debounce, poll, gate |
| `internal/skillregistry/*.go` | Modified | expose `uniqueExistingDirs` if needed |
| `internal/doctor/checks.go` | Modified | warn when watcher unavailable |
| `go.mod` | Modified | add `fsnotify/fsnotify` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Linux non-recursive fsnotify | Med | Walk+Add subdirs; Add on `Create` |
| Thrashing batch save | Med | 500ms debounce + fingerprint check |
| Watcher leak | Low | `activeWatchers` + `Close()` on ctx cancel |

## Rollback Plan

`git revert` one commit; `go mod tidy` removes dep. No migration; revert restores stale-until-restart.

## Dependencies

- `registry.go` `Fingerprint`/`Refresh`/`ProviderPriority`; `fsnotify/fsnotify`; Go 1.25; `go test`+`go vet`; oracle `gentle-pi/skill-registry.ts`

## Success Criteria

- [ ] Edit `SKILL.md` mid-session regens ≤500ms when fingerprint changes; `Cached` → no write
- [ ] Existing dirs watched recursively; missing skipped
- [ ] Env `BIGGZ_`/`GENTLE_PI_` and `--no-skills`/`-ns` gate watcher only
- [ ] fsnotify fail → 30s poll + doctor warn; watcher ok → poll no-op
- [ ] `Close()` cleans watchers/ticker; `go vet` + `go test ./internal/skillregistry` PASS; <400 lines

## Alternatives Considered

- Poll-only: rejected — 30s vs sub-second UX.
- Watch missing dirs: rejected — errors, diverges from `uniqueExistingDirs`.
- Gate initial Refresh: rejected — handoff gates watcher only.
- Per-event toast: rejected — noisy; doctor suffices.
