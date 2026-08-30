# Delta for runtime

## MODIFIED Requirements

### Requirement: Background Subagents 4-Source Policy Resolution

The system MUST resolve background subagents policy via `internal/sdd/background.go` `resolveBackgroundSubagentsPolicy(cwd,opts)` with precedence `project > global > env > default off`. `project` reads `cwd/.biggz/background-subagents.json`, `global` reads `~/.biggz/background-subagents.json` (honoring `GENTLE_PI_CONFIG_HOME`/`BIGGZ_CONFIG_HOME`), `env` reads `BIGGZ_BACKGROUND_SUBAGENTS` (fallback `GENTLE_PI_BACKGROUND_SUBAGENTS`), strict 2-key decode `{"schema":"gentle-pi.background-subagents/v1","policy":"on"|"off"}` where extra keys → malformed → `off` no fallback, `malformed true`. Max 2 JSON reads per resolve. Capability `ready|absent` via `subagent_run` probe.
(Previously: `cwd/.pi/settings.json` and `~/.pi/agent/settings.json` with `backgroundSubagents` key and pi-scoped resolution)

#### Scenario: Project overrides global and env
- GIVEN `cwd/.biggz/background-subagents.json` `{"schema":"gentle-pi.background-subagents/v1","policy":"on"}` and global `off` and env `on`
- WHEN `resolveBackgroundSubagentsPolicy(cwd,opts)` called
- THEN it MUST return `policy on` `source project_file` `malformed false`

#### Scenario: Strict 2-key extra fails closed without fallback
- GIVEN project file `{"schema":"gentle-pi.background-subagents/v1","policy":"on","extra":1}`
- WHEN resolving
- THEN it MUST return `policy off` `malformed true` `source project_file` and MUST NOT consult global/env

#### Scenario: Malformed JSON fails closed
- GIVEN project file contains `{bad`
- WHEN resolving
- THEN it MUST return `policy off` `malformed true` and MUST NOT fall back

#### Scenario: Global beats env when project absent
- GIVEN no project file, global `{"schema":"gentle-pi.background-subagents/v1","policy":"off"}` and env `on`
- WHEN resolving
- THEN it MUST return `policy off` `source global_file`

#### Scenario: Env fallback and default
- GIVEN no project/global and env `on`
- WHEN resolving
- THEN it MUST return `policy on` `source environment`; with no env → `policy off` `source default`

### Requirement: Background Policy Delegate and Reporting

The system MUST own resolution in `internal/sdd/background.go`; `internal/opencode/background.go` `BackgroundPolicy`/`BackgroundResolution` and `internal/agents/pi/adapter.go` MUST be thin delegates to `sdd` resolver. `renderBackgroundSubagentsReport` MUST expose `source`, `policy`, `capability`, `malformed`, `projectFile`/`globalFile` existence and `envValue`, and `loadBackgroundSubagentsPolicy` MUST delegate to `resolve().policy`. Unknown env values MUST be reported as ignored; `wrote` outranked by project file MUST warn.
(Previously: pi adapter owned resolution; opencode recomputed sources)

#### Scenario: Delegate preserves policy
- GIVEN `sdd` resolver returns `on` from project
- WHEN `opencode/background.go` `BackgroundPolicy` called
- THEN it MUST return same `on` without recomputing sources

#### Scenario: Report renders source and malformed
- GIVEN resolution `{source:"project_file",policy:"off",malformed:true}`
- WHEN `renderBackgroundSubagentsReport` called with `capability ready`
- THEN output MUST contain `source=project_file`, `policy=off`, `malformed=true`, and `capability: ready`

#### Scenario: Pi adapter delegates
- GIVEN `pi` `ResolveBackgroundSubagentsPolicy` called
- WHEN invoked
- THEN it MUST delegate to `sdd` resolver and preserve precedence

## ADDED Requirements

### Requirement: Background Capability Probe and Disabled Reporting

The system MUST compute `capability` as `ready` when `subagent_run` tool or `pi-subagents` package is present, else `absent`. Status line MUST be `background subagents: <policy> (decided by <source>; capability: <capability>)` with `disabled|unmanaged` reporting when policy `off` or capability absent. `BIGGZ_BACKGROUND_SUBAGENTS` takes precedence over `GENTLE_PI_BACKGROUND_SUBAGENTS` when both set.

#### Scenario: Capability ready when subagent_run present
- GIVEN `subagent_run` registered in tool list
- WHEN capability probe runs
- THEN it MUST return `ready`

#### Scenario: Capability absent without probe
- GIVEN no `subagent_run` and no `pi-subagents` package
- WHEN probe runs
- THEN it MUST return `absent` and background launches MUST be inert

#### Scenario: Disabled reporting when policy off
- GIVEN `policy off` `capability absent`
- WHEN status line rendered
- THEN it MUST contain `policy: off` and `capability: absent` and `disabled/unmanaged` notice
