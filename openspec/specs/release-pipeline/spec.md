# Release Pipeline Specification

## Purpose

The release pipeline builds, signs, and publishes platform-specific biggz-ai binaries via GoReleaser and GitHub Actions, and exposes the update engine contract for on-demand binary updates.

## Requirements

### Requirement: Build Matrix

The pipeline MUST build exactly 5 static binaries with `CGO_ENABLED=0` for `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64` via LEAF_TARGETS (`goos: [linux,darwin,windows]`, `goarch: [amd64,arm64]`, `ignore: [{goos: windows, goarch: arm64}]`) or 5 explicit builds. Each target MUST produce one archive with a statically-linked binary. `windows/arm64` MUST be excluded.

#### Scenario: Snapshot produces 5 archives

- GIVEN `goreleaser build --snapshot --clean` with no git tag
- WHEN the build completes
- THEN exactly 5 archives MUST exist, one per LEAF_TARGETS entry
- AND each archive MUST contain a statically-linked binary

#### Scenario: Windows ARM64 excluded

- GIVEN the build matrix configuration
- WHEN enumerating targets
- THEN `windows/arm64` MUST NOT appear
- AND only the 5 listed targets MUST be present

### Requirement: Checksum Signing

The pipeline MUST generate `checksums.txt` (SHA-256, `checksum.name_template: checksums.txt`, `algorithm: sha256`) and sign it via minisign as `checksums.txt.minisig` (`signs: [{artifacts: checksum}]`). The public key `minisign.pub` MUST remain at repository root and MUST verify via `minisign -Vm`.

#### Scenario: Signed release bundle

- GIVEN a release triggered by a `v*` tag
- WHEN goreleaser finishes
- THEN `checksums.txt` MUST be present in the release
- AND `checksums.txt.minisig` MUST verify with `minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig`

#### Scenario: Tampered checksum fails verification

- GIVEN a released `checksums.txt` with one byte modified
- WHEN `minisign -Vm` runs against the tampered file
- THEN verification MUST fail

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

The build MUST inject version via `ldflags: -s -w -X github.com/biggs-100/biggz-ai/internal/doctor.BuildVersion={{.Version}}`. `internal/doctor.BuildVersion` MUST equal `{{.Version}}` and MUST be non-empty for tagged builds.

#### Scenario: Doctor displays build version

- GIVEN a binary built by goreleaser with tag `v1.2.3`
- WHEN `biggz doctor` runs
- THEN `BuildVersion` MUST equal `v1.2.3`
- AND `biggz --version` MUST be non-empty

#### Scenario: Snapshot build has version

- GIVEN `goreleaser build --snapshot --clean`
- WHEN binary is executed with `--version`
- THEN `BuildVersion` MUST be non-empty

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

### Requirement: Hermetic CGO Enforcement

The pipeline MUST set `env: [CGO_ENABLED=0]` on every build to guarantee pure-Go `modernc.org/sqlite` without cgo toolchain. Smoke MUST assert no cgo linkage and `go vet` passes.

#### Scenario: Builds enforce CGO_ENABLED=0

- GIVEN any build in `.goreleaser.yaml`
- WHEN inspecting build env
- THEN `CGO_ENABLED` MUST equal `0`

#### Scenario: Vet passes without cgo

- GIVEN `CGO_ENABLED=0`
- WHEN `go vet ./...` runs in smoke
- THEN it MUST pass

### Requirement: Release Checksums Smoke

CI MUST provide `release:checksums` smoke job on PR/main that runs `goreleaser build --snapshot --clean`, then verifies exactly 5 archives exist, `sha256sum -c checksums.txt` passes, `minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig` passes, `sha256(readBinary)==integrity.json` via `VerifyBinary`, and `biggz --version` shows `BuildVersion != ""`.
(Previously: smoke verified archives+checksums+minisig+version but not integrity.json pin)

