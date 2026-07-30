# Apply Progress: Release Pipeline — PR 2 (Update Engine)

## Summary

Implemented the complete update engine package (`internal/update/`) with 7 files covering channel selection, GitHub API client, checksum and signature verification, archive download and extraction, binary replacement, and public key embedding. Added `github.com/jedisct1/go-minisign` as a direct dependency.

## Completed Tasks

- [x] 2.1 Create `internal/update/channel.go` — `Channel` type + `ParseChannel()`
- [x] 2.2 Create `internal/update/client.go` — `Release`/`Asset` types, list/get release via `net/http`
- [x] 2.3 Create `internal/update/verify.go` — `VerifyChecksum()` + `VerifySignature()` via go-minisign
- [x] 2.4 Create `internal/update/download.go` — `DownloadAndExtract()` for tar.gz + zip via stdlib
- [x] 2.5 Create `internal/update/replace.go` — `ReplaceBinary()` Unix os.Rename, `ReplaceHint()` Windows
- [x] 2.6 Create `internal/update/embed.go` — `//go:embed minisign.pub`
- [x] 2.7 Add `github.com/jedisct1/go-minisign` to `go.mod`

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/update/channel.go` | Created | `Channel` type (Stable/Beta), `ParseChannel()`, `SelectRelease()` |
| `internal/update/client.go` | Created | `Release`/`Asset` structs, `ListReleases()`, `GetRelease()` via `net/http` |
| `internal/update/verify.go` | Created | `VerifyChecksum()` (SHA-256), `VerifySignature()` (go-minisign) |
| `internal/update/download.go` | Created | `DownloadAndExtract()` for tar.gz + zip via stdlib |
| `internal/update/replace.go` | Created | `ReplaceBinary()` (Unix rename), `ReplaceHint()` (Windows hint) |
| `internal/update/embed.go` | Created | `//go:embed minisign.pub` |
| `internal/update/minisign.pub` | Created | Copy of root key for embed accessibility |
| `go.mod` | Modified | Added `github.com/jedisct1/go-minisign` direct dependency |
| `go.sum` | Modified | Auto-updated by `go mod tidy` |
| `openspec/changes/release-pipeline/tasks.md` | Modified | Marked Phase 2 tasks as `[x]` |

## Deviations from Design

| Design | Actual | Rationale |
|--------|--------|-----------|
| `//go:embed ../../minisign.pub` | `//go:embed minisign.pub` + copy at `internal/update/minisign.pub` | Go's `//go:embed` prohibits `..` in paths. Copied the key file to the package directory. |
| `VerifyChecksum(archivePath, checksumsPath string)` | `VerifyChecksum(data, checksumsContent []byte)` | Byte-level API is more testable (no filesystem coupling). Task description also says `(data, checksums)`. |
| `DownloadAndExtract(ctx, asset Asset, ...)` | `DownloadAndExtract(ctx, url, destDir, binaryName string)` | Simpler signature; caller extracts URL from Asset struct. Task description matches this approach. |
| `ReplaceHint() string` | `ReplaceHint(modulePath string) string` | Module path needed to construct the `go install` command hint for Windows users. |

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| **Focused test command and exact result** | `go test ./internal/update/... -count=1` → exit 0, no test files (Phase 4 tests pending) |
| **Runtime harness command/scenario and exact result** | `N/A` — update engine is a library package with no CLI entry point yet; CLI wiring is PR 3 |
| **Rollback boundary** | Revert `internal/update/` dir + `go.mod` + `go.sum` — no other files affected |

## Build and Test Results

- `go build ./internal/update/...` → success (no output)
- `go build ./...` → success (no output)
- `go vet ./internal/update/...` → success (no output)
- `go vet ./...` → success (no output)
- `go test ./internal/update/... -count=1` → no test files (expected — Phase 4)
- `go mod tidy` → cleaned up dependency annotations
