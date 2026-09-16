# Tasks: fix-false-green-guards

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1,900–2,400 total; PR1 ~700–850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1a/PR1b (stacked) → PR2…PR6; or PR1 `size:exception` |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

PR1 (~700–850) is an honest `size:exception` candidate or PR1a/PR1b split; PR3 (~500–650) is next. Split at apply if measured >400. Auto-chain proceeds; no ask.

### Suggested Work Units

| # | Goal (start → finish) | PR | Focused test | Runtime harness | Rollback boundary |
|---|----------------------|----|--------------|-----------------|-------------------|
| 1 | Empty tree → checker + fixtures + 71-entry baseline + CI wiring; 64 debt frozen (R1–R3,R5–R9) | PR1 (or PR1a+PR1b) | `go test ./tools/gitexec/... -count=1` | `go build -o /tmp/gitexec ./tools/gitexec/cmd/gitexec && /tmp/gitexec -root .` → exit 0, 64 debt | Revert PR1; prior wiring restored |
| 2 | [PR1] → pre-push hook fail-closed on unknown producer (R11) | PR2 | `go test ./internal/install/... -count=1` | Installed hook via `sh`, `biggz` absent from PATH | Revert template; regenerated on install |
| 3 | [PR2] → `internal/git` surface live, `internal/review` migrated, `convergence.go` absent (R4,R12–R15) | PR3 | `go test ./internal/git/... ./internal/review/... -count=1` | `NewCommand` argv/env goldens; workspace-mutation scratch repo | Revert PR3; sites+entries return together |
| 4 | [PR3] → gatekeeper store-aware, canonical stat, skip-with-reason (R10) | PR4 | `go test ./internal/sdd/... ./cmd/biggz/... -count=1` | store matrix `openspec`/`hybrid`/`""` | Revert PR4; prior signature/callers |
| 5 | [PR4] → `cmd/biggz` spawns routed, argv verbatim (R4,TM-3–TM-5) | PR5 | `go test ./cmd/biggz/... -count=1` | Bare remote: first `push -u`, repeat, raw stderr | Revert PR5; CLI behavior unchanged |
| 6 | [PR5] → release/install/sddattempt migrated, doctor seam clean, debt 0 (R3,R4) | PR6 | `go test ./internal/release/... ./internal/install/... ./internal/sddattempt/... ./internal/doctor/... -count=1` | `/tmp/gitexec -root .` → 0 debt, 7 boundary entries remain | Revert PR6; atomic |

IDs: R1 checker; R2 positive control; R3 baseline; R4 git wrapper; R5 CI forbid; R6 no-fmtSprintf; R7 lint/rapid; R8 cyclomatic; R9 cognitive; R10 gatekeeper; R11 hook; R12–R15 review-authority (tree/error/projection/typed).

## Threat Matrix (verbatim from design; all Applicable)

| Boundary | Design response / RED test |
|---|---|
| TM-1 Applicable — Documentation-like paths | Parse-based classification, not name globs; `.md`/`.yml`/`CMakeLists.txt`/executable Markdown never executed or classified; host `internal/assets/pi/**/*.{ts,js}` → host rule. RED: doc/CI spawn text → none; host fixture → finding. |
| TM-2 Applicable — Git repository selection | Explicit `repo`; `-C` from the absolute root, never caller cwd; cwd fallback in legacy `DetectGitDirs()`. RED: relative/absolute/foreign-cwd/subdir → same repo. |
| TM-3 Applicable — Commit state | pr.go write ops argv verbatim; `write-tree`=index, `HEAD^{tree}`=commit. RED: empty index; `add -A`+commit; unstaged mutation invisible to `write-tree`, visible to workspace snapshot. |
| TM-4 Applicable — Push state | `push -u origin <branch>` preserved; no refspec rewriting; raw stderr. RED: bare remote — first push sets upstream; repeat; failure stderr raw. |
| TM-5 Applicable — PR commands | `gh pr create` untouched (documented non-git boundary); git parts routed; golden argv. RED: `getChangedFiles`/`gh` goldens. |

