# Tasks: orchestrator-synthesis-scannable — Scannable Synthesis + One-line Lifecycle

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~330 (synthesis.go ~100 + status.go ~120 + orchestrator.md ~30 + tests ~80) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Synthesis table+checklist+lifecycle+sanitize | PR 1 (single) | `go test ./internal/sdd -run TestSynthesis -count=1` | `biggz sdd-status` + checkpoint render manual | `internal/sdd/synthesis*.go` + `internal/assets/biggz/biggz-orchestrator.md` |
| 2 | sdd-status 4-block Outcome/Quick path/Details + banner | PR 1 (same) | `go test ./internal/sdd -run TestStatus -count=1` | `biggz sdd-status --json` terminal 40/60/80 cols | `internal/sdd/status*.go` revert alone |

## Phase 1: Foundation — Sanitize Pipeline Check

- [x] 1.1 Verify `internal/tui/sanitize.go` exports `VisibleWidth`, `TruncateToWidth`, `WrapTextWithAnsi` via `x/ansi`+`go-runewidth`; no change needed
- [x] 1.2 Add helper `sanitizeForWidth(s string, w int) string` contract: `ReplaceTabs` → `ansi.Strip` → strip OSC/CONTROL → `TruncateToWidth`

## Phase 2: Core Implementation — Synthesis + Lifecycle + Template

- [x] 2.1 Modify `internal/sdd/synthesis.go` `RenderSynthesis` to emit `| Topic | Decision |` table + `- [x]/[ ]` checklist, `◆ {phase} · {status} · {next}` one-line lifecycle (green/yellow/red + dim detail), `Preview` 300c sanitized + `Diff` `N files ±` sanitized, chunk <7 per-cell `TruncateToWidth`
- [x] 2.2 Modify `internal/sdd/synthesis_gate.go` keep 4 markers, update `HasSynthesis` to pass with table, add `renderLifecycle` helper, preserve `INVALID and will be blocked` + `engram==bigmem`
- [x] 2.3 Modify `internal/assets/biggz/biggz-orchestrator.md` template adds `| Topic | Decision |`, `- [x]`, `◆` placeholders; keep 4-marker example + INVALID rule + 12× REMINDER
- [x] 2.4 Modify `internal/sdd/status.go` (and `status_v2.go` if split) to 4 blocks `Status Overview / Artifact Progress / Next Action / Risks-Blockers` each `Outcome + Quick path + Details`; banner adaptive `TruncateToWidth`; collapse >7 → `… +N more` hint; empty omitted

## Phase 3: Testing — Goldens + Width + Docs Shape

- [x] 3.1 Add unit goldens: table replaces prose (no paragraph), checklist `- [x]/[ ]`, lifecycle color (success green/warning yellow/error red), `Preview` strip ANSI/OSC/controls 300c `VisibleWidth≤300`, `Diff` sanitized, chunk <7 per-cell `VisibleWidth≤budget`, CJK=2 ANSI=0 no split rune ends `…`
- [x] 3.2 Add integration fixtures at 40/60/80 cols: 4 blocks order `Outcome/Quick path/Details`, Quick path numbered `biggz sdd-sync`, collapse hint `… +N more`, banner truncate, `HasSynthesis` with table passes
- [x] 3.3 Verify doc coverage shape: `proposal/spec/design/tasks/verify-report` each starts `Outcome → Quick path → Details` table
- [x] 3.4 Run `go test ./... -count=1` and `go vet ./...` must pass; `orchestrator.test.go` markers + alias `engram==bigmem` invariant holds

## Phase 4: Cleanup

- [x] 4.1 Remove unused prose path in `RenderSynthesis`; ensure `ReadLoop` >50KB retry still verifies length
- [x] 4.2 Confirm no new deps, no migration; rollback is `git revert` single commit

