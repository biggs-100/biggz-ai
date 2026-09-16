# tui-sanitize Specification

## Purpose

Internal refactor: `internal/tui/sanitize.go` (5 helpers, port of `oh-my-pi/packages/tui/src/utils.ts` via `go-runewidth`/`x/ansi`/`lipgloss`) + `internal/git/git.go` single git owner + CI forbid. Prevents overflow/artifacts on Windows/mux. No user-facing API change; `tui` spec unchanged.

## Requirements

### Requirement: ReplaceTabs

The system MUST provide `ReplaceTabs(s string) string` replacing each `\t` with 4 spaces, preserving other runes/ANSI.

#### Scenario: Tabs expanded

- GIVEN `s="a\tb"`
- WHEN `ReplaceTabs(s)` called
- THEN result MUST be `"a    b"` with zero `\t`

#### Scenario: No tabs unchanged

- GIVEN `s="hello"` or `""`
- WHEN `ReplaceTabs(s)` called
- THEN result MUST equal input

### Requirement: VisibleWidth — UAX#11

The system MUST provide `VisibleWidth(s string) int` using `go-runewidth.StringWidth` after stripping ANSI via `x/ansi`. CJK MUST count 2, ASCII 1, SGR 0.

#### Scenario: CJK width

- GIVEN `s="a中b"` (1+2+1)
- WHEN `VisibleWidth(s)` called
- THEN MUST return 4

#### Scenario: ANSI stripped

- GIVEN `s="\x1b[31mhello\x1b[0m"`
- WHEN `VisibleWidth(s)` called
- THEN MUST return 5

### Requirement: TruncateToWidth

The system MUST provide `TruncateToWidth(s string, w int) string` fitting `VisibleWidth ≤ w`, never splitting wide runes, appending `…` (width 1) when truncating so final width ≤ w. `w≤0` MUST return `""`; fits MUST return input.

#### Scenario: Fits unchanged

- GIVEN `s="hello"` (`w=10`)
- WHEN `TruncateToWidth(s,w)` called
- THEN MUST return `"hello"`

#### Scenario: Truncates with ellipsis

- GIVEN `s="hello world"` width 11, `w=8`
- WHEN called
- THEN result MUST end with `…` and `VisibleWidth ≤ 8`

#### Scenario: CJK boundary

- GIVEN `s="a中b中c"`, `w=4`
- WHEN called
- THEN MUST NOT split `中` and width MUST be ≤ w

### Requirement: WrapTextWithAnsi — SGR Coalesce

The system MUST provide `WrapTextWithAnsi(s string, w int) []string` wrapping to `w` visible width, not breaking inside ANSI, re-applying active SGR on continued lines, coalescing adjacent duplicate SGR open/close to one.

#### Scenario: Wraps preserving color

- GIVEN `"\x1b[32mabcdefghij klmnop\x1b[0m"`, `w=10`
- WHEN `WrapTextWithAnsi` called
- THEN MUST return ≥2 lines each `VisibleWidth ≤10` and continuation lines start with `\x1b[32m` if still active

#### Scenario: Coalesce duplicates

- GIVEN `"\x1b[32m\x1b[32mhello\x1b[0m\x1b[0m"`
- WHEN called
- THEN output MUST contain at most one leading `\x1b[32m` and one trailing `\x1b[0m` per segment

### Requirement: ShortenPath — Middle Ellipsis

The system MUST provide `ShortenPath(p string, maxWidth int) string` shortening to `VisibleWidth ≤ maxWidth` via `first/…/last` when long; already fits MUST be unchanged; `maxWidth<4` MUST tail-truncate with `…`.

#### Scenario: Long path middle-shortened

- GIVEN `p="a/b/c/d/e/f/g.txt"`, `maxWidth=10`
- WHEN `ShortenPath` called
- THEN result MUST contain `…`, start with `a/` , end with `g.txt`, width ≤10

#### Scenario: Short unchanged

- GIVEN `p="src/main.go"`, `maxWidth=20`
- WHEN called
- THEN MUST return input

### Requirement: Git Wrapper — Single Owner

The system MUST provide `internal/git` as the sole owner of git process spawns in Go code, exposing `GitStatus`/`GitDiff` (migrating `status.go:detectGitDirs`) plus the surface other packages need so that none of them spawns git directly: a generic runner `Run(ctx, dir, args…)` returning byte-identical stdout and git's unmodified stderr, canonical absolute resolvers (`TopLevel`, `ResolveGitDirs`), and an env/limits-aware constructor for the isolated review runner (stripped `GIT_*`, `LANG=C`, byte caps, `--no-pager`). MUST preserve `os.IsNotExist` handling (not panic). All other Go code MUST NOT call `exec.Command`/`exec.CommandContext` with git and MUST NOT pass a literal `"git"` in command position to any helper. Non-Go host spawns are outside this invariant and are declared in the guard baseline.
(Previously: the wrapper exposed only `GitStatus`/`GitDiff` behind an unexported runner, so 66 spawn sites re-implemented the same primitives outside it.)

#### Scenario: Git missing handled

- GIVEN `git` not on PATH (`IsNotExist`)
- WHEN `GitStatus` called
- THEN MUST return handled error, not panic

