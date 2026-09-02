# Delta for tui

## ADDED Requirements

### Requirement: PRETTY-V2-TUI-01 — Throttled Sync Streaming (16ms / CSI 2026)

The system MUST throttle lens-viewport `syncOutput` in `internal/tui/tui.go` to at most 60fps via 16ms trailing-edge coalescing with flush-on-idle. Frames MUST be wrapped idempotently in `ESC[?2026h`/`ESC[?2026l` only when `isSyncSupported()` is true. The system MUST NOT emit CSI when `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` or `GENTLE_AI_NO_ANIMATION=1` or `TERM=dumb` or `PI_SUBAGENT_CHILD=1`. Each slice MUST be <400 lines and stacked-to-main revertible.

#### Scenario: Throttle coalesces burst
- GIVEN 3 lens updates at t0, t0+5ms, t0+10ms
- WHEN `syncOutput` throttler runs
- THEN system MUST emit one frame at t0+16ms with a single CSI pair and no tearing

#### Scenario: Guard disables sync
- GIVEN `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN frame renders
- THEN output MUST contain zero `ESC[?2026h/l` and MUST NOT garble

#### Scenario: Idempotent nesting
- GIVEN frame already inside `ESC[?2026h`
- WHEN nested `syncOutput` is invoked
- THEN system MUST NOT double-wrap and MUST close with exactly one `ESC[?2026l`

### Requirement: PRETTY-V2-TUI-02 — Tool Pills with Icon / Color / Spinner and +N Collapse

The system MUST render tool pills via `internal/tui/styles/styles.go` (lipgloss tokens) and `internal/assets/pi/biggz-tool-pills.js` with icon, color, and spinner per state (`running`/`queued`/`complete`/`failed`), plus syntax highlight. When pill count >3, the system MUST show first 3 and collapse remainder as `… +N hidden` in order-preserving, collapsible section. The system MUST honor `BIGGZ_PRETTY=0` plain-text fallback and `BIGGZ_NO_ANIMATION` disabling spinner.

#### Scenario: Collapse beyond 3
- GIVEN 5 tool pills `[a,b,c,d,e]`
- WHEN rendered at any width
- THEN output MUST show `a b c` plus `… +2 hidden` and MUST preserve order

#### Scenario: Spinner respects reduced-motion
- GIVEN pill state `running` and `BIGGZ_NO_ANIMATION=1`
- WHEN rendered
- THEN spinner MUST be static and ticks MUST not advance

#### Scenario: Kill-switch plain fallback
- GIVEN `BIGGZ_PRETTY=0`
- WHEN pills render
- THEN output MUST be plain text without lipgloss ANSI and icons MAY be ASCII

### Requirement: PRETTY-V2-TUI-03 — Responsive Inline Word Diff

The system MUST render inline diffs via `internal/tui/diff.go` with `sergi/go-diff`: when viewport width >100 it MUST split `old|new` side-by-side, else unified. Word-level highlights MUST mark added/removed tokens within lines. Diffs >1MB MUST be capped and fall back to line-level highlight without panic.

#### Scenario: Split above threshold
- GIVEN width 120 and changed file with 40-char lines
- WHEN diff renders
- THEN layout MUST be two columns `old | new` with word highlights

#### Scenario: Unified below threshold
- GIVEN width 80
- WHEN same diff renders
- THEN layout MUST be unified single column with inline word highlights

#### Scenario: Cap and fallback
- GIVEN diff payload 1.2MB or malformed hunk
- WHEN rendered
- THEN system MUST cap at 1MB and fall back to line highlight without error

### Requirement: PRETTY-V2-TUI-04 — Gallery Preview and Reduced-Motion / Dumb Guards

The system MUST generate `docs/gallery` previews via `scripts/gallery` at 80c and 100c matching live viewport wrapping/truncation. Gallery regeneration MUST be deterministic and <400 lines. The system MUST disable all animation when `BIGGZ_NO_ANIMATION=1` or `GENTLE_AI_NO_ANIMATION=1` or `TERM=dumb`: `tickCmd` returns nil, sync wrappers suppressed, spinners frozen. When `TERM=dumb`, the system MUST strip ANSI. `BIGGZ_PRETTY=0` MUST disable all pretty features.

#### Scenario: Gallery matches viewport at both widths
- GIVEN `scripts/gallery` executed
- WHEN 80c and 100c galleries compared to live `View()` at same widths
- THEN wrapping, truncation, and token counts MUST match

#### Scenario: Reduced-motion kills ticks and sync
- GIVEN `BIGGZ_NO_ANIMATION=1`
- WHEN `tickCmd()` and `syncOutput` evaluated
- THEN `tickCmd` MUST return nil and frames MUST lack `ESC[?2026h/l`

#### Scenario: Dumb terminal strips ANSI
- GIVEN `TERM=dumb`
- WHEN any pretty view renders
- THEN output MUST contain zero ANSI escapes and no spinner
