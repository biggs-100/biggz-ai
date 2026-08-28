# Design: Build Hermético Multi-Arch

## Technical Approach

Harden `goreleaser` pipeline (no Bazel). Add `ignore:[{goos:windows,goarch:arm64}]` for 5 LEAF_TARGETS, enforce `CGO_ENABLED=0` for `modernc.org/sqlite`, inject `doctor.BuildVersion` via ldflags, generate `checksums.txt` + `.minisig`, gate PRs with `release:checksums` smoke (`snapshot --clean` + verify). No `internal/*` changes.

## Architecture Decisions

### Decision: Build matrix — GoReleaser vs Bazel

| Option | Tradeoff | Decision |
|---|---|---|
| GoReleaser `goos:[linux,darwin,windows]` `goarch:[amd64,arm64]` `ignore:[windows/arm64]` | Single tool, minimal diff | **Chosen** |
| Bazel + `zig cc` glibc 2.17 | Full hermeticity, heavy migration | Rejected |
| 5 explicit builds | Verbose | Alt |

**Rationale**: Proposal excludes Bazel/zig; matrix+ignore gives 5 archives with one line.

### Decision: CGO — CGO_ENABLED=0 vs 1

| Option | Tradeoff | Decision |
|---|---|---|
| `env:[CGO_ENABLED=0]` | Pure-Go, static | **Chosen** |
| `CGO_ENABLED=1` + toolchain | Needs cross toolchain | Rejected |
| Inherit host | Non-hermetic | Rejected |

**Rationale**: `modernc.org/sqlite` pure-Go; smoke runs `go vet` without cgo.

### Decision: ldflags — BuildVersion

| Option | Tradeoff | Decision |
|---|---|---|
| `-s -w -X .../doctor.BuildVersion={{.Version}}` | Spec path, `doctor` reads it | **Chosen** |
| `main.version` | Needs adaptor | Rejected |
| Multi-var commit/date | Widens scope | Rejected |

**Rationale**: Spec mandates this path; snapshot non-empty, tag equals `{{.Version}}`.

### Decision: Signing — minisign vs sha256sum

| Option | Tradeoff | Decision |
|---|---|---|
| `checksum{sha256,checksums.txt}` + `signs:[{artifacts:checksum}]` + `minisign.pub` | Integrity + authenticity | **Chosen** |
| sha256 only | No authenticity | Rejected |
| cosign/sigstore | New infra | Rejected |

**Rationale**: Authenticity required; `minisign.pub` unchanged.

### Decision: CI — smoke vs full matrix

| Option | Tradeoff | Decision |
|---|---|---|
| Single `release-checksums` on ubuntu-latest | Fast, catches drift | **Chosen** |
| Full matrix 3 OS | 3× cost | Rejected |
| No smoke | Fails only on tag | Rejected |

**Rationale**: Go cross-compile makes one runner sufficient.

## Data Flow

```
PR/main → ci:release-checksums → goreleaser snapshot → 5 archives + checksums + .minisig → sha256sum -c + minisign -Vm + biggz --version → PASS/FAIL
tag v* → release.yml → goreleaser release → GitHub Release (5 + checksums + .minisig)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `.goreleaser.yaml` | Modify | Add `ignore:[{goos:windows,goarch:arm64}]`; keep CGO=0, ldflags, checksum, signs |
| `.github/workflows/ci.yml` | Modify | Add `release-checksums` job: `go vet`, snapshot, 5-arch assert, `sha256sum -c`, `minisign -Vm`, `biggz --version` |
| `.github/workflows/release.yml` | Modify | Keep `v*` + `goreleaser-action@v6 release --clean` + `MINISIGN_PRIVATE_KEY` |
| `go.mod` | Read-only | Confirm `modernc.org/sqlite v1.45.0` |
| `minisign.pub` | Referenced | Pinned at root; smoke/release use `-p minisign.pub` |

## Interfaces / Contracts

```yaml
builds:
  - env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore: [{goos: windows, goarch: arm64}]
    ldflags: [-s -w -X github.com/biggs-100/biggz-ai/internal/doctor.BuildVersion={{.Version}}]
checksum: {name_template: checksums.txt, algorithm: sha256}
signs: [{artifacts: checksum, args: [-Sm, "${artifact}", -x, -s, /tmp/minisign.key]}]
```

Smoke: `goreleaser build --snapshot --clean` → 5 archives → `sha256sum -c dist/checksums.txt` → `minisign -Vm dist/checksums.txt -p minisign.pub -x dist/checksums.txt.minisig` → `dist/**/biggz --version`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `BuildVersion` non-empty | `go test ./internal/doctor -run TestVersion` table-driven |
| Integration | 5 archives, checksum+minisig | Snapshot `dist/`; count; `sha256sum -c`; `minisign -Vm`; tamper→fail |
| E2E | `v*` filter, publish | Dry-run `v0.0.0-test` tag; `e2e` builds `biggz.exe` |

Runner: `go test ./... -count=1 -timeout 180s` + `go vet ./...`.

## Threat Matrix

Applies — release artifacts, signing, CI are trust boundaries.

| # | Threat | Applicable | Expected | RED test |
|---|--------|------------|----------|----------|
| 1 | Archive replaced | Yes | `sha256sum -c` fails | Modify byte → mismatch |
| 2 | `checksums.txt` tampered | Yes | `minisign -Vm` fails | Flip byte → fail |
| 3 | `.minisig` missing | Yes | Smoke FAIL | Delete → fail |
| 4 | Wrong key | Yes | Verify fails | Sign other key → fail |
| 5 | CGO injection | Yes | Audit fails | `CGO=1` → fail |
| 6 | Snapshot vs release drift | Yes | Config match | Diff templates |
| 7 | Smoke bypass | Yes | PR blocked | Remove smoke → required check fails |
| 8 | Tag downgrade | N/A | GitHub `v*` controlled | — |
| 9 | Shell injection | N/A | Static args | — |

## Migration / Rollout

No migration. Single PR ~80 lines. Rollback: revert `ignore`, disable smoke `if: false`.

## Open Questions

- [ ] None — `windows/arm64` excluded intentionally.