#### Scenario: Preserves detectGitDirs semantics

- GIVEN worktree where `git rev-parse --git-common-dir` → `/repo/.git`
- WHEN `DetectGitDirs()` called
- THEN MUST return trimmed `commonDir`/`gitDir` identical to prior inline logic

#### Scenario: Generic runner preserves raw output and error

- GIVEN a caller that classifies "not a git repository" by git's stderr wording
- WHEN it runs the call through `git.Run(ctx, dir, …)`
- THEN stdout MUST be byte-identical and the error MUST carry git's unmodified stderr text

#### Scenario: Env-controlled spawn stays in the wrapper

- GIVEN the review frozen-inspector needs `GIT_*` stripped, `LANG=C` and byte caps
- WHEN it spawns git
- THEN the command MUST come from `internal/git`'s env/limits-aware constructor and a golden test MUST assert its resulting environment and args

### Requirement: CI Forbid Git Exec Outside Wrapper

CI (`.github/workflows/ci.yml`) MUST hard-fail through the compiled checker defined by `ci-guard-integrity` when a git spawn site exists outside `internal/git` and is not in the ratchet baseline. The step MUST observe the checker's own exit status — no pipeline whose status belongs to a later command, no `2>/dev/null` hiding a failing checker, no warn-only path. A checker that is absent, unbuildable, or exits non-zero for any reason MUST fail the step.
(Previously: the step was `rg … 2>/dev/null | grep -q .`; on a runner without `rg` the shell's 127 was swallowed, `grep -q .` saw empty stdin, and the step reported success.)

#### Scenario: Violation fails CI

- GIVEN `internal/tui/screens/foo.go` contains `exec.Command("git","status")`
- WHEN CI forbid step runs
- THEN MUST exit non-zero reporting file

#### Scenario: Allowlisted passes

- GIVEN only `internal/git/git.go` contains `exec.Command("git",`
- WHEN CI step runs
- THEN MUST exit zero

#### Scenario: Indirected write op fails CI

- GIVEN `cmd/biggz/pr.go` passes `"git"` to `runCmd` for `add`/`commit`/`push`
- WHEN CI forbid step runs
- THEN MUST exit non-zero naming that file:line

#### Scenario: Absent checker cannot report success

- GIVEN the checker binary is absent from the runner or fails to build
- WHEN the forbid step runs
- THEN the step MUST exit non-zero and MUST NOT print "No forbidden git exec found"

### Requirement: POLISH-TS-01 — Compact Fleet Token Rendering

The system MUST render fleet token metrics in compact form via `formatFleetTokens`/`render`. When `window == spent` or `window < 1000`, the system MUST hide `window` and show only `spent` compact (e.g., `2.2k`). When distinct and `window >= 1000`, it MUST show `window›spent` with `›` separator compact (e.g., `4.1k›2.2k`) in mono muted color and MUST NOT emit repeated `↓ window·spent` per row.

#### Scenario: Window equals spent hides window
- GIVEN `window==spent==2250`
- WHEN `formatFleetTokens` renders
- THEN output MUST be `2.2k` muted mono with no `›` and no `↓`

#### Scenario: Window distinct shows compact pair
- GIVEN `window=4100, spent=2200`
- WHEN rendered
- THEN output MUST be `4.1k›2.2k` with single `›` and muted style

#### Scenario: Window below threshold hides window
- GIVEN `window=800, spent=600`
- WHEN rendered
- THEN output MUST hide window and show only `0.6k` (or `600`) compact

### Requirement: POLISH-TS-02 — Fixed Right Columns Width Guarantees

The system MUST guarantee fixed right columns: `elapsed` 5 visible cells, `tokens` 10 visible cells right-aligned, never truncated to `…` for widths 80..120. The system MUST truncate left content to `floor((width-16)/2)`-like budget so right `visibleWidth` stays constant. `VisibleWidth` MUST use `go-runewidth` after stripping ANSI.

#### Scenario: Right columns constant 80→120
- GIVEN `width=80` and `width=120`
- WHEN same row renders
- THEN `elapsed` MUST be 5c and `tokens` 10c right-aligned with identical `visibleWidth` and no `…`

#### Scenario: Left truncates, right never truncates
- GIVEN narrow 80c row with long agent+model left
- WHEN `TruncateToWidth` applied
- THEN left MAY end with `…` but `elapsed`/`tokens` MUST NOT contain `…`

### Requirement: POLISH-TS-03 — Stable Truncation and CJK Width

The system MUST truncate only left field (L1 left) or L2 activity; it MUST never cut `elapsed`/`tokens` and MUST support recovery via Fleet inspector. `TruncateToWidth` MUST NOT split wide runes, MUST count CJK as width 2 and SGR as 0, and MUST append `…` (width 1) within budget. At 60c narrow, truncation MUST still preserve right columns.

#### Scenario: CJK counts width 2
- GIVEN `s="a中b"` and `w=4`
- WHEN `TruncateToWidth` called
- THEN width MUST be 4 and `中` MUST NOT be split

#### Scenario: Narrow 60c preserves right
- GIVEN `width=60`
- WHEN row renders with long L1+L2
- THEN truncation MUST affect L1-left or L2 only and `elapsed`/`tokens` MUST remain intact
