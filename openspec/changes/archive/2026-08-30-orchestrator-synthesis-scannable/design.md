# Design: orchestrator-synthesis-scannable — Scannable Synthesis + One-line Lifecycle

## Technical Approach

Port gentle's table + checklist + one-line lifecycle + sanitized truncation to biggz synthesis and `sdd-status`. Reuse `internal/tui/sanitize.go` (`VisibleWidth`/`TruncateToWidth` via `x/ansi` + `go-runewidth`) as single width source. `RenderSynthesis` emits `| Topic | Decision |` + `- [x]/[ ]` checklist, single `◆ Phase · Status · Next` (color + dim detail), and sanitizes `Preview` 300c + `Diff` `N files ±` before measure. `Outcome + Quick path + Details` applied to chat, `sdd-status` 4 blocks, and docs. Follows `cognitive-doc-design`.

Maps to proposal and specs: `orchestrator` (checkpoint synthesis + sanitized truncation/chunking) and `sdd-status` (four-block + progressive disclosure).

## Architecture Decisions

### Decision: Sanitization pipeline

| Option | Tradeoff | Decision |
|---|---|---|
| Custom strip + manual width | Reinvents `sanitize.go`, CJK/ANSI drift | Rejected |
| Reuse `sanitize.go` + `ansi.Strip` + `TruncateToWidth` before `VisibleWidth` | One pipeline, ANSI=0 CJK=2, no split wide rune, goldens | **Chosen** |

Pipeline: `ReplaceTabs` → `ansi.Strip` (ANSI/OSC/controls) → `TruncateToWidth` → `VisibleWidth`. Mitigates width miscount risk.

### Decision: Table chunking

| Option | Tradeoff | Decision |
|---|---|---|
| Single large table | Overflows narrow mux, breaks 5s scan | Rejected |
| Chunk <7 rows, per-cell `TruncateToWidth` to column budget, `… +N more` hint | Fits 40–60 cols, progressive disclosure, `VisibleWidth ≤ budget` | **Chosen** |

Used in chat, `sdd-status` Details, docs. Banner uses same `TruncateToWidth`.

### Decision: Lifecycle rendering

| Option | Tradeoff | Decision |
|---|---|---|
| Multi-line banner | Verbose, not scannable | Rejected |
| One line `◆ {phase} · {status} · {next}` + dim detail, `success=green warning=yellow error=red` | Scannable, keeps 4-marker invariant | **Chosen** |

Keeps 4 markers, adds `| Topic | Decision |`, `- [ ]`, `◆`; preserves `INVALID and will be blocked` + `engram==bigmem`.

## Data Flow

```
SubAgentResult → sanitize(Strip+TruncateToWidth) → RenderSynthesis → chat (table+checklist+◆+Preview+Diff)
                                    ├─ Preview 300c + Diff sanitized
                                    └─ chunk <7, per-cell truncate

Status JSON → FormatStatus → 4 blocks (Outcome/Quick path/Details) → terminal
               ├─ Banner TruncateToWidth (adaptive)
               └─ Collapse >7 → … +N hint
```

Sanitize before every `VisibleWidth`; empty blocks omitted; failure → `humanizeFailure`; >50KB → `ReadLoop` retry verify.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/sdd/synthesis.go` | Modify | Table `\| Topic \| Decision \|` + checklist replaces prose; `Preview` 300c + `Diff` via `tui.TruncateToWidth`; chunk <7 per-cell |
| `internal/sdd/synthesis_gate.go` | Modify | Keep 4 markers, `HasSynthesis` passes with table, add `◆` lifecycle helper |
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | Template adds table + `- [ ]` + `◆` placeholders; keep 4 markers + `INVALID`; preserve 12× REMINDER + alias |
| `internal/sdd/status.go` | Modify | 4 blocks `Status Overview / Artifact Progress / Next Action / Risks-Blockers` each `Outcome + Quick path + Details`; banner adaptive; collapse hint |
| `internal/tui/sanitize.go` | Reuse | No change; source of `VisibleWidth`, `TruncateToWidth`, `WrapTextWithAnsi` |

## Interfaces / Contracts

```go
func RenderSynthesis(r SubAgentResult) string // unchanged sig, new rendering
func HasSynthesis(md string) bool             // 4 markers (+table)
func VisibleWidth(s string) int
func TruncateToWidth(s string, w int) string
// new internals
func sanitizeForWidth(s string, w int) string
func chunkTable(rows [][]string, max int) [][][]string // max 7
func renderLifecycle(phase, status, next string) string // ◆ colored
func formatPreview(a string) string // strip → 300 + …
func formatDiff(s string) string    // N files ± sanitized
```

`SubAgentResult` unchanged; `WhatDone` → table rows; `Diff` = summary sanitized; all outputs `VisibleWidth`-bounded.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Table replaces prose, checklist, lifecycle color, Preview/Diff sanitized, chunk <7, banner truncate, empty omitted, alias | Goldens; `VisibleWidth ≤ budget`; CJK=2 ANSI=0 OSC stripped, no split rune, ends `…` |
| Integration | 4 blocks order + `Outcome/Quick path/Details`, Quick path numbered `biggz sdd-sync`, collapse `… +N more`, >50KB loop | Fixtures at 40/60/80 cols; `HasSynthesis` with table |
| E2E | Synthesis same-turn checkpoint, `orchestrator.test.go` markers + INVALID, `go vet` | `go test ./... -count=1`, `go vet ./...` |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Presentation-only reuse of `sanitize.go`; no new commands or paths.

## Migration / Rollout

No migration required. Rollback: `git revert` single commit (<5 min). Delete change dir if needed. No flag.

## Open Questions

- [ ] `sdd-status` width fallback for non-TTY — default 80?
- [ ] Lifecycle palette via `internal/tui/styles` vs inline SGR — follow existing styles.

