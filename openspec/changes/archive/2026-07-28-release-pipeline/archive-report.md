# Archive Report: release-pipeline

**Change**: release-pipeline
**Archived**: 2026-08-25 (originally 2026-07-28, refreshed 2026-08-25)
**Status**: 19/19 tasks complete — verify PASS
**Mode**: Hybrid (BigMem + filesystem)
**Evidence Revision**: sha256:e5414f979febb4f8481d4de9f98fba3c69dda207edab08c4ca2c1b243c8666e8
**Spec Restored**: 6 requirements, 12 scenarios from archive (release-pipeline + cli domains)
**Verify Report**: obs-1787701234200596000-1 (evidence refresh PASS, green suite e5414f97)

## Summary

Release pipeline SDD cycle complete. GoReleaser build matrix (6 targets), minisign signing, GitHub Actions CI/CD, internal/update engine (channel, client, verify, download, replace, embed), CLI `update` subcommand with Unix atomic replace and Windows `go install` fallback, ldflags wiring into `doctor.BuildVersion`. Spec restored from archive 2026-07-28-release-pipeline; evidence refresh verified 6/6 requirements, 12/12 scenarios with `go test ./... -count=1` exit 0, `go build ./... && go vet ./...` exit 0.

Previous archive attempt failed due to provider overload (503) — not logic. Retry legitimate per orchestrator.

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| `release-pipeline` | Verified | Main spec `openspec/specs/release-pipeline/spec.md` already reflects 5 requirements, 7 scenarios; restored BigMem spec matches filesystem — no delta merge needed |
| `cli` | Verified | Main spec `openspec/specs/cli/spec.md` already contains Update Subcommand (5 scenarios); 6/6 req, 12/12 scen combined PASS — no delta merge needed |

## Archive Contents

- proposal.md ✅ (BigMem `sdd/release-pipeline/proposal` → obs-0911a0c57e490799)
- spec.md ✅ (BigMem `sdd/release-pipeline/spec` → obs-0fe6e1191c926fe1, restored from archive)
- design.md ✅ (BigMem `sdd/release-pipeline/design` → obs-0404f73a3b780def)
- tasks.md ✅ (BigMem `sdd/release-pipeline/tasks` → obs-f4071a8151139a6c, 19/19 complete)
- apply-progress ✅ (BigMem `sdd/release-pipeline/apply-progress` → obs-65a8c0b204643e00)
- verify-report ✅ (BigMem `sdd/release-pipeline/verify-report` → obs-1787701234200596000-1, PASS, evidence sha256:e5414f979febb4f8481d4de9f98fba3c69dda207edab08c4ca2c1b243c8666e8)
- explore ✅ (BigMem `sdd/release-pipeline/explore` → obs-95f05a079f100014)

## Source of Truth Updated

The following specs now reflect the new behavior (already synced on 2026-07-28, verified 2026-08-25):
- `openspec/specs/release-pipeline/spec.md` — Build Matrix, Checksum Signing, CI/CD Workflow, Version Ldflags, Channel Selection
- `openspec/specs/cli/spec.md` — Update Subcommand with Unix/Windows/channel/signature/idempotence scenarios

## Task Completion Gate

- `taskProgress.total`: 19
- `taskProgress.completed`: 19
- `taskProgress.pending`: 0
- `allComplete`: true
- Inspection: BigMem `sdd/release-pipeline/tasks` (obs-f4071a8151139a6c) has 0 unchecked `- [ ]` items; all 19 tasks marked `[x]` — gate PASS.

## Verification Gate

- `verify-report` (obs-1787701234200596000-1): verdict `pass`, `blockers: 0`, `critical_findings: 0`
- `requirements: 6/6`, `scenarios: 12/12`
- `evidence_revision: sha256:e5414f979febb4f8481d4de9f98fba3c69dda207edab08c4ca2c1b243c8666e8`
- Build: `go build ./... && go vet ./...` → exit 0
- Tests: `go test ./... -count=1` → exit 0 (full suite green)
- No CRITICAL issues — gate PASS. Prior archive attempt failed on 503 provider overload, not verification logic.

## Review Gate

- `sdd-status` reports `nextRecommended: archive` → `done` after this archive, with empty `blockedReasons` and `remediationState.required: false`.
- `RDD status: enabled` but SDD path is unmanaged per divergences (`biggz has no review authority on the SDD path`); archive does not require `reviewGate.allow` receipt when status is `archive` ready and no review artifacts exist. Verified via `biggz bigmem search sdd/release-pipeline/review` → No results.

## Traceability (Hybrid)

- Artifacts retrieved via `biggz_mem_search` + `biggz_mem_get_observation` per Section B.
- All observation IDs recorded for audit:
  - `sdd/release-pipeline/proposal` → obs-0911a0c57e490799
  - `sdd/release-pipeline/spec` → obs-0fe6e1191c926fe1 (restored 2026-08-25T23:36:36Z)
  - `sdd/release-pipeline/design` → obs-0404f73a3b780def
  - `sdd/release-pipeline/tasks` → obs-f4071a8151139a6c
  - `sdd/release-pipeline/apply-progress` → obs-65a8c0b204643e00
  - `sdd/release-pipeline/verify-report` → obs-1787701234200596000-1
  - `sdd/release-pipeline/explore` → obs-95f05a079f100014
  - `sdd/release-pipeline/archive-report` → obs-1787702054631279104-555889 (this archive)
- Filesystem archive: `openspec/changes/archive/2026-07-28-release-pipeline/` (existing, contains proposal/design/tasks/verify/spec deltas, archive-report.md updated on this archive cycle)
- Main specs verified: `openspec/specs/release-pipeline/spec.md` and `openspec/specs/cli/spec.md` — both already synced, no destructive merge needed.

## SDD Cycle Complete

The change has been fully planned, implemented, verified (evidence refresh PASS green suite e5414f97), and archived.
Ready for the next change.
