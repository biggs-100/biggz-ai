# Spec — 2026-08-27-synthesis-gate-hardening

## Purpose

3-layer hardening of synthesis gate: orchestrator prompt, pi-integration blocking gate, tests/CI. Human MUST see audited `## Sub-agent Result` before deciding. `b0d2fc1` gate existed but was bypassed when no markdown emitted.

## Domains

| Domain | File | Type |
|--------|------|------|
| orchestrator | `specs/orchestrator/spec.md` | Delta ADDED (2 requirements) |
| pi-integration | `specs/pi-integration/spec.md` | Delta MODIFIED+ADDED (2 requirements) |

## Requirements Summary

| ID | Domain | Requirement | Must |
|----|--------|-------------|------|
| R1 | orchestrator | Post-Delegation Human Checkpoint Synthesis — 4 markers same-turn, INVALID blocked | MUST |
| R2 | orchestrator | Synthesis Template Invariant — example + `INVALID` + `REMINDER` | MUST |
| R3 | pi-integration | Advisor Inline Watchdog Advise Mode — 4 markers, source `currentTurnMarkdown→history→lastAssistant` (120s), `isError:true`, thin `<2`/` <50`, `BIGGZ_ADVISE=1`, only `PI_SUBAGENT_CHILD=1` bypass | MUST |
| R4 | pi-integration | Gate Verification and CI — `go vet`, `go test ./...`, `node --test` green for 4 scenarios | MUST |

## Success Criteria Mapping

| Criterion | Spec |
|-----------|------|
| ask without `## Sub-agent Result` → `isError:true` + `Please synthesize…`, original not called | R1, R3 Scenario: Blocking |
| ask with full synthesis (≥2 paths, ≥50 chars, Risks, Next) → pass, no warn | R1 Scenario: Full synthesis, R3 Scenario: Rich |
| thin (`-/1 path`) + `BIGGZ_ADVISE=1` → warn `concern: synthesis is thin` but pass; without flag silent | R3 Scenarios: thin advise / silent |
| CI green: `go vet`, `go test ./...`, `node --test biggz-synthesis-gate.test.mjs` | R4 Scenarios: CI green |

Full detail: see per-domain delta specs.

## Key Detail

Size budget: per-domain deltas <650 words each; scenarios 3-5 lines, RFC 2119, Given/When/Then testable.
