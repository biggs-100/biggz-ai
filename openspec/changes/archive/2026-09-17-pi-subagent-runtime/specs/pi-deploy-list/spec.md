# Delta for pi-deploy-list

## MODIFIED Requirements

### Requirement: Guard Registered for Deploy

The system MUST include `{"pi/biggz-session-guard.js", "biggz-session-guard.js"}` alongside `biggz-tool-interception.js` and `biggz-extension-api.js` in `piExtensionsDeployList()`, MUST keep `biggz-memory-chrome.js` (live pretty layer — deployed, not stale), MUST include the subagent-runtime pack asset(s) from `internal/assets/pi/`, and MUST NOT include `biggz-synthesis-gate.js` or the retired `biggz-wait-pretty.js`.
(Previously: no runtime pack entry and no wait-shim exclusion; `biggz-memory-chrome.js` was wrongly listed as excluded.)

#### Scenario: Deploy list contains the guard

- GIVEN the deploy list in `pi_extensions.go`
- WHEN inspecting its entries
- THEN one entry MUST map asset `pi/biggz-session-guard.js` to target `biggz-session-guard.js`

#### Scenario: Deploy list keeps memory-chrome and excludes retired files

- GIVEN the updated deploy list
- WHEN inspecting its entries
- THEN `biggz-memory-chrome.js` MUST be listed
- AND no entry MUST reference `biggz-synthesis-gate.js` or the retired `biggz-wait-pretty.js`

#### Scenario: Runtime pack entry present

- GIVEN the updated deploy list
- WHEN inspecting its entries
- THEN a subagent-runtime asset under `pi/` MUST map to its `~/.pi/agent/extensions/` target

#### Scenario: Retired wait shim excluded

- GIVEN the updated deploy list
- WHEN inspecting its entries
- THEN no entry MUST reference `biggz-wait-pretty.js`

#### Scenario: Build passes and deployed imports resolve

- GIVEN the updated deploy list
- WHEN running `go build ./...` and deploying extensions
- THEN build MUST pass, deployed dir MUST contain `biggz-session-guard.js` and MUST NOT contain `biggz-synthesis-gate.js` or the retired `biggz-wait-pretty.js`

### Requirement: Stale Wrapper Self-Heal on Upgrade

The system MUST remove stale `~/.pi/agent/extensions/biggz-{synthesis-gate,wait-pretty}.js` via `os.Remove` during upgrade; missing files MUST be silent no-ops.
(Previously: the removal set covered `biggz-memory-chrome.js` and `biggz-synthesis-gate.js`; `biggz-memory-chrome.js` is deployed, not stale.)

#### Scenario: Stale copies removed on upgrade

- GIVEN stale copies of retired extensions (including `biggz-wait-pretty.js`) exist in the deployed extensions dir
- WHEN upgrade apply runs
- THEN the stale files MUST be absent afterwards and install MUST succeed

#### Scenario: Missing stale files are no-op

- GIVEN no stale copies present
- WHEN upgrade apply runs
- THEN removal MUST succeed silently without error

### Requirement: Factory Test Mirror Updated

`biggz-pi-extensions-factory.test.mjs` MUST pin the deploy list's exact entry count (runtime pack added, retired wait shim removed) and still assert every listed entry exports a valid factory function.
(Previously: count guard fixed for the wrapper shrink only.)

#### Scenario: Count guard matches list

- GIVEN the updated deploy list (runtime pack entry in, wait shim out)
- WHEN the factory test runs `node --test`
- THEN the count assertion MUST pass for the new total

#### Scenario: Factory shape still valid

- GIVEN remaining listed extensions
- WHEN test imports each default export
- THEN each MUST be a callable factory function
