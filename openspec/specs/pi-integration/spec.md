# Pi Integration Specification

## Purpose

The Pi Integration domain covers biggz-ai runtime behavior when hosted inside the Pi coding agent (harness, synthesis gate, and inline watchdog). This spec defines the advisor dual-mode watchdog that complements the native RAR gatekeeper with non-blocking concern injection for thin orchestrator synthesis.

## Requirements

### Requirement: Advisor Inline Watchdog Advise Mode

`biggz-synthesis-gate.js` MUST block checkpoint `ask` only when strict `currentTurnMarkdown` lacks 4 markers; history ignored for block. Block MUST be `isError:true` without handler + notify. Thin (count<2 OR len<50) with markers MUST NOT block; MUST emit `concern: synthesis is thin` when `BIGGZ_ADVISE=1` (off default); advise MAY fallback `currentTurn→history→lastAssistant(120s)` for concern only. `PI_SUBAGENT_CHILD=1` bypasses; no orchestrator bypass. `hasSynthesis` stays pass. General without checkpoint tokens MUST bypass.
(Previously: block without same-turn strictness.)

#### Scenario: Missing blocks

- GIVEN lacks 4 markers
- WHEN checkpoint ask
- THEN MUST block `isError:true`

#### Scenario: Thin advises not blocks

- GIVEN `BIGGZ_ADVISE=1`, 4 markers count 1 len 10
- WHEN checkpoint ask
- THEN MUST allow + `concern: synthesis is thin`

#### Scenario: General bypasses

- GIVEN "¿por dónde empezamos?" no checkpoint tokens
- WHEN gate evaluates
- THEN MUST allow without synthesis

### Requirement: Synthesis Gate Verification and CI

`biggz-synthesis-gate.test.mjs` MUST cover 4 gates + `orchestrator.test.go`; MUST cover >50KB loop, envelope reject, pending equality, engram alias. CI MUST run `go vet`, `go test`, `node --test` green.
(Previously: 4 gates only; now adds loop/envelope/pending.)

#### Scenario: Gate tests pass

- GIVEN `node --test biggz-synthesis-gate.test.mjs`
- WHEN fixtures run
- THEN MUST pass and block asserts `isError:true`

### Requirement: Question Envelope Validation

`validateQuestionEnvelope` MUST reject when header>16, label>60, questions>4, or options∉[2,4]; reject MUST be `isError:true` naming limit, NOT call handler, emit fallback. Valid MUST allow.

| Field | Limit |
|-------|-------|
| header | ≤16 |
| label | ≤60 |
| questions | ≤4 |
| options | 2–4 |

#### Scenario: Header too long

- GIVEN header 17 chars
- WHEN validated
- THEN MUST reject naming header 16

#### Scenario: Options range

- GIVEN question with 1 option
- WHEN validated
- THEN MUST reject and emit fallback

#### Scenario: Valid passes

- GIVEN header 12, 3 questions each 3 options <60
- WHEN validated
- THEN MUST allow native ask

### Requirement: POLISH-PI-01 — Throttled Wait via Shim

The system MUST provide shim `internal/assets/pi/biggz-pi-pretty.js` wrapping `asyncWaitUpdate`/`detachedForegroundWaitUpdate` throttled to 3s (prev 1s). When `subagent_wait` waiting with 2-4 runs, it MUST render headline 1 line `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued) — open Fleet for detail` plus optional 1 dim line, total ≤2 lines, collapsing subagent output via `compactResultMaxLines:20` in `subagent-config.json`, and MUST NOT dump full `formatAsyncRunList`.

#### Scenario: Throttle 3s suppresses churn
- GIVEN wait invoked at t0 and t0+1.5s
- WHEN second `asyncWaitUpdate` fires
- THEN shim MUST suppress second render until 3s elapsed

#### Scenario: Headline replaces full list
- GIVEN 3 runs waiting, elapsed 23s
- WHEN shim renders
- THEN output MUST be 1-line headline + optional dim hint (≤2 lines) not full list

#### Scenario: Config collapses output
- GIVEN `subagent-config.json` with `compactResultMaxLines:20`
- WHEN subagent returns long output
- THEN shim MUST collapse beyond 20 lines

### Requirement: POLISH-PI-02 — ADR Upstream and Fallback Strategy

The system MUST document ADR `docs/adr/xxx-pi-subagents-wait.md` evaluating `fork` vs `vendor` vs `extension shim` vs `config` tradeoffs, select `shim biggz-pi-pretty.js` as immediate fallback, and propose PR to `nicobailon/pi-subagents` upstream. Shim MUST be flag-gated revertible in 1 file.

#### Scenario: ADR covers four options
- GIVEN ADR file exists
- WHEN inspected
- THEN it MUST contain sections for fork, vendor, shim, config with tradeoffs and decision for shim+PR

#### Scenario: Shim revertible single file
- GIVEN shim enabled
- WHEN flag disabled or `git revert HEAD`
- THEN wait behavior MUST revert to vendor default with single-file change

### Requirement: REQ-PS4 — Marker Invariant Gate (b0d2fc1)
Gate `hasSynthesis`/`HasSynthesis`/`ShouldBlock`/`isCheckpointAsk` MUST validate by verbatim English markers only, ignoring localized content. 120s same-turn window, Session Recall and thin-advise rules stay language-agnostic.
#### Scenario: Spanish content with English markers passes
- GIVEN markdown Spanish prose but English markers present
- WHEN `hasSynthesis` checks
- THEN it MUST return true and allow `continuar`/`proceed` ask
#### Scenario: Missing marker blocks regardless of language
- GIVEN Spanish synthesis missing `**Artifacts/Paths:**`
- WHEN checkpoint ask within 120s
- THEN gate MUST block `isError:true`/`block:true`
#### Scenario: Thin and Session Recall language-agnostic
- GIVEN thin Spanish synthesis `BIGGZ_ADVISE=1` or `## Session Recall` present or general `¿por dónde empezamos?` without checkpoint tokens
- WHEN gate evaluates
- THEN thin MUST not block (may emit concern), recall MUST allow, general MUST bypass
