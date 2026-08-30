# Delta for skill-registry-watcher

## ADDED Requirements

### Requirement: Watcher Lifecycle

The system MUST provide `internal/skillregistry/watcher.go` with `Start(cwd, ctx)` and `Close()` using `fsnotify`. `Start` MUST create watcher/poll state; `Close` MUST clean `activeWatchers` and ticker, be idempotent; `ctx` cancellation MUST trigger `Close`.

#### Scenario: Start and Close happy path

- GIVEN `Start` with valid `cwd` and existing dirs
- WHEN `SKILL.md` changes then `Close` called
- THEN watchers/ticker MUST be released without leak

#### Scenario: Idempotent Close

- GIVEN `Start` succeeded
- WHEN `Close` called twice or `ctx` cancelled
- THEN second call MUST be no-op without panic

### Requirement: Debounce Coalescing

The system MUST coalesce `fsnotify` events via single `time.Timer` with `WATCH_DEBOUNCE_MS=500`. Events within 500ms MUST reset timer; firing MUST trigger at most one `Refresh` per burst. Non-`SKILL.md` events MUST NOT reset timer.

#### Scenario: Burst coalesced

- GIVEN three `SKILL.md` writes within 300ms
- WHEN timer fires after 500ms silence
- THEN exactly one `Refresh` MUST be attempted

#### Scenario: Spaced events separate

- GIVEN two writes 600ms apart
- WHEN each window elapses
- THEN two `Refresh` attempts MUST occur

### Requirement: Fingerprint-Gated Refresh

The system MUST gate `Refresh` by `Fingerprint(projectRoot)`. If `Result.Cached` is true the system MUST NOT rewrite registry. Only fingerprint change MUST regenerate.

#### Scenario: Changed fingerprint regenerates

- GIVEN `SKILL.md` edit changes content/size
- WHEN debounce fires and fingerprint differs
- THEN `Refresh` MUST regenerate and update cache

#### Scenario: Cached no-op

- GIVEN debounce fires but fingerprint equals cache
- WHEN evaluated
- THEN result MUST be `Cached=true` with no write

### Requirement: Watcher-Only Gate

The system MUST skip watcher when `BIGGZ_NO_SKILL_REGISTRY=1` or alias `GENTLE_PI_NO_SKILL_REGISTRY=1` or flag `--no-skills`/`-ns`. Gate MUST affect watcher/poll only; initial `Refresh` and manual refresh MUST still work.

#### Scenario: Env gates watcher

- GIVEN `BIGGZ_NO_SKILL_REGISTRY=1`
- WHEN `Start` called
- THEN no watchers and no ticker MUST be created

#### Scenario: Alias/flag gate but refresh works

- GIVEN `GENTLE_PI_NO_SKILL_REGISTRY=1` or `--no-skills`
- WHEN watcher gated and `Refresh(root,false)` called
- THEN watcher stays idle but `Refresh` MUST execute

### Requirement: Recursive Watch on Existing Directories

The system MUST resolve targets via `uniqueExistingDirs` over 7 `ProviderPriority` (`user:opencode/biggz/claude/kilo` + `project:skills/opencode/github`), walk existing dirs recursively with `watcher.Add` for subdirs (Linux), `Add` new subdirs on `Create`, skip missing dirs without error.

#### Scenario: Existing dirs watched recursively

- GIVEN existing dirs with nested subdirs containing `SKILL.md`
- WHEN `Start` walks and adds
- THEN every subdir MUST be watched and `SKILL.md` change MUST fire

#### Scenario: Missing skipped and Create adds

- GIVEN one existing and one missing dir, then new subdir created
- WHEN `Start` resolves and later `Create` event arrives
- THEN missing MUST be skipped without error and new subdir MUST be added

### Requirement: Poll Fallback and Doctor Warning

The system MUST start 30s `Ticker` polling `Fingerprint()` when `NewWatcher`/`Add` fails for all dirs; poll MUST `Refresh` only on change. `biggz doctor` MUST return `WARN` when poll active and `PASS` when watcher healthy. Poll MUST NOT run if any watcher succeeded.

#### Scenario: All watches fail triggers poll

- GIVEN all `Add` fail
- WHEN `Start` completes
- THEN 30s ticker MUST poll `Fingerprint` and refresh only if changed

#### Scenario: Doctor warn vs pass and partial success

- GIVEN poll active vs at least one watcher ok
- WHEN `biggz doctor` runs and `Start` state checked
- THEN poll case MUST be `warn`, healthy case MUST be `pass` and no ticker
