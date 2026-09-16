# Exploration: fix-false-green-guards

## Current State

**The invariant**: `internal/git` is declared the sole `exec.Command("git")` owner (comment `// tui-sanitize:` at `ci.yml:219`). The only enforcement is the `forbid-git` CI job. Reality: **57 guard-visible git-exec sites across 23 files outside the wrapper, plus 9 sites the guard regex cannot even see**.

**The actual wrapper surface** (`internal/git/git.go` + `cleanup.go`):

- Exported: `DetectGitDirs() (commonDir, gitDir string)` — no `-C` param, no abs-path resolution; `GitStatus(dir)`; `GitDiff(dir, args...)`; `FetchPrune`, `ListGoneBranches`, `IsMergedTo`, `ListWorktrees`, `PruneBranches`, `PruneWorktrees`.
- Unexported: `runGitOutput(ctx, cwd, args...)` — the generic runner, **not reachable by any other package**.
- What it does NOT cover: generic exported runner with `-C`, rev-parse family (toplevel/short/tree/verify), abs-path git-dir resolution, env/cap-controlled spawning, write ops.

**Because of this, every consumer re-implemented the same primitives**: 6 copies of "resolve git-dir/common-dir via rev-parse" (`sdd/status.go`, `review/gate.go` + `rdd_helpers.go`, `review/store.go`, `sdd/edit_authority.go`, `sddattempt/cas_store.go`, `install.go`, `cli_util.go`, doctor), 7 copies of `rev-parse --show-toplevel`, 3 copies of the `repoArgs` closure + tree resolution + `diff --numstat/--raw` block (`capture.go`, `finalize.go`, `risk.go`), and **2 identical implementations in the same package** (`gate.go:revParseRepoDir` vs `rdd_helpers.go:revParseRDDDir`; `detectRDDDirs` vs `ResolveRDDDirs`).

Precedent for the fix already exists: `internal/tui/screens/status.go:95` was routed to `git.DetectGitDirs()` during tui-sanitize ("delegates to the single wrapper"). Only 3 packages import `internal/git` today (`cli_cleanup.go`, `tui/screens/status.go`, `doctor/stale_branches.go`); no import-cycle risk (`internal/git` imports only `internal/platform`).

**The guard inventory** (`.github/workflows/ci.yml`, all steps use idiom `if W | grep -q .; then fail; fi` with `2>/dev/null`, no `pipefail`):

| # | Job / Step | Line | Domain | Primary check exists? | Failure mode |
|---|-----------|------|--------|----------------------|--------------|
| 1 | `forbid-git` / `Forbid exec.Command git outside internal/git` | 223 | git-exec invariant | **NO — rg is the only check** | Vacuous: scanner dead ⇒ green |
| 2 | `no-fmtSprintf` / `No fmt.Sprintf in lens prompts` | 247 | lens prompts | NO (broken Go replica exists, see below) | Vacuous: scanner dead ⇒ green; **14 real violations sleeping in `readability/lens.go`** |
| 3 | `lint-no-source-grep` / `Fallback rg guard for source-grep` | 292 | source-grep | YES — `go vet -vettool=nosourcegrep` runs BEFORE it and is AST-complete | Vacuous but redundant (step is a weaker duplicate of the primary) |
| 4 | `lint-no-source-grep` / `Ban mock.module` | 302 | mock.module | YES — `tools/nosourcegrep/analyzer.go` already flags `mock.module` (`isMockModuleCall` + SelectorExpr + BasicLit) | Vacuous but redundant |

**Second broken guard copy (Go)**: `internal/review/lens/no_fmt_guard_test.go:83 TestCIGuard_CurrentLensClean` reads repo-relative paths (`"internal/review/lens/readability/lens.go"`), but `go test ./...` sets cwd to the package dir (`internal/review/lens`) → every `os.ReadFile` fails → `continue` at line 95-97 → **silent pass**. Meanwhile `readability/lens.go` carries 14 lines with `fmt.Sprintf`, **0 of them allowlisted** — the test would legitimately FAIL if the path bug were fixed. Related: `docs/testing-guidance.md:64,66,75,97` documents the rg steps as the enforcement contract — a mirror that must move with any guard change.