#### Scenario: Smoke verifies integrity pin
- GIVEN `dist/checksums.txt` and `dist/integrity.json` present after snapshot
- WHEN smoke runs `VerifyBinary` per archive
- THEN `sha256(binary)==integrity.json.binarySha256` MUST pass for all 5 targets

#### Scenario: Smoke fails on missing integrity.json
- GIVEN `integrity.json` missing from `dist/`
- WHEN smoke verifies archives
- THEN job MUST fail

#### Scenario: Smoke verifies hermetic snapshot — unchanged
- GIVEN PR push to main
- WHEN smoke runs `goreleaser build --snapshot --clean`
- THEN 5 archives MUST exist and `sha256sum -c dist/checksums.txt` and `minisign -Vm` MUST pass

### Requirement: Canonical Verify with Integrity Manifest

The system MUST provide `internal/install/verify.go` `VerifyBinary` that computes `sha256(readBinary)` and validates against `integrity.json` via `expectedRuntimeManifest`/`signedReleaseManifest` (port `lib/gentle-ai-binary.ts`). It MUST enforce `isConfined` (binary inside `versionDirectory`/`allowedRoots`), `isSymlink` (lstat non-symlink for binary, manifest, and directories), and `sameFile` (dev+ino+size+mtimeMs before/after). Manifest MUST be canonical JSON `JSON.stringify(expected)+"\n"` with exact key count and value equality; mismatch → `PackageLocalGentleAiBinaryMissingError`. Dev-binary override (`BIGGZ_DEV_BINARY`/`dev-binary.json`) MUST bypass pin but still validate absolute non-symlink executable and recompute digest.

#### Scenario: Valid binary verifies
- GIVEN binary bytes hash equals `integrity.json` `binarySha256` and `expectedRuntimeManifest` matches canonical JSON
- WHEN `VerifyBinary` runs
- THEN it MUST return binary path with no error

#### Scenario: Tampered binary fails
- GIVEN one byte in binary changed so `sha256 != integrity.json`
- WHEN `VerifyBinary` runs
- THEN it MUST fail with `PackageLocalGentleAiBinaryMissingError`

#### Scenario: Symlink rejected
- GIVEN binary path is symlink
- WHEN `isSymlink` check runs
- THEN it MUST return error and verification MUST fail without fallback

#### Scenario: Unconfined path rejected
- GIVEN binary path outside `versionDirectory`
- WHEN `isConfined` evaluates `relative(directory,path)`
- THEN it MUST return false and verification MUST fail

#### Scenario: SameFile detects replacement
- GIVEN `lstat before != after` (dev/ino/size/mtimeMs changed) during verify
- WHEN `sameFile` compares
- THEN it MUST return false and verification MUST fail with replacement error

#### Scenario: Non-canonical manifest rejected
- GIVEN `integrity.json` has extra key or whitespace differs from `JSON.stringify(expected)+"\n"`
- WHEN `isCanonicalManifest` runs
- THEN it MUST return false and verification MUST fail

### Requirement: Release Integrity Manifest Publishing

The pipeline MUST publish `integrity.json` per build via `.goreleaser.yaml` `archives.files` including `integrity.json` and `checksum` `algorithm: sha256` `name_template: checksums.txt`. `signs` MUST sign `checksums.txt` to `checksums.txt.minisig` and `minisign.pub` MUST remain at root. Each archive MUST contain statically-linked binary and `integrity.json` with `version`, `asset`, `assetSha256`, `binarySha256`.

#### Scenario: Goreleaser includes integrity.json
- GIVEN `.goreleaser.yaml` `archives.files` inspected
- WHEN enumerating
- THEN `integrity.json` MUST be present alongside `README.md`/`LICENSE`/`minisign.pub`

#### Scenario: Snapshot contains integrity manifest
- GIVEN `goreleaser build --snapshot --clean`
- WHEN build completes
- THEN `dist/*.tar.gz` MUST extract `integrity.json` with valid `binarySha256` matching `sha256` of binary
