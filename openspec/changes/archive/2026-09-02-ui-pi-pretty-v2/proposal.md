# Proposal: UI Pi Pretty v2

## Intent

Polish Pi harness after 4 ports (sync, hashline, web anchors, advisor). Vanilla Pi minimal; oh-my-pi proves harness wins. Add 5 upgrades (<400 lines each) — streaming, pills, footer, diff, gallery/a11y — no ledger/SDD change.

## Scope

### In Scope
- Sync streaming: `syncOutput` to lens viewport, 60fps/16ms, idempotent CSI 2026.
- Pills/pretty: lipgloss pills (icon/color/spinner) + highlight.
- Footer: `branch|change|lineage|lens 1/4|budget 1/1`, NerdFont→ASCII.
- Diff: split `old|new` if w>100 else unified; word highlight.
- Gallery/mouse/a11y: `docs/gallery` 80/100c, `BIGGZ_MOUSE=1`, `BIGGZ_NO_ANIMATION`/`TERM=dumb`.

### Out of Scope
- Ledger/SDD, lenses, BigMem, orchestrator, Rust/desktop, providers/MCPs.

## Capabilities

### New Capabilities
- None — harness refinements only.

### Modified Capabilities
- `tui`: streaming, pills/pretty, inline diff, gallery/mouse/a11y.
- `pi-integration`: powerline footer, pill streaming, mouse & fallback.

## Approach

5 stacked PRs to `main` (<400 each, auto-chain, revertible):

1 Sync: throttle 16ms, reuse `isSyncSupported()`. 2 Pills: `styles.go`+`biggz-tool-pills.js`, `>3→+N`. 3 Footer: `biggz-footer.js`+`extension-api.js`, `›`/`▕`. 4 Diff: `diff.go`+`sergi/go-diff`, responsive split. 5 Gallery: `scripts/gallery` 80/100c, `enableMouse` gated, reduced-motion kills ticks+sync, dumb strips ANSI. Guards `BIGGZ_PRETTY=0`/`PI_SUBAGENT_CHILD=1`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/tui/tui.go` | Modified | Viewport `syncOutput` throttle |
| `internal/tui/styles/styles.go` | Modified | Pill/diff/footer tokens |
| `internal/tui/screens/*` + `docs/gallery/*` | Modified | Streaming/pretty + preview 80/100c |
| `internal/assets/pi/biggz-tool-pills.js` | Modified | Pills/collapsible |
| `internal/assets/pi/biggz-footer.js` + `extension-api.js` | Modified | Powerline segments + sep registry |
| `openspec/specs/tui/spec.md` | Modified | Delta |
| `openspec/specs/pi-integration/spec.md` | Modified | Delta |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| CSI 2026 garbles | Low | `isSyncSupported()`+`BIGGZ_NO_ANIMATION`/`TERM=dumb` |
| 60fps latency | Low | 16ms trailing flush |
| NerdFont missing | Low | Detect→`▕`/`/` |
| Diff malformed | Med | Cap 1MB, word→line fallback |
| Mouse conflict | Low | Opt-in only |

## Rollback Plan

Revert reverse (`git revert 5..1`), each 1 commit/file. No migration. Kill-switch `BIGGZ_PRETTY=0` (all off) + `BIGGZ_NO_ANIMATION=1`. `biggz install --agent pi` redeploys. Verify `go vet`+`go test`+`node --test`.

## Dependencies

- `internal/tui` sync, `lipgloss`/`theme.go`, Pi deploy (`internal/install`, `embed all:pi`).

## Success Criteria

- [ ] Streaming tear-free, `ESC[?2026h/l`, 60fps, fallback clean.
- [ ] Pills icon/color/spinner, `… +N hidden`, highlight at 80/100c.
- [ ] Footer `branch|change|lineage|lens 1/4|budget 1/1`, fallback ok.
- [ ] Diff splits >100c, word highlight correct.
- [ ] Gallery regenerates, mouse opt-in, reduced-motion+dumb clean.
- [ ] `go test`+`go vet`+`node --test` pass; `install --agent pi` ok; each <400 lines.

## Proposal question round

1 flicker vs diffs vs polish? 2 Footer audience orchestrator vs browser? 3 Invariants: no ledger, `BIGGZ_PRETTY=0`, 400-line hard? 4 Diff/mouse deferrable? 5 Dumb fallback slash ok? Tradeoff perf vs adoption? Assumptions: stacked-to-main, harness-only, <400/slice, mouse off.