**Adjacent same-shape risks found (not among the four, lower severity)**: `complexity` job producer `go run gocyclo ... || true` → empty output ⇒ "no violations" (only `::warning::` if unpinned); `format` job `unformatted=$(gofmt -l .)` → missing tool ⇒ empty ⇒ pass (low risk, toolchain-provided); installed `pre-push.tmpl` hook uses `biggz rdd status 2>/dev/null | grep -q "RDD Status: enabled"` → producer dead ⇒ no block. Counter-example of the right pattern already in-repo: `release-checksums` asserts exact counts (`tar_count -ne 5` ⇒ exit 1) — fail-closed by expected-count, not by absence-of-output.

## Affected Areas

**Complete classification of the 57 guard-visible sites (23 files). Classes: (a) routable to wrapper; (b) legitimately outside; (c) dead/dedup — deletion first.**

| File | Sites (line) | Class | Action |
|------|--------------|-------|--------|
| `cmd/biggz/cli_bigmem.go` | 970, 1236 | (a) | 2 inline `rev-parse --show-toplevel` project-root copies → `git.TopLevel()` |
| `cmd/biggz/cli_codegraph.go` | 153 | (a) | `codegraphGitTopLevel(path)` → `git.TopLevel(path)` (with `-C`) |
| `cmd/biggz/cli_util.go` | 52, 57 | (c)+(a) | local `detectGitDirs()` is a byte-for-byte semantic twin of the wrapper → DELETE, call `git.DetectGitDirs()` (exact tui precedent) |
| `cmd/biggz/export.go` | 98 | (a) | `git log --oneline [--since]` → `git.Run(ctx, cwd, "log", ...)`; LIVE (called at `export.go:49`) |
| `cmd/biggz/pr.go` | 112, 184, 274, 277, 280, 329, 480 | (a) | show-toplevel ×2 → `TopLevel`; 184 → `GitStatus`; 274–280 → composite `ChangedFiles` helper; 480 → `ShortHEAD`-style helper. **Plus 4 guard-INVISIBLE write ops** at 191/197/204/210 (`runCmd("git", checkout/add/commit/push)`) — write surface needs an explicit decision (named mutators vs allowlist) |
| `cmd/biggz/sdd_new.go` | 146 | (c)+(a) | `detectProjectRoot()` duplicates `pr.go:112` verbatim → `git.TopLevel()` |
| `internal/doctor/git.go` | 71 | **(c) DEAD** | `probeCmd` is constructed + `EnsureCommandDir(probeCmd)` called but **never Run/Output** — delete both lines; the comment is misleading |
| `internal/install/install.go` | 372, 375 | (c)+(a) | `ensureRDDEnabled` rev-parse pair is EXACTLY `DetectGitDirs()` semantics → delete 4 lines, call wrapper |
| `internal/release/release.go` | 36, 43, 49, 55, 83, 99 | (a) | `GitStatus` exists for 36; 43/49/99 → rev-parse helpers; 83 tag → explicit mutator; 55 describe → generic Run. Package fully git-dependent by design, LIVE via `cli_misc.go` |
| `internal/review/capture.go` | 527, 534, 538 | (c)+(a) | `repoArgs` closure ×3 + tree resolve + `diff --raw -z`; consolidate into shared wrapper helpers; delete closure |
| `internal/review/convergence.go` | 34, 42, 45, 72 | (a)+BUG | Route + **production fail-open found**: `SnapshotVerificationSubject` swallows git errors (`err == nil` guards) → empty trees → two broken snapshots compare EQUAL → mutation undetected. Must fail closed when git fails |
| `internal/review/finalize.go` | 294, 302, 307, 312 | (c)+(a) | `DeriveOriginalChangedLines` is a near-copy of `risk.go:DeriveRiskInput` (same trees + numstat block) → unify |
| `internal/review/frozen_inspector.go` | 316, 571, 573 | **(b)/(a-variant)** | Hardened isolated runner: env stripped of `GIT_*`, `LANG=C`, byte caps, `--no-pager -C`. Wrapper cannot express this today. Either relocate the spawn primitive into `internal/git` (option: env/limits-aware `NewCommand`) or allowlist with documented rationale — **decision item** |
| `internal/review/gate.go` | 221, 1200, 1225 | (a)+(c) | `ScopeDiff` → `git.Run`; `gitIn` (1225) is a `runGitOutput` twin → `git.Run`; `revParseRepoDir` (1200) → keep ONE of the two identical impls |
| `internal/review/gate_diagnostics.go` | 47, 58, 76, 87 | (a) | diff-tree / shortstat / rev-parse / rev-list → `git.Run` |
| `internal/review/rdd_helpers.go` | 370 | (c) | `revParseRDDDir` (365) is IDENTICAL to `gate.go:revParseRepoDir` (1195); `ResolveRDDDirs` (356) vs `detectRDDDirs` (gate.go:1184) likewise → single implementation (both live: `cli_review.go` ×6, `gate.go:679`, `next_transition.go:76`) |
| `internal/review/reconcile.go` | 318 | (a) | `detectProjectName` show-toplevel → `git.TopLevel` |
| `internal/review/risk.go` | 403, 409, 414, 419 | (c)+(a) | same unify as finalize |
| `internal/review/store.go` | 440, 585 | (a) | `resolveGitCommonDir`/`resolveGitDir` (abs + `EvalSymlinks`) → wrapper needs a canonical resolver; LIVE (`store.go:87`, `gate.go:185`, `compact_burn.go:67`, `authority.go:95`, `lineage_identity.go:107`, `lineage_resolve.go:103`) |
| `internal/sdd/complexity_gate.go` | 110 | (a) | `gitOut` helper (3 callers incl. `status --porcelain --untracked-files=all`) → `git.Run`; LIVE via `gatekeeper.go:136` |
| `internal/sdd/edit_authority.go` | 246 | (a) | `gitCommonDirForPath` (abs + symlinks + memo) → wrapper resolver; memo stays local |
| `internal/sdd/status.go` | 821, 828 | (c)+(a) | `detectGitDirs(workspaceRoot)` duplicate family → single wrapper resolver; LIVE (`status.go:849`) |
| `internal/sddattempt/cas_store.go` | 287 | (a) | `resolveCloneStore` common-dir discovery; **constraint**: `isNotGitRepoError` classifies via git's stderr wording — the wrapper must return raw output/error, not swallow it |

