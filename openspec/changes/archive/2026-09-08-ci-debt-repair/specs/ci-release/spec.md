# ci-release Specification

## Purpose

Deterministic release smoke: pinned, reproducible, signed.

## Requirements

### Requirement: Pinned Reproducible Release Smoke

The goreleaser action MUST be version-pinned; the smoke MUST repro via local snapshot, verifying checksums and minisign signing incl. key paths.

#### Scenario: Snapshot repro passes

- GIVEN the pinned action and snapshot config
- WHEN the smoke runs locally and in CI
- THEN both MUST verify checksums and signatures

#### Scenario: Floating ref rejected

- GIVEN a `latest`/floating action ref
- WHEN reviewed
- THEN it MUST be rejected

#### Scenario: Missing signing artifacts fail

- GIVEN absent key-path or minisign output
- WHEN the smoke runs
- THEN it MUST fail naming the missing artifact
