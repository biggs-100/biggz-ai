# Delta for installer-pipeline

## ADDED Requirements

### Requirement: Pi Deploy Ordering with ProvisionBigMemMCP

`internal/install:Run` MUST order `DeployMCPBinaryToHomeDir` → `ProvisionBigMemMCP` (both `settings.json` + `mcp.json` via `WriteFileAtomic`) → `pi install`. All writes MUST be atomic temp+rename, idempotent, rollback reverse-order, `--dry-run` zero writes, `verifyOrchestratorDeployment` confirms presence.

#### Scenario: Ordered success
- GIVEN fresh pi install
- WHEN `Run` executes
- THEN binary MUST precede `ProvisionBigMemMCP` and `pi install` MUST run last with `Success==true`

#### Scenario: Dry-run zero writes
- GIVEN `--dry-run`
- WHEN `Orchestrator.Run` executes
- THEN only `Prepare` runs and zero files outside TempDir MUST be written

#### Scenario: Rollback atomic
- GIVEN `ProvisionBigMemMCP` succeeded then `pi install` fails
- WHEN rollback triggers
- THEN completed steps MUST roll back reverse-order and no partial files remain
