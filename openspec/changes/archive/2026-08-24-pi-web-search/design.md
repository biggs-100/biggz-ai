# Design: pi-web-search

## Technical Approach

Hosted-API + TLS (Approach 2). Extension `biggz-web-search.js` exposes `web_search`/`web_fetch` gated to `sdd-research` with `open-web` grant. Search: Tavily→Brave→DDG (no-key). Fetch: T1 `net/http`+headers → T2 `tls-client`/`utls` chrome124/safari17 on 403 → T3 headless only if `BIGGZ_WEB_FETCH_HEADLESS=1`. Extract via `go-readability`+`html-to-markdown`, 10s abort, 1MB truncate+annotate, backoff honoring `Retry-After`, loud `FetchBlocked`. Go deploys atomically, `doctor` checks file+env only. Covers REQ-001..007, REQ-INST-001/002, REQ-DIAG-001/002.

## Architecture Decisions

| Decision | Options | Tradeoffs | Choice |
|----------|---------|-----------|--------|
| Provider order | single / chain / parallel | single fragile; parallel wasteful | **Tavily→Brave→DDG**, log order, DDG=`DuckDuckGo (no-key fallback)` |
| Fetch tiers | T1 / T1+T2 / +headless | T1 403-blocked; headless heavy | **T1→T2 on 403, T3 gated** `BIGGZ_WEB_FETCH_HEADLESS` |
| TLS lib | `tls-client`/`utls` vs `curl_cffi` vs `fhttp` | `curl_cffi`=Python sidecar, non-Go-native; `fhttp` manual hello | **`tls-client`/`utls`** Go-native, pinned `chrome124`/`safari17`; `curl_cffi` rejected (Go-native scope) |
| Extract | `go-readability`+`html-to-markdown` vs `turndown` | `turndown` needs Node | **Go libs** fits 1MB cap |
| Exposure | `pi.registerTool` vs MCP | MCP latency | **Extension primary, MCP fallback** |
| Deploy | direct vs `WriteFileAtomic` | direct partial risk | **`WriteFileAtomic`** idempotent, `TempDir`, `//go:embed all:pi` |

## Component Diagram

```
sdd-research(open-web) ──► biggz-web-search.js (ExtensionAPI)
                               ├─SSRF guard─reject→ SSRF/FetchBlocked
                               ├─web_search: Tavily?→Brave?→DDG?(BIGGZ_DDG_FALLBACK) → log order
                               └─web_fetch: T1(net/http) ─403→ T2(tls-client/utls)
                                              429/5xx→backoff(Retry-After)
                                              fail→ headless? (flag) : FetchBlocked
                                                  → readability→md → 1MB/10s caps → {publisher,URL,accessed_at,excerpt}→BigMem+research.md
```

## Data Flow

`web_search`→ check `open-web`+env → ordered call → `blocked`+`Gaps` if none. `web_fetch`→ SSRF check → tier loop (10s `AbortController`, `Retry-After`) → extract+truncate → `FetchBlocked{status,URL,tiers}` never silent partial. `DeployPiWebSearch`→ `assets/pi/*.js`→`WriteFileAtomic` to `~/.pi/agent/extensions/`.

## Interfaces / Contracts

