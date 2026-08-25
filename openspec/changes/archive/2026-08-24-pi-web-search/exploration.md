# Exploration: pi-web-search

## Problem
`biggz-ai` invented `sdd-research` with an `open-web` evidence lane (`biggz-ai.sdd-research-capability/v1` + `biggz-ai.sdd-research/v1`) but Pi has **zero** web search/fetch tooling. `sdd-research` is currently read-only (`read/grep/find/ls` + BigMem) and blocked without a tool grant, so sub-agents cannot collect auditable open-web sources. Prior investigation noted `gentle-pi` explicitly blocks `webfetch`; biggz-ai inherited the gap. Without `web_search` + `web_fetch` with robust 403 handling, the `documentation | open-web` research lifecycle (`internal/assets/skills/_shared/research-lifecycle.md`) can never reach `done` for open-web requests.

## Current State

### How the system works today

- **Pi runtime**: `internal/agents/pi/adapter.go` — Pi config at `~/.pi/agent/` (`APPEND_SYSTEM.md`, `mcp.json`, `settings.json`). Install via `pi install npm:pi-subagents` (FleetView dispatcher, `0.56.0`). BigMem via `biggz-mcp` (`--tools=agent`, prefix `biggz`) provisioned atomically to `mcp.json` + `settings.json` (`ProvisionBigMemMCP`). Extensions auto-discovered from `~/.pi/agent/extensions/*.js` (`piExtensionsDir`). Two extensions today: `biggz-thinking-wrap.js`, `biggz-last-model.js`.
- **SDD sub-agents**: `internal/install/install.go:DeployPiSubAgents()` walks `assets.FS:skills/sdd-*/SKILL.md` → `~/.pi/agent/agents/sdd-*.md` with allowlisted `tools:`. Verified in `internal/install/pi_subagents_test.go`: `sdd-research` and `sdd-explore` are **read-only** (`read, grep, find, ls` + 9 BigMem tools + `mem_*` fallbacks); `sdd-apply` has `read/edit/bash/write`. No `web_*` tools. `internal/assets/opencode/sdd-overlay-multi.json` declares `sdd-research` tools identically (read/write/edit/bash + BigMem only).
- **Research lane**: `internal/assets/skills/sdd-research/SKILL.md` + `internal/assets/prompts/sdd/sdd-research.md` + `internal/assets/skills/_shared/research-lifecycle.md` — admits ONLY `biggz-ai.sdd-research-capability/v1` with exact `documentation`/`open-web` grants; never infers from Bash/MCP/filenames. Persists `biggz-ai.sdd-research/v1` + `biggz-ai.sdd-preproposal/v1` (BigMem `sdd/{change}/research` + `openspec/changes/{change}/research.md`). Hybrid requires identical bytes + revision in both stores. Denial/partial blocks `sdd-propose`.
- **Install/Doctor**: `internal/install/install.go:Run()` deploys `DeployPiSubAgents`, `DeployPiThinkingWrap`, `DeployPiLastModel`, removes legacy `biggz-pi-pretty.js` (pi-pretty via `npm:pi-pretty/dist/index.js` `pi.extensions`). `internal/doctor/pi.go` checks `~/.pi/agent/npm/node_modules/pi-subagents` + `biggz-last-model.js`.
- **Config**: `openspec/config.yaml` — Go project, no test runner yet, all 7 phases required. `go.mod` Go 1.25, no web/TLS/headless deps today.

### Verification against prior investigation claims
- ✅ Pi has NO web search (grep `web_search|web_fetch|TAVILY|BRAVE` = 0 hits — confirmed).
- ✅ `sdd-research` skill exists (dual-tier) and is read-only + BigMem — confirmed via `DeployPiSubAgents` + overlay.
- ✅ FleetView via `pi-subagents` + BigMem MCP + last-model sync — confirmed via `adapter.go`, `doctor/pi.go`, `biggz-last-model.js`.
- ✅ No `pi/biggz-web-search.js` exists yet (`internal/assets/pi` has only 2 files) — gap confirmed.
- ⚠️ TLS impersonation claim (curl_cffi/tls-client) not yet in Go deps — must choose Go equivalent (`fhttp`/`utls`/`tls-client` Go port). Prior `gentle-pi` 403 history implies JA3 block is real but not yet handled.

