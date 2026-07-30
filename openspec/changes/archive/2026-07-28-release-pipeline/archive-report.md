# Archive Report: release-pipeline

**Archived**: 2026-07-28
**Status**: 19/19 tasks complete — verify PASS
**Mode**: Standard

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| `release-pipeline` | Created | New full spec copied to `openspec/specs/release-pipeline/spec.md` — 5 requirements, 7 scenarios |
| `cli` | Updated | Appended 1 ADDED requirement (Update Subcommand) with 5 scenarios to `openspec/specs/cli/spec.md` |

## Archive Contents

- proposal.md ✅
- specs/cli/spec.md ✅ (delta)
- specs/release-pipeline/spec.md ✅ (full)
- design.md ✅
- tasks.md ✅ (19/19 complete)
- apply-progress.md ✅
- verify.md ✅ (PASS, 0 CRITICAL)
- archive-report.md ✅

## Source of Truth Updated

The following main specs now reflect the new behavior:
- `openspec/specs/release-pipeline/spec.md` — Build Matrix, Checksum Signing, CI/CD Workflow, Version Ldflags, Channel Selection
- `openspec/specs/cli/spec.md` — now includes Update Subcommand requirement with 5 scenarios

## Intentional Archive Notes

- 4 scenarios remain UNTESTED (gated by goreleaser CLI or GitHub Actions infra) — documented gaps per design's testing strategy, no implementation issues
- 1 WARNING in verify report about minisign key path in CI — acknowledged, not a CRITICAL blocker per verdict

## SDD Cycle Complete
