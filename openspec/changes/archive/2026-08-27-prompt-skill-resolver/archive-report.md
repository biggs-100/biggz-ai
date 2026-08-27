# Archive Report: prompt-skill-resolver

**Archived**: 2026-08-27
**Change**: prompt-skill-resolver
**Mode**: interactive, openspec, auto-chain, 800 lines, stacked PRs 3 commits core 881 — `strict_tdd off`, `go test ./... -count=1 -timeout 180s`
**Artifact Store**: openspec — `openspec/changes/prompt-skill-resolver` → `openspec/changes/archive/2026-08-27-prompt-skill-resolver/` + `openspec/specs/prompt-skill-resolver/spec.md` source of truth
**Archived to**: `openspec/changes/archive/2026-08-27-prompt-skill-resolver/`
**Previous location**: `openspec/changes/prompt-skill-resolver/` (active)

## Summary

Completed prompt-skill-resolver — Prompt-as-File + `skill://` Resolver. Externalized review lens prompts from `fmt.Sprintf` to 6 static `.md` (`R1-R4/external/shared`) via `text/template {{.Var}}` (`missingkey=error`) embedded through `assets.FS //go:embed all:prompts/review` (oh-my-pi C parity `.md`+Handlebars→Go `text/template`). Added `internal/skillregistry/resolver.go` — 7-provider priority deterministic first-win, non-recursive `os.ReadDir`, `disabledExtensions: skill:<name>`, `ignored/include` globs, and secure `skill://<name>/<path>` via `path.Clean`+reject `..`/absolute+`Join+EvalSymlinks+HasPrefix`. Added CI `no-fmtSprintf` required guard on `internal/review/lens`.