### Affected Areas
- `internal/assets/pi/biggz-web-search.js` — **NEW** Pi extension (`web_search` + `web_fetch` tools, 3-tier fetch, provider dispatch, Markdown extract) — why: Pi extensions are the only way to add tools to Pi (no MCP for web; MCP is BigMem only).
- `internal/install/install.go` — add `piWebSearchDir()`, `DeployPiWebSearch()`, wire into `Run()` (like `DeployPiThinkingWrap`/`DeployPiLastModel`), self-heal legacy, add `PiWebSearch` to `Result` — why: deploy pattern is established; must be atomic via `filemerge.WriteFileAtomic`.
- `internal/install/pi_subagents_test.go` — extend to assert `sdd-research` allowed tools include `web_search`/`web_fetch` when open-web granted — why: current test explicitly asserts tool allowlist.
- `internal/assets/opencode/sdd-overlay-multi.json` — add `web_search`/`web_fetch` to `agent.sdd-research.tools` (conditional on grant) or to extension-provided tools — why: overlay is the OpenCode-visible agent definition.
- `internal/assets/skills/sdd-research/SKILL.md` + `internal/assets/prompts/sdd/sdd-research.md` — document allowed tools, capability + env-key gating — why: research skill must name its tool contract.
- `internal/assets/skills/_shared/research-lifecycle.md` — no change expected (lifecycle already handles `open-web`), but verify `sdd-propose` gate stays fail-closed on `blocked`.
- `internal/doctor/pi.go` — add `PiWebSearchCheck` (extension file exists, env keys present, provider reachable) with `Remedy()` via `biggz install --agent pi` — why: follows `PiSubagentsCheck`/`PiLastModelCheck` pattern.
- `cmd/biggz/cli_doctor_help.go` — register new check in `doctorRun()` slice — why: doctor runner enumerates checks explicitly.
- `internal/assets/embed.go` — `//go:embed all:pi` already covers new file, no change — why: `all:pi` includes `biggz-web-search.js` automatically.
- `go.mod` / `go.sum` — potential new deps if fetch fallback is Go-native (e.g., `github.com/bogdanfinn/tls-client` or `github.com/refraction-networking/utls` + `github.com/go-shiori/go-readability`) — why: TLS + Markdown extract need libraries.
- `docs/comparison-with-gentle.md` — update “What biggz-ai has that gentle-ai doesn't” once shipped.

## Approaches

### 1. Hosted Search API + Simple Fetch (Baseline)
Pi extension `biggz-web-search.js` calls **Tavily** (LLM-native, `TAVILY_API_KEY`) → fallback **Brave Search API** (`BRAVE_API_KEY`) → fallback **DuckDuckGo html** (no key). Fetch via `fetch()`/`Go net/http` with full browser headers + `trafilatura`-like extract to Markdown (simple `go-readability` + `html-to-markdown`). Exponential backoff + `Retry-After` respect, hard fail on 403 (`FetchBlocked` never silent).

- Pros:
  - Minimal deps (no `cgo`, no headless), smallest bundle, easiest to review (<400 lines).
  - Tavily is purpose-built for LLM RAG (answer + sources), Brave is reliable general search, DuckDuckGo covers no-key dev path.
  - Aligns with biggz-ai’s Go-native `biggz-mcp` pattern (no Python sidecar).
  - Fits current Pi extension model (pure JS `fetch`, no native addon).
- Cons:
  - **403 bot blocks remain** on Cloudflare/DDG/WAF sites — exactly the researched failure mode (`gentle-pi` tests block `webfetch`).
  - No JA3/TLS impersonation, no JS challenge handling — `sdd-research` will `partial` on hard sites.
  - DuckDuckGo HTML scraping is brittle and rate-limited.
- Effort: **Low** (1–2 days, extension + deploy + doctor, no new Go deps beyond readability).
- When to choose: MVP if 403 is rare or host-allowlist avoids WAF.

### 2. Hosted Search API + TLS Impersonation (Recommended Core)
Same providers as (1), but fetch is **3-tier with TLS fingerprint rotation**: Tier 1 — Go `net/http` + full browser headers; Tier 2 — `tls-client`/`utls` impersonation (`chrome124`/`safari17` JA3) via Go library (`github.com/bogdanfinn/tls-client` or `github.com/refraction-networking/utls` + `fhttp`); Tier 3 — optional no-op or explicit `FetchBlocked` with evidence. Same Markdown extract + backoff. Extension shells out to a tiny Go helper (`biggz-web-fetch`) or uses a JS `tls-client` equivalent if available in Pi’s Node 20+.

- Pros:
  - Directly addresses the **researched 403 via TLS impersonation** (curl_cffi/tls-client pattern) without headless cost.
  - Handles JA3/TLS fingerprint blocks that defeat header-only fetch (Cloudflare “403 bot” vs “403 ASN” distinction).
  - Still lightweight (single Go binary, no Chromium download, ~5–10MB).
  - Browser fingerprint rotation is cheap and composable with provider fallback.
