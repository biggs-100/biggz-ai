# Design: 2026-08-29-ola1-gentle-hardening — Gentle Hardening Ola 1

## Technical Approach

Verbatim gentle-pi ola 1 port split into 2 stacked PRs (<400 lines each slice). PR1 delivers L1 4-source background policy + L3 scoped surfaces + L2 skill guide/lint. PR2 delivers L4 offline provider contract pin + manifest + CI gates. No new review authority, no BigMem, no network in verification paths. Spec deltas: `specs/runtime`, `specs/orchestrator`, `specs/skills`, `specs/review` (8 requirements, 16 scenarios) — see [Spec References](#spec-references).

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| **L1 Policy 4-source** `resolveBackgroundSubagentsPolicy` | A: toy capability probe B: `project>global>env>default off` fail-closed | A not source-aware, env overrides global incorrectly | **B** — `parseBackgroundSubagentsPolicyFile` per source, `gentleAiConfigHome` resolves `~/.pi/agent`, malformed→off no fallback, `renderBackgroundSubagentsReport` exposes source/policy/cap/malformed |
| **L1 Delegate shape** `background.go` | A: duplicate resolver B: delegate to `pi` resolver | A drift | **B** — `BackgroundPolicy`/`BackgroundResolution` thin wrappers, doc keeps scheduling-only |
| **L3 Path scoping** `isTaskScopedRepositoryRelativePath` | A: full-path glob reject B: first-segment glob only (gentle-pi) | A rejects valid `src/*.go` vs spec | **B** — normalize `\`→`/`, reject empty/absolute/`~`/whitespace/`..`, strip `./`, first segment `*?[]{}` reject; `src/*.go` deviation documented |
| **L3 Enforcement heuristic** `ShouldEnforceScopedSurfaces` | A: always enforce B: `fileCount>=4` | A noisy on small PRs | **B** — 3 allow, 4 per-path enforce + `WRITER_EDIT_SURFACE_REJECTION`; local `ScopedSurfaceRejection` avoids import cycle `sdd→orchestrator` |
| **L2 Lint buckets** `LintSkill` | A: single limit B: 180–450 pass / 450–1000 warn / >1000 fail | B matches gentle-pi style contract | **B** — `CountTokens=len(fields)`, frontmatter `description` single-line quoted ≤250 with `Trigger:`; `check-skill-lint.mjs` maps FAIL 1 / WARN 2 / pass 0 |
| **L4 Contract pin** | A: fetch + compare B: offline SHA256 vs lock | A network, flaky | **B** — `provider-contract.lock.json` 44-file map, `check-provider-contract.mjs` + `VerifyProviderContract` SHA256 hex, `walk` + `relative`, no fetch; 1-byte drift → exit 1 |
| **L4 Manifest** `verify-package-files.mjs` | A: reuse contract script B: separate manifest verify | B isolates concerns | **B** — sorted `relative` walk, unlisted/missing → exit 1 |
| **CI gate** `ci.yml` | A: append to `test` B: parallel jobs after `format` | A serializes | **B** — jobs `skill-lint` (node) + `provider-contract` (node+go) `needs: format` |

## Spec References

- `specs/runtime/spec.md` — 4-source policy and background delegate
- `specs/orchestrator/spec.md` — scoped path validation and fileCount guard
- `specs/skills/spec.md` — guide and lint token buckets
- `specs/review/spec.md` — offline contract/manifest pin and CI

Requirements mirror `tasks.md` 17 tasks (phases 1–4) with GWT scenarios.

## Data Flow

**L1 4-Source Resolution:**
```
cwd/.pi/settings.json → parseBackgroundSubagentsPolicyFile (project)
~/.pi/agent/settings.json → gentleAiConfigHome → parse (global)
env BIGGZ_BACKGROUND_SUBAGENTS / PI_BACKGROUND_SUBAGENTS → (env)
default off
→ resolveBackgroundSubagentsPolicy(cwd,opts) picks first present in order project>global>env>default
→ malformed file → {policy:"off", malformed:true} no fallback to next source
→ renderBackgroundSubagentsReport(source,policy,cap,malformed) for status
→ adapter.load delegates to resolve().policy, keeps Resolve* wrappers
→ opencode/background.go BackgroundPolicy/Resolution delegate to pi resolver
```

**L3 Surfaces:**
```
tasks.md surfaces section → allowedEditSurfacesHeadingRe + headingRe split
→ hasTaskScopedAllowedEditSurfaces(tasksText, ws) scans per-surface path via isTaskScopedRepositoryRelativePath
→ isTaskScoped: \→/, reject empty/absolute/~ / whitespace / ".." segment / glob in first segment
→ rejectUnscopedBoundedWriterDispatch(task, fileCount):
   fileCount<4 → allow
   fileCount>=4 → per-path check, reject if not task-scoped → ScopedSurfaceRejection{Block:true, Reason:WRITER_EDIT_SURFACE_REJECTION}
→ sdd/status.go ShouldEnforceScopedSurfaces(fileCount) + ValidateBoundedWriterSurfaces(input,fileCount) pre-dispatch guard
```

**L2 Skills:**
```
SKILL.md → extractFrontmatter("---\n") → validateFrontmatter(description single-line quoted ≤250, contains Trigger:)
→ body → CountTokens(fields) → 180-450 pass, 450-1000 warn, >1000 fail
→ LintSkill returns (tokens, diags, err), HasHardFailure/HasWarning helpers
→ check-skill-lint.mjs: findSkills(skills/, internal/assets/skills/) → lintFile per SKILL.md → exit 0/1/2
```

**L4 Provider Contract:**
```
contracts/review-integration/v1/** + v2/** → sha256 per file → provider-contract.lock.json {rel:hex}
→ check-provider-contract.mjs: walk roots, sha each, compare map vs lock, drift/unlisted → stderr + exit 1, pass → "check passed N files"
→ internal/contracts/verify.go VerifyProviderContract(lockPath,root) same logic, WalkDir + Rel + ToSlash, hex
→ verify-package-files.mjs: sorted walk vs listed set, missing/unlisted → exit 1
→ CI: skill-lint (node check-skill-lint.mjs) + provider-contract (node check-provider-contract.mjs + node verify-package-files.mjs + go test contracts)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `docs/skill-style-guide.md` | Created | 6-section style contract ported from gentle-pi (Purpose, When, Structure, Frontmatter, Writing rules, Decision gates, Output contract, Registry) |
| `internal/skills/lint.go` | Created | `CountTokens`, `LintSkill`, `extractFrontmatter`, `validateFrontmatter`, `HasHardFailure/HasWarning` |
| `scripts/check-skill-lint.mjs` | Created | Node wrapper delegating to Go semantics, recursive `findSkills`, exit 0 pass / 1 fail / 2 warn |
| `internal/agents/pi/adapter.go` | Modified | `parseBackgroundSubagentsPolicyFile`, `resolveBackgroundSubagentsPolicy(project>global>env>default off)`, `gentleAiConfigHome`, `renderBackgroundSubagentsReport`, `load→resolve().policy`, keep `ResolveBackground*` wrappers |
| `internal/opencode/background.go` | Modified | `BackgroundPolicy`/`BackgroundResolution` delegate to `pi` resolver, scheduling-only doc |
| `internal/orchestrator/surfaces.go` | Created | `isTaskScopedRepositoryRelativePath`, `hasTaskScopedAllowedEditSurfaces`, `rejectUnscopedBoundedWriterDispatch` + `WRITER_EDIT_SURFACE_REJECTION`, regexes `allowedEditSurfacesHeadingRe`, `headingRe`, `boundedWriterAgents` map |
| `internal/sdd/status.go` | Modified | `ShouldEnforceScopedSurfaces(fileCount>=4)`, `ScopedSurfaceRejection`, `ValidateBoundedWriterSurfaces` guard |
| `internal/agents/pi/adapter_test.go` | Created | 4-source precedence, malformed fail-closed, global off beats env on |
| `internal/orchestrator/surfaces_test.go` | Created | Reject `../x` `/etc` `~/x` `*.go` `a[0]` `a b/c`; accept scoped + `./`; 3 allow / 4 per-path rejection |
| `internal/skills/lint_test.go` | Created | 300 pass, 1001 fail, missing trigger fail, 600 warn |
| `internal/sdd/status_guard_test.go` | Created | `ShouldEnforce 3 allow / 4 enforce` |
| `scripts/check-provider-contract.mjs` | Created | Offline SHA256 drift check vs lock for v1+v2, exit 1 on drift/unlisted |
| `scripts/verify-package-files.mjs` | Created | Offline sorted manifest verify, unlisted/missing → exit 1 |
| `contracts/review-integration/provider-contract.lock.json` | Created | SHA256 lock 44 files (42 v1 + 2 v2) |
| `contracts/review-integration/v2/schemas/contract.schema.json` | Created | v2 schema ported from v1 with v2 $id/const |
| `contracts/review-integration/v2/fixtures/contract.fixture.json` | Created | v2 fixture validating against v2 schema |
| `internal/contracts/verify.go` | Created | `VerifyProviderContract` offline SHA256 pin verification |
| `internal/contracts/verify_test.go` | Created | 1-byte drift fails, exact pins pass, offline no fetch |
| `.github/workflows/ci.yml` | Modified | Jobs `skill-lint` + `provider-contract` after `format` |

## Interfaces / Contracts

```go
// L1
func parseBackgroundSubagentsPolicyFile(path string) (policy string, malformed bool)
func resolveBackgroundSubagentsPolicy(cwd string, opts ResolveOpts) BackgroundResolution // project>global>env>default off
func gentleAiConfigHome(homeDir string) string // ~/.pi/agent
func renderBackgroundSubagentsReport(r BackgroundResolution, cap string) string
func (a *Adapter) ResolveBackgroundSubagentsPolicy(cwd string) (string, bool) // wrappers
// L3
func isTaskScopedRepositoryRelativePath(value string) bool
func hasTaskScopedAllowedEditSurfaces(tasksText, workspaceRoot string) bool
func rejectUnscopedBoundedWriterDispatch(task map[string]any, fileCount int) *ScopedSurfaceRejection
const WRITER_EDIT_SURFACE_REJECTION = "Parent must derive or map narrow repository-relative allowed edit surfaces..."
func ShouldEnforceScopedSurfaces(fileCount int) bool // >=4
func ValidateBoundedWriterSurfaces(input map[string]any, fileCount int) *ScopedSurfaceRejection
// L2
func LintSkill(path string) (tokens int, diags []string, err error)
func CountTokens(body string) int
func HasHardFailure(d []string) bool; func HasWarning(d []string) bool
// L4
func VerifyProviderContract(lockPath, root string) error // drift/unlisted → error
```

```js
// scripts
// check-provider-contract.mjs: walk v1+v2, sha256 vs lock, drift → exit 1
// verify-package-files.mjs: sorted relative walk vs lock keys, mismatch → exit 1
// check-skill-lint.mjs: findSkills → lintFile → exit 0/1/2
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit L1 | 4-source precedence, malformed fail-closed, env fallback | `TestResolveBackground` with `t.TempDir` isolated `HOME`/`cwd`, malformed JSON → off+malformed true, global off beats env on |
| Unit L3 | Path scoping, heading parsing, guard | `TestIsTaskScoped`, `TestHasTaskScoped`, `TestRejectUnscoped`, `TestShouldEnforce` — reject `../`/abs/`~`/space/glob, accept `./` normalized, 3 allow/4 enforce |
| Unit L2 | Token buckets, frontmatter | `TestLintSkill` 300 pass, 1001 fail, missing trigger fail, 600 warn, unquoted fail |
| Integration L4 | Drift detection offline | `TestVerifyProviderContract` temp dir: exact pins pass, 1-byte drift fails, missing/unlisted fails, no network |
| E2E | `go vet ./...` PASS, `go test ./... -count=1 -timeout 180s` (flaky `TestReadLoopLarge` pre-existing ignored), `gofmt -l` clean, `node check-provider-contract.mjs` 44 files pass, drift → exit 1 → restore pass |

## Threat Matrix

| Boundary | Applicable | Response | Test |
|----------|------------|----------|------|
| Malformed policy fallback | Yes — silent override | Fail closed `off` no fallback | malformed file → off+malformed true |
| Unscoped writer glob | Yes — overbroad edits | `isTaskScoped` + fileCount>=4 guard + rejection constant | `*.go`/`a[0]` rejected, 4-file enforce |
| Skill prompt injection via description | Yes — unquoted multiline | Quoted single-line ≤250 + trigger required | unquoted/multiline → FAIL |
| Contract drift supply-chain | Yes — pinned files drift | Offline SHA256 pin, 1-byte drift → exit 1 | drift probe |
| Manifest unlisted file | Yes — phantom file | Sorted walk vs lock keys, unlisted → fail | unlisted probe |
| Import cycle sdd→orchestrator | Yes — build | Local rejection type in `sdd/status.go` | `go vet` PASS |

## Migration / Rollout

No migration. Stacked-to-main, `biggz sdd-attempt acquire` per PR (max-attempts 5, max-lines 400). Revert PR2→PR1 via `git revert`. Grandfathered skills >1000 remain FAIL until follow-up refactor, not blocking PR1 gate.

**PR Boundaries:**
- PR1 L1+L3+L2+guide: ~340 lines, `adapter.go` + `surfaces.go` + `lint.go` + `skill-style-guide.md` + tests, rollback reverts PR1 files.
- PR2 scripts+pin+CI: ~99 prod lines, `scripts/*.mjs` + `contracts/**` + `internal/contracts/verify.go` + `ci.yml`, rollback reverts PR2 files isolated from PR1.

Gates per PR: `go vet`, `gofmt`, focused `go test` + `node scripts/*.mjs` + full `go test ./...`.

## Open Questions

- [x] `src/*.go` must-reject vs first-segment rule: implemented first-segment-only per task, documented deviation.
- [ ] Grandfathered skills >1000: follow-up refactor or raise threshold for internal/assets/skills?
