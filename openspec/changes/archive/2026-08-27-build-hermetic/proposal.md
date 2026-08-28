# Proposal: Build Hermético Multi-Arch

## Intent

Releases use `goreleaser`+`minisign` but CI only smokes host arch. Missing enforced 5-target `LEAF_TARGETS`, `CGO_ENABLED=0` guarantee for `modernc.org/sqlite`, and `release:checksums` smoke. Inspired by `oh-my-pi` hermeticity (Bazel `crate_universe` + `zig cc` glibc 2.17 + dual AVX2/baseline), make Go releases hermetic, reproducible, and verified every CI run.

## Scope

### In Scope
- `.goreleaser.yaml` 5 targets: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64` (ignore `windows/arm64`)
- `CGO_ENABLED=0` per build for `modernc.org/sqlite`
- `ldflags` `BuildVersion={{.Version}}` (`-s -w -X .../doctor.BuildVersion`)
- `minisign` `checksums.txt` (sha256) + `checksums.txt.minisig`
- CI `release:checksums` smoke: `goreleaser build --snapshot --clean` + verify checksums/minisig + `biggz --version`

### Out of Scope
- No Bazel, `crate_universe`, `zig cc`, glibc pinning
- No `internal/*` logic changes
- No `hashline`, `tui`, `extension-api` etc.
- No AVX2/baseline `.node` / `PI_NATIVE_VARIANT`

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `release-pipeline`: 5-target `LEAF_TARGETS`, `CGO_ENABLED=0`, `release:checksums` smoke

## Approach

- **LEAF_TARGETS**: `goos: [linux,darwin,windows]` + `goarch: [amd64,arm64]` + `ignore: [{goos: windows, goarch: arm64}]` (or 5 explicit builds); archive per target.
- **CGO matrix**: `env: [CGO_ENABLED=0]` every build; smoke asserts `go vet` + no cgo linkage.
- **Checksums**: `checksum: {name_template: checksums.txt}` + `signs: [{artifacts: checksum}]` via minisign; `minisign.pub` in repo root.
- **Smoke**: CI job `release-smoke` runs snapshot, checks 5 archives exist, `checksums.txt` + `minisign -Vm` passes, `BuildVersion != ""`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `.goreleaser.yaml` | Modified | 5-target matrix, CGO 0, ldflags, checksum/sign |
| `.github/workflows/release.yml` | Modified | `v*` trigger, goreleaser + minisign secret |
| `.github/workflows/ci.yml` | Modified | add `release:checksums` smoke job |
| `minisign.pub` | Referenced | pubkey unchanged |
| `internal/doctor` | Read-only | reads `BuildVersion` via ldflags |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `windows/arm64` expected | Low | Document 5-target; add later if needed |
| `CGO_ENABLED=0` hides cgo bug | Low | `go vet` + pure-Go sqlite path tested |
| minisign secret missing | Med | Smoke fails fast; doc `MINISIGN_PRIVATE_KEY` |
| Snapshot vs tag drift | Low | Same config; snapshot is dry-run |

## Rollback Plan

- Revert `.goreleaser.yaml` to prior commit; re-tag patch if published.
- Disable smoke job (`if: false`) or revert `ci.yml`.
- `checksums.txt.minisig` stays verifiable with old `minisign.pub`.

## Dependencies

- `goreleaser-action@v6`, Go stable, `MINISIGN_PRIVATE_KEY`
- `modernc.org/sqlite v1.45.0` pure-Go (no cgo toolchain)

## Success Criteria

- [ ] `goreleaser build --snapshot --clean` yields 5 archives
- [ ] All builds `CGO_ENABLED=0`; `go vet ./...` passes
- [ ] `biggz --version` / `doctor` shows `BuildVersion == {{.Version}}`
- [ ] `dist/checksums.txt` + `.minisig` present and `minisign -Vm` verifies
- [ ] CI `release:checksums` smoke passes on PR and `main`
