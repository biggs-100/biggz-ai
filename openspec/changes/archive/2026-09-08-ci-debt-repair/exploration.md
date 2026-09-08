## Exploration: ci-debt-repair CI triage (4 fronts)

### Current State
Master CI (run 34265252042): GREEN = Go Format, Complexity, guards, Provider Contract, TestRapid, PR-validation. RED = (1) Skill Lint token overruns, (2) Test matrix ubuntu/macos/windows, (3) E2E x3, (4) Release Checksums Smoke. Verified read-only: lint script fails on WARN (exit 2 → GitHub red); spec (`openspec/specs/skills/spec.md`) pins 180–450 ideal / 700 recommended / 1000 hard; Go mirror `internal/skills/lint.go` matches script; `cmd/biggz/cli_update.go` uses `context.Background()` with live GitHub release calls (no timeout); e2e skips under `-short` so CI-only surface; `.goreleaser.yaml` pins 5 leaf targets (10 archives) + minisign signing.

### Affected Areas
- Front 1 Skill Lint: `scripts/check-skill-lint.mjs`, `internal/skills/lint.go`, `skills/{branch-pr,sdd-apply,sdd-verify,sdd-archive,sdd-design,sdd-explore,sdd-onboard}/SKILL.md`, mirrors under `internal/assets/skills/...`, `skills/_shared/` frontmatter rule, `openspec/specs/skills/spec.md`, `.github/workflows/ci.yml` (skill-lint job)
- Front 2 Test matrix: `cmd/biggz/cli_update.go`, `cmd/biggz/cli_sync_install.go` (`upgradeRun`), `cmd/biggz/cli_upgrade_test.go` (httptest vs live), `internal/components/prompts*.go` + `prompts_test.go` (`TestSharedPromptDir`, `TestWriteSharedPromptFilesIdempotent`), `internal/contracts/envelope_test.go` + `sdd_test.go` (`EnvelopeConformance_*`), `.github/workflows/ci.yml` (test job)
- Front 3 E2E: `e2e/biggz_e2e_test.go` (`TestOrganicRDDStatus`, `TestOrganicRDDDisableEnable`, `TestOrganicHelp`, `TestOrganicReviewStart`, `TestOrganicSDDStatus`, `TestOrganicDoctor`, `TestDockerE2E`), `.github/workflows/ci.yml` (e2e job, `go build -o biggz.exe`)
- Front 4 Release smoke: `.goreleaser.yaml`, `.github/workflows/ci.yml` (release-checksums job), `internal/doctor` (`BuildVersion` ldflags)

### Approaches
1. **Slice per front, sequential PRs (Recommended)** — 4 small stacked PRs, each green-gated independently
   - Pros: bisectable, unblocks green fronts first, small reviews
   - Cons: 4 CI cycles; shared workflow file touched repeatedly
   - Effort: Low per slice (est. 30–120 lines each)
2. **Single PR fixing all 4 fronts** — one big repair PR
   - Pros: one CI cycle, atomic green
   - Cons: large blast radius, one flaky front blocks all, hard review
   - Effort: Medium-High (~300–500 lines + skill rewrites)
3. **Quarantine-first (skip-with-ticket) then fix** — mark flaky tests/E2E non-blocking immediately, fix later
   - Pros: fastest return to green signal; honest debt tracking via issue #24
   - Cons: masks real bugs; requires discipline to pay back
   - Effort: Low (~20–40 lines workflow-only)

### Front verdicts (fix vs quarantine vs spec-change)
- **Front 1 Skill Lint → SPEC-CHANGE + fix (code, not env).** Content exceeds own spec (sdd-apply 3018, sdd-archive 2671, sdd-verify 1790, sdd-onboard 1558, sdd-design 1358, sdd-explore 1175 tokens vs 1000 hard max) AND script exits 2 on WARN so even 450+ ideal-overruns redden the job. Mirrors double the edit surface. Decision: spec-change (raise hard limit and/or make WARN exit 0) + split the 6 oversized skills or tier-trim `model-small` sections. Do NOT just silence lint.
- **Front 2a update/upgrade timeouts → FIX (env-triggered, code-fixable).** `updateRun` uses `context.Background()` (no timeout) and hits live GitHub API; CI sandbox throttling/no-network ⇒ hang until 180s timeout kills the package. Local pass confirms env trigger. Fix: context timeout + skip-if-no-network or force httptest fake server for all network paths.
- **Front 2b components → QUARANTINE-then-fix (likely env).** PASS locally; candidates: XDG_CONFIG_HOME/HOME leakage, Windows FromSlash assumptions, or non-deterministic content breaking idempotence. Needs one CI-log read to confirm.
- **Front 2c contracts → INVESTIGATE (code suspect).** Order/time-sensitive assertions; pass locally but fail under full-matrix load ⇒ possible shared-state flake. Needs failing-assert lines from CI log before verdict.
- **Front 3 E2E x3 → QUARANTINE-then-fix (env-first suspect).** Fail fast on assertions in CI-only surface. Read the 3 failing test names + first assert from CI log; quarantine exactly those 3 with ticket, keep rest blocking.
- **Front 4 Release smoke → FIX (code/tooling).** Candidates: unpinned goreleaser-action latest drift; minisign apt/key-path mismatch; missing archived files. Reproducible locally via snapshot — reproduce first, then pin/fix.

### Recommendation
**Approach 1 (sliced PRs)**, order: Slice A (lint spec-change + WARN exit 0) → Slice D (release pin/repro) → Slice B (update/upgrade timeout fix) → Slice C (e2e quarantine + fix). Total est. ~150–300 lines, no workflow redesign. Before slicing, pull exact failing assert lines from run 34265252042 logs for fronts 2b/2c/3.

### Risks
- Raising lint limits without trimming skills bakes in unmaintainable 3000-token skills; pair limit change with trim/split plan
- `skills/` ↔ `internal/assets/skills/` mirrors drift — fix must update both or automate sync
- Quarantining e2e/contracts without tickets recreates invisible debt
- Unpinned `goreleaser latest` can re-break Slice D at any time; pin the version
- `context.Background()` network calls may exist beyond update/upgrade — grep for other live-API calls in tests

### Ready for Proposal
Yes — propose 4-slice plan above.
