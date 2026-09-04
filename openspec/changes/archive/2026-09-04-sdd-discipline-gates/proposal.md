# Proposal: SDD Discipline Gates

## Intent

SDD discipline exists only in markdown prompts, not code. Two verified gaps let phases launch or questions vanish without human control: (1) synthesis gate blocks every option-bearing ask instead of only post-delegation checkpoints, swallowing the question; (2) preflight silently defaults so SDD runs with no explicit human choice. Make both fail-closed in code.

## Scope

### In Scope
- Narrow synthesis gate to post-delegation checkpoints (`IsCheckpointAsk` only); free-text and Session Preflight option-asks never block
- On block, emit plain-chat fallback in same turn: attempted context + full question, nothing swallowed
- Dispatcher returns `blocked(preflight_missing)` with `nextRecommended: resolve-blockers` until explicit preflight prefs exist (cache/disk explicit vs silent defaults distinguished)
- Mirror Go ↔ JS gate parity + unit tests for both

### Out of Scope
- Installer TUI or any unrelated screen changes
- New preflight questions/options or default value changes
- session_guard / edit_authority behavior changes (models only, untouched)

## Capabilities

### New Capabilities
- `sdd-discipline-gates`: fail-closed synthesis scoping + explicit-preflight admission for SDD dispatcher

### Modified Capabilities
- None

## Approach

1. `synthesis_gate.go` + `biggz-synthesis-gate.js`: `ShouldBlock` requires `IsCheckpointAsk`; `HasOptions` alone no longer blocks. Blocked path returns envelope with `context` + `fallback` (formatted question via existing `formatFallback`) for plain-chat route.
2. `preflight.go`: add `HasExplicitPreflight(cwd)` (cache hit or disk read success); `ResolvePreflightPrefs` keeps defaults but callers check explicitness first.
3. Dispatcher (`status*.go`/`continue.go`): if not explicit → `blocked(preflight_missing)`, `nextRecommended: resolve-blockers`, no phase launch.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/synthesis_gate.go` | Modified | Scope block to `IsCheckpointAsk`; fallback payload |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | JS mirror of above |
| `internal/sdd/preflight.go` | Modified | Explicitness detector |
| `internal/sdd/status*.go`, `continue.go` | Modified | `blocked(preflight_missing)` admission |
| `internal/sdd/*_test.go` | Modified | Gate + admission tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Go/JS gate drift | Med | Parity tests, Go canonical comment |
| Preflight blocks existing flows | Med | Only SDD phase entry gated; explicit once, cached |
| Line budget overrun | Low | Gate files + dispatcher only; no TUI |

## Rollback Plan

Revert the five areas above to HEAD; defaults resume silent behavior. No migration or persisted-state change to undo (disk preflight file, if written, is simply ignored again).

## Dependencies

- None (sibling gates `session_guard.go`, `edit_authority.go` as fail-closed models only)

## Success Criteria

- [ ] Preflight option-ask is never synthesis-blocked; checkpoint ask without synthesis still blocks with context+question fallback
- [ ] No explicit preflight → dispatcher `blocked(preflight_missing)` + `resolve-blockers`
- [ ] `go test ./internal/sdd/...` passes; change under 400 lines
