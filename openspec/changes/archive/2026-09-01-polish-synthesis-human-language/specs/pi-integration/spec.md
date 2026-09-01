# Delta for pi-integration

## ADDED Requirements

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
