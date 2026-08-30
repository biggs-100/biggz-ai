# Archive Report: 2026-08-29-ola1-gentle-hardening — Gentle Hardening Ola 1

**Change**: `2026-08-29-ola1-gentle-hardening`
**Archived to**: `openspec/changes/archive/2026-08-29-ola1-gentle-hardening/`
**Date**: 2026-08-30
**Status**: archived (PASS 10/10 req 32/32, ledger complete)
**Mode**: openspec / auto-chain / stacked-to-main / 400-line budget / Standard (strict_tdd false)

## Summary

Gentle Hardening Ola 1 ports gentle-pi hardening ola 1 to biggz-ai, closing gaps in background subagent policy, writer edit-surface scoping, skill linting, and provider contract pinning. Two stacked-to-main PRs (`auto-chain`, 400-line budget) deliver 4 layers:

- **L1 Runtime** — 4-source policy `project > global > env > default off` with fail-closed malformed handling and per-source reporting (`internal/agents/pi/adapter.go` 248 lines, `internal/opencode/background.go` 14 lines).
- **L3 Orchestrator** — bounded writer surface guard with `isTaskScopedRepositoryRelativePath` and `fileCount >=4` strict enforcement (`internal/orchestrator/surfaces.go` 147 lines, `internal/sdd/status.go` 32 lines).
- **L2 Skills** — style guide + lint with token buckets 180–450 pass / 450–1000 warn / >1000 fail and frontmatter validation (`docs/skill-style-guide.md` 116 lines, `internal/skills/lint.go` 100 lines, `scripts/check-skill-lint.mjs` 36 lines).
- **L4 Provider** — offline SHA256 lock + manifest + CI gates (`scripts/check-provider-contract.mjs` 15 lines, `scripts/verify-package-files.mjs` 15 lines, `contracts/review-integration/provider-contract.lock.json` 44 files, `internal/contracts/verify.go` 46 lines, `.github/workflows/ci.yml` +23 lines).

