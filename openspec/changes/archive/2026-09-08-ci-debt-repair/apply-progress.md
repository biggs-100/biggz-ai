# Apply Progress: ci-debt-repair — Slice A (Skill lint)

**Mode**: Standard (strict_tdd false; no TDD module loaded)
**Delivery**: auto-chain, stacked-to-main. Slice A = PR-A → main.
**Ledger**: token tok-fbfca74ef247a16e549fe70f, revision 1586e150edd1a45b221b822e84f415e4c3fd9d838d424fc8f54ce5bc6ef438ec, req req-apply-a-20260908.

## Completed Tasks (Phase 1 / Slice A)

- [x] 1.1 `HARD_MAX=3200` in `scripts/check-skill-lint.mjs`; WARN-only exits 0, FAIL exits 1 (`exit 2` removed); mirror-drift check added.
- [x] 1.2 Thresholds mirrored in `internal/skills/lint.go` (`HardMax = 3200`); buckets + exit codes + mirror-sync + trim plan recorded in `openspec/specs/skills/spec.md`. `lint_test.go` updated (1001 → WARN test, `HardMax+1` → FAIL test; adjacent file, required for green).
- [x] 1.3 Oversized-body evidence (pre-trim lint, hard limit 1000): sdd-apply 3018, sdd-archive 2671, sdd-verify 1790, sdd-onboard 1558, sdd-design 1358, branch-pr 1336, sdd-explore 1175. (Exploration named 6; branch-pr also exceeded 1000, so 7 files trimmed.)
- [x] 1.4 Maintainer decision TRIM executed: post-trim counts — branch-pr 912, sdd-apply 994, sdd-verify 994, sdd-archive 998, sdd-design 972, sdd-explore 988, sdd-onboard 994 (all ≤1000, both mirrors byte-identical where paired). `_shared` frontmatter fixed (quoted + Trigger; 88-token WARN retained, exits 0).
- [x] 1.5 Verified: temp 600-token skill → WARN, wrapper exit 0; temp 3201-token skill → FAIL, exit 1; one-mirror drift probe → FAIL drift, exit 1; clean tree → exit 0 (probes removed, mirror restored). `go test ./internal/skills/` PASS (6 tests).

## Files Changed

| File | Action | What |
|------|--------|------|
| `scripts/check-skill-lint.mjs` | Modified | HARD_MAX=3200, WARN→exit 0, drift FAIL |
| `internal/skills/lint.go` | Modified | `HardMax = 3200` mirrored |
| `internal/skills/lint_test.go` | Modified | Mid-band WARN + over-max FAIL tests |
| `skills/{branch-pr,sdd-apply,sdd-verify}/SKILL.md` | Modified | Trimmed ≤1000 |
| `internal/assets/skills/{branch-pr,sdd-apply,sdd-verify}/SKILL.md` | Modified | Mirror sync (byte-identical) |
| `internal/assets/skills/{sdd-archive,sdd-design,sdd-explore,sdd-onboard,_shared}/SKILL.md` | Modified | Trimmed ≤1000 / frontmatter fix (internal-only, no counterpart) |
| `openspec/specs/skills/spec.md` | Modified | New buckets, exit codes, mirror sync, trim plan |
| `openspec/changes/ci-debt-repair/tasks.md` | Modified | Phase 1 checked |

Trim method (per design D1): condensed duplicated Language contract, cut pasted templates (branch-pr PR body → template pointer), narration scripts (onboard), Final-State Authority + hygiene prose (archive), ledger/gate prose (verify), redundant step prose; all normative gates, envelopes, and frontmatter preserved.

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test | `go test ./internal/skills/` → ok (6 tests PASS); `node scripts/check-skill-lint.mjs` → exit 0, zero FAIL |
| Runtime harness | Wrapper on live tree = the harness: WARN-only exit 0 proven with temp 600-token file; FAIL paths proven (3201 tokens → exit 1; drift probe → exit 1); probes removed |
| Rollback boundary | Revert the 15 modified files only; slices D/B/C untouched (no shared files) |

## Deviations

- `internal/skills/lint_test.go` edited (not in listed surfaces): required — old test asserted 1001 FAILs, contradicting the spec-change; updated to new buckets.
- 7 files trimmed, not 6: branch-pr (1336) also exceeded the old 1000 max.
- Out-of-scope over-1000 bodies left as WARNs (pass under HARD_MAX=3200): sdd-tasks 2025, sdd-spec 1628, sdd-propose 1627, sdd-sync 1042, systemic-issue-triage 1025. Follow-up trims tracked under #24.

## Status

Phase 1 (Slice A) 5/5 complete. Ready for verify (Slice A). Remaining: Slices D, B, C (other agents/slices; files untouched).

---

# Apply Progress: ci-debt-repair — Slice D (Release smoke)

**Mode**: Standard (strict_tdd false; no TDD module loaded)
**Delivery**: auto-chain, stacked-to-main. Slice D = PR-D → main.
**Ledger**: token tok-3a9844046687640151c08cbe, req req-apply-d-20260908.