- Cons:
  - Adds a Go dep with `utls` maintenance risk (Chrome version drift, `cgo` not needed but `utls` tracks upstream).
  - Impersonation is not a silver bullet — Cloudflare Turnstile/CAPTCHA/JS challenge still blocks (needs headless).
  - Pi extension → Go helper IPC adds complexity (extension spawns `biggz-web-fetch` vs pure JS).
- Effort: **Medium** (3–5 days, add `tls-client`, helper binary, header/fingerprint rotation, tests for 403 fixture).
- When to choose: **Default** — solves the stated “without 403 bot blocks” requirement at reasonable cost; matches the prior investigation’s “must handle 403 via TLS impersonation”.

### 3. Hosted Search API + TLS + Headless Patchright Fallback (Full Robust)
Same as (2) plus Tier 3 = **Patchright/Playwright headless** (Chromium) for JS-challenge sites, gated by `BIGGZ_WEB_FETCH_HEADLESS=1` or capability flag. exponential backoff across all tiers, then `FetchBlocked` with evidence. Still emit Markdown via `trafilatura`-like extract after headless render.

- Pros:
  - Strongest coverage: defeats JS challenges, Cloudflare `cf_clearance`, SPA-rendered docs.
  - Matches the prompt’s “3-tier: Go net/http → tls-client impersonation → optional Patchright” exactly.
  - Can reuse for future `documentation` lane that needs rendered JS docs.
- Cons:
  - **Heavy**: Chromium download (~150–400MB), flaky in CI/Windows, slow (seconds per fetch), `biggz doctor` must check headless availability.
  - Security surface: headless browser in a Pi sub-agent needs SSRF allowlist, timeout/RAM caps, and `granted_roots`-style scoping.
  - Most `sdd-research` sources are docs/blogs that do NOT need JS — overkill for ~80% of queries.
  - Increases review budget risk (>400 lines, needs chained PR).
- Effort: **High** (1–2 weeks, `patchright` Node dep, Go ↔ Node bridge, Windows CI, flake mitigation).
- When to choose: Only if Tier 2 still yields >10% `FetchBlocked` on real `sdd-research` runs; gate behind explicit env/capability to keep happy-path cheap.

| Approach | 403 Coverage | Deps / Size | Complexity | Maintenance |
|----------|--------------|-------------|------------|-------------|
| 1 Hosted + Simple | Low (headers only) | None / ~0 | Low | Low |
| 2 Hosted + TLS (Recommended) | Medium (JA3) | 1 Go lib / ~10MB | Medium | Medium (utls version) |
| 3 Hosted + TLS + Headless | High (JS) | Chromium / ~300MB | High | High (browser) |

## Recommendation
**Approach 2 (Hosted API + TLS Impersonation) as the single-PRs-shippable core, with Approach 3 behind a feature flag.**

Rationale:
- Directly satisfies `Change: pi-web-search — add web search + robust web fetch tools to pi agent so sub-agents can do sdd-research with open-web evidence without 403 bot blocks` with evidence-backed 403 handling (JA3), without paying headless cost upfront.
- Fits biggz-ai constraints: Go project (`go 1.25`, no Python), `~/.pi/agent/extensions/*.js` auto-discovery, `filemerge.WriteFileAtomic` deploy, `biggz-ai.sdd-research-capability/v1` open-web gate + env keys (`TAVILY_API_KEY`/`BRAVE_API_KEY`) + allowlist (`sdd-research` only).
- Staged rollout protects review budget (400-line guard): Phase 1 ships Approach 1 (extension + providers + simple fetch + Markdown), Phase 2 adds TLS impersonation as a follow-up slice or same PR if under budget. Headless (Approach 3) becomes a separate change only on measured `FetchBlocked` evidence.
- Failure mode is explicit: exponential backoff + `Retry-After`, then `FetchBlocked` (never silent), so `sdd-research` correctly emits `partial`/`blocked` with auditable sources rather than hallucinating.

Implementation sketch (recommended):
- `internal/assets/pi/biggz-web-search.js` — registers `web_search` (query, limit, provider fallback) + `web_fetch` (url, extract Markdown, 3-tier, rotation) via Pi `ExtensionAPI` (verify `pi.registerTool` exists; fallback to `pi.registerCommand` + MCP relay if Pi lacks tool API — spike required).
- `internal/install/install.go` — `DeployPiWebSearch()` (like `DeployPiThinkingWrap`), wire into `Run()`, add `piWebSearchDir()` helper, remove legacy if needed.
- Capability gating: `sdd-research` only sees `web_search`/`web_fetch` when `biggz-ai.sdd-research-capability/v1` grants `open-web` AND (`TAVILY_API_KEY` or `BRAVE_API_KEY` or explicit `BIGGZ_DDG_FALLBACK=1`). DuckDuckGo fallback is no-key but marked `publisher: DuckDuckGo (no-key fallback)` in `research.md` sources.
- Markdown extract: `go-readability` or `github.com/JohannesKaufmann/html-to-markdown` (or JS `turndown` in extension) to satisfy `biggz-ai.sdd-research/v1` source `excerpt` + claim traceability.