**Guard-INVISIBLE sites the regex `exec\.Command.*git` misses (9, exact)**: `pr.go:191,197,204,210` (`runCmd("git", ...)` write ops: `checkout -b`, `add -A`, `commit`, `push`); `internal/doctor/review.go:82,93,251`, `internal/doctor/version.go:86`, `internal/doctor/git.go:76` (`c.execFn("git", ...)` — injectable diagnostics seam). Doctor's usage is **(b) legitimately injected** (the checker must simulate git present/absent/failing); the pr.go write ops are **(a) real debt** currently unenforced. True total: 57 visible + 9 invisible = 66 sites; only 23+3 files visible to tooling today.

**Touched by the fix**: `.github/workflows/ci.yml` (4 steps/jobs), `tools/nosourcegrep/**` (singlechecker → multichecker if Approach B), `internal/review/lens/no_fmt_guard_test.go`, `docs/testing-guidance.md`, plus the 23 migration files above.

## Root-Cause of the Vacuous Scan

**Verdict: (i) `rg` is genuinely missing on the runner.** Evidence chain:

1. Mechanism: `rg` absent → shell exits 127 instantly, message goes to `2>/dev/null`; `grep -q .` receives empty stdin → exit 1 → `if` false → "pass". `set -e` does NOT fire because the failure is inside an `if` condition (errexit exemption). Reproduced verbatim by the orchestrator with 57 matches in-tree.
2. Timing: **1.35 ms** between the two echoes. Fork+exec of `rg` alone costs ≈3–10 ms warm; a real scan of ~60K lines with 4 globs costs tens of ms. 1.35 ms is below the floor of *any* successful `rg` invocation — consistent only with "command not found" being rejected by the shell.
3. Environment: no workflow step installs ripgrep (only `apt-get install -y minisign` at `ci.yml:391`); ubuntu-24.04 runner inventory has no ripgrep.
4. Consistency: if `rg` WERE present, the guard would fail today (57 matches ⇒ `grep -q .` exit 0 ⇒ `exit 1`); master is green. The archive report `2026-08-27-tui-sanitize` even recorded "48 matches" against this exact command while declaring the guard "correctly configured to hard-fail" — locals had rg, the runner did not.

