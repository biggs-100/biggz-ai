# Release Pipeline Specification

## Purpose

The release pipeline builds, signs, and publishes platform-specific biggz-ai binaries via GoReleaser and GitHub Actions, and exposes the update engine contract for on-demand binary updates.

## Requirements

### Requirement: Build Matrix

The pipeline MUST build static binaries (CGO_ENABLED=0) for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, and windows/arm64.

#### Scenario: Snapshot produces all archives

- GIVEN `goreleaser build --snapshot` with no git tag
- WHEN the build completes
- THEN one archive MUST exist per platform/arch combination
- AND each archive MUST contain a statically-linked binary

### Requirement: Checksum Signing

The pipeline MUST generate a SHA-256 checksums.txt and sign it with minisign. The public key MUST be committed to the repository root.

#### Scenario: Signed release bundle

- GIVEN a release triggered by a v* tag
- WHEN goreleaser finishes
- THEN checksums.txt MUST be present in the release
- AND checksums.txt.minisig MUST verify against the committed public key

### Requirement: CI/CD Workflow

A GitHub Actions workflow MUST trigger on v* tag pushes, run goreleaser, and publish artifacts to GitHub Releases.

#### Scenario: Tag push publishes release

- GIVEN a maintainer pushes a v1.2.3 tag
- WHEN the workflow runs
- THEN goreleaser MUST build, sign, and publish
- AND the GitHub Release MUST contain archives, checksums.txt, and checksums.txt.minisig

#### Scenario: Non-version tag skipped

- GIVEN a non-version tag push (e.g., docs-v2)
- WHEN the workflow evaluates the tag
- THEN the workflow MUST NOT run

### Requirement: Version Ldflags

The build MUST inject version, commit SHA, and timestamp via ldflags into `main.version`. The `doctor.BuildVersion` field MUST reflect this injected value.

#### Scenario: Doctor displays build version

- GIVEN a binary built by goreleaser
- WHEN `biggz doctor` runs
- THEN BuildVersion MUST match the git tag
- AND the version MUST be non-empty

### Requirement: Channel Selection

The update engine MUST read `BIGGZ_CHANNEL`. When unset or set to `stable`, it MUST select the latest stable (non-prerelease) GitHub Release. When set to `beta`, it MUST include pre-releases.

#### Scenario: Default stable channel

- GIVEN BIGGZ_CHANNEL is unset
- WHEN the engine fetches latest release
- THEN it MUST select the latest stable release

#### Scenario: Beta channel selects pre-releases

- GIVEN BIGGZ_CHANNEL=beta
- WHEN the engine fetches latest release
- THEN it MUST include pre-releases and select the most recent
