# Tasks: 2026-08-29-ola1-gentle-hardening — Gentle Hardening Ola 1

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 380–440 (PR1 ~340 + PR2 <100) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 (L1+L3+lint+guide) → PR2 (mjs+pin) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | L1 4-source + L3 surfaces + lint + guide | PR1 (~340, base: main) | `go test ./internal/agents/pi -run TestResolveBackground` + `go test ./internal/orchestrator -run TestIsTaskScoped` | Manual: project>global/env; 3 allow, 4 enforce | `adapter.go`,`background.go`,`surfaces.go`,`lint.go`,`skill-style-guide.md`,`status.go` — revert PR1 |
| 2 | Scripts + hash pin + CI | PR2 (<100, base: PR1) | `node scripts/check-provider-contract.mjs` && `node scripts/check-skill-lint.mjs` | Manual offline: drift fails, exact passes | `scripts/*.mjs`,`contracts/**`,`ci.yml` — revert PR2 |

## Phase 1: Foundation — Guide + Lint

- [x] 1.1 Create `docs/skill-style-guide.md` 6 sections from `gentle-pi/docs/skill-style-guide.md`; `description` quoted trigger `<=250`
- [x] 1.2 Create `internal/skills/lint.go` with `LintSkill` + `CountTokens` — buckets 180–450 pass / 450–1000 warn / >1000 fail; fail if frontmatter missing/multi-line/unquoted/no trigger
- [x] 1.3 Create `scripts/check-skill-lint.mjs` wrapper — exit 0 pass, 1 fail, 2 warn

## Phase 2: Core — L1 Policy + L3 Surfaces

- [x] 2.1 Modify `internal/agents/pi/adapter.go`: add `parseBackgroundSubagentsPolicyFile` + `resolveBackgroundSubagentsPolicy(cwd,opts)` with `project>global>env>default off`; malformed→off no fallback
- [x] 2.2 Modify `internal/agents/pi/adapter.go`: add `gentleAiConfigHome` + `renderBackgroundSubagentsReport` (source/policy/cap/malformed), delegate `load→resolve().policy`, keep `Resolve*` wrappers
- [x] 2.3 Modify `internal/opencode/background.go`: add delegate helper, keep scheduling-only doc
- [x] 2.4 Create `internal/orchestrator/surfaces.go`: `isTaskScoped…` (`\`→`/`, reject empty/absolute/`~`/whitespace/`..`, glob `*?[]{}`), `hasTaskScoped…`, `rejectUnscoped…` + `WRITER_EDIT_SURFACE_REJECTION`
- [x] 2.5 Modify `internal/sdd/status.go`: wire pre-dispatch guard when `fileCount>=4` strict (3 allow, 4 enforce per-path)

## Phase 3: Integration — Pin + CI

- [x] 3.1 Create `scripts/check-provider-contract.mjs` offline SHA256 for `contracts/review-integration/v1+v2` vs lock; 1-byte drift → exit 1, no fetch
- [x] 3.2 Create `scripts/verify-package-files.mjs` offline manifest verify; mismatch → exit 1
- [x] 3.3 Commit `contracts/review-integration/v1+v2` SHA256 lock/manifest (add v2 if missing)
- [x] 3.4 Modify `.github/workflows/ci.yml`: add jobs `skill-lint` + `provider-contract` after `format`

## Phase 4: Verification

- [x] 4.1 L1 tests `adapter_test.go`: project on>global/env → on; malformed → off no fallback + flag; global off + env on (project absent) → off from global
- [x] 4.2 L3 tests `surfaces_test.go`: reject `../x` `/etc` `~/x` `*.go` `a[0]` `a b/c`; accept scoped + `./` normalized; 3→allow, 4→per-path + `WRITER_EDIT_SURFACE_REJECTION`
- [x] 4.3 L2 tests `internal/skills/lint_test.go`: 300 pass; 1001 fail; missing trigger fail; 600 warn
- [x] 4.4 L4 tests `scripts/` + `internal/contracts/verify_test.go`: 1-byte drift fails, exact pins pass, offline no fetch
- [x] 4.5 Run `go vet ./...` + `go test ./... -count=1 -timeout 180s` + `gofmt -l`; verify PR1 ~340 / PR2 <100 via `git diff --stat`
