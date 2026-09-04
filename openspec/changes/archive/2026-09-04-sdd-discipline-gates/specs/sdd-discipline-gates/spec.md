# SDD Discipline Gates Specification

## Purpose

Fail-closed code gates for two SDD discipline gaps: synthesis-gate
over-blocking (swallowed questions) and silent preflight defaults
(ungated phase launch). Go canonical, JS mirror. Installer TUI untouched.

## Requirements

### Requirement: REQ-DG-1 — Checkpoint-scoped synthesis block

`ShouldBlock` (Go `internal/sdd/synthesis_gate.go`, JS mirror
`internal/assets/pi/biggz-synthesis-gate.js`) MUST require
`IsCheckpointAsk`; `HasOptions` alone MUST NOT block. Free-text asks and
Session Preflight option-asks MUST never block. Go is canonical; JS MUST
mirror. Both sides MUST have unit tests.

#### Scenario: Checkpoint ask without synthesis blocks

- GIVEN a post-delegation checkpoint ask with 2–4 options
- WHEN no `## Sub-agent Result` synthesis precedes it in the 120s window
- THEN `ShouldBlock` returns true with blocked reason

#### Scenario: Free-text ask never blocks

- GIVEN an ask with no options (free text)
- WHEN the synthesis gate evaluates it
- THEN `ShouldBlock` returns false regardless of synthesis presence

#### Scenario: Preflight option-ask never blocked

- GIVEN a Session Preflight ask bearing options
- WHEN the synthesis gate evaluates it
- THEN `ShouldBlock` returns false (preflight is not a checkpoint)

#### Scenario: Go/JS parity

- GIVEN the same checkpoint / free-text / preflight inputs
- WHEN evaluated by Go and JS gates
- THEN both return identical block / no-block verdicts

### Requirement: REQ-DG-2 — Blocked-path fallback envelope

On block, the gate MUST emit a same-turn plain-chat payload carrying the
attempted context plus the full question text — nothing swallowed. The
question body MUST be formatted via existing `formatFallback`.

#### Scenario: Blocked checkpoint emits fallback

- GIVEN a checkpoint ask blocked for missing synthesis
- WHEN the gate returns the block envelope
- THEN the envelope contains `context` AND `fallback` with the full question

#### Scenario: Fallback preserves full question

- GIVEN a blocked ask with options and prompt text
- WHEN `formatFallback` renders the fallback
- THEN all options and prompt text are present verbatim (truncation sanitized only)

### Requirement: REQ-DG-3 — Explicit-preflight admission

`HasExplicitPreflight(cwd)` MUST distinguish explicit prefs (cache hit or
disk read success) from silent defaults; `ResolvePreflightPrefs` MUST keep
returning defaults but callers MUST check explicitness first. Until
explicit, the dispatcher (`sdd-status` / `continue`) MUST return
`blocked(preflight_missing)` with `nextRecommended: resolve-blockers` and
MUST NOT launch any phase.

#### Scenario: No explicit preflight blocks dispatch

- GIVEN no cached prefs and no preflight file on disk
- WHEN the dispatcher receives an SDD phase entry
- THEN it returns `blocked(preflight_missing)` + `resolve-blockers`, no launch

#### Scenario: Explicit preflight admits dispatch

- GIVEN cached prefs or a readable disk preflight file
- WHEN the dispatcher receives an SDD phase entry
- THEN it proceeds to normal phase routing

#### Scenario: Defaults alone do not admit

- GIVEN only `ResolvePreflightPrefs` silent defaults (no cache, no disk)
- WHEN explicitness is checked
- THEN `HasExplicitPreflight` returns false

### Requirement: REQ-DG-4 — Parity and regression guard

Existing checkpoint behavior MUST remain unchanged (synthesis-first
checkpoints still pass; missing-synthesis checkpoints still block), and the
installer TUI MUST be untouched (zero files changed under installer/TUI
screens).

#### Scenario: Valid checkpoint still passes

- GIVEN a checkpoint ask preceded same-turn by valid synthesis markdown
- WHEN the gate evaluates it
- THEN `ShouldBlock` returns false

#### Scenario: Installer TUI untouched

- GIVEN the completed change diff
- WHEN installer/TUI screen files are listed
- THEN the list is empty
