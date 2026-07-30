# Design: Release Pipeline

## Technical Approach

Four-layer architecture: (1) GoReleaser + CI/CD builds and signs release artifacts, (2) `internal/update/` engine discovers, verifies, and downloads releases, (3) CLI `update` subcommand wires the engine, (4) ldflags inject version metadata into `doctor.BuildVersion`. The update path is pure Go — no shell execution — using `net/http` for the GitHub API and Go stdlib for archive extraction.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|---|---|---|---|
| GitHub API client | `net/http` direct | `google/go-github` | API surface is 3 endpoints; external dep adds supply chain risk for ~80 lines |
| Sig verification | `github.com/jedisct1/go-minisign` | Manual ed25519 crypto | Canonical lib by minisign author; matched to GoReleaser signing output |
| Archive extraction | stdlib `archive/tar`+`gzip`, `archive/zip` | External unpack libs | Known archive structure; zero new deps for extraction |
| Binary replace (Unix) | `os.Rename` over temp dir | `cp`+`rm` via shell | Atomic on same filesystem; no shell injection surface |
| Binary replace (Windows) | Print `go install` hint | `MoveFileEx` | Running .exe cannot be overwritten; hint is correct, safe fallback |
| Channel selection | `BIGGZ_CHANNEL` env var | Config file, flag-only | Zero-config, composable with shell, discoverable via `--help` |
| Public key trust anchor | Committed `minisign.pub` + `//go:embed` | Download from release | In-repo key is versioned in git; embed prevents runtime file-not-found |

## Data Flow

```
GitHub Releases API ──→ client.go ──→ channel.go (stable/beta filter)
                              │
                     download.go (fetch archive + checksums.txt + .minisig)
                              │
                       verify.go (SHA256 binary vs checksums.txt, then minisig verify checksums.txt)
                              │
                    replace.go (Unix: os.Rename / Windows: print go install hint)
```

## File Changes

| File | Action | Description |
|---|---|---|
| `.goreleaser.yaml` | Create | Build matrix (6 targets), ldflags, minisign signing, archive per platform |
| `.github/workflows/release.yml` | Create | Trigger on `v*`, `goreleaser-action`, publish to GitHub Releases |
| `internal/update/client.go` | Create | List releases, get latest (stable/prerelease), download asset URLs |
| `internal/update/verify.go` | Create | SHA256 checksum match + go-minisign signature verification |
| `internal/update/download.go` | Create | HTTP GET to temp dir, extract binary from archive |
| `internal/update/replace.go` | Create | `os.Rename` atomic replace (Unix), hint message (Windows) |
| `internal/update/channel.go` | Create | Parse `BIGGZ_CHANNEL`, filter releases by `.Prerelease` |
| `internal/update/embed.go` | Create | `//go:embed minisign.pub` for runtime access |
| `minisign.pub` | Create | Public key committed to repo root |
| `scripts/preflight.sh` | Create | Local minisign keygen + signing preflight check |
| `cmd/biggz/main.go` | Modify | Add `update` case in switch + `updateRun()` function |
| `go.mod` | Modify | Add `github.com/jedisct1/go-minisign` |
| `internal/doctor/version.go` | No change | `var BuildVersion` already exists; ldflags injection is sufficient |

## Key Interfaces

```go
// internal/update/client.go
type Release struct {
    TagName     string `json:"tag_name"`
    Prerelease  bool   `json:"prerelease"`
    Assets      []Asset `json:"assets"`
}
type Asset struct {
    Name string `json:"name"`
    URL  string `json:"browser_download_url"`
}

// internal/update/channel.go
type Channel int
const (
    ChannelStable Channel = iota
    ChannelBeta
)
func ParseChannel(env string) Channel

// internal/update/verify.go
func VerifyChecksum(archivePath, checksumsPath string) error
func VerifySignature(checksumsPath, sigPath, pubKey []byte) error

// internal/update/download.go
func DownloadAndExtract(ctx context.Context, asset Asset, destDir, binaryName string) (string, error)

// internal/update/replace.go
func ReplaceBinary(src, dst string) error  // non-Windows
func ReplaceHint() string                   // Windows: "go install ..."
```

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | Channel parse, archive extract, checksum verify, version compare | Table-driven, no network |
| Integration | Full download+verify cycle | Test against a known GitHub release with real checksums |
| E2E | `goreleaser build --snapshot` produces valid signed archives | CI smoke test on PR |
| Security | Minisign verify rejects tampered checksums | Corrupt digest test, wrong public key test |

## Threat Matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | Archives extracted by known binary name; no path-as-code execution |
| Git repository selection | N/A | Update engine has zero git interaction |
| Commit state | N/A | No commit operations in the update path |
| Push state | N/A | No push operations in the update path |
| PR commands | N/A | No PR automation in this change |

All applicable rows propagate to tasks as explicit RED-test requirements. The update engine uses no shell commands, no subprocess spawning, and no VCS automation — every cross-boundary operation (HTTP download, archive extraction, file rename) uses Go's standard library.

## Ldflags Wiring

GoReleaser injects version directly into `doctor.BuildVersion`:

```yaml
ldflags:
  - -s -w
  - -X github.com/biggz-ai/biggz/internal/doctor.BuildVersion={{ .Version }}
```

No code change needed in `version.go` — the `var BuildVersion string` on line 23 is the ldflags target.

## Open Questions

- [ ] Minisign key lifecycle: generate once manually via `minisign -G` or automate via CI secret? First release requires manual keygen.
- [ ] GitHub API auth: unauthenticated requests are rate-limited to 60/hr. Should `client.go` accept `GITHUB_TOKEN` env var? Not critical for MVP — the update command is user-initiated, not daemon.
- [ ] Archive layout: goreleaser nests binaries under `{binary}_{version}_{os}_{arch}/{binary}` — verify this matches extraction path logic.
