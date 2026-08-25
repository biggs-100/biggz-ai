# Proposal: pi-web-search

## Intent

`sdd-research` has `open-web` but Pi has no `web_search`/`web_fetch` — researchers are read-only+BigMem and always `blocked`/`partial`. Add web tools so `sdd-research` collects auditable evidence with 403-resilient fetch.

## Scope

### In Scope
- Extension `biggz-web-search.js`: `web_search` (query/limit) + `web_fetch` (URL→Markdown)
- Providers: **Tavily (`TAVILY_API_KEY`) → Brave (`BRAVE_API_KEY`) → DuckDuckGo** (no-key)
- 3-tier fetch: T1 `net/http`+headers → T2 TLS (`tls-client`/`utls` `chrome124`/`safari17`) → T3 headless `BIGGZ_WEB_FETCH_HEADLESS=1`
- Markdown extract, exponential backoff+`Retry-After`, loud `FetchBlocked`
- Gated to `sdd-research` only when `open-web`+keys (or `BIGGZ_DDG_FALLBACK=1`)
- `DeployPiWebSearch()` + `PiWebSearchCheck`

### Out of Scope
- Headless bundle (separate change if `FetchBlocked` >10%)
- Other lanes stay read-only+BigMem — only `sdd-research` needs open-web
- Python sidecar — Go-native only

## Capabilities

### New Capabilities
- `pi-web-search`: `web_search`/`web_fetch`, fallback, 3-tier/TLS, Markdown extract, exponential backoff, `FetchBlocked`, SSRF, headless flag

### Modified Capabilities
- `agent-install`: `DeployPiWebSearch()`/`Result.PiWebSearch` via `filemerge.WriteFileAtomic`
- `system-diagnostics`: `PiWebSearchCheck` (file+env, no live probe)

## Approach

Exploration **Approach 2 (Hosted API + TLS) as core**, Approach 3 flagged. Extension via Pi `ExtensionAPI` (`pi.registerTool` spike; fallback MCP). Slice <400 lines; headless deferred.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/pi/biggz-web-search.js` | New | Tool schemas, dispatch, 3-tier, extract, backoff |
| `internal/install/install.go` | Modified | `DeployPiWebSearch()`, wire `Run()`, `Result` |
| `internal/install/pi_subagents_test.go` | Modified | Assert `web_*` in `sdd-research` allowlist |
| `internal/assets/opencode/sdd-overlay-multi.json` | Modified | Expose `web_*` to `sdd-research` |
| `internal/assets/skills/sdd-research/SKILL.md` | Modified | Gating docs (`open-web`+keys+flag) |
| `internal/doctor/pi.go`+`cmd/biggz/cli_doctor_help.go` | Modified | `PiWebSearchCheck` + runner |
| `go.mod`/`go.sum` | Modified | `tls-client`/`utls` + readability |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `pi.registerTool` absent | Medium | Spike; fallback MCP `--tools=web` |
| `utls` Chrome drift → 403 | Medium | Pin `chrome124`/`safari17`, Tier 1 fallback |
| Missing keys / cost | High | Gate on keys; `blocked`+`Gaps`; `doctor --fix` hint |
| SSRF / key leak | Medium | `sdd-research`-only; block `localhost`/`169.254`/private/`file:`; env-only keys; 10s/1MB cap |
| Hybrid divergence | Low | Verify identical bytes+revision |

## Rollback Plan

Delete `~/.pi/agent/extensions/biggz-web-search.js` (install self-heals), revert commit, `go mod tidy` drops deps. No migration — open-web returns to `blocked`/`partial`.

## Dependencies

- `TAVILY_API_KEY`/`BRAVE_API_KEY` env (billable); DDG marked `publisher: DuckDuckGo (no-key fallback)`
- `tls-client` or `utls`+`fhttp`; `go-readability`/`html-to-markdown`
- Pi `0.56.0` + `pi-subagents`

## Success Criteria

- [ ] `sdd-research` open-web: `web_search`→`web_fetch`→Markdown `excerpt` with `publisher/URL/accessed_at` in BigMem+file
- [ ] Order `Tavily→Brave→DuckDuckGo` logged; missing keys → explicit `blocked`/`partial`
- [ ] 403 T1→T2 then `FetchBlocked`; backoff respects `Retry-After`; T3 needs `BIGGZ_WEB_FETCH_HEADLESS`
- [ ] Non-research blocked from `web_*`; SSRF guards pass
- [ ] `biggz doctor` shows check; `install --agent pi` atomic; keys never logged
