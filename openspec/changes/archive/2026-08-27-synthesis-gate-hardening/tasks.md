# Tasks: Synthesis Gate Hardening

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 280–450 (5 files: 3 mods, 1 new, 1 doc) |
| 400-line budget risk | Low (budget 800 → ~45% budget) |
| Chained PRs recommended | No |
| Suggested split | Single PR — convergent layers |
| Delivery strategy | auto-chain |
| Chain strategy | pending (single PR; split→stacked-to-main) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | 3-layer gate (prompt+blocking+tests+docs) | PR 1 | `go vet ./... && go test ./internal/assets/biggz -count=1 && node --check internal/assets/pi/biggz-synthesis-gate.js && node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` | Manual: sub-agent then `ask` with/without markdown; thin via `BIGGZ_ADVISE=1` | Revert commit; 5 files — no migration |

## Phase 1: Foundation — Pi Gate Hardening

- [x] 1.1 Audit `internal/assets/pi/biggz-synthesis-gate.js`: 4 markers, source `currentTurn→history→lastAssistant` 120s, thin `count<2||len<50`, advise `BIGGZ_ADVISE=1`, bypass only `PI_SUBAGENT_CHILD=1`, expose `_biggzSynthesisGate`
- [x] 1.2 Fix gate: block `{isError:true, text:"Please synthesize..."}` no `original()` + `notify` error; thin advise `concern: synthesis is thin` warning only, no model call; `recordText` handles race, resets after `original()`
- [x] 1.3 Verify helpers `hasSynthesis`/`isThinSynthesis`/`getArtifactsMetrics`/`checkSynthesisPrecondition`; Given `Artifacts: -` Then thin true; Given 3 paths 120 chars Then thin false

## Phase 2: Orchestrator Template + Integration Test

- [x] 2.1 Verify `internal/assets/biggz/biggz-orchestrator.md` has copy-paste block with 4 markers, `INVALID and will be blocked`, and 12× `REMINDER: synthesis markdown is separate...`
- [x] 2.2 Create `internal/assets/biggz/orchestrator.test.go` reading template, asserting 4 markers + `INVALID` + `REMINDER`; Given drift removes marker When `go test` Then fail
- [x] 2.3 Run `go vet ./internal/assets/biggz/... && go test ./internal/assets/biggz -count=1` — exit 0

## Phase 3: Unit Tests — 4 Gate Scenarios

- [x] 3.1 Verify `internal/assets/pi/biggz-synthesis-gate.test.mjs` covers 4 scenarios: missing→`isError:true` not-called, rich→pass, thin+`BIGGZ_ADVISE=1`→warn pass, thin no-flag→silent; plus child bypass + same-turn race
- [x] 3.2 Cover helpers: Given `Artifacts: -` Then `isThinSynthesis` true; Given rich 3 paths 120 chars Then false; Given missing Then `hasSynthesis` false
- [x] 3.3 Run `node --check .../biggz-synthesis-gate.js && node --test .../biggz-synthesis-gate.test.mjs` — all green, asserts `originalCalled==false` on block

## Phase 4: Documentation + CI Verification

- [x] 4.1 Update `docs/architecture.md` with `### Synthesis Gate (3-layer defense)` — prompt→gate→tests/CI, blocking vs advise, source priority, bypass
- [x] 4.2 Final CI: `go vet ./... && go test ./... -count=1 -timeout 180s && node --check .../biggz-synthesis-gate.js && node --test .../biggz-synthesis-gate.test.mjs` — exit 0
