## Exploration: release-pipeline

### Current State

**gentle-ai** (reference implementation) has a complete 4-layer update system:

1. **Check layer** (`internal/update/check.go`): GitHub API version comparison with TTL-based cooldown (6h in `state.json`), channel-aware (stable vs beta via `GENTLE_AI_CHANNEL` env var), concurrent checks via goroutines.
2. **Strategy layer** (`internal/update/upgrade/strategy.go`): Routes to brew, `go install`, binary download, or script based on platform and Homebrew ownership. gentle-ai on Linux/macOS always uses signed binary download; Windows uses `go install` or manual fallback.
3. **Download + verify layer** (`internal/update/upgrade/download.go`): Fetches SHA256 `checksums.txt`, verifies detached minisign signature (key injected via ldflags), downloads archive, checks digest, extracts from tar.gz, does atomic `os.Rename`.
4. **Self-update entry** (`internal/app/selfupdate.go`): Runs before CLI dispatch, guarded by env vars (`GENTLE_AI_NO_SELF_UPDATE`, `GENTLE_AI_SELF_UPDATE_DONE`), prompts user, manages PendingSync state.

**gentle-ai release pipeline** (`.goreleaser.yaml`):
- Builds: `linux/darwin` amd64+arm64, `CGO_ENABLED=0`, with `-trimpath`
- ldflags inject `minisign.publicKey` + version
- Archives: `tar.gz` with LICENSE, README, docs, contracts
- Checksum: SHA256 `checksums.txt`
- Signing: `minisign -S` on checksums.txt, trusted comment binds `repo=X;tag=Y`
- Homebrew: publishes to `Gentleman-Programming/homebrew-tap`
- Channels: stable (latest GitHub release) / beta (main HEAD via `go install @main` + `GENTLE_AI_CHANNEL=beta`)

Preflight script (`scripts/release-signing-preflight.sh`) validates:
- Secret key exists with `600` permissions
- Public key derives correctly
- Canonical trust anchors match
- Canary sign+verify succeeds with correct trusted comment binding

**biggz-ai current state:**
- `internal/release/release.go`: Minimal — `CheckGitState()`, `Tag()`, `VerifyTag()` for git operations only
- `internal/doctor/version.go`: Has `BuildVersion` ldflags injection point but **no version tag exists** in the repo, no ldflags set in any build script
- `cmd/biggz/main.go`: `release status|tag|verify` subcommand wired, **no `update` command**
- `go.mod`: No minisign dependency, no HTTP client for download, no archive extraction code
- **No `.goreleaser.yaml`**, no `.github/` directory, no `Makefile`, no CI/CD config
- `internal/install/install.go`: Deploys skills/configs but has no binary self-update logic
- `cmd/biggz-mcp/main.go`: Has hardcoded `"version": "1.0.0"` in server info

### Affected Areas

| File | Why affected |
|------|-------------|
| `cmd/biggz/main.go` | New `update` subcommand + startup update check |
| `internal/release/release.go` | Binary download, verify, replace logic |
| `internal/doctor/version.go` | Update check integration (reuse cooldown pattern) |
| `go.mod` | Add `go-minisign`, `go-github` deps |
| `cmd/biggz/main.go` (top) | Add `main.version` ldflags variable |
| `.goreleaser.yaml` (new) | GoReleaser build config |
| `.github/workflows/release.yml` (new) | CI/CD release workflow |
| `scripts/release-signing-preflight.sh` (new) | Minisign preflight validation |
| `internal/install/install.go` | May need to handle binary self-replacement |
| `openspec/config.yaml` | Register new `release-pipeline` change phases |

### Approaches

#### 1. **Full gentle-ai port** — Replicate the entire 4-layer system
- Port `internal/update/` (check, cooldown, registry, github client)
- Port `internal/update/upgrade/` (download, strategy, executor, minisign verify)
- Port `.goreleaser.yaml` + CI workflow + signing preflight
- Implement `biggz update` command + startup check
- **Pros**: Proven architecture, complete feature set, battle-tested edge cases (Windows locking, stale PATH, Homebrew routing, beta channel)
- **Cons**: Significant code volume (15+ files), need to adapt gentle-ai specific patterns (multi-tool registry, Homebrew, OpenCode plugins) that biggz doesn't need
- **Effort**: High

#### 2. **Minimal MVP** — GoReleaser + version check + manual update hint
- Add `.goreleaser.yaml` with linux/darwin/windows builds
- Add GitHub Actions release workflow with minisign signing
- Add `main.version` ldflags injection
- Startup check: simple git tag comparison (reuse `doctor.VersionCheck`)
- `biggz update` command: prints latest version + download URL, user installs manually
- **Pros**: Minimal code changes, fast to ship, signing from day one
- **Cons**: No auto-update, no binary replacement, no channels, no rollback
- **Effort**: Low

