# Delta for system-diagnostics

## ADDED Requirements

### Requirement: SDD Asset Drift Read-Only Checks

The system MUST add `biggz doctor` RO checks `sddGlobalAssetDriftCount` and `sddLocalAgentOverrideCount` computed via `assets/managed.go:ManagedAssetHash` SHA256 against `managed-assets.json` v1, report `warn: Global SDD asset drift N` when `N>0` with status `warn` (not `fail`), expose no `--fix`, and keep Runner panic-isolated via `diagnostics/doctor.go`.

#### Scenario: Global drift warn
- GIVEN one global `sdd-*.md` hash differs from manifest
- WHEN `biggz doctor` runs
- THEN `sddGlobalAssetDriftCount` MUST be `1` and result MUST be `warn` with message containing `Global SDD asset drift 1`

#### Scenario: Local override warn
- GIVEN local agent override hash differs
- WHEN doctor runs
- THEN `sddLocalAgentOverrideCount` MUST reflect count and status `warn`

#### Scenario: No drift pass and no fix
- GIVEN all hashes match
- WHEN `biggz doctor` and `biggz doctor --json` run
- THEN both counts MUST be `0` with `pass`; CLI MUST NOT accept `--fix`

#### Scenario: Panic isolation
- GIVEN drift check panics
- WHEN `Runner.Run()` completes
- THEN drift result MUST be `warn`/`fail` with panic message and other checks unaffected
