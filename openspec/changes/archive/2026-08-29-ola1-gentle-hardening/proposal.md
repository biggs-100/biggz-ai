# Proposal: 2026-08-29-ola1-gentle-hardening — Gentle Hardening Ola 1

## Intent

Port gentle-pi hardening ola 1 to biggz-ai to close gaps in background subagent policy, writer edit-surface scoping, skill linting, and provider contract pinning. The change aligns runtime, orchestrator, and review/CI behavior with gentle-pi without widening scope beyond the 4 hardening layers.

## Problem

- **L1 Runtime — 4-source policy gap:** `internal/agents/pi/adapter.go` used a toy capability probe (`pi-subagents` package exists → on) instead of gentle-pi's 4-source policy (`project > global > env > default off`) with fail-closed malformed handling and per-source reporting. No `resolveBackgroundSubagentsPolicy` or `BackgroundResolution` delegate.
- **L3 Orchestrator — unbounded writer surfaces:** No `isTaskScopedRepositoryRelativePath` / `rejectUnscopedBoundedWriterDispatch` guard. Delegated writers could receive absolute, parent-traversal, or glob paths without fileCount heuristic enforcement.
- **L2 Skills — lint/guide gap:** No `internal/skills/lint.go` (`LintSkill`/`CountTokens`) or `docs/skill-style-guide.md`. Skill frontmatter validation (single-line quoted `description` with trigger, 250-char limit) and token buckets (180–450 ideal, 1000 hard fail) missing.
- **L4 Provider contract drift:** No offline SHA256 lock (`contracts/review-integration/provider-contract.lock.json`), no `scripts/check-provider-contract.mjs` / `verify-package-files.mjs`, and no CI gates `skill-lint` / `provider-contract`. Drift was undetectable offline.

## Solution

Two PRs stacked-to-main (`auto-chain`, review budget 400 lines):

- **PR1 (~340 lines, base: main):** L1 4-source policy + report (`adapter.go`, `background.go`), L3 surfaces (`surfaces.go`, `status.go` guard), L2 guide + lint (`skill-style-guide.md`, `lint.go`, `check-skill-lint.mjs`) with tests.
- **PR2 (<100 lines prod, base: PR1):** L4 scripts + hash pin + CI (`check-provider-contract.mjs`, `verify-package-files.mjs`, `contracts/**` lock, `internal/contracts/verify.go`, `.github/workflows/ci.yml` jobs).

Approach: verbatim gentle-pi port, helpers minimal, offline-only verification (no fetch), fail-closed on malformed/drift.

## Scope

### In Scope

- `internal/agents/pi/adapter.go`: `parseBackgroundSubagentsPolicyFile`, `resolveBackgroundSubagentsPolicy(cwd,opts)`, `gentleAiConfigHome`, `renderBackgroundSubagentsReport`; keep `Resolve*` wrappers.
- `internal/opencode/background.go`: delegate `BackgroundPolicy`/`BackgroundResolution`.
- `internal/orchestrator/surfaces.go`: `isTaskScopedRepositoryRelativePath` (normalize `\`→`/`, reject empty/absolute/`~`/whitespace/`..`, first-segment glob `*?[]{}`), `hasTaskScopedAllowedEditSurfaces`, `rejectUnscopedBoundedWriterDispatch` + constant `WRITER_EDIT_SURFACE_REJECTION`.
- `internal/sdd/status.go`: `ShouldEnforceScopedSurfaces` (`fileCount >= 4` strict: 3 allow, 4 enforce), `ValidateBoundedWriterSurfaces` guard.
- `docs/skill-style-guide.md` (6 sections), `internal/skills/lint.go` (`LintSkill`, `CountTokens`, `HasHardFailure`), `scripts/check-skill-lint.mjs` (exit 0 pass / 1 fail / 2 warn).
- `scripts/check-provider-contract.mjs` + `scripts/verify-package-files.mjs` (offline SHA256 vs lock), `contracts/review-integration/provider-contract.lock.json` + `v2` schema/fixture, `internal/contracts/verify.go`, CI jobs `skill-lint` + `provider-contract`.
- Tests: `adapter_test.go` (4-source), `surfaces_test.go` (reject/accept, guard), `lint_test.go` (buckets), `status_guard_test.go`, `verify_test.go` (drift), scripts drift probes.

### Out of Scope

- Renames, TUI/MCP, lenses R1–R4, `agent/*`, new `ReviewGate`, BigMem, or SDD contract changes beyond the 4-file guard.
- Refactoring existing >1000-token skills (grandfathered; lint reports FAIL but does not auto-fix).
- Networked verification or credential handling.

## Success Criteria

- [ ] L1: `project on > global off + env on → on from project_file`; malformed → `off` + `malformed true` no fallback; `global off + env on (project absent) → off from global`; `go test ./internal/agents/pi -run TestResolveBackground` PASS.
- [ ] L3: `../x`, `/etc`, `~/x`, `*.go`, `a[0]`, `a b/c` rejected; `src/pkg/file.go` and `./scoped` accepted; `fileCount 3 allow / 4 per-path + WRITER_EDIT_SURFACE_REJECTION` enforced.
- [ ] L2: `LintSkill` 300 tokens pass, 1001 fail, missing trigger fail, 600 warn; `check-skill-lint.mjs` exit codes 0/1/2 match `go test ./internal/skills`.
- [ ] L4: `check-provider-contract.mjs` exact 44-file lock passes, 1-byte drift → exit 1, offline no fetch; `VerifyProviderContract` same; `go test ./internal/contracts` PASS.
- [ ] CI: `ci.yml` contains jobs `skill-lint` + `provider-contract` after `format`; `go vet ./...` PASS; `gofmt -l` clean; PR1 ~340 / PR2 <100 prod verified.
- [ ] `biggz sdd-status --json` shows `artifacts proposal/spec/design/tasks done`, `nextRecommended verify`, `taskProgress 17/17`.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Malformed policy silently falls back to env | Medium | Fail closed to `off` without fallback, flag `malformed true` |
| Glob first-segment vs full-path mismatch | Low | Porter verbatim gentle-pi first-segment rule; document `src/*.go` deviation |
| Existing skills >1000 fail CI | High | Grandfathered; CI reports but PR1 not gated, refactor follow-up |
| Import cycle `sdd/status.go` → `orchestrator` | Medium | Local `ScopedSurfaceRejection` instead of import |
| Lock drift on Windows `\` paths | Low | `filepath.ToSlash` + `relative(root,f)` in scripts |

## Rollback Plan

Revert PR2 then PR1 via `git revert <sha>` (stacked-to-main, no migration). PR1 reverts `docs/skill-style-guide.md`, `internal/skills/lint.go`, `scripts/check-skill-lint.mjs`, `adapter.go`, `background.go`, `surfaces.go`, `status.go` + tests. PR2 reverts `scripts/*.mjs` contracts pin, `internal/contracts/verify*`, `ci.yml` jobs. No BigMem, no data loss.

## Dependencies

- gentle-pi `docs/skill-style-guide.md` and `resolveBackgroundSubagentsPolicy` source (verbatim).
- Go 1.25, `go test ./... -count=1 -timeout 180s`, `go vet`, `gofmt`, Node 20 for `check-*.mjs`.
- `contracts/review-integration/v1` + `v2` fixtures/schemas (freeze walk).

## Alternatives Considered

- Single PR 440 lines vs 2 PR slice: rejected — exceeds 400 budget, review burnout. 2 PRs keep each slice reviewable and independently revertible.
- Capability probe keep toy `on/off` vs 4-source: rejected — not fail-closed, env cannot override intent per project.
