# Delta for pi-integration

## ADDED Requirements

### Requirement: Native-Only Pi Memory Path

The system MUST serve `biggz_mem_*` via native `/mcp` (`pi-mcp-adapter@^2`) with no wrapper fallback. Provisioning and annotations requirements stay unchanged.

#### Scenario: Fresh install serves tools without wrappers

- GIVEN fresh install with both wrappers excluded
- WHEN operator runs `pi list` or `/mcp`
- THEN native `biggz_mem_*` (at least `biggz_mem_save`) MUST appear

#### Scenario: Doctor gate passes natively

- GIVEN provisioned `settings.json` + `mcp.json`
- WHEN `biggz doctor` runs `pi-mcp-adapter` check
- THEN result MUST be PASS (dir present, version `2.x`, `bigmem` valid, `biggz-mcp --help` exit 0)

#### Scenario: Native tool handle truthy in harness

- GIVEN Pi harness with native adapter loaded
- WHEN extension calls `pi.getTool("biggz_mem_save")`
- THEN result MUST be truthy and wrappers MUST stay no-op/absent

#### Scenario: Session-guard factory intact

- GIVEN wrappers removed from deploy list
- WHEN Pi starts and loads `biggz-session-guard.js` factory
- THEN Pi MUST start without `Extension does not export a valid factory function`

### Requirement: Phase-2 Stability Gate

The system MUST delete wrapper sources only when ALL criteria pass after one-release soak with zero fallback-firing reports.

| # | Criterion |
|---|-----------|
| 1 | `doctor pi-mcp-adapter` PASS |
| 2 | `pi.getTool("biggz_mem_save")` truthy, `pi list` shows `biggz_mem_*` |
| 3 | `settings.json`+`mcp.json` carry command/args/type/imports/directTools |
| 4 | `go vet` + `go test` + `node --check` green |
| 5 | One-release soak, zero fallback reports |

#### Scenario: All criteria pass allows Phase 2

- GIVEN all five criteria verified after soak
- WHEN release manager approves source deletion
- THEN deletion of both JS sources MAY proceed

#### Scenario: Any criterion fails blocks Phase 2

- GIVEN any single criterion failing or soak incomplete
- WHEN Phase 2 deletion evaluated
- THEN sources MUST be retained and deploy-list exclusion stays

### Requirement: Single-Commit Rollback

The system MUST support single-commit revert restoring wrappers, plus reinstall and remedy paths.

#### Scenario: Revert restores wrappers

- GIVEN Phase 1 commit reverted
- WHEN `biggz install --agent pi` re-runs
- THEN both wrappers MUST redeploy and `pi list` MUST show them

#### Scenario: Remedy re-provisions native path

- GIVEN native adapter missing or stale
- WHEN operator runs remedy `pi install npm:pi-mcp-adapter@^2`
- THEN `doctor pi-mcp-adapter` MUST return to PASS

## REMOVED Requirements

### Requirement: Adapter-Aware Wrapper Fallback

(Reason: wrappers are permanent no-ops when native tool present; 67KB dead fallback plus duplicate gate no longer justified.)
(Migration: native `/mcp` via `pi-mcp-adapter@^2` replaces fallback; manual copy of kept sources during soak only; Go `synthesis_gate.go` stays canonical.)
