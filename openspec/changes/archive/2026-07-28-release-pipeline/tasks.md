# Tasks: Release Pipeline

## Review Workload Forecast

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

| Field | Value |
|-------|-------|
| Estimated changed lines | ~680 |
| Suggested split | PR 1: Infra → PR 2: Engine → PR 3: CLI + Tests |

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Release infra: goreleaser + workflow + signing | PR 1 | `goreleaser build --snapshot` | Run goreleaser locally | Revert `.goreleaser.yaml`, `release.yml`, `scripts/`, `minisign.pub` |
| 2 | Update engine: client, verify, download, replace | PR 2 | `go test ./internal/update/...` | N/A (test-only, no CLI yet) | Revert `internal/update/` dir + `go.mod` |
| 3 | CLI update cmd + all tests | PR 3 | `go test ./...` | `go build && ./biggz update --help` | Revert `cmd/biggz/main.go` |

## Phase 1: Release Infrastructure

- [x] 1.1 Create `.goreleaser.yaml` — 6-target matrix, ldflags, minisign signing, archive per platform
- [x] 1.2 Create `.github/workflows/release.yml` — trigger on `v*`, goreleaser-action, publish to Releases
- [x] 1.3 Create `scripts/preflight.sh` — local minisign keygen + signing preflight
- [x] 1.4 Generate `minisign.pub` and commit to repo root
- [x] 1.5 Smoke: `goreleaser build --snapshot` produces signed archives for all platforms

## Phase 2: Update Engine

- [x] 2.1 Create `internal/update/channel.go` — `Channel` type + `ParseChannel()`
- [x] 2.2 Create `internal/update/client.go` — `Release`/`Asset` types, list/get release via `net/http`
- [x] 2.3 Create `internal/update/verify.go` — `VerifyChecksum()` + `VerifySignature()` via go-minisign
- [x] 2.4 Create `internal/update/download.go` — `DownloadAndExtract()` for tar.gz + zip via stdlib
- [x] 2.5 Create `internal/update/replace.go` — `ReplaceBinary()` Unix os.Rename, `ReplaceHint()` Windows
- [x] 2.6 Create `internal/update/embed.go` — `//go:embed minisign.pub`
- [x] 2.7 Add `github.com/jedisct1/go-minisign` to `go.mod`

## Phase 3: CLI Wiring + Ldflags

- [x] 3.1 Add `update` case to switch in `cmd/biggz/main.go` + `updateRun()` wiring channel → download → verify → replace
- [x] 3.2 Confirm `doctor.BuildVersion` var exists (ldflags target; no code change needed)

## Phase 4: Tests

- [x] 4.1 Unit: `channel.go` — table-driven parse/select stable vs beta scenarios
- [x] 4.2 Unit: `verify.go` — checksum match/fail, signature valid/invalid, wrong key rejection
- [x] 4.3 Unit: `download.go` — archive extraction from in-memory tar.gz/zip buffers
- [x] 4.4 Unit: `replace.go` — temp dir atomic rename simulation
- [x] 4.5 CI smoke: verify `goreleaser build --snapshot` output archives pass checksum + verify
