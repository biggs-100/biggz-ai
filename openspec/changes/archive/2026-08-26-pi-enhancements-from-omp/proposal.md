# Proposal: Pi Enhancements from oh-my-pi (TUI Sync, Hashline, Web Anchors, Advisor)

## Intent

biggz-ai runs inside Pi as a harness. Pi's upstream fork `oh-my-pi` (omp.sh, fork of mariozechner/pi) ships UX and reliability improvements that biggz-ai currently lacks. This change ports 4 high-ROI enhancements (excluding themes per user request) to make Pi usage flicker-free, edits robust under concurrency, research auditable, and SDD orchestration self-checking — without importing omp's Rust/Bazel/desktop platform.

## Scope

### In Scope
- **TUI synchronized output + bracketed paste** — `internal/tui/tui.go` and screens: wrap renders in CSI 2026 (`ESC[?2026h`/`ESC[?2026l`) for atomic updates, handle bracketed paste (`ESC[200~` / `ESC[201~`) for >10 line pastes. Mirrors `omp/packages/tui` differential rendering + sync.
- **Hashline (edit by content hash)** — `internal/filemerge/` + `internal/review/correction.go`: add `Hashline` helper that computes content hash of target range, validates before edit, and retries with fresh hash on mismatch. Prevents `file modified since last read` when two subagents edit nearby regions. Inspired by `omp/hashline`.
- **Web search with markdown anchors** — `internal/assets/pi/biggz-web-search.js`: keep provider chain Tavily->Brave->DDG, but on fetch return structured markdown with anchors/headings preserved (reuse readability port), and make `web_fetch` return same shape. No API key change.
- **Advisor inline watchdog (advise mode)** — `internal/assets/pi/biggz-synthesis-gate.js`: extend from blocking gate to dual-mode. Keep blocking when synthesis markers missing, add non-blocking `advise` that injects `concern` note via `pi.on(tool_call)` when orchestrator synthesis is thin (has markers but low artifact detail). Complements existing gatekeeper; never auto-fixes.

### Out of Scope
- Themes (explicitly excluded by user)
- omp Rust core (~80K), Bazel, Bun workspaces, Python/Bun kernel bridge, `computer`/`browser-relay`/`collab-web`, Hindsight/Mnemopi backends, ACP/Zed, `snapcompact`, `stats`, `dap/lsp` ops — all deferred as platform growth, not harness growth
- New providers or MCP servers

## Capabilities

### New Capabilities
- `tui-sync`: atomic TUI rendering with synchronized output and bracketed paste
- `hashline`: content-hash guarded edits for concurrent SDD apply
- `web-anchors`: anchor-preserving markdown fetch for research
- `advisor-advise`: non-blocking inline concern injection for thin synthesis

### Modified Capabilities
- `tui`: screens gain sync/bracketed-paste awareness
- `filemerge`: gains hash validation
- `pi-web-search`: gains readability anchor path
- `synthesis-gate`: gains advise branch

## Approach

1. **TUI sync** — Add `syncOutput` helper in `internal/tui/tui.go` that wraps `Render()` with `ESC[?2026h` prefix and suffix. Add `bracketedPaste` detection in input handling: if `ESC[200~` seen, buffer until `ESC[201~`, emit single paste event. Gate behind `BIGGZ_NO_ANIMATION` already exists. Port tests from `omp/packages/tui` for sync markers.
2. **Hashline** — New `internal/filemerge/hashline.go`: `ComputeHash(content)`, `ApplyWithHash(path, oldHash, newContent)`. Modify `edit` tool path to compute hash at read time, store in `correction.go` budget, validate at write time; on mismatch re-read and surface `needs_attention` with fresh hash instead of aborting whole batch. Unit tests with concurrent edit simulation.
3. **Web anchors** — In `biggz-web-search.js`, after fetch, run existing readability path but preserve `id` anchors and heading hierarchy; annotate truncated (1MB) with anchor offset. Ensure `web_fetch` uses same path. Add `web-tools.md` note. Tests: no network, use fixture HTML.
4. **Advisor advise** — In `biggz-synthesis-gate.js`, keep `PI_SUBAGENT_CHILD` guard. Add `ADVISE_MODE` (env `BIGGZ_ADVISE=1` or settings). When markers present but `Artifacts/Paths` count < expected (heuristic: <2 paths or <50 chars), emit `pi.notify` concern instead of blocking. No model call; purely heuristic.

Order: TUI sync first (isolated), then hashline (filemerge), then web anchors (JS only), then advisor (JS only). Each is independently shippable; auto-chain will slice if >800 lines.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/tui/tui.go` | Modified | Add sync wrapper + bracketed paste detection |
| `internal/tui/screens/*` | Modified | Opt into sync where flicker observed |
| `internal/filemerge/*` | Modified + New | New `hashline.go`, modify write path |
| `internal/review/correction.go` | Modified | Store hash, budget check |
| `internal/assets/pi/biggz-web-search.js` | Modified | Anchor-preserving readability |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Add advise branch |
| `openspec/specs/tui/spec.md` | Modified | Add sync/bracketed paste requirements |
| `openspec/specs/pi-integration/spec.md` | Modified | Add hashline + advisor requirements |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| CSI 2026 unsupported terminal garbles output | Low | Detect `TERM`/`BIGGZ_NO_ANIMATION`, fallback to plain render; gate tests |
| Hashline false positives block valid edits | Med | Hash only the exact range, not whole file; allow force flag |
| Web anchor parsing breaks on malformed HTML | Med | Keep 1MB truncate+annotate, never throw; fixture tests |
| Advise noise spams orchestrator | Med | Heuristic thresholds + env opt-in, default off until proven |

## Rollback Plan

Revert the 4 commits independently (each feature is one commit). `git revert` restores TUI/filemerge/JS to prior. No data migration; `biggz install` redeploys JS assets.

## Dependencies

- Existing `internal/tui` + `internal/filemerge` + Pi asset deploy pipeline (`internal/install`)
- Existing `biggz-web-search` provider chain and SSRF guards

## Success Criteria

- [ ] `internal/tui` renders wrapped in `ESC[?2026h/l` and bracketed paste >10 lines arrives as single event (test with fixture sequence)
- [ ] `hashline` prevents `file modified since last read` on concurrent nearby edits (property test with 2 writers)
- [ ] `web_search` fixture HTML returns markdown with `# heading {#anchor}` preserved; truncate annotates
- [ ] `synthesis-gate` in advise mode emits `concern` (not block) on thin synthesis, still blocks on missing markers
- [ ] `go test ./...` + `go vet` pass; `biggz install --agent pi` redeploys JS without error