#### 3. **Binary download + auto-replace** (recommended MVP)
- Builds on approach 2
- Add `github.com/jedisct1/go-minisign` for signature verification
- `internal/update/` package: fetch releases + verify + download + atomic replace (Unix) / `movefile` (Windows)
- `biggz update` command: auto-download + replace with rollback on failure
- Channels via release tag pattern matching (stable=`v*`, beta=`v*-beta*`)
- Startup check with cooldown (no prompt, just advisory)
- **Pros**: Real auto-update, signature verification, works cross-platform with proper Windows handling
- **Cons**: Moderate effort, Windows binary lock still needs special handling (rename-on-reboot or temp-bat shim)
- **Effort**: Medium

### Recommendation

**Approach 3 (Binary download + auto-replace)**, but scoped to an MVP that ships iteratively:

**Phase 1 — Foundation:**
1. `.goreleaser.yaml` — linux/darwin/windows amd64+arm64 with CGO_ENABLED=0
2. `main.version` ldflags + `doctor.BuildVersion` wiring
3. GitHub Actions release workflow with minisign signing
4. Minisign key pair generation (documented in repo)

**Phase 2 — Update system:**
5. `internal/update/` package (simplified from gentle-ai): GitHub release list, checksum verify, signature verify
6. `internal/update/upgrade/` (simplified): binary download + atomic replace
7. Windows: use `movefile` + `MoveFileEx(MOVEFILE_DELAY_UNTIL_REBOOT)` for locked binary, or temp-bat launcher

**Phase 3 — Commands:**
8. `biggz update` command (check + download + replace)
9. Startup check (cooldown-gated, advisory only, no prompt)
10. Channel support (`BIGGZ_CHANNEL=beta` maps to `v*-beta*` release filter)

### Risks

1. **Windows binary replacement**: Running `.exe` cannot be overwritten via `os.Rename`. gentle-ai sidesteps this by redirecting Windows to `go install`. biggz needs a proper solution: `MoveFileEx` with `MOVEFILE_DELAY_UNTIL_REBOOT` (requires admin), or write a temp launcher that waits + replaces, or use `rename` syscall supported in Go 1.24+ on Windows. **Severity: High**

2. **Minisign key management**: No key pair exists for biggz-ai. Key must be generated, kept in GitHub Actions secrets, and the public key must be injected at build time. Loss of secret key = no new signed releases. **Severity: Medium**

3. **No existing CI/CD**: Zero GitHub Actions workflows, no `.github/` directory. Entire CI pipeline must be built from scratch including Go setup, GoReleaser, signing, Homebrew tap publishing. **Severity: Medium**

4. **go-minisign is not a Go stdlib dep**: It's a third-party package (`github.com/jedisct1/go-minisign`) — supply chain risk, needs to be vendored or pinned. **Severity: Low**

5. **Version zero**: biggz-ai has no existing tags and no established version. First release needs careful semver decision (start at `v0.1.0` or `v1.0.0`?). **Severity: Low**

### Key Files Found

**gentle-ai (reference):**
- `internal/update/check.go` — Concurrent GitHub API version checks
- `internal/update/cooldown.go` — TTL-gated update check via state.json
- `internal/update/github.go` — GitHub REST client (releases, commits)
- `internal/update/types.go` — ToolInfo, InstallMethod, UpdateResult types
- `internal/update/registry.go` — Static tool registry
- `internal/update/upgrade/download.go` — minisign verify + binary download + atomic replace
- `internal/update/upgrade/strategy.go` — Strategy dispatch (brew, go-install, binary, script)
- `internal/update/upgrade/executor.go` — Upgrade orchestrator with pre-backup
- `internal/update/upgrade/types.go` — Upgrade status + report types
- `internal/app/selfupdate.go` — Startup update check with user prompt
- `.goreleaser.yaml` — Multi-platform builds + checksum + minisign signing
- `scripts/release-signing-preflight.sh` — Key + canary validation

**biggz-ai (current):**
- `internal/release/release.go` — Git tag/verify only (105 lines)
- `internal/release/release_test.go` — Version pattern tests
- `internal/doctor/version.go` — `BuildVersion` ldflags injection point
- `cmd/biggz/main.go` — CLI with `release` subcommand but no `update`
- `internal/install/install.go` — Binary copy deploy (for MCP companion binary)
- `go.mod` — No update-related deps (no go-minisign, no HTTP client with timeouts)
- `cmd/biggz-mcp/main.go` — Hardcoded version string

### Ready for Proposal

Yes. The exploration is complete with a clear recommended approach (Approach 3, phased MVP). The orchestrator should tell the user:

- The gentle-ai update system is a mature reference architecture but too complex to port wholesale (multi-tool registry, Homebrew, OpenCode plugins are irrelevant to biggz)
- Recommended approach is a phased MVP starting with GoReleaser + signing, then binary auto-update with minisign verification, then command integration
- Windows binary lock is the highest-risk item — needs research into `MoveFileEx` or Go 1.24+ `rename` behavior
- No GitHub CI/CD infrastructure exists — requires full pipeline setup
- No minisign key exists for biggz-ai — must be generated as part of implementation
