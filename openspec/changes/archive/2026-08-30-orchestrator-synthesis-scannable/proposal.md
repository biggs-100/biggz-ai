# Proposal: orchestrator-synthesis-scannable — Scannable Synthesis + One-line Lifecycle

## Intent

Make orchestrator output scannable in 5s: port gentle's table + one-line lifecycle + sanitized truncation to chat, `sdd-status` 4 blocks, and doc templates.

## Scope

### In Scope
- Synthesis: prose `What was done` → `| Topic | Decision |` table + checklist; shape `Outcome + Quick path + Details` (cognitive-doc-design)
- Lifecycle: one line `◆ Phase · Status · Next` warning/success/error + dim detail (gentle-ai-renderer)
- Truncation: `Preview` 300c + `Diff` via `stripAnsi/stripOsc/CONTROL_CHAR` + `TruncateToWidth` before measure (`internal/tui/sanitize.go`)
- Coverage: chat + `sdd-status` 4 blocks + docs (`proposal/spec/design/tasks/verify-report`)

### Out of Scope
- New lenses, BigMem migration, palette/animation beyond lifecycle line
- Business logic changes

## Capabilities

### New Capabilities
- None — internal presentation refactor, no new domain

### Modified Capabilities
- `orchestrator`: table+checklist replaces prose; one-line lifecycle; sanitized truncation — `internal/sdd/synthesis.go`, `synthesis_gate.go`, `internal/assets/biggz/biggz-orchestrator.md`
- `sdd-status`: 4 blocks → `Outcome + Quick path + Details`, progressive disclosure, chunking <7

## Approach

Reuse `internal/tui/sanitize.go` (`VisibleWidth`/`TruncateToWidth` + `x/ansi`/`go-runewidth`) + gentle terminal-theme sanitization. Update `RenderSynthesis` to table+checklist, orchestrator template to one-line lifecycle, `sdd-status` to 4-block Outcome+Quick path+Details with collapse+hint, banner adaptive. Follow cognitive-doc-design: answer first, chunking, signposting.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/synthesis.go` | Modified | Table + checklist, Preview 300c + Diff sanitized |
| `internal/sdd/synthesis_gate.go` | Modified | Keep 4 markers, doc one-line lifecycle |
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | Template table + `◆ Phase · Status · Next` |
| `internal/sdd/status*.go` | Modified | 4 blocks Outcome+Quick path+Details, chunk <7 |
| `openspec/changes/{change}/proposal.md` | New | This proposal |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Width miscount (CJK/ANSI) breaks truncation | Low | Reuse `sanitize.go` + golden tests |
| Table overflows narrow mux | Low | `TruncateToWidth` per cell, chunk <7 rows |
| Template drift (orchestrator.test.go) | Low | Preserve 4 markers + INVALID rule, add table markers |

## Rollback Plan

`git revert` single commit: restore `synthesis.go` prose, revert orchestrator template, revert status 4-block rendering. No migration. Delete `openspec/changes/orchestrator-synthesis-scannable/` if needed. <5 min.

## Dependencies

- `internal/tui/sanitize.go` — existing
- gentle `terminal-theme`/`ai-renderer`, `cognitive-doc-design` skill
- `orchestrator.test.go` invariant

## Success Criteria

- [ ] Synthesis: `| Topic | Decision |` table + checklist (no prose)
- [ ] Lifecycle: one line `◆ Phase · Status · Next` + dim detail
- [ ] `Preview` 300c + `Diff` sanitized (`stripAnsi/Osc` + `TruncateToWidth`)
- [ ] `sdd-status` 4 blocks Outcome+Quick path+Details, 5s scannable
- [ ] Docs in cognitive-doc-design shape; `go test` + `go vet` pass
