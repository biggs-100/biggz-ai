# Proposal: release-pipeline

## Intent

biggz-ai has no release infrastructure: no binaries to download, no version injection, no signing, no CI/CD. Users must build from source. This change implements a complete release pipeline so users can install, update, and verify biggz binaries with trust guarantees.

## Scope

### In Scope
- GoReleaser config for linux/darwin/windows amd64+arm64 builds
- Minisign checksum signing + verification
- GitHub Actions release workflow
- `main.version` ldflags injection + `doctor.BuildVersion` wiring
- `internal/update/` package: GitHub release discovery, checksum + signature verify, binary download
- `biggz update` command with atomic binary replacement (Unix) / `go install` fallback (Windows)
- Stable + beta channels via `BIGGZ_CHANNEL=beta` env var
- Minisign key generation documented in repo

### Out of Scope
- Startup update check with prompt (advisory only, no blocking prompt)
- Homebrew tap publishing
- Automatic rollback on failed update (manual recovery only)
- Multi-tool update registry (bigz only has one binary)

## Capabilities

### New Capabilities
- `release-pipeline`: GoReleaser build matrix, minisign signing, CI/CD workflow, binary artifact distribution. Covers the entire release lifecycle from tag to published release.

### Modified Capabilities
- `cli`: Add `update` subcommand — on-demand binary update with channel awareness, platform dispatch (signed binary on Unix, `go install` on Windows), and minisign signature verification.

## Approach

Phased MVP building on the existing `internal/release/release.go` (git tag/verify) and `internal/doctor/version.go` (BuildVersion ldflags hook):

1. **Release infra**: `.goreleaser.yaml` with CGO_ENABLED=0, ldflags, minisign signing. GitHub Actions workflow on tag push.
2. **Update engine**: `internal/update/` — GitHub releases API call, checksums.txt download, minisign verify, archive extract, atomic `os.Rename` (Unix) / `go install` hint (Windows).
3. **CLI wiring**: `biggz update` command in `cmd/biggz/main.go`, channel env var, advisory version info.

Reference architecture from gentle-ai's 4-layer update system, trimmed to single-binary needs.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `.goreleaser.yaml` | New | GoReleaser build config |
| `.github/workflows/release.yml` | New | CI/CD release workflow |
| `cmd/biggz/main.go` | Modified | Add `update` subcommand + version ldflags |
| `internal/update/` | New | Update engine (check, download, verify, replace) |
| `internal/release/release.go` | Modified | May expose version/tag helpers for update |
| `internal/doctor/version.go` | Modified | Wire BuildVersion from ldflags |
| `go.mod` | Modified | Add `go-minisign` dependency |
| `scripts/` | New | Signing preflight script |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Windows binary lock (can't overwrite running .exe) | High | `go install` fallback; no `MoveFileEx` complexity |
| No existing minisign key pair | High | Generate during implementation; document in repo |
| No CI/CD infra from scratch | Medium | Single workflow file; reuse gentle-ai pattern |
| go-minisign supply chain risk | Low | Pin in go.mod; vet dep |

## Rollback Plan

Revert the `.goreleaser.yaml`, workflow, `internal/update/`, and CLI changes. Existing `release` subcommand is unaffected. Users on a broken version run `go install github.com/biggz-ai/biggz@previous-tag`.

## Dependencies

- `github.com/jedisct1/go-minisign` for signature verification
- GoReleaser (GitHub Actions runner auto-installs via goreleaser-action)
- Minisign CLI for local signing preflight

## Success Criteria

- [ ] `goreleaser build --snapshot` produces signed archives for all platforms
- [ ] `biggz update` downloads, verifies, and replaces the binary on Linux/macOS
- [ ] `biggz update` suggests `go install` on Windows
- [ ] `BIGGZ_CHANNEL=beta biggz update` picks beta releases
- [ ] Release workflow triggers on `v*` tag, publishes to GitHub Releases
