### Requirement: Help Search and Filter

The Help TUI MUST provide live search over `helpData` via a `textinput` model, filtering case-insensitively across `Title`, `Keys[].Key`, `Keys[].Desc`, and `Paragraph`.

#### Scenario: Filter narrows results

- GIVEN Help TUI with all `helpData` entries visible
- WHEN user types `backup` in the filter input
- THEN only entries whose Title/Keys/Paragraph contain `backup` MUST remain visible

#### Scenario: Case-insensitive match

- GIVEN filter contains `DASHBOARD`
- WHEN model applies filter to `helpData`
- THEN entry with Title `Dashboard` MUST match

#### Scenario: Empty filter shows all

- GIVEN filter is cleared to `""`
- WHEN model recomputes view
- THEN all help entries MUST be visible

#### Scenario: No matches placeholder

- GIVEN filter `zzzz_no_match`
- WHEN no `helpData` entry matches
- THEN view MUST show empty-state message and zero shortcut rows

### Requirement: Help Viewport and Shortcut Rendering

The Help model MUST render filtered results in a `viewport` with `lipgloss` styles, displaying shortcut tables per entry without owning storage logic.

#### Scenario: Viewport displays shortcuts

- GIVEN a filtered `HelpContent` with 8 keys
- WHEN `View()` renders
- THEN output MUST contain the Title, Paragraph, and a table of Key/Desc rows styled via `styles`

#### Scenario: Scroll in viewport

- GIVEN viewport height 10 with 30 rows of content
- WHEN user presses `down`/`j`
- THEN viewport offset MUST advance and newly visible rows MUST render

#### Scenario: Narrow terminal truncation

- GIVEN terminal width 40
- WHEN viewport line exceeds width
- THEN line MUST be truncated via `TruncateToWidth` with `…` and MUST NOT overflow

### Requirement: Filter Controls and Overlay Access

The Help TUI MUST support `/` to focus filter input, `ESC` to clear filter or close overlay, and `?` to open help from dashboard.

#### Scenario: ESC clears active filter

- GIVEN filter `dash` has narrowed results
- WHEN user presses `ESC` once
- THEN filter MUST reset to `""` and full list MUST restore

#### Scenario: ESC closes overlay when filter empty

- GIVEN Help screen visible with empty filter
- WHEN user presses `ESC`
- THEN model MUST emit navigation back to dashboard

#### Scenario: Input focus does not trigger navigation

- GIVEN filter input is focused
- WHEN user types `?` or `q`
- THEN characters MUST be inserted into filter and MUST NOT trigger help toggle or quit

### Requirement: Help Content Reuse and Animation Guard

The Help model MUST reuse the existing `helpData` map and `GetHelp`/`HelpContent` types without duplicating content, and MUST honor `BIGGZ_NO_ANIMATION`/`TERM=dumb` via `syncOutput`.

#### Scenario: Reuses helpData source

- GIVEN `internal/tui/screens/help.go` helpData defines 15+ entries
- WHEN Help model loads
- THEN displayed titles/keys MUST equal `helpData` values with no hard-coded duplicate table

#### Scenario: Animation disabled disables sync wrapper

- GIVEN `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN Help `View()` is wrapped via `syncOutput`
- THEN output MUST NOT contain `ESC[?2026h`/`ESC[?2026l`

#### Scenario: teatest covers search and navigation

- GIVEN `teatest` harness with `TERM=dumb` and temp dir isolation
- WHEN test types filter text, presses `ESC`, and scrolls viewport
- THEN model state MUST transition as above and goldens MUST match deterministic render
