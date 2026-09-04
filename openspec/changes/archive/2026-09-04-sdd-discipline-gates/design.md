# Design: SDD Discipline Gates

## Technical Approach

Fail-closed code for two gaps: synthesis over-blocking and silent preflight defaults.
`ShouldBlock` narrows to `IsCheckpointAsk`; blocked path emits same-turn plain-chat
fallback via existing `formatFallback`/`FormatFallback`. `HasExplicitPreflight`
gates dispatcher entry with `blocked(preflight_missing)` + `resolve-blockers`.
Go canonical (`internal/sdd/synthesis_gate.go`), JS mirror
(`internal/assets/pi/biggz-synthesis-gate.js`). Installer TUI untouched.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Gate on `IsCheckpointAsk` only vs keep `HasOptions` OR | OR swallows preflight option-asks; checkpoint tokens already cover 2–4 option checkpoints | Require `IsCheckpointAsk`; `HasOptions` alone never blocks |
| New fallback formatter vs reuse `formatFallback`/`FormatFallback` | New formatter risks limit drift (>16 header, >60 label) | Reuse existing formatters; envelope = `{context, fallback}` |
| Explicitness bit in `ResolvePreflightPrefs` return vs separate `HasExplicitPreflight` | Changing return breaks callers relying on silent defaults | Add `HasExplicitPreflight(cwd)`; `ResolvePreflightPrefs` behavior unchanged |
| Gate every dispatcher path vs SDD phase entry only | Broad gating breaks status reads, archive, non-SDD flows | Gate SDD phase entry only (`sdd-status`/`continue` phase launch) |

## Data Flow

```
ask ──→ ShouldBlock? ──no──→ dispatch
  │         │(IsCheckpointAsk only)
  │        yes + no synthesis ──→ {block, context, fallback: formatFallback(params)}
  │                                └─→ plain-chat same turn, nothing swallowed

sdd-status/continue ──→ HasExplicitPreflight? ──no──→ blocked(preflight_missing) + resolve-blockers
  (phase entry only)         │(cache hit OR disk read ok; defaults alone = false)
                            yes ──→ normal phase routing
```

`HasExplicitPreflight` precedence: cache (`GetPreflightPrefs` ok) > disk
(`ReadSddPreflightToDisk` ok) > false. Silent defaults never admit.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/sdd/synthesis_gate.go` | Modify | `ShouldBlock` requires `IsCheckpointAsk`; blocked envelope adds `context`+`fallback` via `FormatFallback` (`question.go`) |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Mirror: skip block unless `isCheckpointAsk`; blocked `tool_call`/wrapped-`execute` returns `{block:true, reason + fallback}` via existing `formatFallback` |
| `internal/sdd/preflight.go` | Modify | Add `HasExplicitPreflight(cwd, home...) bool` (cache > disk > false) |
| `internal/sdd/status.go` + `continue.go` (or shared admission helper) | Modify | Phase-entry admission: if `!HasExplicitPreflight` → `blocked(preflight_missing)`, `nextRecommended: resolve-blockers`, no launch |
| `internal/sdd/synthesis_gate_test.go` | Modify | REQ-DG-1/2: checkpoint blocks, free-text/preflight never block, fallback carries full question |
| `internal/sdd/preflight_*_test.go` (new or extend) | Create/Modify | REQ-DG-3: no prefs blocks, cache/disk admits, defaults alone deny |
| JS mirror tests (`node --test`) | Modify | REQ-DG-1/4 parity: same fixtures as Go |

Out of scope: installer/TUI screens (zero files), `session_guard`/`edit_authority` (models only),
new preflight questions or default value changes.

## Interfaces / Contracts

```go
// preflight.go — explicitness bit; ResolvePreflightPrefs unchanged.
func HasExplicitPreflight(cwd string, home ...string) bool
```

Blocked fallback envelope (both sides): `{ block: true, reason: string,
context: string /* attempted ask summary */, fallback: string /* full question verbatim */ }`.
JS `formatFallback` / Go `FormatFallback(QuestionEnvelope)` already enforce
envelope limits; truncation sanitized only, options/prompt verbatim.

Dispatcher contract: `blocked(preflight_missing)` + `nextRecommended:
"resolve-blockers"`, no phase side effects.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit (Go) | `ShouldBlock`: checkpoint w/o synthesis blocks; free-text + preflight option-ask never block; valid synthesis passes; fallback has context+full question | `go test ./internal/sdd/ -run 'Synthesis|Preflight'` |
| Unit (Go) | `HasExplicitPreflight`: empty→false; cache→true; disk→true; defaults-only→false; dispatcher returns `blocked(preflight_missing)`+`resolve-blockers` | Same suite, table-driven |
| Parity (JS) | Same fixtures as Go (checkpoint/free-text/preflight) → identical verdicts | `node --test` on gate mirror tests |
| Regression | Existing checkpoint pass/block unchanged; installer/TUI diff empty | Full `go test ./internal/sdd/...` + `git diff --stat` check |

## Threat Matrix

| Row | Applicable | Safe / Failure behavior + RED test |
|---|---|---|
| Gate bypass (`PI_SUBAGENT_CHILD`, Session Recall exception) | Applicable | Bypass paths unchanged; recall exception stays same-turn only. RED: child checkpoint + recall-without-synthesis fixtures still behave as before |
| Go/JS drift | Applicable | Go canonical comment; shared fixture vectors run on both sides. RED: parity test fails on verdict mismatch |
| Newly-blocked flows (preflight gates existing automation) | Applicable | Only SDD phase entry gated; reads/archive/non-SDD unaffected; explicit once, cached. RED: status-read without preflight still succeeds; phase entry without preflight blocks |

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file
classification, or process-integration boundary beyond the above.

## Migration / Rollout

No migration required. Revert listed files to HEAD restores silent defaults;
disk preflight file if written is ignored again.

## Open Questions

None — spec answers all; no scope expansion.
