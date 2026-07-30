# Proposal: doctor-command

## Intent

Diagnose system health for biggz-ai installations. Users need a single command to detect misconfigurations, corruption, missing binaries, and environment issues — with optional auto-remediation.

## Scope

### In Scope
- `biggz doctor` subcommand with ~15 health checks
- `internal/doctor/` package with abstract types (Check, Runner, Result, Report, Remedy)
- `--fix` flag for auto-remediation actions
- Human-readable tabla humana output (default) + `--json` machine output
- Review store health check (`.git/biggz/review-transactions/`)

### Out of Scope
- Health dashboard or daemon mode
- Telemetry or usage reporting
- Remote/SSH diagnostics
- Plugin health checks (plugins don't exist yet)
- Windows-specific disk space (deferred to `--fix` that can check disk)

## Capabilities

### New Capabilities
- `system-diagnostics`: Health check framework — types, runner (panic isolation), severity-categorized report, atomic remediations. Checks: bigmem SQLite integrity, config directory structure, MCP binary presence, review store chain integrity, PATH shadowing, disk space, git availability, version info, backup state.

### Modified Capabilities
- `cli`: Add `bigggz doctor` subcommand dispatch, `--fix` and `--json` flag parsing, tabla humana renderer, external format dispatch.

## Approach

Two-layer mirror of gentle-ai's doctor architecture:

1. **`internal/doctor/`** — Abstract types: `CheckID`, `Status` (pass/warn/fail), `Result`, `Check` (ID + Run func), `Runner` (ordered with panic isolation per check), `Report` (severity buckets: CRITICAL/WARNING/INFO), `Remedy` (ID + description + action func).
   - One file per check family: `bigmem.go`, `binary.go`, `config.go`, `disk.go`, `review.go`, `path.go`, `git.go`, `version.go`, `backup.go`.

2. **CLI integration** — New `doctorRun()` in `cmd/biggz/main.go`. `--fix` iterates remedies and executes. `--json` marshals `Report` struct. Default renderer: grouped by severity, per-check `[ok]`/`[!!]`/`[xx]`, summary counts.

3. **Existing reuse**: `store.Validate()` (review), `Store.Doctor()` (bigmem), `exec.LookPath` (PATH), `os.Stat` (files).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/doctor/*.go` | New | ~10 files: types + check implementations |
| `cmd/biggz/main.go` | Modified | Add `case "doctor"` + `doctorRun()` function |
| `internal/bigmem/full.go` | Modified | Enhance `Doctor()` with corruption check |
| `openspec/specs/cli/spec.md` | Modified | Doctor subcommand requirements |
| `openspec/specs/system-diagnostics/spec.md` | New | Full spec for health check framework |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| SQLite lock contention on bigmem check | Med | Degrade to warn, never crash — use timeout |
| PATH scanning false positives (dirs named `biggz`) | Low | Only match executables, ignore non-files |
| `--fix` modifies user state | Med | Print what would change before executing; require confirmation |
| Windows permission errors on disk check | Low | Degrade gracefully to WARNING |

## Rollback Plan

Revert `cmd/biggz/main.go` switch statement. Remove `internal/doctor/` directory. No migration needed — doctor is read-only (except `--fix` actions, which are self-reverting on re-install).

## Dependencies

- `golang.org/x/sys/windows` for `GetDiskFreeSpaceExW` (disk space check) — already indirect dep

## Success Criteria

- [ ] `biggz doctor` runs all ~15 checks and produces severity-grouped output
- [ ] `biggz doctor --json` produces valid JSON parseable by `jq`
- [ ] `biggz doctor --fix` remediates at least bigmem reconnect and biggz-mcp binary deploy
- [ ] Each check is independently unit-testable via `Check.Run` interface
- [ ] Runner panics in one check don't affect others