Shipped as **stacked PRs — 3 commits, 22 files, 1559 total (core 881 prod+resolver+prompt)** within the 800-line budget via `auto-chain` + `stacked-to-main` (PR1 templates+loader 439 → PR2 resolver+CI 399 → PR3 tests 721). All **18/18 tasks** complete, **4/4 requirements, 15/15 scenarios** verified PASS, `go vet ./...` clean, `gofmt -l` clean, `rg fmt.Sprintf` clean, `go test ./...` PASS after fix-warnings re-verification.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 18/18 marked [x] — `allComplete: true`, `pending: 0` (`biggz sdd-status --json` `total:18 completed:18`) |
| Verify verdict | ✅ PASS — 0 blockers, 0 CRITICAL, 0 WARNING (re-verified tras fix-warnings) |
| Spec compliance | ✅ 4/4 requirements, 15/15 scenarios COMPLIANT |
| Build | ✅ `go vet ./...` exit 0 (`e3b0c44298fc...` empty hash), `gofmt -l .` 0 unformatted, `rg html/template` 0 hits (prompt uses `text/template` only) |
| Tests | ✅ `go test ./internal/skillregistry ./internal/review/lens -count=1 -timeout 180s -v` → PASS (skillregistry 20 PASS + 1 SKIP privilege→fallback PASS, lens 27 PASS), `go test ./... -count=1 -timeout 180s` → 0 failures after fix-warnings |
| Evidence | `evidence_revision sha256:e820b3fde2499cae72f865b2a13ed914b41ca9a1cf33d0e11cb3e4fb956b13c9`, `test_output_hash sha256:e820b3fd...`, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `biggz sdd-verify-validate --requirements 4 --scenarios 15` PASS |
| Review gate | N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` emits no `reviewGate`; `nextRecommended: archive`, `dependencies.archive: ready`, `blockedReasons: []` — gate PASS |
| Task gate | PASS — persisted `openspec/changes/prompt-skill-resolver/tasks.md` shows 18 [x], 0 [ ] |

## Spec Compliance

**Verdict**: PASS (per `openspec/changes/prompt-skill-resolver/verify-report.md`, evidence_revision `sha256:e820b3f...`, validated via `biggz sdd-verify-validate`)

| Metric | Value |
|--------|-------|
| Requirements | 4/4 compliant |
| Scenarios | 15/15 compliant |
| Tasks | 18/18 complete (Phase 1:3/3, Phase 2:6/6, Phase 3:3/3, Phase 4:4/4, Phase 5:2/2) |
| Blockers | 0 |
| Critical findings | 0 |
| Build | `go vet ./...` → 0, `gofmt -l .` → 0 |
| Tests | `go test ./... -count=1 -timeout 180s` → PASS (0 failures) |
| Evidence revision | `sha256:e820b3fde2499cae72f865b2a13ed914b41ca9a1cf33d0e11cb3e4fb956b13c9` — focused suite `go test ./internal/skillregistry ./internal/review/lens` |
| Production lines | core 881 (templates 77 + embed 2 + resolver 269 + prompt 77 + lens migrations ~456), total 1559 incl. tests — stacked PRs keep each slice <800 |

**Detailed matrix** (from verify-report — 15/15 COMPLIANT):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Prompt Templates via go:embed and text/template | Templates embedded | `TestTemplatesEmbedded` (skillregistry) + `TestPromptTemplates_Embedded` (lens) — `assets.FS.ReadDir prompts/review ==6` | ✅ COMPLIANT |
| Prompt Templates via go:embed and text/template | Missing variable fails | `TestMissingVarFails` + `TestPromptMissingVarFails` + `TestLoadPromptMissingKeyOption` (missingkey=error) | ✅ COMPLIANT |
| Prompt Templates via go:embed and text/template | No fmt.Sprintf for prompts | `TestCIGuard_CurrentLensClean` + `rg fmt.Sprintf` probe 0 non-allowlisted + CI `no-fmtSprintf` | ✅ COMPLIANT |
| Prompt Templates via go:embed and text/template | Successful render interpolates | `TestRenderNoBraces` + `TestPromptSuccessfulInterpolates` (contains values, no `{{`) | ✅ COMPLIANT |
| Skill Registry Scanning — Priority, Non-Recursive, Filtering | Priority deterministic | `TestPriorityDeterministic` (provider 2 wins over 5 via `ProviderPriority [7]string` first-win) | ✅ COMPLIANT |
| Skill Registry Scanning — Priority, Non-Recursive, Filtering | Non-recursive ignores nested | `TestNonRecursive` (ScanSkillsFromDir tmp → 1 entry) | ✅ COMPLIANT |
| Skill Registry Scanning — Priority, Non-Recursive, Filtering | disabledExtensions excludes | `TestDisabledExtensions` (`skill:foo` excludes foo) | ✅ COMPLIANT |
| Skill Registry Scanning — Priority, Non-Recursive, Filtering | Glob filtering applied | `TestGlobFiltering` + `TestIncludeGlob` (`ignored *_test*` excludes bar_test) | ✅ COMPLIANT |
| Skill URI Resolution with Containment | Valid URI resolves inside root | `TestValidInside` (`skill://foo/docs/a.md` returns bytes under root) | ✅ COMPLIANT |
| Skill URI Resolution with Containment | Traversal with .. rejected | `TestTraversalDotDot` (`skill://foo/../../etc/passwd` → error) | ✅ COMPLIANT |
| Skill URI Resolution with Containment | Symlink escape rejected | `TestSymlinkEscape` + `TestSymlinkEscape_WindowsFallback` + `TestIsSubpath_Boundary` (EvalSymlinks+HasPrefix, Windows fallback PASS) | ✅ COMPLIANT |
| Skill URI Resolution with Containment | Absolute path rejected | `TestAbsoluteRejected` (`skill://foo//etc/passwd` → error) | ✅ COMPLIANT |
| CI No-fmtSprintf Guard | CI fails on fmt.Sprintf | `TestCIGuard_PromptFmtSprintfFails` + `.github/workflows/ci.yml:jobs.no-fmtSprintf` | ✅ COMPLIANT |
| CI No-fmtSprintf Guard | CI passes when clean | `TestCIGuard_CleanPasses` + `TestCIGuard_CurrentLensClean` + `rg` probe 0 hits | ✅ COMPLIANT |
| CI No-fmtSprintf Guard | Allowlisted exception permitted | `TestCIGuard_AllowlistedPasses` (`//lint:ignore no-fmtSprintf` filtered) | ✅ COMPLIANT |

## Spec Sync

Delta specs merged into main specs (source of truth) before archive. In openspec mode `openspec/specs/` is the audit authority; filesystem wins on conflict.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| prompt-skill-resolver | Created (new domain) | 4 requirements, 15 scenarios — Prompt Templates, Skill Registry Scanning, Skill URI Resolution, CI No-fmtSprintf Guard | `openspec/specs/prompt-skill-resolver/spec.md` ✅ (113 lines, 4101 bytes) |

No existing main spec to preserve — delta was a full spec, copied directly `openspec/changes/prompt-skill-resolver/specs/prompt-skill-resolver/spec.md → openspec/specs/prompt-skill-resolver/spec.md`. No REMOVED/RENAMED/MODIFIED (new domain). Subsequent consumers read from `openspec/specs/prompt-skill-resolver/spec.md`.

## Verification Gate

- **Review gate**: N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` emits no `reviewGate` field; `nextRecommended: archive`, `dependencies: {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done, archive:ready}`, `blockedReasons: []` — gate PASS. No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required.
- **Task gate**: PASS — persisted `openspec/changes/prompt-skill-resolver/tasks.md` shows 18/18 [x], 0 [ ] pending. `taskProgress: {total:18, completed:18, pending:0, allComplete:true}`, `applyState: all_done`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, verifyReport:done}`.
- **Build & Tests**: PASS — `go vet ./...` 0, `gofmt -l .` 0, `go test ./internal/skillregistry ./internal/review/lens -count=1 -timeout 180s` PASS (evidence_revision `e820b3fd…`), `go test ./... -count=1 -timeout 180s` PASS after fix-warnings.
- **Verify report**: PASS — `openspec/changes/prompt-skill-resolver/verify-report.md`, verdict `pass`, 0 blockers, 0 critical, 4/4 req, 15/15 scen, validated via `biggz sdd-verify-validate --requirements 4 --scenarios 15`.
- **Fix-warnings re-verification**: Previous warnings resolved before archive — (1) `go test ./...` 2/42 fail in `internal/install` (PI_SUBAGENT_CHILD=1 env flake) fixed via `t.Setenv("PI_SUBAGENT_CHILD","")` isolation in `bigmem_provision_test.go` (parent path) + second-half child guard via explicit `t.Setenv("1")`; proof: stashed master + PI=1→FAIL, PI=""→PASS, with fix + PI=1→PASS, full suite now 0 failures. (2) `TestSymlinkEscape` Windows SKIP fixed via privilege-aware skip (`A required privilege is not held` only) + `TestSymlinkEscape_WindowsFallback` (isSubpath+Clean) and `TestIsSubpath_Boundary` (prefix-collision) both PASS on Windows, Linux CI covers real symlink — containment now has deterministic coverage on both platforms.
- **Remediation**: Not required — verify already PASS, no failed evidence revision, no ledger remediation needed. `applyState: all_done`, `nextRecommended: archive`.

## Implementation Summary

- **Prompt Templates** (`internal/assets/prompts/review/` 6 `.md` 77 lines + `internal/assets/embed.go` + `internal/review/lens/prompt.go` 77 lines): `external.md 13 + r1-risk.md 13 + r2-readability.md 13 + r3-reliability.md 13 + r4-resilience.md 13 + shared.md 12` via `//go:embed all:prompts/review` through `assets.FS`; loader `LoadPrompt(name)` via `assets.FS.ReadFile`+`template.Option("missingkey=error")` — missing var returns error; migrated `internal/review/lens/*/*.go` (`readability/lens.go: renderReadabilityPrompt`, `reliability/lens.go`, `resilience/lens.go`, `external/adapter.go`, `readability/complexity.go`) from `fmt.Sprintf` prompts to `LoadPrompt+Execute`; no `html/template` for prompts; lenses keep `LensInput` purity.
- **Resolver** (`internal/skillregistry/resolver.go` 269 lines + `registry.go` 149 modified): `var ProviderPriority = [7]string{"user:opencode","user:biggz","user:claude","user:kilo","project:skills","project:opencode","project:github"}` deterministic first-win; `ScanSkillsFromDir(dir string, opts ScanOpts) ([]Entry, error)` via `os.ReadDir` non-recursive (top-level only, `entry.IsDir` filter, no WalkDir); `disabledExtensions: skill:<name>` via map, `ignored/include` via `path.Match`+`filepath.Match`; wired in `registry.go` via `scanAllSkillsWithOpts` + `scanDirWithOpts` + `seen` dedupe (provider 2 wins over 5); `ResolveSkillURI(uri string, roots map[string]string) ([]byte, error)` via `path.Clean`+reject `..`/absolute+`filepath.Join`+`EvalSymlinks`+`isSubpath(HasPrefix)` with boundary guard; covers `../`, `//etc`, symlink escape; valid returns bytes.
- **CI Guard** (`.github/workflows/ci.yml` +24 lines): job `no-fmtSprintf` runs `rg -n 'fmt\.Sprintf' internal/review/lens --glob '!*_test.go' | grep -v '//lint:ignore no-fmtSprintf'` required for merge; all prompt-adjacent `fmt.Sprintf` carry `//lint:ignore no-fmtSprintf` narrow allowlist (non-prompt heuristic messages only); 0 non-allowlisted hits.
- **Commits** (stacked PRs, `stacked-to-main`, `auto-chain`): `2610dea` prompt templates+loader (13 files 439), `0b655e5` resolver+CI (3 files 399), `9520530` tests (6 files 721) — 22 files total, core 881, forecast 700-850 within budget via chaining; fix-warnings diff `internal/install/bigmem_provision_test.go` (11) + `internal/skillregistry/resolver_traversal_test.go` (69) re-verified before archive.
- **Tests** (lens 27 PASS, skillregistry 20 PASS+1 SKIP→fallback PASS): `TestTemplatesEmbedded`, `TestMissingVarFails`, `TestRenderNoBraces`, `TestLoadPromptMissingKeyOption`, `TestNoHtmlTemplate`, `TestPriorityDeterministic`, `TestNonRecursive`, `TestDisabledExtensions`, `TestGlobFiltering`, `TestIncludeGlob`, `TestTraversalDotDot`, `TestSymlinkEscape`, `TestSymlinkEscape_WindowsFallback`, `TestIsSubpath_Boundary`, `TestAbsoluteRejected`, `TestValidInside`, `TestCIGuard_*` (4), prompt lens tests (4) — all PASS; `go test ./...` 0 failures, 0 skipped without fallback.
- **Design** (5 decisions): `text/template` vs `html/template` (escapes corrupt diffs), `all:prompts/review` vs per-file, `[7]string` first-win vs map, `os.ReadDir` non-recursive vs WalkDir, `Clean+EvalSymlinks+HasPrefix` vs Clean only.

## Archive Contents

| Artifact | Status | Path (archived) | Notes |
|----------|--------|-----------------|-------|
| proposal.md | ✅ | `openspec/changes/archive/2026-08-27-prompt-skill-resolver/proposal.md` | 73 lines, In Scope 6 .md + resolver + CI guard |
| spec (delta) | ✅ | `openspec/changes/archive/2026-08-27-prompt-skill-resolver/specs/prompt-skill-resolver/spec.md` | 113 lines, 4 req 15 scen — source synced to `openspec/specs/prompt-skill-resolver/spec.md` |
| design.md | ✅ | `openspec/changes/archive/2026-08-27-prompt-skill-resolver/design.md` | 5097 bytes, 5 decisions, data flow + file changes + threat matrix |
| tasks.md | ✅ (18/18 [x]) | `openspec/changes/archive/2026-08-27-prompt-skill-resolver/tasks.md` | 62 lines, 3+6+3+4+2 phases, 0 [ ] — gate PASS |
| verify-report.md | ✅ PASS | `openspec/changes/archive/2026-08-27-prompt-skill-resolver/verify-report.md` | verdict pass, 4/4 15/15, evidence_revision `e820b3fd...` |
| archive-report.md | ✅ | `openspec/changes/archive/2026-08-27-prompt-skill-resolver/archive-report.md` | this file |

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/prompt-skill-resolver/spec.md` (113 lines, 4101 bytes) — new domain, 4 requirements (Prompt Templates, Skill Registry Scanning, Skill URI Resolution, CI No-fmtSprintf Guard) + 15 Given/When/Then scenarios

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
Next `biggz sdd-status --json` shows this change under `archived` with `nextRecommended: done`. Active `openspec/changes/prompt-skill-resolver/` no longer exists. Ready for the next change.

---
*Artifact Store*: `openspec` (repo-local, `openspec/config.yaml` `strict_tdd: false`)
*Preflight*: `interactive, openspec, auto-chain, 800 lines, stacked PRs 3 commits core 881, strict_tdd off, go test ./... -count=1 -timeout 180s`
*Ledger*: no `reviewGate` required; `archive:ready` via `nextRecommended: archive`
*Evidence*: `go vet ./...` clean, `gofmt -l .` clean, `rg 'fmt\.Sprintf' internal/review/lens` clean, `ls internal/assets/prompts/review/*.md` 6, `go test ./internal/skillregistry ./internal/review/lens` PASS, `go test ./...` PASS (post fix-warnings)