## Risks
- **Pi Extension Tool API unknown** — `biggz-last-model.js`/`biggz-thinking-wrap.js` only use `pi.on`/`pi.registerCommand`; `pi.registerTool` for `web_search`/`web_fetch` is assumed from `gentle-pi`/`pi-subagents` patterns. If Pi lacks tool registration, fallback is an MCP server or `subagent` delegation — both inflate scope. *Mitigate*: spike a minimal `registerTool` extension before spec.
- **TLS lib churn** — `utls`/`tls-client` tracks Chrome versions; outdated impersonation re-triggers 403. *Mitigate*: pin `chrome124`/`safari17`, add weekly `doctor` check for stale fingerprint, allow header-only fallback.
- **Provider keys & cost** — Tavily/Brave require keys, rate-limited, billable; missing keys degrade to DuckDuckGo scraping (brittle, TOS risk). *Mitigate*: gate via `biggz-ai.sdd-research-capability/v1` + env, fail loudly with `blocked` + `Gaps: missing TAVILY_API_KEY`, add `biggz doctor --fix` hint.
- **Headless scope creep** — if Approach 3 is bundled, PR exceeds 400-line budget, Windows CI flakes, and SSRF surface widens. *Mitigate*: keep Approach 3 behind `BIGGZ_WEB_FETCH_HEADLESS` flag, separate change, allowlist `open-web` domains, timeout (10s) + size cap (1MB Markdown).
- **Hybrid persistence divergence** — `sdd-research` must write identical `research.md` bytes to BigMem + filesystem (like `research-lifecycle.md` gate). Web fetch flake causes one-sided write → `blocked` on divergence. *Mitigate*: retain `Retain Intent Before Source Access` (canonical desired content) and re-verify both stores before readiness, as spec requires.
- **Legal/TOS** — scraping DuckDuckGo HTML or fetching with impersonation may violate site TOS. *Mitigate*: prefer hosted APIs (TOS-compliant), require `open-web` grant, log `publisher`/`URL`/`accessed_at` for audit, never infer capability.

## Open Questions
1. **Pi tool registration**: Does `ExtensionAPI` expose `pi.registerTool(name, schema, handler)` in the Pi version biggz-ai targets, or must `web_search`/`web_fetch` be exposed via MCP (`biggz-mcp` new `--tools=web` group) or `subagent` delegation? Spike needed before `sdd-propose`.
2. **TLS helper shape**: Pure JS `fetch` with `tls-client` npm vs Go helper `biggz-web-fetch` spawned by the extension? Go helper reuses `biggz` binary path resolution (`BiggzMCPPath` pattern) but adds IPC.
3. **Provider priority & key presence**: Order `Tavily → Brave → DuckDuckGo` or `Brave → Tavily`? Tavily is better for RAG, Brave for freshness. Should missing keys block `sdd-research` (`blocked`) or degrade to `partial` with gap?
4. **Markdown extract library**: Go `go-readability` vs JS `turndown` in extension vs Python `trafilatura` sidecar? Must be auditable and not require Python.
5. **Headless gate**: Should `BIGGZ_WEB_FETCH_HEADLESS` be a `biggz-ai.sdd-research-capability/v1` grant (`open-web-headless`) or a simple env flag? Capability gate is stricter and lifecycle-correct.
6. **SSRF / allowlist**: Should `web_fetch` be restricted to `open-web` domains or allow any URL? Proposal should define `In Scope` (docs, blogs) vs `Out of Scope` (internal, localhost).
7. **Doctor coverage**: Should `biggz doctor` probe a live `TAVILY_API_KEY` with a test query (cost) or just check env presence? Prefer presence-only + `--fix` hint.

## Ready for Proposal
**Yes** — with one spike before `sdd-propose`: verify Pi `ExtensionAPI` tool registration (`pi.registerTool`) in the installed Pi version and confirm `~/.pi/agent/extensions/*.js` can expose `web_search`/`web_fetch` to `sdd-research` sub-agents. If the spike passes, proceed to `sdd-propose` with the staged Approach 2 plan; if it fails, proposal must choose the MCP fallback and adjust affected areas accordingly.

- Next phase: `sdd-propose` (interactive question round on provider priority, headless gate, and SSRF scope).
- Artifact: `openspec/changes/pi-web-search/exploration.md` (this file).
