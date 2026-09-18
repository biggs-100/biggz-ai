# Delta for agent-install

## MODIFIED Requirements

### Requirement: Pi MCP Adapter in InstallCommand

`Adapter.InstallCommand` MUST include `npm:pi-mcp-adapter@^2` before `npm:pi-subagents-j0k3r@1.6.1` (exact pin, never floating), idempotent, offline-tolerant with doc fallback. The pinned identity MUST also be used by `desiredPiPackages` reconciliation.
(Previously: j0k3r referenced unpinned/floating.)

#### Scenario: Includes adapter in order

- GIVEN Pi `InstallCommand` invoked
- WHEN the list is generated
- THEN it MUST contain `npm:pi-mcp-adapter` with `^2` preceding `npm:pi-subagents-j0k3r@1.6.1`

#### Scenario: Pin is exact

- GIVEN install reconciliation runs
- WHEN desired packages are computed
- THEN `pi-subagents-j0k3r` MUST carry `@1.6.1` and MUST NOT float

#### Scenario: Idempotent second run

- GIVEN adapter already installed
- WHEN command runs again
- THEN it MUST succeed without duplication

#### Scenario: Offline harmless

- GIVEN npm unreachable
- WHEN `pi install` fails
- THEN error MUST hint doc fallback and MCP JSON MUST remain harmless

### Requirement: REQ-INST-001 — Pi Web Search Extension Deployment

The system MUST provide `DeployPiWebSearch(ctx, homeDir)` that writes `internal/assets/pi/biggz-web-search.js` to `~/.pi/agent/extensions/biggz-web-search.js` via `filemerge.WriteFileAtomic`. It MUST create parent directories, MUST be idempotent, MUST integrate with `Run()` and `Result.PiWebSearch`, and MUST support TempDir routing for tests. Subagent deployment MUST be served by `PiExtensionsStep.deploySubAgents` (the live path on the `Run()` pipeline); the dead `DeployPiSubAgents`, `DeployPiWaitPretty`, and `DeployPiPrettyWrapper` helpers MUST be removed, with no remaining install-flow or test references.
(Previously: `Run()` was stated to call `DeployPiSubAgents` alongside the standalone deploy helpers; the dead-helper removal was unaccounted.)

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
- THEN `PiExtensionsStep` MUST deploy `biggz-web-search.js` and the subagent files via `PiExtensionsStep.deploySubAgents`
- AND the removed `DeployPiSubAgents`/`DeployPiWaitPretty`/`DeployPiPrettyWrapper` MUST have no remaining call sites

#### Scenario: TempDir isolation for tests

- GIVEN a `plugintest.FakeAgent` with `TempDir` set
- WHEN `DeployPiWebSearch` is invoked
- THEN the file MUST be written under `TempDir` and no file outside `TempDir` MUST be modified

#### Scenario: Self-heal removes legacy if present

- GIVEN a legacy extension `biggz-pi-pretty.js` exists (or any deprecated web-search variant)
- WHEN `DeployPiWebSearch` or `Run()` executes
- THEN the legacy file MUST be removed atomically if its content is outdated

## ADDED Requirements

### Requirement: j0k3r Retirement and Cutover Reconcile

Once the subagent-runtime marker is deployed, install/upgrade MUST reconcile Pi settings so `pi-subagents-j0k3r` is absent (no dual registration with the runtime) and MUST drop it from `InstallCommand`; reverting sources plus reinstall MUST restore the pinned package.

#### Scenario: Cutover removes j0k3r

- GIVEN the runtime marker deployed under `~/.pi/agent/extensions/`
- WHEN `biggz install --agent pi` runs
- THEN `pi-subagents-j0k3r` MUST be removed from settings `packages` and MUST NOT appear in `InstallCommand`

#### Scenario: No dual registration after cutover

- GIVEN cutover applied
- WHEN Pi loads extensions
- THEN only the runtime MUST register delegation tools

#### Scenario: Rollback restores pinned j0k3r

- GIVEN cutover reverted
- WHEN `biggz install --agent pi` re-runs
- THEN `pi-subagents-j0k3r@1.6.1` MUST be reinstalled
