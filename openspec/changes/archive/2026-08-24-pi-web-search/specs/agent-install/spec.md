# Delta for agent-install

## ADDED Requirements

### Requirement: REQ-INST-001 — Pi Web Search Extension Deployment

The system MUST provide `DeployPiWebSearch(ctx, homeDir)` that writes `internal/assets/pi/biggz-web-search.js` to `~/.pi/agent/extensions/biggz-web-search.js` via `filemerge.WriteFileAtomic`. It MUST create parent directories, MUST be idempotent, MUST integrate with `Run()` and `Result.PiWebSearch`, and MUST support TempDir routing for tests.

#### Scenario: Atomic deploy creates extension

- GIVEN Pi is installed and `homeDir` resolves to `~/.pi/agent`
- WHEN `DeployPiWebSearch(ctx, homeDir)` is called
- THEN `extensions/biggz-web-search.js` MUST exist with embedded bytes written atomically via temp+rename
- AND `Result.PiWebSearch` MUST indicate deployed

#### Scenario: Idempotent second deploy

- GIVEN `biggz-web-search.js` already exists with identical embedded bytes
- WHEN `DeployPiWebSearch` is called again
- THEN no file MUST be modified and the function MUST return success

#### Scenario: Deploy via Run()

- GIVEN `install --agent pi` invokes `Run(ctx, cfg)`
- WHEN `Run()` executes
- THEN it MUST call `DeployPiWebSearch` alongside `DeployPiSubAgents`, `DeployPiThinkingWrap`, and `DeployPiLastModel`

#### Scenario: TempDir isolation for tests

- GIVEN a `plugintest.FakeAgent` with `TempDir` set
- WHEN `DeployPiWebSearch` is invoked
- THEN the file MUST be written under `TempDir` and no file outside `TempDir` MUST be modified

#### Scenario: Self-heal removes legacy if present

- GIVEN a legacy extension `biggz-pi-pretty.js` exists (or any deprecated web-search variant)
- WHEN `DeployPiWebSearch` or `Run()` executes
- THEN the legacy file MUST be removed atomically if its content is outdated

### Requirement: REQ-INST-002 — Overlay and Skill Gating Integration

The system MUST expose `web_search`/`web_fetch` to `sdd-research` only via `internal/assets/opencode/sdd-overlay-multi.json` and `sdd-research` skill docs. Non-research agents MUST NOT receive the tools. `internal/assets/embed.go` with `//go:embed all:pi` MUST automatically include the new asset without code change.

#### Scenario: Overlay allows web tools for sdd-research

- GIVEN `sdd-overlay-multi.json` is merged for `sdd-research`
- WHEN agent tools are resolved under `open-web` grant
- THEN `web_search` and `web_fetch` MUST be in the tool allowlist

#### Scenario: Non-research overlay unchanged

- GIVEN `sdd-overlay-multi.json` is resolved for `sdd-explore` or `sdd-apply`
- WHEN tools are listed
- THEN `web_search`/`web_fetch` MUST be absent

#### Scenario: Embed coverage

- GIVEN `biggz-web-search.js` exists under `internal/assets/pi`
- WHEN `assets.FS` is read
- THEN the file MUST be included via `//go:embed all:pi` without modifying `embed.go`
