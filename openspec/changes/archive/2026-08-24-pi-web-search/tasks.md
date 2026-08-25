# Tasks: pi-web-search

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700–800 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 (~250 lines each) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Extension skeleton, SSRF guard, provider fallback, atomic deploy | PR 1 | `go test ./internal/install -run TestDeployPiWebSearch -count=1` | `biggz install --agent pi && ls ~/.pi/agent/extensions/biggz-web-search.js` | `internal/assets/pi/biggz-web-search.js`, `internal/install/pi_web_search.go`, `go.mod` |
| 2 | 3-tier fetch T1→T2 TLS(chrome124/safari17)→T3 gated + backoff + Markdown + caps | PR 2 | `go test ./internal/assets/pi -run TestWebFetch -count=1` | `BIGGZ_DDG_FALLBACK=1` search then `web_fetch` 403→T2, 429→backoff, check `FetchBlocked` | `biggz-web-search.js` fetch tiers + extract |
| 3 | Install/overlay/doctor wiring, gating docs, verification | PR 3 | `go test ./... -count=1` | `biggz doctor --json` shows `pi-web-search`; `doctor --fix` redeploys | `internal/doctor/pi_web_search.go`, `cli_doctor_help.go`, `sdd-overlay-multi.json`, `SKILL.md` |

## Phase 1: Foundation

- [x] 1.1 Create `internal/assets/pi/biggz-web-search.js` with `pi.registerTool("web_search"/"web_fetch")` schemas and SSRF guard (block `file:/data:/ftp:/gopher:`, `localhost/127/10/172.16/192.168/169.254/::1/fe80/fc00`, DNS re-check)
- [x] 1.2 RED test SSRF: `web_fetch("http://192.168.1.10/docs")` → SSRF error no fetch (REQ-006)
- [x] 1.3 Add `go.mod` deps `tls-client`/`utls`+`fhttp` (`chrome124`/`safari17`), `go-readability`/`html-to-markdown`; `go mod tidy`
- [x] 1.4 Create `internal/install/pi_web_search.go`: `DeployPiWebSearch(ctx,homeDir)` via `filemerge.WriteFileAtomic`, dirs, idempotent, TempDir, legacy cleanup

## Phase 2: Core Implementation

- [x] 2.1 Implement `web_search` in `biggz-web-search.js`: order `TAVILY→BRAVE→DDG`, log `providerOrder` without keys, DDG `publisher: DuckDuckGo (no-key fallback)`, `BIGGZ_DDG_FALLBACK` gate, `blocked+Gaps` (REQ-001)
- [x] 2.2 Implement `web_fetch` T1 `net/http`+headers + 10s `AbortController` → `FetchBlocked`; 1MB truncate+annotate (REQ-003)
- [x] 2.3 Implement T2 `tls-client`/`utls` on 403, T3 headless only if `BIGGZ_WEB_FETCH_HEADLESS=1` else `FetchBlocked{status,URL,tiers}`; 429/5xx backoff with `Retry-After`, never partial (REQ-002/004)
- [x] 2.4 Implement HTML→Markdown extract via `go-readability`+`html-to-markdown`, emit `publisher/URL/accessed_at/excerpt` (REQ-003/007)
- [x] 2.5 RED tests: no key leak, 10s abort, 1MB cap, `Retry-After` ≥2s, exhausted→`FetchBlocked` (REQ-003/004/006)

## Phase 3: Integration / Wiring

- [x] 3.1 Modify `internal/install/install.go`: wire `DeployPiWebSearch` in `Run()`, extend `Result.PiWebSearch` (REQ-INST-001) — PR1 slice (autonomous deploy via Run, TempDir)
- [x] 3.2 Modify `internal/assets/opencode/sdd-overlay-multi.json`: expose `web_*` to `sdd-research` only (REQ-INST-002/005)
- [x] 3.3 Modify `internal/assets/skills/sdd-research/SKILL.md`: document `open-web`+key gating and flags (REQ-005)
- [x] 3.4 Create `internal/doctor/pi_web_search.go`: `PiWebSearchCheck` ID `pi-web-search` file+env only, pass/warn/fail→INFO/WARNING/CRITICAL, `Remedy: biggz install --agent pi` (REQ-DIAG-001)
- [x] 3.5 Modify `cmd/biggz/cli_doctor_help.go`: register check in `doctorRun()` with panic isolation, `--json`/`--fix` (REQ-DIAG-002)
- [x] 3.6 Modify `internal/install/pi_subagents_test.go`: assert `web_*` in `sdd-research`, absent in others

## Phase 4: Testing / Verification

- [x] 4.1 Unit tests: SSRF, gating, provider order (Tavily first/DDG fallback/no provider blocked), timeout, truncate, backoff (`statFn`/`getenv` mock)
- [x] 4.2 Integration: `DeployPiWebSearch` atomic/idempotent/`t.TempDir`/`//go:embed all:pi`, overlay, `PiWebSearchCheck` states + panic isolation
- [x] 4.3 Manual E2E: live Tavily, DDG fallback, 403→T2, `FetchBlocked`, `biggz doctor --json/--fix` cycle
