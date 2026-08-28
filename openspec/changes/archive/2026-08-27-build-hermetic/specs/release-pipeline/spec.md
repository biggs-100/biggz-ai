# Delta for release-pipeline

## MODIFIED Requirements

### Requirement: Build Matrix

The pipeline MUST build exactly 5 static binaries with `CGO_ENABLED=0` for `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64` via LEAF_TARGETS (`goos: [linux,darwin,windows]`, `goarch: [amd64,arm64]`, `ignore: [{goos: windows, goarch: arm64}]`) or 5 explicit builds. Each target MUST produce one archive with a statically-linked binary. `windows/arm64` MUST be excluded.
(Previously: 6 targets including windows/arm64 without explicit LEAF_TARGETS ignore)

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
(Previously: generic SHA-256 + minisign without explicit template/algorithm/verification command)

#### Scenario: Signed release bundle

- GIVEN a release triggered by a `v*` tag
- WHEN goreleaser finishes
- THEN `checksums.txt` MUST be present in the release
- AND `checksums.txt.minisig` MUST verify with `minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig`

#### Scenario: Tampered checksum fails verification

- GIVEN a released `checksums.txt` with one byte modified
- WHEN `minisign -Vm` runs against the tampered file
- THEN verification MUST fail

### Requirement: Version Ldflags

The build MUST inject version via `ldflags: -s -w -X github.com/biggs-100/biggz-ai/internal/doctor.BuildVersion={{.Version}}`. `internal/doctor.BuildVersion` MUST equal `{{.Version}}` and MUST be non-empty for tagged builds.
(Previously: vague ldflags into main.version without exact package path or flags)

#### Scenario: Doctor displays build version

- GIVEN a binary built by goreleaser with tag `v1.2.3`
- WHEN `biggz doctor` runs
- THEN `BuildVersion` MUST equal `v1.2.3`
- AND `biggz --version` MUST be non-empty

#### Scenario: Snapshot build has version

- GIVEN `goreleaser build --snapshot --clean`
- WHEN binary is executed with `--version`
- THEN `BuildVersion` MUST be non-empty

## ADDED Requirements

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

CI MUST provide a `release:checksums` smoke job running on PR and `main` that executes `goreleaser build --snapshot --clean`, then verifies exactly 5 archives exist, `sha256sum -c checksums.txt` passes, `minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig` passes, and `biggz --version` shows `BuildVersion != ""`.

#### Scenario: Smoke verifies hermetic snapshot

- GIVEN a PR or push to `main`
- WHEN smoke runs `goreleaser build --snapshot --clean`
- THEN 5 archives MUST exist in `dist/`
- AND `sha256sum -c dist/checksums.txt` MUST pass
- AND `minisign -Vm dist/checksums.txt -p minisign.pub -x dist/checksums.txt.minisig` MUST pass
- AND `dist/*biggz* --version` MUST output non-empty BuildVersion

#### Scenario: Smoke fails on missing signature

- GIVEN `dist/checksums.txt.minisig` is missing
- WHEN smoke attempts `minisign -Vm`
- THEN the job MUST fail
