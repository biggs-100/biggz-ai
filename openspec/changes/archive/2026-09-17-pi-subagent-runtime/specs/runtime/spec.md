# Delta for runtime

## MODIFIED Requirements

### Requirement: Background Subagents 4-Source Policy Resolution

The system MUST resolve background subagents policy via `internal/sdd/background.go` `resolveBackgroundSubagentsPolicy(cwd,opts)` with precedence `project > global > env > default off`. `project` reads `cwd/.biggz/background-subagents.json`, `global` reads `~/.biggz/background-subagents.json` (honoring `GENTLE_PI_CONFIG_HOME`/`BIGGZ_CONFIG_HOME`), `env` reads `BIGGZ_BACKGROUND_SUBAGENTS` (fallback `GENTLE_PI_BACKGROUND_SUBAGENTS`), strict 2-key decode `{"schema":"gentle-pi.background-subagents/v1","policy":"on"|"off"}` where extra keys → malformed → `off` no fallback, `malformed true`. Max 2 JSON reads per resolve. Capability `ready|absent` via the runtime marker probe (deployed subagent-runtime extension under `~/.pi/agent/extensions/`), NOT third-party package presence.
(Previously: capability § probed `subagent_run`; now probed via the own runtime marker.)

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

### Requirement: Background Capability Probe and Disabled Reporting

The system MUST compute `capability` as `ready` when the deployed subagent-runtime marker (runtime extension under `~/.pi/agent/extensions/`) is present, else `absent`; third-party `pi-subagents*` package presence alone MUST NOT yield `ready`. Status line MUST be `background subagents: <policy> (decided by <source>; capability: <capability>)` with `disabled|unmanaged` reporting when policy `off` or capability absent. `BIGGZ_BACKGROUND_SUBAGENTS` takes precedence over `GENTLE_PI_BACKGROUND_SUBAGENTS` when both set.
(Previously: `ready` when `subagent_run` tool or `pi-subagents` package was present.)

#### Scenario: Capability ready when runtime marker present

- GIVEN the deployed subagent-runtime marker present
- WHEN the capability probe runs
- THEN it MUST return `ready`

#### Scenario: Capability absent without runtime marker

- GIVEN no runtime marker present
- WHEN the probe runs
- THEN it MUST return `absent` and background launches MUST be inert

#### Scenario: Legacy package alone is not ready

- GIVEN `pi-subagents-j0k3r` installed but no runtime marker
- WHEN the probe runs
- THEN it MUST return `absent`

#### Scenario: Disabled reporting when policy off

- GIVEN `policy off` `capability absent`
- WHEN status line rendered
- THEN it MUST contain `policy: off` and `capability: absent` and `disabled/unmanaged` notice