Rejected alternatives: `--glob` combination is valid rg syntax; shell is bash with `set -e` (insufficient, not causal); `grep -q` semantics are correct-if-scanner-lives. All are *concealers* of the true cause, not the cause. Secondary latent fail-open confirmed: even with rg installed, any scanner failure (glob error, I/O) ⇒ empty ⇒ green, because `pipefail` is absent and `2>/dev/null` hides diagnostics.

**Disambiguating tests available to the orchestrator**: (a) `command -v rg || echo RG-MISSING` as a probe step (decisive); (b) locally, with rg installed, run the guard line on master → must print the 57 hits and the step must take ≫1.35 ms (proves the idiom itself works, isolating the environment); (c) add `set -o pipefail` to a copy of the step and observe rg's 127 surfacing — proving pipefail alone flips it fail-closed.

**Same-failure-mode inventory (complete)**: the 4 rg steps above + `no_fmt_guard_test.go` (path bug) + `complexity` job (`|| true` producer) + `format` job (`$(gofmt -l .)` substitution) + installed `pre-push.tmpl` rg-shaped checks. Over-fix guard: steps 3 and 4 need NO new machinery — the Go primary (`go vet -vettool=nosourcegrep`) already covers both domains AST-complete; their correct fix is deletion, not repair.

## Approaches

| | **A: keep rg, install + fail-closed** | **B: Go-based checker (extend vettool pattern)** | **C: hybrid + positive control** |
|---|---|---|---|
| Shape | `sudo apt-get update && apt-get install -y ripgrep` (minisign precedent) + `set -euo pipefail` + `command -v rg` preflight + explicit `exit 1` | New analyzer in `tools/` (flag `exec.Command`/`exec.CommandContext` with literal `"git"` **and any call passing literal `"git"` as command-position arg**, catching `runCmd`/`execFn`); `singlechecker` → `multichecker`; CI: `go build -o /tmp/guard … && go vet -vettool=/tmp/guard ./...` | Keep rg steps; add sentinel (e.g. `rg --files | wc -l` sanity or known-count assertion) |
| Pros | Smallest diff (~30 YAML lines); fast | Compiled + versioned in-repo; exit-code fail-closed by construction; sees indirection rg never will; extends an already-exercised pattern (`nosourcegrep` + `.golangci.yml` custom + `analysistest` fixtures); no external binary; same tool on 3 OSes | Cheap; keeps local ergonomics |
| Cons | External mutable dependency (image drift, apt flake); idiom remains fail-open unless *every* future step re-adds preflight+pipefail+explicit-exit; **cannot fix the guard's blind spots** (runCmd/execFn indirection; multi-line `exec.Command` formatting evasion) | Bigger diff (~250–400 LOC incl. tests); `go vet` runs package-by-package — needs in-repo allowlist of the 66 transitional sites as explicit Go code | More machinery, less certainty ("scanner ran" ≠ "scanner scanned correctly"); sentinel itself is rot-prone shell; still rg-dependent; known-count couples guard to debt inventory |
| Effort | Low | Medium | Low–Medium |
| Silent degrade again? | **Yes** — external binary + fail-open idiom family re-copyable | **No structurally** — build failure = step failure = red; no external dependency | Partially — two shell layers to keep honest |

## Recommendation

**Adopt B, scoped, with two surgical deletions and one repaired Go test** — it is the only approach whose failure mode is "red CI", satisfying `issue-root-resolution`'s ranking #2 (static guard-ratchet) while shrinking the system:

