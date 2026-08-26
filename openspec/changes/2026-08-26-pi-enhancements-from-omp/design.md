# Design: Pi Enhancements from oh-my-pi (TUI Sync, Hashline, Web Anchors, Advisor)

## Technical Approach

Port 4 isolated enhancements from `oh-my-pi` without its Rust/Bazel platform. Each maps 1:1 to a delta: `tui` (CSI 2026 + bracketed paste), `filemerge` (hashline), `pi-web-search` (anchor-preserving fetch), `pi-integration` (advisor advise). Minimal edits to Go/JS assets, preserve SSRF/env contracts, sliceable commits ordered TUI→hashline→web→advisor.

## Architecture Decisions

### Decision: TUI sync gating

| Option | Tradeoff | Decision |
|---|---|---|
| Flag `BIGGZ_SYNC=1` | Explicit but manual | Rejected |
| Always CSI 2026 | Garbles `TERM=dumb` | Rejected |
| **Auto-detect: `TERM` supports AND `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION`unset AND `TERM!=dumb`** | Zero-config, safe | **Chosen** |

Central `syncOutput(frame) string` helper; screens opt in for flicker-prone views. Mirrors `omp/packages/tui` without porting renderer.

### Decision: Bracketed paste

| Option | Tradeoff | Decision |
|---|---|---|
| Bubbletea native | Requires upgrade, still splits | Rejected |
| **Detect `ESC[200~`/`ESC[201~`, buffer, emit single `PasteMsg`** | Atomic >10 lines, not keys | **Chosen** |

Flush on timeout/next non-paste for incomplete sequence.

### Decision: Hashline semantics

| Option | Tradeoff | Decision |
|---|---|---|
| Whole-file SHA-256 | False positives | Rejected |
| **Exact-range SHA-256 hex** | Precise | **Chosen** |
| Silent retry | Hides concurrency | Rejected |
| **Warn-and-stop: `needs_attention`+`freshHash`, no overwrite, batch continues** | Per assumption | **Chosen** |

Caller `correction.go` stores hash at read, validates at write. `force` bypasses.

### Decision: Web anchors

| Option | Tradeoff | Decision |
|---|---|---|
| Full readability lib | Heavy | Rejected |
| **Extend `htmlToMarkdown` regex: capture `id`→`## T {#id}`, preserve order** | Minimal, fixture-testable | **Chosen** |
| Split search/fetch paths | Diverges | Rejected |

Single `extractWithAnchors(html,baseUrl)` for both tools. Truncate annotates `[truncated: 1MB — offset at {#nearest}]`.

### Decision: Advisor advise

| Option | Tradeoff | Decision |
|---|---|---|
| Always on | Noise | Rejected |
| Model-based check | Cost/latency | Rejected |
| **Heuristic `paths<2\|\|len<50`, gated `BIGGZ_ADVISE=1`, default OFF, keep `PI_SUBAGENT_CHILD=1` bypass, `pi.notify` concern (not block)** | Zero cost, opt-in | **Chosen** |

Blocking on missing markers unchanged; advise only when markers present but thin.

### Decision: Implementation order

| Option | Tradeoff | Decision |
|---|---|---|
| Parallel PRs | Dependency risk | Rejected |
| **Sequential TUI→hashline→web→advisor** | <400 lines/slice, revertible | **Chosen** |

## Data Flow

```
TUI: tea.View → syncOutput? → ESC[?2026h+frame+ESC[?2026l] → term
     tea input → buf ESC[200~..ESC[201~] → PasteMsg → screen

Hashline: read range → ComputeHash → store BeforeHash
          ApplyWithHash? hash==disk → WriteFileAtomic : return HashMismatchError{freshHash}

Web: web_search/fetch → extractWithAnchors → markdown {#id} → 1MB cap+annotate

Advisor: question call → CHILD?skip : !hasSynthesis?block : thin&&ADVISE?notify(concern)+allow : allow
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/tui.go` | Modify | `syncOutput`, `isSyncSupported`, `PasteMsg`, paste buffer |
| `internal/tui/screens/*.go` | Modify | Opt flicker-prone views into `syncOutput` |
| `internal/tui/tui_test.go` | Modify | Sync marker, fallback, paste fixture tests |
| `internal/filemerge/hashline.go` | Create | `ComputeHash`, `ApplyWithHash`, `HashMismatchError` |
| `internal/filemerge/hashline_test.go` | Create | Range vs whole-file, concurrent, force tests |
| `internal/review/correction.go` | Modify | Store BeforeHash, validate at write |
| `internal/assets/pi/biggz-web-search.js` | Modify | `extractWithAnchors`, shared path, truncation annotate |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Advise branch, `BIGGZ_ADVISE` gate, heuristic |

## Interfaces / Contracts

```go
func ComputeHash(content []byte) string
type HashMismatchError struct { Code, FreshHash, Path string }
func ApplyWithHash(path, expectedHash string, newContent []byte, force bool) error

func syncOutput(frame string) string
func isSyncSupported() bool
type PasteMsg struct { Text string }

// JS: extractWithAnchors(html, baseUrl) -> {markdown, anchors[]}
// heading id="x" -> "## T {#x}", relative href resolved via baseUrl
// advisor: thin = pathsCount<2 || pathsLen<50; advise → pi.notify("concern","warning")
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit Go | sync auto-detect, PasteMsg, ComputeHash range, ApplyWithHash mismatch/force | `go test ./internal/tui ./internal/filemerge -count=1` |
| Unit JS | anchors, truncation annotate, malformed HTML, shared parity | Fixture HTML, no network |
| Integration | correction stores hash, concurrent 2-writer → needs_attention | Simulate stale hash write |
| Integration | advisor thin/rich × ADVISE on/off × CHILD bypass | Mock pi.on/registerTool |
| E2E | `go vet`+`go test ./...`, `biggz install --agent pi` | CI gate |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. TUI=escape wrapping, hashline=`os` file hash, web reuses SSRF guard, advisor=heuristic notify.

## Migration / Rollout

No migration. Flags: `BIGGZ_ADVISE` default OFF; existing `BIGGZ_NO_ANIMATION` reused. Each feature one commit, `git revert` safe, `biggz install` redeploys JS. Auto-chain slices if >400 lines.

## Open Questions

- [ ] `syncOutput` for all screens or opt-in subset? → Opt-in, measure in verify.
- [ ] `HashMismatchError` vs `review.Finding`? → Typed error mapped at boundary.
