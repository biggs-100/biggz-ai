# Tasks: add-odd-lane

## Review Workload Forecast

~775–955 lines · auto-chain · PR 1 → PR 2 → PR 3

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | PR | Focused test | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | ODD contract surfaces | PR 1 | `go test ./internal/assets/ -run TestOddLaneContract` | `go test ./internal/assets/biggz/ -run TestOrchestrator` (≤120 guard) | skill + `AGENTS.md` row, then delegation doc |
| 2 | Read-only ODD array | PR 2 | `go test ./internal/sdd/ -run TestOdd` | `biggz sdd-status --json` (temp workspace) | drop `odd` array + new files |
| 3 | Declared subroute | PR 3 | `go test ./internal/sdd/ -run TestSubroute` | `biggz sdd-continue <change>` | `Subroute` + continue lines; `route` intact |

## Slice 1 — ODD contract surfaces (PR 1) · est. 210–260 lines
Files: `internal/assets/biggz/biggz-orchestrator-delegation.md`, `internal/assets/skills/odd/SKILL.md`, `AGENTS.md`, `internal/assets/odd_lane_contract_test.go`

- [x] 1.1 RED `internal/assets/odd_lane_contract_test.go`: embedded FS ships `skills/odd/SKILL.md`, frontmatter parses, delegation doc lists 7 ordered steps, no `odd/plans`.
- [x] 1.2 GREEN: create `internal/assets/skills/odd/SKILL.md` (frontmatter like `rdd-defect-workflow`; sections `Objective`/`Tasks`/`Evidence`/`Next`; no BigMem mirror, no `biggz odd-*`).
- [x] 1.3 Append 7-step protocol (track-before-write; read-only no artifact) to `internal/assets/biggz/biggz-orchestrator-delegation.md`; `biggz-orchestrator.md` untouched.
- [x] 1.4 Add single `odd` row to `AGENTS.md`.
- [x] 1.5 Close: tick boxes; `apply-progress.md` evidence.
- [x] 1.6 Budget ≤400 (est. 210–260).
- Keep passing: `go test ./internal/assets/...`; `orchestrator_test.go` ≤120 guard.

## Slice 2 — Read-only ODD array (PR 2) · est. 380–430 lines
Files: `internal/sdd/odd.go`, `internal/sdd/odd_test.go`, `cmd/biggz/cli_sdd.go` (~:139, :154, :169, :228), `cmd/biggz/odd_status_cli_test.go`

- [x] 2.1 RED `internal/sdd/odd_test.go`: counts 5/3, 2/0, `- [X]`, sorted; skips missing/unreadable/non-regular/non-`.md`/dir; UTC `lastTouched`; fixtures in `t.TempDir()`.
- [x] 2.2 GREEN `internal/sdd/odd.go`: `OddDocument`, `ScanOddDocuments`, `RenderOddDocuments`; reuse `countTaskProgressText`; skip, never fatal.
- [x] 2.3 RED `cmd/biggz/odd_status_cli_test.go`: JSON `odd[0]` keys; `odd: []` always present; human `<path> — 3/5 tasks — <ts>`; isolation: `active` empty, `nextRecommended`/`blockedReasons` unchanged; malformed + proposal-done → `nextRecommended=spec`.
- [x] 2.4 GREEN `cmd/biggz/cli_sdd.go`: `odd` in JSON (~:139) and human output (~:154/:169/:228); schema v2; never in routing.
- [x] 2.5 D4: change-less organic → `active: []`, temp doc listed, no synthetic dir; SDD cases use real change fixtures.
- [x] 2.6 Close: tick boxes; `apply-progress.md`.
- [x] 2.7 Budget ≤400; if >380, split PR 2a (`odd.go`+tests) / 2b (CLI).
- Keep passing: `go test ./internal/sdd/ -run 'TestStatus|TestRoute'`; `go test ./cmd/biggz/ -run TestSddStatus`.

## Slice 3 — Declared subroute (PR 3) · est. 185–265 lines
Files: `internal/sdd/status.go` (~:517, :759), `internal/sdd/subroute_test.go`, `cmd/biggz/cli_sdd.go` (~:1044–1053), `cmd/biggz/sdd_continue_picker_test.go`

- [x] 3.1 RED `internal/sdd/subroute_test.go`: declared `direct-inline`/`delegated-direct` → organic+subroute; undeclared omitted; invalid ignored; SDD suppresses; change-less → no route/subroute, no error, no synthetic dir.
- [x] 3.2 GREEN `internal/sdd/status.go`: `Subroute`; read `state.yaml` in both paths; emit only `route==organic` + declared.
- [x] 3.3 Update `sdd_continue_picker_test.go` + `cli_sdd.go` continue block: `route: organic`, `subroute: direct-inline`; omitted when undeclared.
- [x] 3.4 Close: tick boxes; `apply-progress.md`.
- [x] 3.5 Budget ≤400 (est. 185–265).
- Keep passing: `go test ./internal/sdd/ -run 'TestRoute|TestStatusV2'`; `go test ./cmd/biggz/ -run TestSddContinue`.

## Dependencies
1 → 2 → 3 stacked on main; 1 independent; 3 shares `cli_sdd.go` with 2 and lands after it. No gatekeeper/session/workload/edit-authority/verify changes; no `openspec/specs/**`, `config.yaml`, or extra `AGENTS.md` edits.

## Verification Map (16)

| Scenario | Task |
|---|---|
| ODD-001-S1/S2 | 1.2,1.3 |
| ODD-002-S1 | 1.2,1.3 |
| ODD-002-S2 | 1.1,1.2 |
| ODD-003-S1/S2 | 1.2,1.3 |
| ODD-004-S1 | 2.5 |
| ODD-004-S2 | GAP |
| OR-003-S1/S2/S3 | 3.1 |
| OR-003-S4 | 3.1 |
| OR-003-S5 | 3.3 |
| SS-ODD-001-S1 | 2.1,2.3 |
| SS-ODD-001-S2 | 2.3,2.5 |
| SS-ODD-001-S3 | 2.1,2.3 |

**Gap:** ODD-004-S2 has no owning task (prose); ODD-002-S2 and ODD-003-S1/S2 are textual-only.

## Rollback
- Slice 1: revert skill + `AGENTS.md` row, then delegation append; `odd/tasks/*.md` never reverted.
- Slice 2: drop `odd`, delete `odd.go`/tests.
- Slice 3: drop `Subroute` + continue lines; `route` stays `organic`.
- Deltas revert by deleting the change folder.
