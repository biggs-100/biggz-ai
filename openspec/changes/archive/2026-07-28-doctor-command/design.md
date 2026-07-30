# Design: doctor-command

## Technical Approach

Mirror gentle-ai's doctor architecture: a `internal/doctor/` package with a Check interface, panic-isolated Runner, severity-categorized Report, and atomic Remedies. Each system domain gets its own file. CLI integration adds a switch-case `case "doctor"` in `cmd/biggz/main.go` with `--json`, `--fix`, and a custom severity-grouped table renderer. Existing `Store.Doctor()` and `Store.Validate()` methods are reused and enhanced where needed.

## Architecture Decisions

| Option | Tradeoffs | Decision |
|--------|-----------|----------|
| Check interface with `Run(ctx) (*Result)` vs free functions | Interface enables uniform runner, test mocking, and remedy attach | **Interface** — one contract for all checks |
| Per-check panic `recover()` in Runner vs each check self-recover | Centralized is DRYer; less cognitive load per check implementation | **Centralized in Runner.RunAll()** — wraps each check call |
| Manual flag parse vs `flag` package | Existing code uses manual parse (e.g., `installRun`); consistency wins | **Manual** — follow existing `os.Args[2:]` loop pattern |
| Custom table renderer vs external lib | Existing review commands already use `fmt.Printf("%-36s ...")` — no table lib in deps | **Custom render** — `fmt.Fprintf` with severity grouping |
| Enhance existing `bigmem.Store.Doctor()` vs separate check | `Doctor()` already opens DB; adding `PRAGMA integrity_check` keeps the health logic colocated | **Enhance Doctor()** — add `Corrupt bool` field |

## Data Flow

```
CLI (biggz doctor [--json] [--fix])
  │
  ▼
doctorRun() ──▶ internal/doctor.Runner.RunAll(ctx) ──▶ Report
  │                    │                                  │
  │              ┌─────┼──────┬──────────┐              severity
  │              ▼     ▼      ▼          ▼               buckets
  │           bigmem binary config  review...
  │           check  check  check  check
  │              │
  └── if --fix ──▶ iterate Results → execute Remedies
      └── output: JSON or human table
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/doctor/types.go` | Create | CheckID, Status, Result, Check interface, Runner, Report, Remedy types |
| `internal/doctor/runner.go` | Create | Runner.RunAll() — ordered execution with per-check panic isolation |
| `internal/doctor/bigmem.go` | Create | bigmem check — open store, PRAGMA integrity_check, map to Result |
| `internal/doctor/binary.go` | Create | MCP binary presence check via os.Stat on expected ~/.biggz/biggz-mcp |
| `internal/doctor/config.go` | Create | Verify ~/.biggz/ dir structure and key subdirectories |
| `internal/doctor/disk.go` | Create | Free disk space check (platform-specific, build tags) |
| `internal/doctor/review.go` | Create | Enumerate review lineages, call Store.Validate() on each |
| `internal/doctor/path.go` | Create | Scan PATH for biggz/biggz-mcp duplicates |
| `internal/doctor/git.go` | Create | exec.LookPath("git"), verify .git directory |
| `internal/doctor/version.go` | Create | Compare embedded version vs latest git tag |
| `internal/doctor/backup.go` | Create | List backups, check last timestamp within 7 days |
| `internal/doctor/doctor_test.go` | Create | Tests for types, runner, each check |
| `cmd/biggz/main.go` | Modify | Add `case "doctor"`, `doctorRun()`, help text, printHelp entry |
| `internal/bigmem/full.go` | Modify | Enhance `Doctor()` with `PRAGMA integrity_check`, add `Corrupt bool` to `DoctorResult` |

## Interfaces / Contracts

```go
type CheckID string
type Status int

const (
    StatusPass Status = iota
    StatusWarn
    StatusFail
)

type Result struct {
    ID       CheckID `json:"id"`
    Status   Status  `json:"status"`
    Message  string  `json:"message"`
    Severity string  `json:"severity"` // "CRITICAL" | "WARNING" | "INFO"
    Error    string  `json:"error,omitempty"`
}

type Check interface {
    ID() CheckID
    Run(ctx context.Context) *Result
    Remedy() *Remedy // nil when no remediation available
}

type Remedy struct {
    ID          string
    Description string
    Action      func(ctx context.Context) error
}

type Runner struct {
    Checks []Check
}

func (r *Runner) RunAll(ctx context.Context) *Report

type Report struct {
    Critical []*Result `json:"critical"`
    Warning  []*Result `json:"warning"`
    Info     []*Result `json:"info"`
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Check interface, Runner panic isolation | Table-driven with `rapid` property tests covering panic/recover |
| Unit | Each check implementation | Mock external deps (exec, os.Stat, bigmem.Open) — return known pass/warn/fail states |
| Unit | Report severity bucketing | Verify bucket assignment and counts match spec scenarios |
| Unit | Remedy execution and error propagation | Inject failing/succeeding Action funcs |
| Integration | `doctorRun()` end-to-end with temp dirs and git repo | Run check set, verify output format (JSON and table) |

## Threat Matrix

| Boundary | Applicability | Design Response | Planned RED Tests |
|----------|---------------|-----------------|-------------------|
| Documentation-like paths | **N/A** — doctor does not classify doc paths as executables | — | — |
| Git repository selection | **Applicable** — git check calls `exec.Command("git", "rev-parse")`, review check reads `.git/biggz/review-transactions/` | All git commands use `exec.Command` with no `-C` flag (cwd authority); review check wraps `os.ReadDir` with ENOENT guard | Missing `.git` dir → WARNING; `git` not on PATH → CRITICAL |
| Commit state | **N/A** — doctor does not examine or modify commit state | — | — |
| Push state | **N/A** — doctor does not push | — | — |
| PR commands | **N/A** — doctor does not create PRs | — | — |

## Migration / Rollout

No migration required. Doctor is additive — new files and a new switch case. The `bigmem.Store.Doctor()` signature change (adding `Corrupt bool`) is backward-compatible: existing callers still get `DoctorResult`, they just see a new field.

## Open Questions

- [ ] Windows disk check: use `golang.org/x/sys/windows.GetDiskFreeSpaceExW` or `syscall.Statfs` with build tags? Prefer `golang.org/x/sys/windows` since it's already an indirect dep.
- [ ] Version embedding: add `-ldflags` at build time, or parse from git tag at runtime? For dev builds, git tag is reliable; for release builds, ldflags is precise. Use both: ldflags with git fallback.
