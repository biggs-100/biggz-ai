# Delta for managed-assets

## ADDED Requirements

### Requirement: ManagedAssetHash Exposure for Doctor Drift

The system MUST expose `ManagedAssetHash` in `internal/assets/managed.go` computing SHA256 for `managed-assets.json` v1 entries, enabling `biggz doctor` RO drift comparison. It MUST preserve existing hash-skip (`--force` overwrites), retirement removal, and global `sdd-*.md` install semantics.

#### Scenario: Hash exposed for drift
- GIVEN `managed-assets.json` v1 lists `sdd-*.md` with SHA256
- WHEN `ManagedAssetHash` is called for a path
- THEN it MUST return `sha256:<hex>` matching file content

#### Scenario: Doctor consumes hash read-only
- GIVEN `ManagedAssetHash` returns drift vs file on disk
- WHEN `sddGlobalAssetDriftCount` compares
- THEN mismatch MUST increment count without writing

#### Scenario: Existing skip/force/retire preserved
- GIVEN source/dest hashes match
- WHEN `copyDirectoryFiles` runs without/with `--force`
- THEN without MUST skip, with MUST overwrite; retired MUST be deleted