## Completed Tasks (Phase 2 / Slice D)

- [x] 2.1 Snapshot repro: prior attempt `goreleaser release --snapshot --clean` built dist fine (since cleaned); this batch did not re-run full snapshot per continuation brief. Newest v6 patch verified via GitHub API: `v6.4.0` (no `v6.3.1` tag exists — 404); goreleaser stable `v2.18.1` (local toolchain `v2.18.0`).
- [x] 2.2 Pinned `goreleaser-action` to `v6.4.0` + SHA comment `e435ccd777264be153ace6237001ef4d979d3a7a` and explicit `version: v2.18.1` in `.github/workflows/ci.yml` (release-checksums job only; `latest`/floating ref gone).
- [x] 2.3 No `.goreleaser.yaml` fix needed: repro clean; `/tmp/minisign.key` throwaway key path by design; archive files `README.md`/`LICENSE`/`minisign.pub`/`integrity.json` present; YAML parses; `go build ./cmd/biggz` exit 0.

## Files Changed

| File | Action | What |
|------|--------|------|
| `.github/workflows/ci.yml` | Modified | release-checksums job only: action `v6`→`v6.4.0` + SHA, `version: latest`→`v2.18.1` |
| `openspec/changes/ci-debt-repair/tasks.md` | Modified | Phase 2 checked |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test | `python yaml.safe_load` on `ci.yml` + `.goreleaser.yaml` → YAML OK; pinned step resolves to `goreleaser-action@v6.4.0` / `v2.18.1` |
| Runtime harness | `go build ./cmd/biggz` → exit 0; full `goreleaser release --snapshot --clean` proven by prior attempt (dist built fine, since cleaned — not re-run) |
| Rollback boundary | Revert `.github/workflows/ci.yml` hunk only; `.goreleaser.yaml` untouched; slices A/B/C untouched |

## Deviations

- Pinned `v6.4.0`, not `v6.3.1` as tasks.md text said: `v6.3.1` tag does not exist upstream (API 404; tag list jumps `v6.3.0`→`v6.4.0`). Design D3 ("verify newest v6 patch at repro") authorizes the correction.
- No `.goreleaser.yaml` edit: repro revealed nothing to fix.

## Status

Phase 2 (Slice D) 3/3 complete. Ready for verify (Slice D). Remaining: Slices B, C.

---

# Apply Progress: ci-debt-repair — Slice B (Bounded update/upgrade tests)

**Mode**: Standard (strict_tdd false; no TDD module loaded)
**Delivery**: auto-chain, stacked-to-main. Slice B = PR-B → main.
**Ledger**: token tok-ccd3523c7baa9824e24185d8, revision 8eabd59d1991593a7bc4c4cb0836c7d5b8f353de6ddd1fc892813bdc6beb9671, req req-apply-b-20260908.
**Skill**: `use-modern-go` consulted (`list` on both edited files); no returned guideline applied to this edit — `context.WithTimeout` + `defer cancel()` is the idiom the design mandates and no newer guideline supersedes it.

## Completed Tasks (Phase 3 / Slice B)

- [x] 3.1 Live-call asserts: CI run 34265252042 unreachable via API (design open question stands); inferred from grep — `cmd/biggz/cli_upgrade_test.go:109` (`goRunBiggz(t, "update")` with no `BIGGZ_GITHUB_API_BASE`, live `api.github.com`) and `:143` (`goRunBiggz(t, "upgrade", "--dry-run")`, same live path). Both swallow errors (`_ = cmd2.Run()`), so the CI failure mode was hang/flake on unbounded `context.Background()`, not a red assert.
- [x] 3.2 `context.WithTimeout(context.Background(), 30*time.Second)` + `defer cancel()` in `updateRun` (`cmd/biggz/cli_update.go:41`; added `time` import) and `upgradeRun` (`cmd/biggz/cli_sync_install.go:215`; `time` already imported). `syncRun`'s `Background` (line 27, local `Detect` only, no network) intentionally untouched per design scope.
- [x] 3.3 Wired existing `fakeReleasesServer` (`httptest`) into `TestUpdate_CheckOnlyDoesNotCreateBackup` and `TestUpgrade_DryRunDoesNotMutate` (`BIGGZ_GITHUB_API_BASE=srv.URL`, `v9.9.9` release). Zero `t.Skip` added — the fake is localhost-hermetic and works offline, so no quarantine was needed.
- [x] 3.4 Verified (see evidence table). Unticketed skips blocked: `rg "t\.Skip" cmd/biggz/cli_upgrade_test.go` → zero matches.

## Files Changed

