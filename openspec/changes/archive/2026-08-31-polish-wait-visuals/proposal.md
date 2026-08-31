# Proposal: polish-wait-visuals

## Intent

Cut wait noise -70% keep diagnostics. FleetView `pi-subagents` flat rows, repeated metrics, no hierarchy, 1s jitter. Fix inside `biggz-ai` + ADR upstream.

## Scope

### In Scope
- Tokens 4.1k→2.2k hide window if =spent/<1k; fixed cols elapsed 5c tokens 10c right never truncated
- 2 lines/row: L1 glyph agent model·state + fixed right cols; L2 dim tool/activity
- Workflow 2 lines: L1 name·state L2 dim gate/next/output+failure action; nested `│` dim
- Header `X running·Y queued·cap U/L·pane ⚠` + `elapsed·tok` 2 groups; panes `── panes ──` collapsible
- `visibleWorkflowRows` at end; throttle 1s→3s + 1-line headline via `biggz-pi-pretty.js`
- Colors dim elapsed/muted tokens/solid state; ADR `xxx-pi-subagents-wait.md` (shim+PR) + config `compactResultMaxLines:20`

### Out of Scope
- C zen/docs-only (rejected); direct `pi-subagents` edits — ADR only; revert gate; lenses/BigMem

## Capabilities

### New Capabilities
- None — presentation refactor

### Modified Capabilities
- `tui`: 2-line layout, fixed cols, `│`, header 2 groups, panes, `visibleWorkflowRows`
- `tui-sanitize`: compact tokens + width guarantees right never cut
- `orchestrator`: table tokens + wait headline
- `pi-integration`: shim `biggz-pi-pretty.js` 1-line wait

## Approach

B = A + 2 lines + header 2 groups + panes + shim + ADR. A flat hierarchy, C loses actionability — both rejected. Shim wraps hooks no vendor; ADR PR path.

## Affected Areas

| Area | Impact | Desc |
|------|--------|------|
| `internal/tui/sanitize.go` | Modified | Token compact, fixed cols |
| `internal/sdd/synthesis.go` | Modified | Table 10c right, left truncate |
| `internal/assets/pi/biggz-pi-pretty.js` | New | Throttle 3s + 1-line + collapse |
| `internal/assets/pi/subagent-config.json` | Modified | `compactResultMaxLines:20` |
| `docs/adr/xxx-pi-subagents-wait.md` | New | ADR shim+PR |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Overflow 80c | Low | Left-only truncate; golden 80→120c |
| 2-line scroll | Low | Rows at end + throttle |
| Shim vs upstream | Med | Flag gated, revert 1 file, ADR PR |
| Hide <1k | Low | Only window==spent or <1k |

## Rollback Plan

`git revert HEAD` ~150 lines, no migration, <5 min.

## Dependencies

- `sanitize.go`/`synthesis.go`/Pi API existing; upstream ADR target only

## Success Criteria

- [ ] Header ≤2 metrics +1 hint; no row >2 inline points
- [ ] Elapsed/tokens vertical alignment constant 80→120 cols
- [ ] Hierarchy distinguishable at 1m (indent + `│`)
- [ ] Truncation only L1 left/L2 activity, never elapsed/tokens
- [ ] No jitter >1s without layout shift
- [ ] Wait row ≤1 line +1 dim, no scroll (3 steps ≤2 lines)
- [ ] `node --test biggz-synthesis-gate` 22/22, `go vet` PASS, TUI width tests PASS
