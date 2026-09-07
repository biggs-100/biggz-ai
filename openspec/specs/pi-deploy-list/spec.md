# Pi Deploy List Specification

## Purpose

The new guard module MUST ship with pi: `internal/install/steps/pi_extensions.go`
MUST list it for deploy so the static ESM imports resolve in the deployed
`~/.pi/agent/extensions/` directory.

## Requirements

### Requirement: Guard Registered for Deploy

The system MUST include the entry `{"pi/biggz-session-guard.js",
"biggz-session-guard.js"}` in the pi extensions deploy list in
`internal/install/steps/pi_extensions.go`, alongside the existing
`biggz-tool-interception.js` and `biggz-extension-api.js` entries.

#### Scenario: Deploy list contains the guard

- GIVEN the deploy list in `pi_extensions.go`
- WHEN inspecting its entries
- THEN one entry MUST map asset `pi/biggz-session-guard.js` to target `biggz-session-guard.js`

#### Scenario: Build passes and deployed imports resolve

- GIVEN the updated deploy list
- WHEN running `go build ./...` and deploying extensions
- THEN the build MUST pass and the deployed directory MUST contain `biggz-session-guard.js`
  next to both importing extensions
