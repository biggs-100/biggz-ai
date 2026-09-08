# Proposal: ci-debt-repair

## Intent

Master CI is red on 4 pre-existing fronts exposed after PR #22 unblocked `ci.yml`. Restore green CI via 4 sliced fixes; every quarantine ticketed to issue #24.

## Scope

### In Scope

- **Slice A — Skill lint (spec-change + trim):** raise hard limit and/or WARN exit 0; trim/split 6 oversized skills (1175–3018 tokens vs 1000 max); update mirrors together.
- **Slice B — Test timeouts (fix):** context timeout + no-network skip or httptest fake for `updateRun`/`upgradeRun`; confirm 2b/2c cause from CI logs, quarantine-then-fix if env-flake.
- **Slice C — E2E (quarantine-then-fix):** quarantine exactly the 3 failing tests with ticket refs; fix env causes where cheap.
- **Slice D — Release smoke (fix):** local snapshot repro; pin goreleaser-action; fix minisign/key-path gaps.
- Quarantines ALWAYS reference issue #24; zero new skips without a ticket.

### Out of Scope

- Workflow redesign; new CI jobs; unrelated refactors.

## Capabilities

### New Capabilities

- None — no new product behavior.

### Modified Capabilities

- `skills`: lint token thresholds and/or WARN exit code change (Slice A spec-change).

## Approach

Stacked PRs in order **A → D → B → C** (fastest unblock first, flakiest last), each green-gated. First spec task: pull exact failing asserts for 2b/2c/3 from CI logs (file:line TBD — run 34265252042 not reachable via API).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `openspec/specs/skills/spec.md`, `scripts/check-skill-lint.mjs`, `internal/skills/lint.go` | Modified | Limits / exit code |
| `skills/*/SKILL.md`, `internal/assets/skills/` | Modified | Trim/split skills |
| `cmd/biggz/cli_update.go`, `cli_sync_install.go`, `cli_upgrade_test.go` | Modified | Timeouts, fakes |
| `e2e/biggz_e2e_test.go`, `.github/workflows/ci.yml` | Modified | Ticketed quarantines |
| `.goreleaser.yaml`, `.github/workflows/ci.yml` | Modified | Pinned action, smoke fix |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Lint raise bakes in 3000-token skills | Med | Pair with trim/split plan |
| Mirror drift | Med | Update both or sync check |
| Quarantines go permanent | Med | Every skip refs #24 |
| Unpinned goreleaser re-breaks D | Low | Pin action in Slice D |

## Rollback Plan

Revert per slice, newest first (C → B → D → A). Quarantine revert = remove skips; spec revert = restore prior limits.

## Dependencies

- Issue #24 approval for quarantine policy (pending); CI log access for 2b/2c/3 asserts.

## Success Criteria

- [ ] `ci.yml` fully green on master (lint, test matrix, e2e, release smoke).
- [ ] Zero new skips without a #24 ticket reference.
- [ ] No workflow redesign, no new CI jobs.

## Proposal question round

Review: (1) raise limit + trims, or trim-only? (2) quarantine under #24 approved, or full-fix pre-merge? (3) order A→D→B→C OK?
