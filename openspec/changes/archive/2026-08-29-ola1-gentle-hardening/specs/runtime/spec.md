# Delta for runtime

## ADDED Requirements

### Requirement: Background Subagents 4-Source Policy Resolution

The system MUST resolve background subagents policy via `internal/agents/pi/adapter.go` `resolveBackgroundSubagentsPolicy(cwd,opts)` with precedence `project > global > env > default off`. `project` reads `cwd/.pi/settings.json`, `global` reads `gentleAiConfigHome(homeDir)` (`~/.pi/agent/settings.json`), `env` reads `BIGGZ_BACKGROUND_SUBAGENTS`/`PI_BACKGROUND_SUBAGENTS`. Malformed JSON MUST fail closed to `off` without falling back to the next source and MUST set `malformed true`.

#### Scenario: Project overrides global and env

- GIVEN `cwd/.pi/settings.json` contains `{"backgroundSubagents":"on"}` and global is `off` and env is `on`
- WHEN `resolveBackgroundSubagentsPolicy(cwd, opts)` is called
- THEN it MUST return `policy on` with source `project_file` and `malformed false`

#### Scenario: Malformed file fails closed without fallback

- GIVEN `cwd/.pi/settings.json` contains malformed JSON `{bad`
- WHEN `resolveBackgroundSubagentsPolicy` reads it
- THEN it MUST return `policy off` with `malformed true` and MUST NOT fall back to global or env

#### Scenario: Global off beats env on when project absent

- GIVEN no project file, global `~/.pi/agent/settings.json` is `{"backgroundSubagents":"off"}` and env is `on`
- WHEN resolving
- THEN it MUST return `policy off` from `global` and MUST NOT use env

#### Scenario: Default off when no source present

- GIVEN no project, no global, and no env variable
- WHEN resolving
- THEN it MUST return `policy off` with source `default`

### Requirement: Background Policy Delegate and Reporting

The system MUST provide `internal/opencode/background.go` `BackgroundPolicy`/`BackgroundResolution` as thin delegates to the `pi` resolver, and `internal/agents/pi/adapter.go` `renderBackgroundSubagentsReport` MUST expose `source`, `policy`, `capability`, and `malformed` for status reporting while `loadBackgroundSubagentsPolicy` delegates to `resolve().policy` and keeps `Resolve*` wrappers.

#### Scenario: Delegate preserves policy

- GIVEN `pi` resolver returns `on` from project
- WHEN `opencode/background.go` `BackgroundPolicy` is called
- THEN it MUST return the same `on` without recomputing sources

#### Scenario: Report renders source and malformed

- GIVEN resolution is `{source:"project_file", policy:"off", malformed:true}`
- WHEN `renderBackgroundSubagentsReport` is called
- THEN output MUST contain `source=project_file`, `policy=off`, and `malformed=true`
