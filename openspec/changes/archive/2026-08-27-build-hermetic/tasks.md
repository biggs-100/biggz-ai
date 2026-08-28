# Tasks: Build Hermético Multi-Arch

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 80–120 |
| 400-line budget risk | Low |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Harden `.goreleaser.yaml` (ignore windows/arm64, CGO 0, ldflags, checksum/sign) | PR 1 commit 1 | `goreleaser check && go vet ./...` | `goreleaser build --snapshot --clean && ls dist/* | wc -l` | Revert `.goreleaser.yaml` |
| 2 | Add `release-checksums` smoke to `.github/workflows/ci.yml` | PR 1 commit 2 | `go vet ./... && go test ./... -count=1 -timeout 180s` | `goreleaser build --snapshot --clean && sha256sum -c dist/checksums.txt && minisign -Vm` | Revert `ci.yml` |

## Phase 1: Foundation & Audit

- [x] 1.1 Audit `.goreleaser.yaml` (`yq .builds`): `CGO_ENABLED=0`, ldflags `doctor.BuildVersion`, checksum/sign — Done: gap `ignore` found
- [x] 1.2 Add `ignore: [{goos: windows, goarch: arm64}]` to `.goreleaser.yaml` — Done: `yq` shows ignore; snapshot = 5 archives
- [x] 1.3 Verify `internal/doctor/version.go` and `minisign.pub` — Done: `go vet ./...` ok

## Phase 2: Core Implementation

- [x] 2.1 Validate matrix `goos[linux,darwin,windows]` + `goarch[amd64,arm64]` + ignore → 5 targets — Done: snapshot 5, no windows/arm64
- [x] 2.2 Add `release-checksums` job to `.github/workflows/ci.yml` (PR+main, needs format): goreleaser+minisign, `go vet`, snapshot, 5-archive assert, `sha256sum -c`, `minisign -Vm`, `biggz --version` — Done: `rg release-checksums ci.yml` hits
- [x] 2.3 Confirm `.github/workflows/release.yml` has `tags ['v*']`, `goreleaser-action@v6`, `MINISIGN_PRIVATE_KEY` — Done: rg hits

## Phase 3: Testing & Threat Verification (RED)

- [x] 3.1 RED Threat 1 archive replaced: mutate byte in `dist/*.tar.gz`, `sha256sum -c` — Done: fails
- [x] 3.2 RED Threat 2 checksums tampered: flip byte in `dist/checksums.txt`, `minisign -Vm` — Done: fails
- [x] 3.3 RED Threat 3 missing sig: delete `dist/checksums.txt.minisig`, `minisign -Vm` — Done: FAIL
- [x] 3.4 RED Threat 4 wrong key: sign temp key `minisign -G`, verify with `minisign.pub` — Done: fails
- [x] 3.5 RED Threat 5 CGO injection: build `CGO_ENABLED=1` → env check fails — Done: smoke detects
- [x] 3.6 RED Threat 6 drift: `goreleaser check` snapshot vs release diff — Done: none
- [x] 3.7 RED Threat 7 bypass: delete smoke job → required check blocks PR — Done: verified
- [x] 3.8 Green specs: `CGO_ENABLED=0`, `go vet`, `BuildVersion` non-empty, `sha256sum -c` + `minisign -Vm` — Done: `go test ./... -count=1 -timeout 180s` green

## Phase 4: Integration & Final Gates

- [x] 4.1 Full harness: `go vet ./...` + `go test ./... -count=1 -timeout 180s` + snapshot → 5 archives + checksums/minisig + `--version` — Done: all green
- [x] 4.2 Cleanup: no `internal/*` changes, update checkboxes — Done: `git diff --stat` only `.goreleaser.yaml` + `ci.yml`

Dependencies: 1.1→1.2→2.1→2.2→3.x→4.1; Phase 3 needs `dist/`.
