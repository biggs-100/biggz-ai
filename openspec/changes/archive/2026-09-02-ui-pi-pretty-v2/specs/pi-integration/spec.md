# Delta for pi-integration

## ADDED Requirements

### Requirement: PRETTY-V2-PI-01 — Powerline Footer with Segment Contract and NerdFont Fallback

The system MUST render powerline footer via `internal/assets/pi/biggz-footer.js` and `extension-api.js` with segments `branch|change|lineage|lens 1/4|budget 1/1` in order. Segments MUST use powerline separators (``/``/`›`) when NerdFont detected, else fallback to ASCII `▕` and `/`. The system MUST register separators via extension-api separator registry and respect `BIGGZ_PRETTY=0` (footer off) and `TERM=dumb` (ASCII only). Slice MUST be <400 lines, stacked-to-main, revertible in one commit.

#### Scenario: Footer segment order
- GIVEN branch `main`, change `ui-pi-pretty-v2`, lineage `2`, lens `1/4`, budget `1/1`
- WHEN footer renders
- THEN output MUST contain `main ▕ ui-pi-pretty-v2 ▕ lineage 2 ▕ lens 1/4 ▕ budget 1/1` order left-to-right

#### Scenario: NerdFont fallback
- GIVEN NerdFont missing
- WHEN footer renders
- THEN separators MUST be `▕` and `/` with zero NerdFont glyphs and no garble

#### Scenario: Kill-switch disables footer
- GIVEN `BIGGZ_PRETTY=0`
- WHEN Pi harness initializes
- THEN footer MUST not render and extension-api MUST not inject powerline

### Requirement: PRETTY-V2-PI-02 — Pill Streaming via Extension API

The system MUST stream pill updates via `extension-api.js` to the Pi viewport using the same 16ms throttle and `isSyncSupported()` guard as TUI `syncOutput`. `biggz-tool-pills.js` MUST be collapsible (`>3 → +N`) and highlight-aware, updating incrementally without full re-render dump. The system MUST NOT stream pills when `BIGGZ_PRETTY=0` or `PI_SUBAGENT_CHILD=1`.

#### Scenario: Throttled incremental pill update
- GIVEN 2 pill updates within 16ms via extension-api
- WHEN viewport flushes
- THEN Pi MUST show coalesced pill state with single extension-api write

#### Scenario: Collapsible preserves order via API
- GIVEN 4 pills streamed via extension-api
- WHEN footer/pill area renders
- THEN display MUST be 3 pills plus `… +1 hidden` preserving input order

#### Scenario: Subagent child bypass
- GIVEN `PI_SUBAGENT_CHILD=1`
- WHEN extension-api pill streaming invoked
- THEN system MUST suppress pill injection and render plain fallback

### Requirement: PRETTY-V2-PI-03 — Opt-In Mouse Gating

The system MUST gate mouse support via `BIGGZ_MOUSE=1` opt-in using `enableMouse` in Pi harness. Default MUST be mouse off. When enabled, the system MUST still respect `BIGGZ_PRETTY=0` and `BIGGZ_NO_ANIMATION=1`/`TERM=dumb` (mouse off when pretty/animation disabled). Enable MUST be reversible in one revert and <400 lines.

#### Scenario: Default mouse off
- GIVEN `BIGGZ_MOUSE` unset
- WHEN Pi harness starts
- THEN `enableMouse` MUST NOT be called and mouse events MUST be ignored

#### Scenario: Opt-in enables mouse
- GIVEN `BIGGZ_MOUSE=1` and `BIGGZ_PRETTY!=0` and `TERM=xterm-256color`
- WHEN harness initializes
- THEN `enableMouse` MUST be invoked and mouse events MUST be handled

#### Scenario: Guard overrides opt-in
- GIVEN `BIGGZ_MOUSE=1` but `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN harness evaluates mouse
- THEN mouse MUST remain disabled and no `enableMouse` call MUST occur
