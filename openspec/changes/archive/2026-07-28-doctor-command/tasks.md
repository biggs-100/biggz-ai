# Tasks: doctor-command

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 800–1000 |
| 800-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: framework + checks → PR 2: CLI + bigmem + tests |
| Delivery strategy | force-chained (stacked-to-main) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Types, Runner, Report, Remedy + all 9 check implementations | PR 1 | `go test ./internal/doctor/... -run TestRunner` | `go build ./internal/doctor/...` (no CLI, compiles only) | `git rm -r internal/doctor/` |
| 2 | CLI wiring, bigmem Doctor() enhancement, full test suite | PR 2 | `go test ./internal/doctor/... -v` | `go build && ./biggz doctor --json \| gojq .` | Revert `cmd/biggz/main.go` + `internal/bigmem/full.go` |

## Phase 1: Core Framework (PR 1)

- [x] 1.1 Create `internal/doctor/types.go` — CheckID, Status, Result, Check interface, Report with severity buckets, Remedy struct
- [x] 1.2 Create `internal/doctor/runner.go` — Runner with RunAll(ctx) wrapping each check call in defer/recover; captured panics → StatusFail Result

## Phase 2: Check Implementations (PR 1)

- [x] 2.1 Create `internal/doctor/bigmem.go` — opens bigmem store, runs PRAGMA integrity_check, maps to Result
- [x] 2.2 Create `internal/doctor/binary.go` — os.Stat on `~/.biggz/biggz-mcp`, verifies executable mode
- [x] 2.3 Create `internal/doctor/config.go` — verify `~/.biggz/` dir and required subdirectory tree
- [x] 2.4 Create `internal/doctor/review.go` — enumerate review lineages, call store.Validate() on each, handle ENOENT on missing .git dir (WARNING)
- [x] 2.5 Create `internal/doctor/path.go` — split PATH, os.Stat for biggz/biggz-mcp per dir, report duplicates as WARNING
- [x] 2.6 Create `internal/doctor/disk.go` — platform-specific (build tags) free disk check via `golang.org/x/sys/windows`, <500 MB → WARNING
- [x] 2.7 Create `internal/doctor/git.go` — exec.LookPath("git") (CRITICAL if missing), exec git rev-parse (WARNING if not a repo)
- [x] 2.8 Create `internal/doctor/version.go` — compare embedded ldflags version vs latest git tag, mismatch → INFO with both versions
- [x] 2.9 Create `internal/doctor/backup.go` — list backups, verify newest timestamp within 7 days, stale → WARNING with age

## Phase 3: CLI Integration + bigmem Enhancement (PR 2)

- [x] 3.1 Enhance `internal/bigmem/full.go` — add `Corrupt bool` to DoctorResult, run PRAGMA integrity_check in Doctor()
- [x] 3.2 Modify `cmd/biggz/main.go` — add `case "doctor"` in switch, help text in printHelp, import doctor package
- [x] 3.3 Create `doctorRun()` in `cmd/biggz/main.go` — manual flag parse for `--json` and `--fix`, build Runner with all 9 checks
- [x] 3.4 Custom table renderer in `doctorRun()` — severity-grouped sections, `[ok]`/`[!!]`/`[xx]` icons, summary footer
- [x] 3.5 `--json` output — marshal Report to os.Stdout with json.MarshalIndent
- [x] 3.6 `--fix` iteration — after checks complete, iterate Results with non-nil Remedy, execute Action, re-run affected checks

## Phase 4: Tests (PR 2)

- [x] 4.1 Write tests for types/runner — panic isolation (B panics, A and C complete), severity bucketing, remedy dispatch
- [x] 4.2 Write tests for bigmem check — mock store with known corrupt/clean state
- [x] 4.3 Write tests for binary check — temp dirs with/without biggz-mcp executable
- [x] 4.4 Write tests for config check — temp dir with missing subdirectory
- [x] 4.5 Write tests for review check — mock store.Validate() returning pass/fail
- [x] 4.6 Write tests for path check — temp PATH with duplicate entries
- [x] 4.7 Write tests for disk check — mock syscall return values
- [x] 4.8 Write tests for git check — git not in PATH → CRITICAL; missing .git dir → WARNING (threat matrix)
- [x] 4.9 Write tests for version check — mock git tag comparison
- [x] 4.10 Write tests for backup check — temp backup dir with old/new timestamps
- [x] 4.11 Integration test: end-to-end `doctorRun()` with temp dirs, verify JSON and table output
