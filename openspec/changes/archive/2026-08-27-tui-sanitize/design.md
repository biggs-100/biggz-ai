# Design: tui-sanitize — TUI Sanitization + Git Wrapper

## Technical Approach

Port `oh-my-pi/packages/tui/src/utils.ts` to `internal/tui/sanitize.go` (5 pure helpers) via `go-runewidth` + `x/ansi` + `lipgloss` (promote indirect to direct). `VisibleWidth` UAX#11 (CJK 2, SGR 0); `WrapTextWithAnsi` SGR coalesce. Single git owner `internal/git/git.go` (`GitStatus`/`GitDiff`/`DetectGitDirs`, `os.IsNotExist` preserved) replaces `screens/status.go:detectGitDirs`. 3–4 screens adopt helpers; CI hard-fails `exec.Command("git",` outside `internal/git`. New domain `tui-sanitize` (not `hashline-lite` delta).

## Architecture Decisions

### Decision: Helper location

| Option | Tradeoff | Decision |
|---|---|---|
| Inline per-screen | Duplicated width bugs, no CJK/ANSI | Rejected |
| Extend `internal/filemerge` | Couples atomic-write to TUI render | Rejected |
| **`internal/tui/sanitize.go`** | Single width authority, matches `styles` pattern | **Chosen** |

Keeps `filemerge` untouched; pure helpers isolated.

### Decision: Width stack

| Option | Tradeoff | Decision |
|---|---|---|
| `lipgloss.Width` alone | Miscounts ANSI (SGR not stripped) | Rejected |
| Custom counter | Re-invents UAX#11, misses CJK | Rejected |
| **`go-runewidth` (UAX#11) + `x/ansi` strip, `lipgloss` for styles** | `VisibleWidth=runewidth.StringWidth(ansi.Strip(s))`; v0.0.19/v0.11.6 already indirect; SGR coalesce correct | **Chosen** |

Matches `oh-my-pi` split; `lipgloss` for render only.

### Decision: State model

| Option | Tradeoff | Decision |
|---|---|---|
| Per-batch snapshot `map[path][]byte` (hashline-lite) | Needed for hash guards, not width | Rejected |
| Global cache | Cross-screen pollution | Rejected |
| **Stateless pure `string→string/int`** | No lifecycle, table/golden tests, safe on narrow mux | **Chosen** |

All 5 pure transforms; contrasts with `hashline-lite` snapshot.

## Data Flow

```
raw (tabs/ANSI/CJK/path)
 ├─ ReplaceTabs → spaces
 ├─ VisibleWidth → ansi.Strip → runewidth.StringWidth → int
 ├─ TruncateToWidth → loop VisibleWidth, no wide-split, append … (w=1)
 ├─ WrapTextWithAnsi → parse SGR, wrap ≤w, re-apply active SGR, coalesce dup
 └─ ShortenPath → first/…/last or tail-… if w<4

screens.View(width) → helpers → lipgloss → Bubbletea frame
status.go → git.DetectGitDirs() → exec.Command("git", rev-parse) + IsNotExist
CI: rg 'exec\.Command.*git' → allowlist internal/git/** → fail otherwise
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/sanitize.go` | Create | 5 helpers (ReplaceTabs, VisibleWidth UAX#11, TruncateToWidth, WrapTextWithAnsi SGR coalesce, ShortenPath) |
| `internal/tui/sanitize_test.go` | Create | Table/golden: CJK, ANSI, truncate boundary, SGR coalesce |
| `internal/git/git.go` | Create | Single owner: GitStatus/GitDiff/DetectGitDirs, IsNotExist-safe |
| `internal/git/git_test.go` | Create | IsNotExist + trimmed parity with old inline |
| `internal/tui/screens/status.go` | Modify | Use `git.DetectGitDirs` |
| `internal/tui/screens/*.go` | Modify | 3–4 renderers width-guarded via helpers |
| `.github/workflows/ci.yml` | Modify | Hard-fail `rg exec\.Command.*git` outside allowlist |
| `go.mod`/`go.sum` | Modify | Promote `go-runewidth`, `x/ansi` to direct |

## Interfaces / Contracts

```go
// internal/tui/sanitize.go
package tui
func ReplaceTabs(s string) string
func VisibleWidth(s string) int // ansi.Strip → runewidth.StringWidth
func TruncateToWidth(s string, w int) string // ≤w, no wide split, … w=1
func WrapTextWithAnsi(s string, w int) []string // ≤w/line, SGR re-apply+coalesce
func ShortenPath(p string, maxWidth int) string // first/…/last; w<4 tail-…

// internal/git/git.go
package git
func DetectGitDirs() (commonDir, gitDir string)
func GitStatus(dir string) ([]byte, error)
func GitDiff(dir string, args ...string) ([]byte, error)
```

Invariants: `w≤0→""`; CJK=2 SGR=0; no split `中`; wrap prepends active SGR once.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | 5 helpers (tabs, CJK 1+2+1=4, ANSI, truncate+ellipsis, SGR coalesce, path) | `go test ./internal/tui -run TestSanitize` table/golden |
| Unit | Git wrapper IsNotExist + trimmed parity | `go test ./internal/git` with missing git + fixture |
| Integration | Screens ≤80-char no overflow, ANSI preserved | `View()` + `VisibleWidth` assert |
| CI gate | Forbid violation detection | Fixture file outside allowlist fails, inside passes |

`go vet` + `gofmt` + `go test ./... -count=1 -timeout 180s` clean.

## Threat Matrix

No routing/PR/push/commit/executable boundary changed. Wrapper consolidates subprocess; CI forbid check-only.

| Boundary | Adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `README.sh` | N/A: no classification changed | — | — |
| Git repository selection | `git -C`, relative/absolute | N/A: wrapper reuses caller dir/cwd like prior inline | — | — |
| Commit state | staged, `commit -a`, empty | N/A: no commit path | — | — |
| Push state | tracking, first push, refspec | N/A: no push path | — | — |
| PR commands | `--head`, env prefix, composed | N/A: no PR automation | — | — |

N/A rows need no tasks; IsNotExist + CI forbid in Testing Strategy above.

## Migration / Rollout

No migration. Revert: delete `sanitize.go`/`git.go`, revert screens, remove CI step, `go mod tidy` <5 min.

## Open Questions

- [ ] `ShortenPath` separator `/` vs `filepath.Separator` on Windows? Assume `/` per spec.
- [ ] Which 3–4 screens beyond `status.go`? Confirm in tasks.