```ts
// pi/biggz-web-search.js
export default function(pi){
  pi.registerTool("web_search",{parameters:{query:"string",limit:"number?"}, handler: async ({query,limit})=>{results, providerOrder, publisher}})
  pi.registerTool("web_fetch",{parameters:{url:"string"}, handler: async ({url})=>FetchResult})
}
// FetchResult = {markdown,excerpt,publisher,URL,accessed_at} | {error:"FetchBlocked"|"SSRF"|"blocked",status,URL,tiers,Gaps}
// env: TAVILY_API_KEY, BRAVE_API_KEY, BIGGZ_DDG_FALLBACK, BIGGZ_WEB_FETCH_HEADLESS
// if pi.registerTool undefined → fallback MCP biggz-mcp --tools=web
```
```go
// internal/install/pi_web_search.go
func DeployPiWebSearch(ctx context.Context, homeDir string) (filemerge.WriteResult, error)
// internal/doctor/pi_web_search.go: PiWebSearchCheck — file+env only, pass/warn/fail→INFO/WARNING/CRITICAL, notes headless, Remedy: biggz install --agent pi
```
Deps: `tls-client`/`utls`+`fhttp`, `go-readability`, `html-to-markdown`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/assets/pi/biggz-web-search.js` | Create | Schemas+dispatch, SSRF guard, provider chain, 3-tier+backoff, 10s/1MB extract |
| `internal/install/pi_web_search.go` | Create | `DeployPiWebSearch` atomic+idempotent, TempDir, legacy cleanup |
| `internal/install/install.go` | Modify | Wire `DeployPiWebSearch` in `Run()`, extend `Result.PiWebSearch` |
| `internal/assets/opencode/sdd-overlay-multi.json` | Modify | `web_*` in `sdd-research` allowlist only |
| `internal/assets/skills/sdd-research/SKILL.md` | Modify | Gating docs (`open-web`+keys/fallback/headless) |
| `internal/doctor/pi_web_search.go` | Create | `PiWebSearchCheck` file+env, pass/warn/fail→INFO/WARNING/CRITICAL |
| `cmd/biggz/cli_doctor_help.go` | Modify | Register `PiWebSearchCheck` in `doctorRun()` |
| `internal/install/pi_subagents_test.go` | Modify | Assert `web_*` presence for sdd-research, absence otherwise |
| `go.mod/sum` | Modify | Add `tls-client`/`utls`, readability deps |

## Security Considerations

- **SSRF**: reject `file:`/`data:`/`ftp:`/`gopher:`; block `localhost`,`127.0.0.0/8`,`10/8`,`172.16/12`,`192.168/16`,`169.254/16`,`::1`,`fe80::/10`,`fc00::/7`; DNS re-check; no fetch.
- **Secrets**: env-only (`TAVILY_API_KEY`,`BRAVE_API_KEY`); logs provider names only.
- **Caps**: 10s `AbortController`→`FetchBlocked`; 1MB truncate+annotate; `Retry-After` honored, else exponential.
- **Gating**: only `sdd-research` with `open-web`+(key or `BIGGZ_DDG_FALLBACK=1`); others→`blocked`+`Gaps`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit | SSRF reject, no key leak, 10s abort, 1MB truncate, Retry-After, provider order, gating | Table tests, inject `statFn`/`getenv`, mock `fetch` |
| Integration | `DeployPiWebSearch` atomic/idempotent, `TempDir`, `//go:embed`, overlay, `PiWebSearchCheck` states | `t.TempDir`, `WriteFileAtomic`, `Runner.RunAll` |
| E2E (manual) | live Tavily→no DDG, `BIGGZ_DDG_FALLBACK` DDG, 403→T2, `FetchBlocked` | env-gated |

## Threat Matrix

Per `internal/assets/skills/sdd-design/references/threat-matrix.md`:

| Boundary | Cases | Applicability | Response | RED tests |
|----------|-------|---------------|----------|-----------|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, MDX exec | N/A — no exec classification; Markdown rendered only | — | — |
| Git repository selection | `git -C`, relative/absolute | N/A — no git cwd | — | — |
| Commit state | staged, `commit -a`, empty | N/A — no VCS commit | — | — |
| Push state | tracking, first push, refspec | N/A — no push | — | — |
| PR commands | `--head`, env prefix, composed | N/A — no `gh pr` | — | — |

No routing/shell/PR boundary changed; security handled via Security Considerations above.

## Migration / Rollout

No migration. Additive install; self-heals legacy stale extension. Rollback: delete `~/.pi/agent/extensions/biggz-web-search.js`, `go mod tidy`; `sdd-research open-web` reverts to `blocked`/`partial`.

## Open Questions

- [ ] **Spike `pi.registerTool`**: confirm `ExtensionAPI.registerTool` exists (mirrors `pi.registerCommand`); if absent fallback MCP `biggz-mcp --tools=web`.
- [ ] **Headless**: `BIGGZ_WEB_FETCH_HEADLESS=1` enables T3; bundle deferred unless `FetchBlocked`>10%, no chromium this slice.
- [ ] **DDG publisher** `DuckDuckGo (no-key fallback)` locked per REQ-001.