Out of scope: gatekeeper `nextPhaseValid` routing table (`internal/sdd/gatekeeper.go:92`) — `state.yaml` `discovered_defects`; no task.

## Phase 1: PR1a — Checker, Fixtures, Baseline

- [x] 1.1 RED (TM-1): doc/CI spawn text → 0 findings; host `internal/assets/pi/**` fixture → finding. (TM-1,R1)
- [x] 1.2 RED fixtures: direct `exec.Command("git")`, `runCmd`/`execFn` indirection, alias — fail pre-classifier. (R1,R2)
- [x] 1.3 `tools/gitexec/guard.go` + `cmd/gitexec/main.go`: parse-based, alias-aware; literal `"git"` in command position; `file:line`; exit 1. (R1)
- [x] 1.4 Census + fixture-count: 0 resolved `.go` targets, or 0 findings unless N matched → fail. Scope `*.go` minus `internal/git/**`, tests, `testdata/`, `e2e/`, `openspec/`. (R2)
- [x] 1.5 `tools/gitexec/ratchet.go` + `.github/guard-baseline.txt` (`rule<TAB>path<TAB>line`): `(rule,path)` count match; new/stale/partial → exit 1; `-update` refreshes. (R3)
- [x] 1.6 Seed 71 entries: 64 `go-git-spawn` debt + 7 `declared-boundary` (2 host, 5 doctor `execFn`); freshness evidence. (R3,TM-1)
- [x] 1.7 Absence/build-fail/127 fails closed; `-root .` exits 0 on frozen tree. (R1)

## Phase 2: PR1b — CI Wiring, Lens, Docs

- [x] 2.1 `ci.yml`: build checker then bare run (no pipeline); delete all `rg` steps → zero `rg` invocations. (R5,R7)
- [x] 2.2 `no_fmt_guard_test.go`: repo-root reads; hard-fail on unreadable/zero files; no `continue`. (R6)
- [x] 2.3 `readability/lens.go`: mark all 14 non-prompt sites `//lint:ignore no-fmtSprintf`; scope stays package-wide. (R6)
- [x] 2.4 vet/format jobs observe own status: `pipefail`/no pipeline; missing/failing `go vet`,`gofmt` fails, never a pass verdict. (R7)
- [x] 2.5 Complexity producers: drop `|| true` for gocyclo/gocognit; build/non-zero fails job; zero evaluated with in-scope diff → fail. (R8,R9)
- [x] 2.6 `docs/testing-guidance.md`: remove `rg` contract; document observed-status. (R7)

## Phase 3: PR2 — Pre-push Hook

- [x] 3.1 RED hook test: `biggz` absent / `rdd status` non-zero and no lineage → non-zero exit or explicit notice; never silent 0. (R11)
- [x] 3.2 `pre-push.tmpl`: distinguish enabled/disabled/unknown; unknown never collapses to disabled; `SKIP_RDD_GATE=1` audit records `rdd: enabled`. (R11)

## Phase 4: PR3 — `internal/git` + Review Wave

Landed as PRs #88–#92 (merged to master, `internal/review` at zero debt).

- [x] 4.1 `internal/git/exec.go`: `Run`, `TopLevel`, `ResolveGitDirs`, `RevParse`, `NewCommand`; raw stderr; `DetectGitDirs()` byte-compatible; `os.IsNotExist` no panic. (R4)
- [x] 4.2 Golden: `NewCommand` env (`GIT_*` stripped, `LANG=C`, `--no-pager`) + argv. (R4)
- [x] 4.3 RED (TM-2): relative/absolute/foreign-cwd/subdir → same repo; `-C` from absolute root, never caller cwd. (R4,TM-2)
- [x] 4.4 Migrate `internal/review/{store,gate,rdd_helpers,capture,finalize,risk,frozen_inspector,reconcile,gate_diagnostics}.go`; delete baseline entries same PR. (R4)
- [x] 4.5 Delete `internal/review/convergence.go` + tests (zero callers): absence satisfies R12–R15. (R12–R15)

## Phase 5: PR4 — Gatekeeper Store-Aware

