# Tasks: polish-wait-visuals

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 150-180 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR (5 commits) |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| U | Goal | PR | Test | Harness | Rollback |
|---|------|----|------|---------|----------|
| 1 | sanitize compact+cols | c1 | `go test ./internal/tui -run TestSanitize` | 80→120c golden 60c | `tui/sanitize.go` |
| 2 | synthesis 10c+headline | c2 | `go test ./internal/sdd -run TestSynthesis` | `node --test biggz-synthesis-gate` | `sdd/synthesis.go` |
| 3 | screens 2-line/│/header/panes | c3 | `go test ./internal/tui -run TestSanitize` | FleetView 80/120c | `tui/screens/*` |
| 4 | shim 3s + headline ≤2 | c4 | `node --test biggz-pi-pretty` | mock t0/1.5/3s | `biggz-pi-pretty.js` |
| 5 | config+ADR+verify | c5 | `go vet ./...` | `biggz sdd-verify-validate` | `subagent-config.json`+`adr` |

## Phase 1: Setup

- [x] 1.1 Inspect `tui/sanitize.go`, `screens/sanitize.go`, `sdd/synthesis.go`, `subagent-config.json`.
- [x] 1.2 `go vet` + `go test ./internal/tui -run TestSanitize` PASS.

## Phase 2: Sanitize Compact

- [x] 2.1 `tui/sanitize.go` `compactK`/`formatFleetTokens` — GIVEN 2250==2250 THEN `2.2k` no ›.
- [x] 2.2 GIVEN 4100/2200 THEN `4.1k›2.2k` muted 10c.
- [x] 2.3 GIVEN 800/600 THEN hide window.
- [x] 2.4 Mirror `screens/sanitize.go` — `go vet` parity.
- [x] 2.5 CJK/ANSI `VisibleWidth` strip+runewidth `Truncate` CJK2 SGR0 `…1` — GIVEN `a中b` w4 THEN 4 no split; 80/120 right constant.

## Phase 3: Synthesis Table

- [x] 3.1 `sdd/synthesis.go` budget17 `(40-6)/2` chunk7 right 5c/10c — GIVEN 80 vs120 THEN right equal.
- [x] 3.2 Headline POLISH-ORCH-02 — GIVEN 2 runs 23s THEN `Wait 23s · 2 runs (…) — open Fleet…` ≤2.

## Phase 4: Screens 2-line

- [x] 4.1 `row(width)` L1 glyph·state+5c/10c L2 dim — GIVEN 100c THEN layout ok.
- [x] 4.2 Workflow 2-line `│` dim — GIVEN gate fail THEN L2 dim + `│`.
- [x] 4.3 Header 2 groups `g1 muted·g2 dim` ≤2 nums+hint — GIVEN 2/1 cap4/8 pane⚠ 12s·3k THEN ok.
- [x] 4.4 Panes `── panes ──` `panesCollapsed` — GIVEN collapsed THEN header only.
- [x] 4.5 `visibleWorkflowRows` tail — GIVEN 10 limit6 THEN first6 + `… +4 hidden`.

## Phase 5: Shim Throttle

- [x] 5.1 `assets/pi/biggz-pi-pretty.js` wrap `asyncWaitUpdate` 3s — GIVEN t0+1.5s THEN suppress.
- [x] 5.2 Headline ≤2 — GIVEN 3 runs 23s THEN 1 solid+1 dim no dump.

## Phase 6: Config+ADR

- [x] 6.1 `subagent-config.json` `compactResultMaxLines:100→20` — >20 collapsed.
- [x] 6.2 `docs/adr/xxx-pi-subagents-wait.md` fork/vendor/shim/config → shim+PR — 4 sections 1-file revert.

## Phase 7: Verify

- [x] 7.1 `go test ./internal/tui -run TestSanitize` CJK2/ANSI0/60c PASS.
- [x] 7.2 `go test ./internal/sdd -run TestSynthesis` compact/fixed PASS.
- [x] 7.3 `node --test biggz-synthesis-gate` 22/22 PASS.
- [x] 7.4 `go vet ./...` PASS.
- [x] 7.5 `biggz sdd-verify-validate` 14 req/31 scenarios + 80→120c + throttle mock.
