# Delta for pi-deploy-list

## MODIFIED Requirements

### Requirement: Guard Registered for Deploy

The system MUST include `{"pi/biggz-session-guard.js", "biggz-session-guard.js"}` alongside `biggz-tool-interception.js` and `biggz-extension-api.js` in `piExtensionsDeployList()`, and MUST NOT include `biggz-memory-chrome.js` nor `biggz-synthesis-gate.js`.
(Previously: list included both wrappers as deployed fallback.)

#### Scenario: Deploy list contains the guard

- GIVEN the deploy list in `pi_extensions.go`
- WHEN inspecting its entries
- THEN one entry MUST map asset `pi/biggz-session-guard.js` to target `biggz-session-guard.js`

#### Scenario: Deploy list excludes both wrappers

- GIVEN the updated deploy list
- WHEN inspecting its entries
- THEN no entry MUST reference `biggz-memory-chrome.js` or `biggz-synthesis-gate.js`

#### Scenario: Build passes and deployed imports resolve

- GIVEN the updated deploy list
- WHEN running `go build ./...` and deploying extensions
- THEN build MUST pass, deployed dir MUST contain `biggz-session-guard.js` and MUST NOT contain either wrapper

## ADDED Requirements

### Requirement: Stale Wrapper Self-Heal on Upgrade

The system MUST remove stale `~/.pi/agent/extensions/biggz-{memory-chrome,synthesis-gate}.js` via `os.Remove` during upgrade; missing files MUST be silent no-ops.

#### Scenario: Stale copies removed on upgrade

- GIVEN stale wrapper copies exist in deployed extensions dir
- WHEN upgrade apply runs
- THEN both stale files MUST be absent afterwards and install MUST succeed

#### Scenario: Missing stale files are no-op

- GIVEN no stale copies present
- WHEN upgrade apply runs
- THEN removal MUST succeed silently without error

### Requirement: Factory Test Mirror Updated

`biggz-pi-extensions-factory.test.mjs` MUST drop both wrapper entries, fix the count guard, and still assert every listed entry exports a valid factory function.

#### Scenario: Count guard matches shrunk list

- GIVEN updated deploy list with 2 fewer entries
- WHEN factory test runs `node --test`
- THEN count assertion MUST pass for the reduced total

#### Scenario: Factory shape still valid

- GIVEN remaining listed extensions
- WHEN test imports each default export
- THEN each MUST be a callable factory function