1. **forbid-git** → Go analyzer in `tools/` (multichecker), covering both literal-arg and indirection shapes; CI step becomes `go build && go vet -vettool ./...` — no rg.
2. **source-grep fallback + mock.module steps** → **DELETE** (strictly weaker duplicates of the nosourcegrep primary that already runs first; deletion > repair per the "shrink the system" rule).
3. **no-fmtSprintf** → replace the rg step with the repaired Go test (`no_fmt_guard_test.go`: repo-root-anchored paths, hard-fail on unreadable file), after resolving the 14 sleeping `readability/lens.go` sites (they look like finding-ID/evidence strings — same category the other lenses mark; **D-item**: mark vs refactor).
4. **Sequencing (auto-chain, 400-line budget)**: PR1 = guards fail-closed + transitional explicit allowlist (current debt frozen, new violations fail); PR2..N = per-package migration waves where each PR routes to an extended `internal/git` surface (`Run(ctx, dir, args…)`, `TopLevel(repo)`, `ResolveGitDirs(repo)`, rev-parse helpers, `NewCommand(env, repo, args…)` for the frozen inspector) and DELETES the duplicates (`repoArgs` ×3, `revParseRDDir`/`revParseRepoDir` twins, `detectGitDirs` ×3, dead `probeCmd`); final PR = empty allowlist. Every wave must not reintroduce the shape: guard is hard-on from PR1.
5. **D-items for the maintainer**: (1) readability/lens.go markers; (2) frozen_inspector disposition (relocate spawn into wrapper vs allowlist with rationale); (3) write-op surface in pr.go (`checkout/add/commit/push`) — named wrapper mutators vs allowlist; (4) transitional allowlist acceptance (ratchet) vs all-at-once migration (would need a giant PR — breaks the 400-line budget).

## Risks

- **Flipping the guards fail-closed makes the correction visible and RED**: 57 + 9 live sites, 14 lens violations, unless PR1 carries the frozen allowlist. Without sequencing, the change blocks itself.
- **The debt is growing while hidden** (48 → 55 → 57); every week the allowlist freeze is delayed adds migration cost. The `forbid-git` invariant is the least-enforced and most-violated rule in the repo.
- **`convergence.go` fail-open is production code, not just CI**: a broken/missing git silently yields "converged" verification snapshots — remediation must include the fail-closed fix, not only relocation.
- **Guard blind spots must be closed by the new checker**, else the migration converges to a new false-green: `runCmd("git")` indirection and multi-line `exec.Command` formatting evade the rg regex.
- **Docs mirror drift**: `docs/testing-guidance.md` (and the `.golangci.yml` comment) pin the rg enforcement as the contract; they must be updated in the same PR or the mirror lies in the opposite direction.
- **Doctor package must NOT be naively routed**: `execFn` injection is the testing seam for "git missing" diagnostics; classify as legitimate-outside and let the new checker allowlist `internal/doctor` *indirection*, keeping the raw `exec.Command` sites forbidden there.

## Ready for Proposal

**Yes** — with the D-items above as the only needed human decisions. The mechanism map is complete (every claim anchored to file:line), the root cause is singular (rg absent + fail-open idiom family), and the fix shape is deletion-heavy (net line delta should be strongly negative after waves).

---

## Orchestrator Gatekeeper Verification

Independently re-verified at `3d4556f8` before persisting. **5 of 5 load-bearing claims CONFIRMED**; two corrections and two new findings.

