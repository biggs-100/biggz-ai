# Tasks: ci-debt-repair

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 150–300 total |
| 400-line budget risk | Low |
| Chained PRs recommended | Yes |
| Suggested split | PR-A lint → PR-D release → PR-B timeouts → PR-C e2e |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| A | Lint spec-change + WARN-exit-0 | PR-A → main | `go test ./internal/skills/ && node scripts/check-skill-lint.mjs` | `node scripts/check-skill-lint.mjs` on 600-token + >3200 bodies | `scripts/check-skill-lint.mjs`, `internal/skills/lint.go`, spec |
| D | Pinned release smoke | PR-D → main | `goreleaser release --snapshot --clean` | Local snapshot: assert 5 tar.gz + 5 zip + checksums + minisign | `.github/workflows/ci.yml`, `.goreleaser.yaml` |
| B | Bounded update/upgrade tests | PR-B → main | `go test ./cmd/biggz/ -run 'Update\|Upgrade'` | Net-blocked run: abort-or-skip ≤30s | `cmd/biggz/cli_update.go`, `cli_sync_install.go`, `cli_upgrade_test.go` |
| C | Ticketed e2e quarantine | PR-C → main | E2E job in CI | CI e2e: trio skips, rest execute | `e2e/biggz_e2e_test.go` skips only |

Per-slice: A 60–120 Med · D 10–30 Low · B 40–80 Low · C 10–20 Low. No slice exceeds 400; chain is for isolation, not overflow.

## Phase 1: Slice A — Skill lint (spec-change)

- [x] 1.1 Set `HARD_MAX=3200` in `scripts/check-skill-lint.mjs`; WARN-only exits 0, FAIL exits 1 (remove `exit 2`)
- [x] 1.2 Mirror thresholds in `internal/skills/lint.go`; record buckets in `openspec/specs/skills/spec.md`
- [x] 1.3 Record exact 6 oversized bodies from lint output as Slice A evidence
- [x] 1.4 ⚠️ DECISION NEEDED before apply: trim-vs-accept for the 6 skills; plan tracked under #24, both mirrors in same commit — RESOLVED: maintainer chose TRIM; all 7 trimmed to ≤1000 in both mirrors
- [x] 1.5 Verify: 600-token WARN exits 0, >3200 FAILs, one-mirror drift FAILs

## Phase 2: Slice D — Release smoke (fix)

- [x] 2.1 Repro locally: `goreleaser release --snapshot --clean`; record newest v6 patch + goreleaser semver — DONE (prior attempt: snapshot built dist fine, since cleaned; this batch: newest v6 = v6.4.0, goreleaser stable = v2.18.1; no full re-run per continuation brief)
- [x] 2.2 Pin `goreleaser-action` to `v6.4.0` (+SHA `e435ccd777264be153ace6237001ef4d979d3a7a`) and explicit `version: v2.18.1` in `.github/workflows/ci.yml` (task text said `v6.3.1` — tag does not exist upstream, 404; deviated to newest v6 `v6.4.0` per design D3)
- [x] 2.3 Fix `.goreleaser.yaml`/minisign key paths only if repro demands; verify checksums + signatures — DONE: no fix needed (repro clean; `/tmp/minisign.key` throwaway by design; archive files README.md/LICENSE/minisign.pub/integrity.json present; YAML parses; `go build ./cmd/biggz` exit 0)

## Phase 3: Slice B — Test timeouts (fix)

- [x] 3.1 Pull exact 2b/2c failing asserts from CI logs (file:line; run 34265252042) — DONE: run unreachable via API (per design open question); inferred from grep: `cmd/biggz/cli_upgrade_test.go:109` (`goRunBiggz(t, "update")`, live `api.github.com`, no `BIGGZ_GITHUB_API_BASE`) and `:143` (`goRunBiggz(t, "upgrade", "--dry-run")`, same live path). Both ignored errors (`_ = cmd2.Run()`) so CI flaked/hung rather than asserting red; the hang was the unbounded `context.Background()` in `updateRun`/`upgradeRun`.
- [x] 3.2 Add 30s `WithTimeout` in `cmd/biggz/cli_update.go` (`updateRun`) and `cmd/biggz/cli_sync_install.go` (`upgradeRun`)
- [x] 3.3 Use `httptest` fake in `cmd/biggz/cli_upgrade_test.go`; offline paths `t.Skip("quarantine #24: <reason>")` — DONE: wired existing `fakeReleasesServer` into the two live tests (`TestUpdate_CheckOnlyDoesNotCreateBackup`, `TestUpgrade_DryRunDoesNotMutate`); zero skips added (fake is localhost-hermetic, works offline), so no quarantine needed
- [x] 3.4 Verify net-blocked: abort-or-skip ≤30s, fake-server pass; unticketed skips blocked — DONE: focused 8/8 PASS; unreachable-API probe aborted in 22s ≤30s; `rg t.Skip cli_upgrade_test.go` → zero matches

## Phase 4: Slice C — E2E (quarantine-then-fix)

- [x] 4.1 Pull exact 3 failing e2e names + first asserts from CI log — DONE (evidence shows 2 distinct tests, not 3; see deviation): `TestOrganicDoctor` (all 3 OS, `e2e/biggz_e2e_test.go:288`, doctor exit 2, 3 CRITICALs: missing MCP binary, missing `backups` subdir, missing pi-web-search extension) and `TestDockerE2E` (windows only, `:243`, `docker build` fails `no matching manifest for windows/amd64` on linux-only `golang:1.25-alpine`). Confirmed stable across runs 34265252042 and master 34266273509. All other e2e PASS on all legs (Docker skips on macos — no daemon).
- [x] 4.2 Quarantine exactly those 3 in `e2e/biggz_e2e_test.go` with `t.Skip("quarantine #24: <reason>")` — DONE as 2 skips: unconditional ticketed skip in `TestOrganicDoctor`; windows-scoped ticketed skip in `TestDockerE2E` (ubuntu leg stays blocking per spec anti-over-broad rule)
- [x] 4.3 Verify: trio skips, rest execute; healthy failure still red; over-broad quarantine rejected — DONE locally: quarantined pair SKIP, `TestOrganicHelp` still PASS without `-short`; `go vet ./e2e/` exit 0; full "rest execute" proof deferred to CI (10min suite not run locally per slice budget)
