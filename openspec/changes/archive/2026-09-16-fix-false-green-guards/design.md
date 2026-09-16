# Design: fix-false-green-guards

## Technical Approach

False-green surfaces become commands whose exit status is the verdict: `tools/gitexec` self-checks (fixture count), censuses targets (zero = RED), matches the baseline (stale = RED), reports `file:line`. PR1 freezes 64 debt + 7 boundary sites; waves route packages to `internal/git`; dead (zero callers) `convergence.go` deleted — spec allows absence; frozen inspector freezes tree identity.

## Architecture Decisions

| # | Options | Decision |
|---|---|---|
| D1 Checker | multichecker `go vet -vettool` (type-aware; per-package — no ratchet/census) · AST guard test (deleted file → green) · standalone binary | Standalone `tools/gitexec`; only whole-run form failing closed on absence; alias-aware AST + fixtures mitigate type loss. CI: `go build -o /tmp/gitexec ./tools/gitexec/cmd/gitexec && /tmp/gitexec -root .` (no pipeline). Fail RED: absent 127, build, zero `.go`, fixture ≠ N, unreadable. |
| D2 Baseline | exact `(rule,path,line)` equality (rots) · `(rule,path)` counts | `.github/guard-baseline.txt`: `rule<TAB>path<TAB>line`; rules `go-git-spawn` (64→0), `declared-boundary` (2 host + 5 doctor, persist). Count match per `(rule,path)`; `-update` refreshes lines; mismatch = exit 1. Upstream: entry matching nothing = FAILURE. |
| D3 `internal/git` | named mutators · generic runner + constructor | `Run`, `TopLevel`, `ResolveGitDirs`, `RevParse`, `NewCommand` (below); `DetectGitDirs()` byte-compatible (`cli_util.go:50`, `sdd/status.go:820`, `install.go:372`). `Run` keeps raw stdout + stderr (`cas_store.go:316` classifies by wording); `ResolveGitDirs` absorbs `store.go:440,585`, `edit_authority.go:246`; `NewCommand` carries the frozen-inspector env (`GIT_*` stripped, `LANG=C`, `--no-pager`). |
| D4 Gatekeeper | infer store from result · explicit store input | `Gatekeeper(openspecRoot, changeName, completedPhase string, store ArtifactStore, result *PhaseResult)`; store from `ResolvePreflightPrefs(cwd)` (already normalized); canonical phase artifact decides pass/fail; declared paths stat'ed (repo-root, change-dir), never proof; `""` → skip with reason; empty list skips (`checkContract` fails). No input ⇒ `filepath.Join(changeDir, art.Path)` double-prefix (#2346). |
| D5 Slices | one PR (~1500 lines) · checker-first (dead code, unprotected window) · stacked | Six stacked slices (below); PR1 irreducible; 400-line ceiling; staleness forces entry deletion with sites. |

## Data Flow

```
gitexec: fixtures, census, baseline, exit 0/1, one CI step
wave: site → git.Run, entry deleted same PR
Gatekeeper(store): canonical stat, pass/fail/skip
```

## File Changes

| File | Action (slice) |
|------|----------------|
| `tools/gitexec/guard*.go`, `ratchet.go`, `testdata/fixtures/**`, `cmd/gitexec/main.go`; `.github/guard-baseline.txt` | Create (1) |
| `.github/workflows/ci.yml`; `docs/testing-guidance.md`; `internal/review/lens/{no_fmt_guard_test.go,readability/lens.go}`; `internal/install/assets/hooks/pre-push.tmpl` | Modify (1, 2) |
| `internal/git/exec.go` + goldens; `internal/review/{store,gate,rdd_helpers,capture,finalize,risk,frozen_inspector,reconcile,gate_diagnostics}.go` | Create/Modify (3) |
| `internal/review/convergence.go` | Delete (3) |
| `internal/sdd/{gatekeeper,status,complexity_gate,edit_authority}.go`, `cmd/biggz/cli_sdd.go` | Modify (4) |
| `cmd/biggz/{pr,cli_util,cli_bigmem,cli_codegraph,export,sdd_new}.go` | Modify (5) |
| `internal/release/release.go`, `internal/install/install.go`, `internal/sddattempt/cas_store.go`, dead `internal/doctor/git.go:71-72` | Modify (6) |

Verify — 1 fixtures/build; 2 lens `-list`/hook; 3 golden env/argv/TM-2..4; 4 store matrix; 5 argv/TM-3..5; 6 `go test ./...`.

## Interfaces / Contracts

```go
func Run(ctx context.Context, dir string, args ...string) ([]byte, error)
func TopLevel(ctx context.Context, repo string) (string, error)
func ResolveGitDirs(ctx context.Context, repo string) (worktreeDir, commonDir string, err error)
func RevParse(ctx context.Context, repo string, args ...string) (string, error)
type ExecOptions struct { Repo string; NoPager bool; ExtraEnv []string; Stdout, Stderr io.Writer }
func NewCommand(opts ExecOptions, args ...string) *exec.Cmd
```

Exits: 0 clean, 1 failure. Scope: `*.go` minus `internal/git/**`, `*_test.go`, `testdata/`, `e2e/`, `openspec/`.

## Testing Strategy

| Layer | What / Approach |
|-------|-----------------|
| Unit | classifier (direct, `runCmd`, `execFn`, alias, `LookPath`), baseline new/stale/partial, lens zero/unreadable, hooks |
| Integration | fixture count; `-update`; `Run` stderr; `NewCommand` env/argv golden; store matrix; hook `sh` |
| E2E | `go build`, `go test`, `go vet`; CI once |

## Threat Matrix

All five Applicable; statements propagate verbatim to `sdd-tasks`; RED tests first.

| Boundary | Design response / RED test |
|---|---|
| TM-1 Applicable — Documentation-like paths | Parse-based classification, not name globs; `.md`/`.yml`/`CMakeLists.txt`/executable Markdown never executed or classified; host `internal/assets/pi/**/*.{ts,js}` → host rule. RED: doc/CI spawn text → none; host fixture → finding. |
| TM-2 Applicable — Git repository selection | Explicit `repo`; `-C` from the absolute root, never caller cwd; cwd fallback in legacy `DetectGitDirs()`. RED: relative/absolute/foreign-cwd/subdir → same repo. |
| TM-3 Applicable — Commit state | pr.go write ops argv verbatim; `write-tree`=index, `HEAD^{tree}`=commit. RED: empty index; `add -A`+commit; unstaged mutation invisible to `write-tree`, visible to workspace snapshot. |
| TM-4 Applicable — Push state | `push -u origin <branch>` preserved; no refspec rewriting; raw stderr. RED: bare remote — first push sets upstream; repeat; failure stderr raw. |
| TM-5 Applicable — PR commands | `gh pr create` untouched (documented non-git boundary); git parts routed; golden argv. RED: `getChangedFiles`/`gh` goldens. |

Safe: no pipeline/swallowed status; failures name `file:line`.

## Migration / Rollout

PR1 freezes debt; waves delete entries with sites; slices revert. Final: `go-git-spawn`=0; `declared-boundary` checked.

## Open Questions

Resolved: `chain_strategy=stacked-to-main` + cached `delivery_strategy=auto-chain` (last slice merges `master`).
Resolved: boundary persists (specified): debt = zero counted entries (never absence check); boundary uncounted, stale-dies; criterion: zero debt entries, never zero lines.
Resolved: spec split `ci-guard-integrity`/`ci-guard-baseline`; counts unchanged.
Open: PR1 vs 400 lines (`size:exception` or split); apply-gate call.
