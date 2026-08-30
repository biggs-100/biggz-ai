# Delta for release-pipeline

## ADDED Requirements

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

## MODIFIED Requirements

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