| File | Action | What |
|------|--------|------|
| `cmd/biggz/cli_update.go` | Modified | 30s timeout in `updateRun` (+`time` import) |
| `cmd/biggz/cli_sync_install.go` | Modified | 30s timeout in `upgradeRun` |
| `cmd/biggz/cli_upgrade_test.go` | Modified | Fake server in the 2 live tests |
| `openspec/changes/ci-debt-repair/tasks.md` | Modified | Phase 3 checked |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test | `go test ./cmd/biggz/ -run 'TestUpdate_\|TestUpgrade_' -v -count=1` → PASS, 8/8 (11s). `go vet ./cmd/biggz/` → exit 0 |
| Runtime harness | Unreachable-API probe: `BIGGZ_GITHUB_API_BASE=http://10.255.255.1 go run ./cmd/biggz update` → `error: listing releases …` in 22s (≤30s bound; OS dial timeout fired first, 30s ctx is the backstop). Never hangs |
| Rollback boundary | Revert the 3 `cmd/biggz` hunks only; slices A/D/C untouched (their files unmodified by this slice) |

## Deviations

- 3.1 answered by inference, not CI-log read: run 34265252042 not fetchable from this environment; file:line scope confirmed by grep instead.
- 3.3 added no `t.Skip("quarantine #24: …")`: hermetic fake covers the offline path, so quarantine scope is empty. If a future live-only test appears, it must carry the #24 ticket ref.

## Status

Phase 3 (Slice B) 4/4 complete. Ready for verify (Slice B). Remaining: Slice C.

---

# Apply Progress: ci-debt-repair — Slice C (Ticketed e2e quarantine)

**Mode**: Standard (strict_tdd false; no TDD module loaded)
**Delivery**: auto-chain, stacked-to-main. Slice C = PR-C → main.
**Ledger**: token tok-871043dd40ef56d465b86554, revision 0e0b73c3c026ff67befd92793b846302156ce6843584083103bbaee477991db8, req req-apply-c-20260908.
**Skill**: `use-modern-go` consulted (`list` on `e2e/biggz_e2e_test.go`); no returned guideline applies to this edit (skip-guard + `runtime.GOOS` check; no goroutines/contexts/collections touched).

## Completed Tasks (Phase 4 / Slice C)

- [x] 4.1 Exact failing e2e from CI logs (runs 34265252042 + master 34266273509, identical): `TestOrganicDoctor` — FAIL all 3 OS, first assert `e2e/biggz_e2e_test.go:288` (`doctor failed: exit status 2`; 3 CRITICALs: missing MCP binary, missing `backups` subdir, missing pi-web-search extension in bare runner); `TestDockerE2E` — FAIL windows only, first assert `:243` (`Docker build failed: exit status 1`; `no matching manifest for windows/amd64` for linux-only `golang:1.25-alpine`). All other e2e PASS everywhere (Docker skips on macos, no daemon).
- [x] 4.2 Quarantined exactly those in `e2e/biggz_e2e_test.go`, every skip citing #24: unconditional `t.Skip("quarantine #24: …")` in `TestOrganicDoctor`; windows-scoped `if runtime.GOOS == "windows" { t.Skip("quarantine #24: …") }` in `TestDockerE2E` (+`runtime` import) so the green ubuntu leg stays blocking.
- [x] 4.3 Verified (see evidence table). Zero unticketed quarantine skips (remaining `t.Skip`s are pre-existing `-short`/docker-availability env guards).

## Files Changed

| File | Action | What |
|------|--------|------|
| `e2e/biggz_e2e_test.go` | Modified | 2 ticketed quarantine skips + `runtime` import (~10 lines) |
| `openspec/changes/ci-debt-repair/tasks.md` | Modified | Phase 4 checked |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test | `go test ./e2e/ -run 'TestOrganicDoctor\|TestDockerE2E' -count=1 -v` → both SKIP, package PASS (3.5s). `go test ./e2e/ -run 'TestOrganicHelp' -count=1 -v` → PASS (healthy test still executes, quarantine not over-broad). `go vet ./e2e/` → exit 0; `gofmt -l e2e/` → clean |
| Runtime harness | Local probe (pre-quarantine): built `biggz.exe`, ran `TestOrganicDoctor` without `-short` → PASS locally (0 CRITICAL, installed home) vs FAIL in bare CI (3 CRITICALs) — proves env-triggered failure, quarantine correct; probe binary removed. Full 10min e2e suite NOT run locally per slice budget; "rest execute" proven in CI |
| Rollback boundary | Revert the `e2e/biggz_e2e_test.go` hunk only; slices A/D/B untouched (their files unmodified by this slice) |

## Deviations

- 2 quarantined tests, not 3: tasks.md assumed 3 failing e2e, but CI evidence across two runs shows exactly 2 distinct failures (Doctor ×3 OS, Docker ×windows only). Quarantining a 3rd (passing) test would violate the spec's over-broad-quarantine rule — spec wins.
- No fix-instead-of-quarantine: neither failure has an obvious small in-slice fix (Doctor needs a CI `install` step or hermetic HOME; Docker-on-Windows needs LCOW/a linux builder — both beyond the ~10–20-line budget). Real fixes tracked under #24.
- `TestDockerE2E` skip is OS-conditional rather than unconditional, so the passing ubuntu leg keeps blocking per spec.

## Status

Phase 4 (Slice C) 3/3 complete. Ready for verify (Slice C). All slices A/D/B/C applied; full-suite verify per slice in CI.
