# Proposal: tui-sanitize — TUI Sanitization + Git Wrapper

## Intent

Centralize TUI sanitization + git wrapper to prevent overflow/artifacts on Windows/mux and duplication. Renderers inline ad-hoc truncation without ANSI/CJK width; 4+ scattered `exec.Command("git",...)` duplicate `os.IsNotExist` handling. One-day quick win after hashline-lite.

## Scope

### In Scope
- `internal/tui/sanitize.go` — `ReplaceTabs`, `TruncateToWidth`, `WrapTextWithAnsi`, `ShortenPath`, `VisibleWidth` (go-runewidth + charmbracelet/x/ansi + lipgloss)
- Migrate 3-4 renderers (`internal/tui/screens/*.go`) to helpers
- `internal/git/git.go` — single wrapper `GitStatus`/`GitDiff` with `os.IsNotExist` handling
- CI guard forbidding `exec.Command("git",...)` outside `internal/git`
- Port logic from `oh-my-pi/packages/tui/src/utils.ts`

### Out of Scope
- Business logic changes, new lenses, hashline-lite modifications
- New palette/themes, animation, or screen UX redesign
- Whole-file git abstraction beyond status/diff

## Capabilities

### New Capabilities
- None — internal refactor, no user-facing spec change

### Modified Capabilities
- None — `tui` spec unchanged; sdd-spec adds delta if needed.

## Approach

Port `packages/tui/src/utils.ts` to Go via `go-runewidth` (CJK) + `x/ansi` (ANSI strip) + `lipgloss`. Pure helpers in `sanitize.go` with table tests. Migrate 3-4 screens. Extract `internal/git/git.go` (`GitStatus`/`GitDiff`), migrate `status.go:detectGitDirs`. CI forbid: `rg exec.Command.*git` allowlist `internal/git`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/tui/sanitize.go` | New | Helpers: tabs→spaces, truncate, wrap, shorten, width |
| `internal/tui/screens/*.go` | Modified | 3-4 renderers adopt helpers |
| `internal/git/git.go` | New | Single git wrapper (GitStatus/GitDiff) |
| `internal/tui/screens/status.go` | Modified | Use wrapper for `detectGitDirs` |
| `.github/workflows/ci.yml` | Modified | Forbid `exec.Command git` outside `internal/git` |
| `go.mod` | Modified | Promote deps to direct |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Width miscount (CJK/ANSI) breaks truncation | Low | `go-runewidth` + `x/ansi` + golden tests |
| Git wrapper regression (missing `IsNotExist`) | Low | Preserve `os.IsNotExist` pattern, table tests |
| Over-truncation on narrow mux panes | Low | `WrapTextWithAnsi` respects ANSI, existing width msg |

## Rollback Plan

`git revert` single commit: delete `internal/tui/sanitize.go` + `internal/git/git.go`, revert screen call sites to inline logic, remove CI forbid step, `go mod tidy`. No migration/data loss. <5 min.

## Dependencies

- `go-runewidth`, `x/ansi`, `lipgloss` (already indirect); `oh-my-pi/packages/tui/src/utils.ts` ref

## Success Criteria

- [ ] `sanitize.go` 5 helpers; `go test ./internal/tui -run TestSanitize` pass; ANSI+CJK correct
- [ ] 3-4 screens migrated, no overflow on 80-char pane
- [ ] `internal/git` sole `exec.Command git` owner; `rg` elsewhere = 0
- [ ] CI forbid fails on new git exec outside wrapper
- [ ] `go test ./... -count=1 -timeout 180s` + `go vet` + `gofmt` clean

## Proposal question round

Assumptions — answer/skip/correct:
1. Wrapper: `GitStatus`/`GitDiff` only vs broader?
2. `ShortenPath`: middle `a/…/z` vs end?
3. CI guard hard fail vs warn? Assumes minimal + middle + hard fail.
