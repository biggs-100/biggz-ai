# Tasks: tui-sanitize — TUI Sanitization + Git Wrapper

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 520–580 (+10 del, excl. go.sum) |
| 400-line budget risk | Medium |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Delivery strategy | auto-chain |
| Chain strategy | pending (single PR; split→stacked-to-main) |
| Suggested split | Single PR, 2 commits |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `sanitize.go` 5 helpers + `sanitize_test.go` | PR 1 c1 | `go test ./internal/tui -run TestSanitize -count=1` | `go vet ./internal/tui` + 80-char no-overflow | Del `sanitize*.go`, revert `go.mod` |
| 2 | `internal/git` wrapper + screen migrations + CI guard | PR 1 c2 | `go test ./internal/git -count=1 && go test ./... -count=1 -timeout 180s` | `rg 'exec\.Command.*git' --glob '!internal/git/**'`=0; `gofmt -l` clean | Del `internal/git/`, revert screens+`ci.yml` |

Commits revertible; if >800 split at unit boundary.

## Phase 1: Foundation

- [x] 1.1 Promote `go-runewidth`+`x/ansi` to direct in `go.mod`, `go mod tidy` — Done: `go list -m` direct, `go vet` clean.
- [x] 1.2 Create `internal/tui/sanitize.go` stubs for 5 helpers with docs — Done: `go vet ./internal/tui` pass.

## Phase 2: Core — Helpers + Git Wrapper

- [x] 2.1 `ReplaceTabs` (→4 spaces) + `VisibleWidth` (`ansi.Strip`→`runewidth.StringWidth`) — Done: `a中b`=4, ANSI `hello`=5.
- [x] 2.2 `TruncateToWidth` (w≤0→"", no wide-split, `…` w=1) — Done: fits unchanged, `hello world` w=8 ends `…` ≤8, CJK not split.
- [x] 2.3 `WrapTextWithAnsi` (wrap ≤w, SGR re-apply, coalesce dup) — Done: ≥2 lines ≤10, continuation has `\x1b[32m`, dup coalesced.
- [x] 2.4 `ShortenPath` (`first/…/last`, <4 tail-…) — Done: long `a/b/.../g.txt` ≤10, short unchanged.
- [x] 2.5 Create `internal/git/git.go` (`DetectGitDirs`/`GitStatus`/`GitDiff`, `os.IsNotExist` no panic) — Done: missing git returns error.

## Phase 3: Integration

- [x] 3.1 Migrate `internal/tui/screens/status.go` to `git.DetectGitDirs()` — Done: `rg detectGitDirs` only in `internal/git`.
- [x] 3.2 Migrate 3–4 renderers (`screens/*.go`) to helpers for `View(width)` — Done: `VisibleWidth(View(80))≤80`.
- [x] 3.3 Add CI hard-fail `rg 'exec\.Command.*git'` outside `internal/git/` in `.github/workflows/ci.yml` — Done: violation fails, allowlisted passes.

## Phase 4: Testing

- [x] 4.1 `sanitize_test.go` table/golden for 9 spec scenarios — Done: `go test ./internal/tui -run TestSanitize -count=1` pass.
- [x] 4.2 `git_test.go` (IsNotExist + trimmed parity) — Done: `go test ./internal/git -count=1` pass.
- [x] 4.3 Integration: `go test ./... -count=1 -timeout 180s` + `go vet ./...` + `gofmt -l` + `rg git` allowlist — Done: all clean.

## Phase 5: Cleanup

- [x] 5.1 Remove inline truncation, verify `filemerge` untouched, `go mod tidy` — Done: `git diff --stat` = 8 files.

## Dependencies

1→2→3→4→5. 2.5∥2.1–2.4. 3 needs 2; 4 needs 2+3. Threat N/A — no RED beyond 4.2/4.3.

## Evidence

```
go test ./internal/tui -run TestSanitize -count=1
go test ./internal/git -count=1
go test ./... -count=1 -timeout 180s
go vet ./...; gofmt -l .; rg 'exec\.Command.*git' -n --glob '!internal/git/**'
```
