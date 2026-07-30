# Exploration: doctor-command

## Current State

### gentle-ai doctor architecture (reference)

**Two-layer architecture:**

1. **`internal/doctor/` (abstract types)** — `CheckID`, `Status` (pass/warn/fail), `Result` (Name+Status+Detail+Remedy), `Check` (ID+Run func), `Runner` (ordered execution with panic isolation), `Report`, `Remedy` (ID+Description+Category+ActionMode+SupportedPlatforms), `RemedyID`, `RemedyCategory`

2. **`internal/cli/doctor.go` (platform-aware)** — `RunDoctor(ctx, w)` as entry point:
   - **Tool PATH checks**: `checkOneTool()` → uses `exec.LookPath()`, scans all PATH dirs for duplicates honoring PATHEXT on Windows, `.ps1` shim fallback
   - **Special gentle-ai clause**: `doctorInvokedGentleAIClause()` → names the invoked binary via `os.Executable()` and compares with PATH-resolved copy
   - **state.json check**: `checkStateJSON()` → file exists? parseable? agents installed? config dirs exist?
   - **Engram reachability**: `checkEngramReachable()` → HTTP GET to `$ENGRAM_BASE_URL/health`
   - **Disk space**: `checkDiskSpace()` → uses `storage.AvailableBytes()` (Win32 `GetDiskFreeSpaceExW` on Windows); thresholds 10MB=fail, 100MB=warn
   - **Core tools**: gentle-ai, gga, engram
   - **Agent tools**: derived from `state.json → InstalledAgents` via `agentToolBinaries` map
   - **Renderer**: `renderDoctorReport()` → text report with `[ok]`/`[!!]`/`[xx]` icons, summary counts, healthy/degraded/unhealthy status

### biggz-ai current health surfaces

| Surface | Location | What can fail |
|---------|----------|---------------|
| **bigmem** | `~/.biggz/bigmem/bigmem.db` | SQLite corrupt, WAL locked, permissions, disk full |
| **install artifacts** | `~/.biggz/` | Dir missing, MCP binary missing, stale files |
| **review store** | `.git/biggz/review-transactions/<lineage>/` | Hash chain broken, HEAD missing, file corruption |
| **config** | Agent's opencode.json | Stale MCP command path, missing orchestrator prompt |
| **MCP binary** | `~/.biggz/biggz-mcp.exe` | Not deployed, old version, PATH shadowed |
| **git state** | Local repo | Needs git for review/release operations |

## Affected Areas

- `cmd/biggz/main.go` — new `"doctor"` case in switch statement
- `internal/doctor/` — new package (abstract check types)
- `internal/cli/doctor.go` — not applicable (biggz doesn't have cli/ subpackage yet)
- `internal/bigmem/bigmem.go` — already has `Doctor()` method, may enhance
- `internal/review/store.go` — already has `Validate()` method, reusable
- `internal/install/install.go` — for checking install state
- `go.mod` — may need `golang.org/x/sys/windows` for disk space (already indirect dep)
- `internal/release/release.go` — version info for reporting

## Approaches

### Approach 1: Mirror gentle-ai two-layer pattern

Package structure:
```
internal/doctor/
  doctor.go       — abstract types (Check, Runner, Result, Report, Remedy)
  bigmem.go       — bigmem health check
  binary.go       — binary PATH resolution check
  config.go       — config dir and file integrity checks
  disk.go         — disk space check
  review.go       — review store integrity check
```

CLI integration via `cmd/biggz/main.go`:
```go
case "doctor":
    os.Exit(doctorRun())
```

- Pros: Clean separation, testable, gentle-ai compatible pattern, reusable types
- Cons: More code upfront
- Effort: Medium

### Approach 2: Flat CLI package

Put everything in `cmd/biggz/main.go` or a single `internal/doctor/doctor.go`.

- Pros: Less initial code
- Cons: Hard to test, no separation of concerns
- Effort: Low

## Recommendation

**Approach 1** — mirror gentle-ai's two-layer pattern. The abstract types in `internal/doctor/` (Check, Runner, Result, Report, Remedy) are worth extracting because:
1. The `Runner` with panic isolation per check is critical for a diagnostic tool
2. The `Status` (pass/warn/fail) + `Remedy` pattern gives actionable output
3. Future commands (like `refresh` or the existing `verify-validate`) could reuse the report types

## MVP Check List (prioritized)

| Priority | Check ID | What it checks | How | Depends on |
|----------|----------|---------------|-----|------------|
| **P0** | `bigmem:store` | SQLite open + basic query + observation count | Open store, run `Stats()` | bigmem package |
| **P0** | `biggz:binary` | Running binary vs PATH-resolved | `os.Executable()` + `exec.LookPath("biggz")` + compare | — |
| **P0** | `biggz-mcp:binary` | MCP binary exists at `~/.biggz/biggz-mcp.exe` | `os.Stat()` | — |
| **P1** | `biggz:config-dir` | `~/.biggz/` exists and has expected structure | `os.Stat()` each expected sub-dir | — |
| **P1** | `review:integrity` | Review transaction chain integrity | `store.Validate()` per lineage | review package |
| **P1** | `path:shadow` | Duplicate biggz/binaries in PATH | Scan all PATH dirs (like gentle-ai) | — |
| **P2** | `disk:space` | Free space on `~/.biggz/` filesystem | Win32 `GetDiskFreeSpaceExW` | `golang.org/x/sys/windows` |
| **P2** | `git:available` | Git binary on PATH | `exec.LookPath("git")` | — |
| **P3** | `backup:state` | Backup store integrity | Check backup files exist and parse | backup package |
| **P3** | `version:info` | Report build version + git commit | `debug.ReadBuildInfo()`, ldflags | — |

## Risks

- **PATH injection**: Windows PATH scanning must handle PATHEXT, case-insensitive comparisons, `.ps1` shims, and non-executable files (already solved pattern in gentle-ai)
- **Permissions**: Disk space check can fail with `ERROR_ACCESS_DENIED` on Windows — must degrade gracefully to warn
- **Cross-platform**: Only Windows in MVP scope, but architecture must support future Unix (darwin/linux) — use build tags for platform-specific disk space
- **False positives**: Directory named "biggz" in PATH should not count as duplicate binary
- **State absence**: First-time install has no `~/.biggz/` dir — must warn not fail
- **Lock contention**: SQLite may be locked by another process — check should degrade to warn, not crash
- **BigMem Doctor already exists**: `bigmem.Doctor()` returns `DoctorResult` — may want to enhance or replace for consistency

## Ready for Proposal

Yes.

## Key Files Found

### gentle-ai (reference)
- `internal/doctor/doctor.go` — abstract check types (151 lines)
- `internal/cli/doctor.go` — platform-aware checks (566 lines)
- `internal/storage/space_windows.go` — Win32 disk space
- `internal/storage/space.go` — generic disk space dispatcher

### biggz-ai
- `cmd/biggz/main.go` — CLI entry point (1033 lines)
- `internal/bigmem/bigmem.go` — SQLite store with existing `Doctor()` (247 lines)
- `internal/bigmem/full.go` — extended tools including `DoctorResult` (353 lines)
- `internal/review/store.go` — content-addressed store with `Validate()` (340 lines)
- `internal/install/install.go` — install logic (378 lines)
- `go.mod` — depends on `modernc.org/sqlite`, `golang.org/x/sys` (indirect)
- `internal/release/release.go` — version tagging