| Claim | Verdict | Evidence |
|-------|---------|----------|
| `internal/doctor/git.go:71` `probeCmd` is dead code | ✅ CONFIRMED | `rg -n "probeCmd" internal/doctor/` returns only lines 71 and 72 — constructed, `EnsureCommandDir` called, never executed |
| `internal/review/convergence.go` swallows git errors | ✅ CONFIRMED | `if out, err := baseCmd.Output(); err == nil { subject.BaseTree = … }` (line 34) and the candidate twin (line 45) — error branch is empty, tree stays `""` |
| `TestCIGuard_CurrentLensClean` silently passes | ✅ CONFIRMED | `no_fmt_guard_test.go:86-97`: repo-relative paths + `if err != nil { continue }`; `go test` cwd is the package dir, so every read fails |
| `internal/git` has no generic exported runner | ✅ CONFIRMED | Exported surface = `DetectGitDirs()`, `GitStatus(dir)`, `GitDiff(dir, args...)` + `cleanup.go` functions; `runGitOutput` is unexported at `cleanup.go:144` |
| `repoArgs` ×3 + rev-parse twins | ✅ CONFIRMED | `capture.go:521`, `finalize.go:282`, `risk.go:393`; `gate.go:1184 detectRDDDirs`/`1195 revParseRepoDir` vs `rdd_helpers.go:356 ResolveRDDDirs`/`365 revParseRDDDir` |
| 57 visible + 9 invisible sites | ✅ CONFIRMED EXACT | visible = 57 (`rg -n 'exec\.Command.*git'` with the guard's own globs); invisible = 4 (`pr.go:191,197,204,210`) + 5 (`doctor`: `git.go:76`, `review.go:82,93,251`, `version.go:86`) = 9 |

**Correction 1 — the lens violation count is 14 lines, not ~11.** `rg -c "fmt\.Sprintf" internal/review/lens/readability/lens.go` = 14, and **0** of them carry `//lint:ignore no-fmtSprintf`. The sleeping debt is larger than reported.

**Correction 2 — two additional non-Go git spawns exist outside the guard's reach entirely.** A broader sweep for a literal `"git"` command argument returns 2 shipped assets the Go-regex guard can never see: `internal/assets/pi/codegraph-tools.ts:80` (`execFileSync("git", ["rev-parse","--show-toplevel"], …)`) and `internal/assets/pi/biggz-footer.js:422` (`run("git", ["status","--porcelain=v1", …])`). Both are host-side pi extensions, so the likely disposition is a documented scope boundary rather than migration — but the scope of "sole owner" must state it explicitly, or the invariant keeps a silent loophole. Two other sweep hits are NOT violations: `internal/release/release.go:29` and `internal/doctor/git.go:58` are `LookPath`, not spawns.

---

## Correction and Upstream Comparison (post-exploration)

A comparative pass against the two upstream references — `C:\Users\USER\Desktop\herramientas\gentle-ai` and `C:\Users\USER\Desktop\herramientas\gentle-pi` (biggz-ai's declared git `upstream` remote) — **refutes two claims in the analysis above**. Both are recorded here rather than silently rewritten; the original text stands as the trail.

### Correction 1 — `convergence.go` is NOT a production fail-open. It is unrouted dead code.

The analysis above calls it "a production fail-open, not just CI". **That is wrong.** Caller graph, verified: `SnapshotVerificationSubject` is referenced only at `convergence.go:27` (its own definition) and `convergence.go:65`; its sole caller is `Resnapshot` (`convergence.go:64`), which itself has **zero callers** anywhere in the repo (including tests). The whole module — `VerificationSubject`, `SnapshotVerificationSubject`, `Resnapshot` — never executes in production or in CI.

Severity drops accordingly: **latent shape defect in dead code**, not a live fail-open. Two independent defects remain real inside it:

1. `if out, err := baseCmd.Output(); err == nil { … }` with an empty error branch (lines 34 and 45) — a broken git yields `""` trees and `Resnapshot()` returns `nil` (converged).
2. **New**: for `projection == "workspace"`, `candCmd` is `rev-parse HEAD^{tree}` — the *identical* command used for the base. Workspace mutation is therefore invisible **even with a perfectly healthy git**. This is a logic defect independent of the fail-open.

Consequence for the change: the remedy is **delete or port**, not "harden production code". gentle-ai's counterpart is fail-closed and provides the exact port target (see Correction 3).

### Correction 2 — the `lint-no-source-grep` "Go primary" is ALSO inert in CI. The premise for deleting steps 3 and 4 is false.

The analysis above states: *"steps 3 and 4 need NO new machinery — the Go primary (`go vet -vettool=nosourcegrep`) already covers both domains AST-complete; their correct fix is deletion, not repair."* **That premise does not hold.**

`.github/workflows/ci.yml:273-282`:

```yaml
      - name: Run nosourcegrep vet (primary)
        run: |
          set -e
          if ! go vet -vettool=/tmp/nosourcegrep ./... 2>&1 | tee /tmp/vet.log; then
            echo "::error::nosourcegrep vet flagged source-grep"
            exit 1
          fi
          echo "nosourcegrep vet passed"
```

`set -e` is set but **`pipefail` is not**, and no workflow in this repo declares `shell:` or `defaults:` (verified: zero matches for `shell:|defaults:` across `.github/workflows/*.yml`), so GitHub's default `bash -e` applies. In a pipeline, the exit status is the **last** command's — here `tee`, which succeeds. `! 0` is false, so the error branch is never entered and the step prints `nosourcegrep vet passed` and exits 0. **A real vet violation is swallowed.**

So the blast radius of the defect family is larger than four `rg` steps:

| Check in `lint-no-source-grep` | Mechanism | Status |
|---|---|---|
| nosourcegrep vet ("primary") | `if ! go vet … \| tee` | **Inert** — `!` negates `tee`, not `go vet` |
| golangci-lint custom | `golangci-lint run … \| tee \|\| echo "::warning::"` | Advisory by construction (non-blocking `\|\|`) |
| Fallback rg guard for source-grep | `rg … 2>/dev/null \| grep -q .` | Inert (scanner absent) |
| Ban mock.module | `rg … 2>/dev/null \| grep -q .` | Inert (scanner absent) |

The whole job is decorative. Any plan that assumes "a compiled primary already guards this domain" must first prove the primary's wiring actually observes the tool's exit status.

### Upstream comparison — none of the five defect families is inherited

Both upstreams were checked for the same apparatus. Neither has `internal/git/`, `tools/`, `internal/sdd/gatekeeper.go`, `internal/review/convergence.go`, any `grep -q` guard in CI, any `|| true` guard step, or any `//lint:ignore` marker.

| Defect family | Classification | Upstream mechanism available to port |
|---|---|---|
| Broken `rg` guards (fail-open absence checks) | **Invented by biggz-ai** (rule and violation); both upstreams solved the *class* differently | Positive expected-set assertions (`go test -list` + throw, `jq -e` counts), compiled checkers (`go run ./internal/gofmtcheck`), ratchets whose **stale baseline is a failure**, skip→fail env gates |
| 66 unrouted git spawns | **Invented by biggz-ai** — no upstream invariant exists to violate | Only hardening mechanics: gentle-ai's `runGit*` funnel + typed errors + test seam (`snapshot.go:1885-2010`); gentle-pi's env sanitizer + fail-closed probes (`lib/review-repository.ts`) |
| Gatekeeper store-blind path bug | biggz-ai surface; **the identical class was already found and fixed upstream** | gentle-ai issue #2346 fixed exactly this in `baseStatus` by passing `ArtifactStore` as an input; upstream resolves canonical artifact names per store (`status.go:1544 resolveArtifactPaths`, `:1238 engramArtifactPaths`, `artifact_states.go`) and never trusts caller-declared relative paths |
| `convergence.go` fail-open | biggz-ai surface; upstream has a **fail-closed counterpart** | `internal/reviewtransaction/verification_contract.go` + `receipt.go:461 validGitTree` (`^([0-9a-f]{40}\|[0-9a-f]{64})$` **and** rejects the all-zero OID) plus error propagation on every snapshot build |
| 14 lens `fmt.Sprintf` lines | **Invented by biggz-ai** — rule and violation both | **None.** gentle-ai uses `fmt.Sprintf` freely for prompts (`internal/components/sdd/boundedreview.go:309,369,388,437`); no upstream rule exists. The invariant's own value is now an open question |

**Corrections to the guard inventory itself**: the four `rg` steps plus the inert vet primary plus the two decorative `complexity`/`format` producers constitute a **seven**-surface failure family, not four. Conversely, gentle-ai's own `windows-runtime` Stress step still carries one residual instance of the vacuous class (`go test -run '^…$'` with no `-list` probe), so the upstreams are not a clean-room model either — their *dominant* idiom is simply the correct one.

**Design consequence**: for families 1, 3 and 4 there is a proven upstream shape to copy instead of invent (positive expected sets, store-aware resolution, identity validation). For families 2 and 5 there is nothing upstream at all — those two are design decisions with no precedent, and family 5's rule may not earn its keep.
