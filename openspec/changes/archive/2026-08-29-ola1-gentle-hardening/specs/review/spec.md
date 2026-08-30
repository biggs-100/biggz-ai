# Delta for review

## ADDED Requirements

### Requirement: Provider Contract Offline SHA256 Pin Verification

The system MUST provide `scripts/check-provider-contract.mjs` and `internal/contracts/verify.go` `VerifyProviderContract(lockPath, root) error` that offline-verify `contracts/review-integration/v1/**` and `v2/**` files against `contracts/review-integration/provider-contract.lock.json` via SHA256 hex. `1`-byte drift, unlisted, or missing file MUST cause drift error (`drift <path>` or `unlisted <path>`) and exit `1`; exact pins MUST pass with `check passed N files`. No network fetch (`no fetch`), and manifest covers 44 files (42 v1 + 2 v2). `v2` schema/fixture are ported from `v1` with `v2` identifiers and MUST validate.

#### Scenario: Exact pins pass offline with no fetch

- GIVEN `provider-contract.lock.json` matches SHA256 of every file under `v1` and `v2`
- WHEN `node scripts/check-provider-contract.mjs` or `VerifyProviderContract` runs offline (no network)
- THEN it MUST exit `0` / return nil and report `check passed 44 files`

#### Scenario: One-byte drift fails

- GIVEN one byte is appended to `contracts/review-integration/v1/fixtures/contract.fixture.json`
- WHEN verification runs
- THEN it MUST exit `1` / return `drift <path>` error and print `offline only`

#### Scenario: Unlisted file fails

- GIVEN a new file is added under `contracts/review-integration/v1/` without updating the lock
- WHEN verification runs
- THEN it MUST report `unlisted <path>` and fail

### Requirement: Package Manifest Offline Verification

The system MUST provide `scripts/verify-package-files.mjs` that offline-verifies the sorted relative walk of `contracts/review-integration/v1/**` + `v2/**` against the lock keys (`provider-contract.lock.json`) without reading file contents beyond existence. Unlisted walked file or missing listed key MUST cause `unlisted`/`missing` error and exit `1`; exact match MUST pass `verify passed N files`.

#### Scenario: Exact manifest passes

- GIVEN walked files exactly match lock keys (44 files)
- WHEN `node scripts/verify-package-files.mjs` runs
- THEN it MUST exit `0` with `verify passed 44 files`

#### Scenario: Unlisted file in manifest check fails

- GIVEN a walked file is not in the lock set
- WHEN the script runs
- THEN it MUST report `unlisted <rel>` and exit `1`

#### Scenario: Missing listed key fails

- GIVEN a lock key has no corresponding file on disk
- WHEN the script runs
- THEN it MUST report `missing <rel>` and exit `1`

### Requirement: CI Skill-Lint and Provider-Contract Jobs

The system MUST modify `.github/workflows/ci.yml` to add jobs `skill-lint` (Node 20, `node scripts/check-skill-lint.mjs`) and `provider-contract` (Node 20 + Go stable, `node scripts/check-provider-contract.mjs` and `node scripts/verify-package-files.mjs`) that run after `format` (`needs: format`).

#### Scenario: CI contains skill-lint job after format

- GIVEN `.github/workflows/ci.yml` is parsed
- WHEN inspecting jobs
- THEN `skill-lint` MUST exist with `needs: format` and `run: node scripts/check-skill-lint.mjs`

#### Scenario: CI contains provider-contract job with both checks

- GIVEN the workflow file
- WHEN inspecting `provider-contract` job
- THEN it MUST run both `check-provider-contract.mjs` and `verify-package-files.mjs` with `needs: format`
