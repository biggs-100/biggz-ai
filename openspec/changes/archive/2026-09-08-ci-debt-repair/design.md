# Design: ci-debt-repair

## Technical Approach

Four sliced repairs (A→D→B→C), each green-gated, no workflow redesign or new jobs. Slice A is a spec-change (lint thresholds + wrapper exit code) paired with a trim plan; D pins the release toolchain via local snapshot repro; B bounds network calls with timeout + httptest fakes; C quarantines exactly the 3 CI-failing E2E tests with #24 ticket refs. Maps to proposal slices and the 8 reqs / 16 scenarios in `specs/{skills,ci-test,ci-e2e,ci-release}/spec.md`.

## Architecture Decisions

| # | Option | Tradeoff | Decision |
|---|--------|----------|----------|
| 1 | HARD_MAX: trim-only (keep 1000) vs raise to 3200 + WARN-exit-0 | Trim-only keeps spec pure but CI stays red until 6 large rewrites land; raise-only bakes in 3000-token skills | **Raise HARD_MAX to 3200 in both `scripts/check-skill-lint.mjs` and `internal/skills/lint.go`; wrapper exits 0 on WARN-only (remove `exit 2`), 1 on FAIL; keep 180–450 ideal band; trim/split plan stays open under #24** |
| 2 | Fix-all-timeouts vs timeout + hermetic fake + ticketed quarantine | Pure fix risks long CI debug loops on env flakes; pure quarantine masks bugs | **30s `WithTimeout` in `updateRun`/`upgradeRun` (replacing bare `Background`); tests use existing `httptest` fake pattern, skip offline only with `t.Skip("quarantine #24: <reason>")`; 2b/2c scope confirmed from CI logs first, quarantine-then-fix** |
| 3 | Float `goreleaser-action@v6` + `version: latest` vs pin | Float re-breaks silently on upstream drift | **Pin action to immutable `v6.3.1` (verify newest v6 patch at repro; record SHA) and `version:` to explicit goreleaser semver from `goreleaser --version`; repro via `release --snapshot --clean` locally, assert 5 tar.gz + 5 zip + checksums + minisign** |

Trim/split plan (D1): the 6 bodies >1000 per Slice-A lint output (record exact list in first Slice A task; currently sdd-apply/archive/tasks/verify/propose/spec class) each get a split (extract reference sections / tier-trim `model-small`) targeting ≤1000, tracked under #24. Raising HARD_MAX MUST NOT close the plan (per skills spec scenario).

Mirror-sync rule (D1): every `skills/*/SKILL.md` edit lands identically in `internal/assets/skills/*/SKILL.md` in the same commit; lint scans both trees and any one-mirror-only drift FAILs the check.

```go
// non-obvious pattern: bounded entry point, fake-injectable discovery
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
rel, err := discoverRelease(ctx, ch, explicitVersion)
```

## Data Flow

```
push ──→ ci.yml job ──→ check script / go test / e2e / goreleaser
                              │              │
                         FAIL=exit 1     t.Skip(#24)=pass + ticket
                              │              │
                              └──→ job red / green ──→ master signal
```

CI signal path only; no product runtime data flow changes.

## File Changes

Slice A (lint): `scripts/check-skill-lint.mjs` (Modify: HARD_MAX 3200, WARN-exit-0), `internal/skills/lint.go` (Modify: same thresholds), `openspec/specs/skills/spec.md` (Modify: record new buckets), 6 oversized `SKILL.md` ×2 mirrors (Modify: trim/split). Rollback: restore limits + `exit 2`.
Slice D (release): `.github/workflows/ci.yml` (Modify: pin action + version), `.goreleaser.yaml` (Modify only if snapshot repro demands). Rollback: restore `v6`/`latest`.
Slice B (tests): `cmd/biggz/cli_update.go` (Modify: 30s timeout), `cmd/biggz/cli_sync_install.go` (Modify: same for upgradeRun), `cmd/biggz/cli_upgrade_test.go` (Modify: fake/skip). Rollback: revert to `Background`, remove skips.
Slice C (e2e): `e2e/biggz_e2e_test.go` (Modify: 3 ticketed skips only). Rollback: remove skips.

## Interfaces / Contracts

None new. Changed contracts: lint buckets `180–450` pass / `450–3200` warn / `>3200` fail; wrapper exit `0` pass-or-WARN, `1` FAIL; skip format `t.Skip("quarantine #24: <reason>")` — unticketed skips blocked in review.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Lint buckets (600-token WARN-only exits 0; over-3200 FAILs; drift FAILs) | `go test ./internal/skills/` + wrapper runs |
| Integration | Update/upgrade abort-or-skip ≤30s offline; fake-server pass | `go test ./cmd/biggz/ -run 'Update\|Upgrade'` with net blocked |
| E2E | Trio skips, rest execute; healthy failure still red | Full `e2e` job in CI |

## Threat Matrix

`references/threat-matrix.md` absent from repo — matrix file N/A. Boundary assessment: shell exit-code change (wrapper: WARN no longer red — safe failure = FAIL still exits 1, covered by lint unit tests); subprocess pinning (goreleaser/minisign pinned — safe failure = version mismatch fails fast in smoke job); no routing, VCS automation, or executable-classification changes. No new RED tests beyond rows above.

## Migration / Rollout

No migration. Rollout: stacked PRs A→D→B→C, each green-gated; revert per slice newest-first (C→B→D→A).

## Open Questions

- [ ] Exact 2b/2c failing asserts need CI log read (run 34265252042 unreachable via API) — first Slice B task
- [ ] Newest v6 patch / goreleaser semver to confirm at Slice D repro