- [x] 5.1 RED store-matrix test: repo-/change-relative resolve; declared `Path` never proof; missing canonical names absolute path; `""` → skipped with reason. (R10)
- [x] 5.2 `Gatekeeper(..., store ArtifactStore, ...)`: store from `ResolvePreflightPrefs`; canonical stat per store; no double-prefix join. (R10)
- [x] 5.3 Update callers (`cli_sdd.go`, preflight `both`); `checkContract` empty-list skip; delete migrated entries. (R10)

## Phase 6: PR5 — `cmd/biggz` Wave

- [x] 6.1 RED (TM-3): write ops argv verbatim; empty index; `add -A`+commit; unstaged mutation invisible to `write-tree`, visible to workspace snapshot. (TM-3)
- [x] 6.2 RED (TM-4): bare remote — first push sets upstream; repeat; failure stderr raw. (TM-4)
- [x] 6.3 RED (TM-5): `getChangedFiles`/`gh pr create` goldens; `gh` untouched, git parts routed. (TM-5)
- [x] 6.4 Migrate `cmd/biggz/{pr,cli_util,cli_bigmem,cli_codegraph,export,sdd_new}.go`; delete same-PR entries. (R4) — complete: `pr`+`cli_util` landed (#97, debt 31→18); `cli_bigmem`, `cli_codegraph`, `export`, `sdd_new` landed in the follow-up slice (debt 18→13).

## Phase 7: PR6 — Final Wave, Debt Zero

- [x] 7.1 Migrate `internal/release/release.go`, `internal/install/install.go`, `internal/sddattempt/cas_store.go`; dedupe rev-parse pairs. (R4) — complete: all three routed through `internal/git` (`Run`/`RevParse`/`ResolveGitDirs`); `ResolveGitDirs` dedupes the install rev-parse pair; raw stderr kept for `isNotGitRepoError`.
- [x] 7.2 Delete dead `internal/doctor/git.go:71-72`; keep 5 doctor boundary entries live (stale → fail). (R3) — complete: dead probe removed; the 5 `declared-boundary` doctor entries stay live (git.go line refreshed 76→69).
- [x] 7.3 Debt = 0: scan reports zero `go-git-spawn`, boundary entries remain; never an absence check. (R3) — complete: `gitexec -root .` → exit 0, self-check 9 sites, 0 debt, 7 boundary, baseline matched.

## Phase 8: Verification

- [x] 8.1 `go test ./... -count=1 -timeout 180s`, `go vet`, `gofmt -l`; CI once: guards observe own status, zero `rg`, no `|| true`. (all,R5,R7–R9) — complete with one declared deviation: the local suite ran `-timeout 600s`, not the literal 180s, because `internal/review` measures 288.5s on Windows against the runner budget fixed in #94 (the 180s literal would red by design on Windows). 61 packages ok, 0 FAIL; `go vet ./...` clean; `gofmt -l .` reports `internal/review/rdd_helpers.go` — the already-recorded `gofmt-not-toolchain-stable` defect (gofmt 1.26.1 vs the committed 1.27 form), not a regression of this change; CI's Go Format job (which floats on `stable`) is the authority and passes. CI once on PR #99: 18/18 checks pass, compiled checker step green with zero `rg` and no `|| true`.
- [x] 8.2 Line 115 scenario true: violation fixture fails CI, allowlisted tree passes. (R5) — complete: in a detached worktree at branch HEAD a planted `internal/release/zz_probe.go` (`exec.Command("git", …)`) made the checker report `go-git-spawn internal/release/zz_probe.go 5` and exit 1 (the CI step would fail); the same tree with the file removed exits 0 with `0 debt, 7 boundary, baseline matched`. Worktree removed afterwards; the working tree was never modified.

Coverage: R1→1.1–1.3,1.7; R2→1.4; R3→1.5–1.6,7.2–7.3; R4→4.1–4.4,6.4,7.1; R5→2.1,8.2; R6→2.2–2.3; R7→2.1,2.4,2.6; R8–R9→2.5; R10→5.1–5.3; R11→3.1–3.2; R12–R15→4.5; TM-1→1.1; TM-2→4.3; TM-3→6.1; TM-4→6.2; TM-5→6.3.