All 17 tasks complete, 10 requirements 32 scenarios compliant, verification PASS with ledger bound evidence.

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| **L1 Policy 4-source** | `resolveBackgroundSubagentsPolicy(project>global>env>default)` fail-closed, `gentleAiConfigHome`, `renderBackgroundSubagentsReport` | Verbatim gentle-pi port; toy probe not source-aware; env must not override global when project absent |
| **L1 Delegate shape** | Thin wrappers in `internal/opencode/background.go` delegating to `pi` resolver | Avoid recompute drift; scheduling-only doc preserved |
| **L3 Path scoping** | First-segment glob only (`*?[]{}` in first segment), `\`→`/`, strip `./`, reject empty/absolute/`~`/whitespace/`..` | Matches gentle-pi; full-path glob would reject valid `src/*.go` patterns |
| **L3 Enforcement heuristic** | `ShouldEnforceScopedSurfaces(fileCount >=4)` strict 3 allow / 4 per-path enforce with `WRITER_EDIT_SURFACE_REJECTION` | Reduces noise on small PRs; local `ScopedSurfaceRejection` avoids import cycle `sdd→orchestrator` |
| **L2 Lint buckets** | `CountTokens=len(fields)`, frontmatter single-line quoted ≤250 with `Trigger:` marker, 180–450 pass / 450–1000 warn / >1000 fail | Matches gentle-pi style contract; `check-skill-lint.mjs` maps FAIL→1 / WARN→2 / pass→0 |
| **L4 Contract pin** | Offline SHA256 vs `provider-contract.lock.json` (44 files: 42 v1 + 2 v2), no fetch; 1-byte drift → exit 1 | Deterministic offline CI; network fetch flaky and drift-undetectable |
| **L4 Manifest** | Separate `verify-package-files.mjs` sorted relative walk vs lock keys | Isolates concerns from content hash verification |
| **CI gate** | Parallel jobs `skill-lint` + `provider-contract` after `format` (`needs: format`) | Parallelism; `skill-lint` Node 20, `provider-contract` Node 20 + Go stable |

Deviations documented (non-blocking): `src/*.go` spec vs first-segment rule (kept first-segment per task), `ScopedSurfaceRejection` local type to avoid cycle, `HashFile` helper inlined to keep PR2 <100 lines, 14 grandfathered skills >1000 tokens remain FAIL (not auto-fixed per proposal Risks).

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| `orchestrator` | Updated | 2 ADDED requirements (Task-Scoped Repository-Relative Path Validation 5 scenarios, Bounded Writer Dispatch Surface Guard 3 scenarios) appended to `openspec/specs/orchestrator/spec.md` (4→6 requirements) |
| `runtime` | Updated | 2 ADDED requirements (Background Subagents 4-Source Policy Resolution 4 scenarios, Background Policy Delegate and Reporting 2 scenarios) appended to `openspec/specs/runtime/spec.md` (7→9 requirements) |
| `review` | Updated | 3 ADDED requirements (Provider Contract Offline SHA256 Pin Verification 3 scenarios, Package Manifest Offline Verification 3 scenarios, CI Skill-Lint and Provider-Contract Jobs 2 scenarios) appended to `openspec/specs/review/spec.md` (4→7 requirements) |
| `skills` | Created | 3 ADDED requirements (Skill Style Guide Presence 2 scenarios, LintSkill Token Buckets and Frontmatter Validation 5 scenarios, Check-Skill-Lint Wrapper Exit Codes 3 scenarios) → new `openspec/specs/skills/spec.md` (0→3 requirements, 10 scenarios) |

**Totals**: 4 domains, 10 requirements, 32 scenarios. Delta specs at `openspec/changes/archive/2026-08-29-ola1-gentle-hardening/specs/{orchestrator,review,runtime,skills}/spec.md` preserved.

## Files Changed

| File | Action | What Was Done | Lines |
|------|--------|---------------|-------|
| `docs/skill-style-guide.md` | Created | 6 normative sections ported from `gentle-pi/docs/skill-style-guide.md` (Purpose, When, Structure, Frontmatter ≤250, Writing rules 180–700–1000, Decision gates, Output contract, Registry) | +116 |
| `internal/skills/lint.go` | Created | `CountTokens`, `LintSkill`, `extractFrontmatter`, `validateFrontmatter`, `HasHardFailure/HasWarning` with buckets 180–450/450–1000/>1000 | +100 |
| `scripts/check-skill-lint.mjs` | Created | Node wrapper `findSkills` under `skills/` + `internal/assets/skills/`, mirrors Go semantics, exit 0/1/2 | +36 |
| `internal/agents/pi/adapter.go` | Modified | Replaced toy probe with `parseBackgroundSubagentsPolicyFile`, `resolveBackgroundSubagentsPolicy(project>global>env>default)`, `gentleAiConfigHome`, `renderBackgroundSubagentsReport`, delegate `load→resolve().policy`, keep `Resolve*` wrappers | +248 |
| `internal/opencode/background.go` | Modified | Added `BackgroundPolicy`/`BackgroundResolution` thin delegates to `pi` resolver, scheduling-only doc | +14 |
| `internal/orchestrator/surfaces.go` | Created | `isTaskScopedRepositoryRelativePath` (`\`→`/`, IsAbs, `~`, `./` strip, whitespace, `..`, first-segment `*?[]{}`), `hasTaskScopedAllowedEditSurfaces`, `rejectUnscopedBoundedWriterDispatch` + `WRITER_EDIT_SURFACE_REJECTION` | +147 |
| `internal/sdd/status.go` | Modified | Added `ShouldEnforceScopedSurfaces(fileCount>=4)` strict + `ValidateBoundedWriterSurfaces` guard + `ScopedSurfaceRejection` local type | +32 |
| `internal/agents/pi/adapter_test.go` | Created | 4-source precedence, malformed fail-closed, global off beats env on, Parse/Report, GentleAiConfigHome, Bound manifest | +108 |
| `internal/orchestrator/surfaces_test.go` | Created | Reject `../x` `/etc` `~/x` `*.go` `a[0]` `a b/c`; accept scoped + `./`; 3 allow / 4 per-path + WRITER rejection | +75 |
| `internal/skills/lint_test.go` | Created | 300 pass, 1001 fail, missing trigger fail, 600 warn, CountTokens | +105 |
| `internal/sdd/status_guard_test.go` | Created | `ShouldEnforce` 3 allow / 4 enforce, ValidateBounded 3/4 cases | +30 |
| `scripts/check-provider-contract.mjs` | Created | Offline SHA256 walk `v1+v2` vs lock 44 files, drift/unlisted → exit 1 `offline only`, no fetch | +15 |
| `scripts/verify-package-files.mjs` | Created | Offline sorted manifest walk vs lock keys, unlisted/missing → exit 1 | +15 |
| `contracts/review-integration/provider-contract.lock.json` | Created | SHA256 lock 44 entries (42 v1 + 2 v2) JSON map `rel:hex` | +46 |
| `contracts/review-integration/v2/schemas/contract.schema.json` | Created | v2 schema ported from v1 with v2 `$id`/`const` | + ~52 |
| `contracts/review-integration/v2/fixtures/contract.fixture.json` | Created | v2 fixture validating against v2 schema | + ~52 |
| `internal/contracts/verify.go` | Created | `VerifyProviderContract(lockPath, root)` WalkDir+Rel+ToSlash SHA256 hex, `Verify` offline, no fetch | +46 |
| `internal/contracts/verify_test.go` | Created | 1-byte drift fails, exact pins pass, offline no fetch, envelope conformance | +93 |
| `.github/workflows/ci.yml` | Modified | Jobs `skill-lint` (Node 20, `node scripts/check-skill-lint.mjs`, `needs: format`) + `provider-contract` (Node 20+Go, both mjs `needs: format`) | +23 |
| **PR1 total** |  | 13 files, ~1135 lines (prod ~340 + tests + lock + v2) base: `main` → `2ff2737` |  |
| **PR2 total** |  | 10 files, ~397 lines (prod 99 + 46 lock + 104 v2 + 93 test) base: `PR1` → `d0c527e` |  |

No files outside design table were changed by ola1. Stacked-to-main auto-chain, review budget Medium (380–440 estimated, actual PR1 ~340 prod + PR2 99 prod = 439 within budget per slice <400 each).

## Verification Outcome

**Verdict**: PASS — 10/10 requirements, 32/32 scenarios COMPLIANT

**Evidence**:
- `evidence_revision`: `sha256:dd17a229d3a25812446ccc4b6e46d301bc119b8f90b04ae8507effd4d9a1aac2` (SHA256 of combined focused test output `/tmp/verify-ola1.out`)
- `ledger`: `1831beae932c0384f406ac5b3f579b3c937caee9429eb2b69ff1ef81dcf0b9aa` complete (`tok-2cd832ceb0fff3b03117bed5` verify 10 req 32 scen, max-attempts 3, max-lines 400, revision `9e24b660181c4121e47296f25fa4d36e8f4ca37447140151caf89b589c5acf01` → settle `verify-10-32-settle-001` passed)
- `test_command`: `go test ./internal/agents/pi ./internal/orchestrator ./internal/skills ./internal/contracts ./internal/sdd -count=1 -v && node scripts/check-provider-contract.mjs && node scripts/verify-package-files.mjs` → exit 0
- `build_command`: `go vet ./...` → exit 0, hash `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty stdout), `gofmt -l` clean on delta
- `validator`: `biggz sdd-verify-validate --input verify-report.md --requirements 10 --scenarios 32` → **admitted** (10/10, 32/32)
- `verify date`: 2026-08-30 (report persisted 2026-08-29 19:24)

**Test slices** (all PASS):

| Command | Result | Evidence |
|---------|--------|----------|
| `go test ./internal/agents/pi -run TestResolveBackground -count=1 -v` | PASS 4+ | ProjectOverrides `on` project_file, MalformedFailsClosed `off`+malformed true no fallback, GlobalOverridesEnv `off` global, EnvFallbackAndDefault |
| `go test ./internal/agents/pi -count=1 -v` | PASS | + Parse, Report, GentleAiConfigHome, Bound 64KiB |
| `go test ./internal/orchestrator -count=1 -v` | PASS | Rejects `../x` `/etc` `~/x` `*.go` `a[0]` `a b/c`, Accepts `src/pkg/file.go` `./src/file.go` backslash normalized, second-segment glob allowed |
| `go test ./internal/skills -count=1 -v` | PASS | 300 pass, 1001 FAIL, missing trigger FAIL, 600 WARN, CountTokens |
| `go test ./internal/contracts -count=1 -v` | PASS | ExactPinsPass, OneByteDriftFails, OfflineNoFetch, 8 conformance + 4 schema |
| `go test ./internal/sdd -run TestShouldEnforce -count=1 -v` + `TestValidateBounded` | PASS | 3 allow / 4 enforce, per-path WRITER rejection |
| `node scripts/check-provider-contract.mjs` | PASS | `check passed 44 files` (42 v1 + 2 v2, no fetch) |
| `node scripts/verify-package-files.mjs` | PASS | `verify passed 44 files` |
| `node scripts/check-skill-lint.mjs` | FAIL (expected grandfathered) | 14 FAIL / 8 WARN (`branch-pr` 1336, `sdd-apply` 3018, `sdd-verify` 1791, etc.) — correctly reports >1000 hard fail; change's own files within buckets; documented per proposal Risks, not introduced by PR1/PR2 |
| `go test ./... -short -count=1 -timeout 180s` (full) | FAIL (2 pre-existing, not delta) | `TestReadLoopLarge` (pending_test.go:106 save large verify failed for large-pending) and `TestOrchestratorSynthesisTemplateInvariant` (omit-empty markers) both fail on base `9c73f6f` before PR1/PR2 (verified via checkout 9c73f6f); no file outside design introduced by ola1 |

**Modern Go guidelines**: `skills/use-modern-go/scripts/run-tool.sh list --file-path` consulted for `adapter.go`, `status.go`, `lint.go`, `surfaces.go`, `verify.go` — 40+ idioms listed (wg.Go, t.Context, json_omitzero, slices.Contains, cmp.Or, clear, etc.); no critical modernization missed; trivial opportunities (strings.Cut, slices.Sort) not worth parity-port churn.

**Coverage**: Behavioral table-driven unit tests + offline drift probes per spec; not threshold-gated for this change.

## Archive Contents

- `proposal.md` ✅ (6.5K, intent/scope/rollback/success criteria for 4 layers, 2 PR split)
- `specs/orchestrator/spec.md` ✅ (2 ADDED req, 8 scenarios)
- `specs/runtime/spec.md` ✅ (2 ADDED req, 6 scenarios)
- `specs/skills/spec.md` ✅ (3 ADDED req, 10 scenarios)
- `specs/review/spec.md` ✅ (3 ADDED req, 8 scenarios — 44 files pin)
- `design.md` ✅ (12.6K, architecture decisions, data flow, file changes, threat matrix, PR boundaries)
- `tasks.md` ✅ (17/17 [x] — Phase 1 1.1–1.3, Phase 2 2.1–2.5, Phase 3 3.1–3.4, Phase 4 4.1–4.5; 0 unchecked; workload Medium auto-chain stacked-to-main)
- `apply-progress.md` ✅ (12K, PR1 `tok-6e4556fc7df0cbd439f53e24` + PR2 `tok-89193f8b0e9ba1b813bbcb1f`, work unit evidence tables, deviations, Workload/PR Boundary, 17/17 complete)
- `verify-report.md` ✅ (27.8K, yaml frontmatter pass 0 blockers 0 critical 10/10 32/32, completeness, build & tests execution, 32/32 compliance matrix, correctness, coherence, issues, verdict PASS)
- `archive-report.md` ✅ (this file)

Active directory `openspec/changes/2026-08-29-ola1-gentle-hardening/` no longer exists; change now solely under `openspec/changes/archive/2026-08-29-ola1-gentle-hardening/`.

## Source of Truth Updated

The following specs now reflect the new behavior (spec sync completed before archive move):

- `openspec/specs/orchestrator/spec.md` — 6 requirements (4 existing + 2 ola1)
- `openspec/specs/runtime/spec.md` — 9 requirements (7 existing + 2 ola1)
- `openspec/specs/review/spec.md` — 7 requirements (4 existing + 3 ola1)
- `openspec/specs/skills/spec.md` — new, 3 requirements (10 scenarios)

All delta requirements preserved verbatim with scenarios; non-delta requirements unchanged.

## Final-State Facts (2026-08-30)

- **Verify PASS**: 2026-08-30, evidence `sha256:dd17a229d3a25812446ccc4b6e46d301bc119b8f90b04ae8507effd4d9a1aac2`, ledger `1831beae932c0384f406ac5b3f579b3c937caee9429eb2b69ff1ef81dcf0b9aa` complete, admitted validator.
- **Compliance**: 32/32 scenarios COMPLIANT (0 PARTIAL/UNTESTED/FAILING) per spec compliance matrix with covering tests + source-verified offline checks.
- **Pre-existing full-suite failures (not delta)**: 2 failures in `go test ./... -short` remain on base `9c73f6f` and after ola1: `TestReadLoopLarge` (Windows save large verify) and `TestOrchestratorSynthesisTemplateInvariant` (omit-empty markers) — both outside ola1 delta file set, documented.
- **Grandfathered skill lint**: `node scripts/check-skill-lint.mjs` reports 14 FAIL / 8 WARN on current repo (e.g., `branch-pr` 1336, `sdd-apply` 3018); implementation correctly enforces 1000 hard max; grandfathered per proposal Risks mitigation, not introduced by PR1/PR2; PR1 not gated, PR2 CI jobs added but 14 FAIL remains open follow-up (refactor or threshold `explain`).
- **Ledger**: `1831beae932c0384f406ac5b3f579b3c937caee9429eb2b69ff1ef81dcf0b9aa` complete via `biggz sdd-attempt acquire/settle` bounded verify.
- **Contract pin**: 44 files PASS (`check-provider-contract` + `verify-package-files` offline, no fetch), 1-byte drift → exit 1 verified via temp drift probe.

## Commits

- **Base**: `9c73f6f` feat(sdd): PR3 hybrid locator + rescope wedge (parity-gentle-69 #3) [stacked merge]
- **PR1** `2ff2737` feat(runtime,orchestrator,skills): PR1 gentle hardening L1 4-source policy + L3 surfaces + lint guide (ola1) — ~340 prod lines, base `main`
- **PR2** `d0c527e` feat(review,ci): PR2 provider contract pin + manifest + L4 tests (ola1) — 99 prod lines (`scripts/*.mjs` 15+15 + `verify.go` 46 + `ci.yml` 23), base `PR1`
- **Backfill** (untracked at verify time, now archived): `proposal.md`, `design.md`, `specs/` (4 deltas), `verify-report.md` — backfilled after commits `2ff2737`/`d0c527e` as untracked, now versioned via archive move
- **Ahead**: 4 commits ahead `origin/master` (`2ff2737`, `d0c527e` ola1 + `f6d636d` ola3 + `9f6c8be` ola2 guardrails stacked)
- **Total diff vs base**: `57 files 4953 insertions(+), 24 deletions(-)` includes ola1 + stacked ola2/ola3; ola1 alone PR1 13 files 1135 lines + PR2 10 files 397 lines (prod 340+99)

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. `biggz sdd-status` reports `IsArchived: true`, `nextRecommended: done`, `HasProposal/HasSpecs/HasDesign/HasTasks/HasApply/HasVerify: true`, `TasksTotal: 17 TasksDone: 17`. All 17 tasks `[x]` with no unchecked. No `CRITICAL` issues. Archive audit trail preserved under `openspec/changes/archive/2026-08-29-ola1-gentle-hardening/` (proposal, 4 specs, design, tasks, apply-progress, verify-report, archive-report). Ready for next change.

## Risks / Open Questions

- 14 grandfathered skills >1000 tokens remain FAIL for `skill-lint` CI gate until refactored (split references/assets) or threshold adjusted with `use-modern-go explain`.
- 2 pre-existing full-suite test failures (`TestReadLoopLarge`, `TestOrchestratorSynthesisTemplateInvariant`) tracked for next parity gate; not caused by ola1.
- Isolated temp-dir tests for `check-skill-lint.mjs` exit 0/2 paths and `BackgroundPolicy` delegate invariant test suggested as SUGGESTION in verify-report (not blocking).
