# Exploration: enforce-session-close-summary

Issue: biggz-ai #9 — session-close summary unenforced outside SDD.
Change: `enforce-session-close-summary` · Preflight: interactive / openspec / auto-chain / 400 lines.

## Current state

- The close protocol exists only as system-prompt text (BigMem protocol) and as a Go
  gate in `internal/sdd/session_guard.go` (`HasSessionSummary`,
  `SaveSessionSummaryWithFallback`, `VerifySessionSummary`,
  `IsSessionSummaryBlocked`).
- The gate is only invoked from `internal/sdd/status.go:895` (`done`/`apply` path);
  grep confirms zero callers outside `internal/sdd/`.
- Pi DOES have a blocking close hook, contrary to the initial triage assumption:
  `pi.on("session_stop", ...)` supports `{ block: true, reason }`, verified in
  `internal/assets/pi/biggz-extension-api.js:454`,
  `biggz-tool-interception.js:143`, `biggz-pi-pretty.js:283`.
  The existing pattern blocks on `BIGGZ_PENDING_FINDINGS`/`BIGGZ_PENDING_LENSES`.
- `session_shutdown` / `session_end` (`biggz-last-model.js:299-303`) are best-effort
  flush hooks with NO blocking power — not gates.
- `biggz hooks` (`internal/hooks/manager.go`, `biggz hooks run`) only knows
  `on_review_start/on_review_complete/on_apply_done/on_pr_created/on_install_done`;
  execution is voluntary, so it cannot enforce.
- No dedicated `biggz session-close` command exists. The bash path already works via
  `biggz bigmem save "Session summary" "<content>" --type session_summary --scope
  project --project <proj>` (`cmd/biggz/cli_bigmem.go`, `save` branch), and
  `saveViaBash` (`session_guard.go:230`) already wraps it. MCP exposes
  `mem_session_summary` (`cmd/biggz-mcp/main.go:902,1268`).

## Affected areas

- `internal/assets/pi/biggz-tool-interception.js` — candidate for extending the
  existing `session_stop` guard with `session_summary` verification.
- `internal/assets/pi/biggz-extension-api.js` — duplicated `session_stop` that must
  converge (one guard, not two implementations).
- `cmd/biggz/main.go` — dispatch; registers an eventual `session-close` command.
- `cmd/biggz/cli_bigmem.go` — `save` branch already serves as bash fallback.
- `internal/sdd/session_guard.go` — reusable as-is; note `IsSessionSummaryBlocked`
  currently gates only project `biggz-ai` and accepts `session-fallback.md`.
- `internal/hooks/manager.go` — possible `on_session_close` event, voluntary only.

## Approaches

1. **Extend `session_stop` in the pi extension (verify → fallback → block).**
   Pros: only point with real blocking power outside SDD; proven pattern; no new
   binary; covers TUI. Cons: bypassable without the extension / non-TUI; JS must
   shell out to `biggz` (~100-300ms on close path). Effort: Low.
2. **New `biggz session-close` subcommand (thin wrapper over session_guard.go).**
   `VerifySessionSummaryWithWorkspace` + `SaveSessionSummaryWithFallbackForChange`
   as CLI (`--cwd`, `--json`, `--check-only` / `--save`); exit 0 = present,
   exit 1 + `blocked(session_summary_missing)` = absent; degraded to
   `session-fallback.md` if both fail. Pros: single Go implementation, testable,
   reusable by MCP-less agents, hooks, scripts; auditable `--json`. Cons: needs
   build/distribute; still voluntary where the extension doesn't call it.
   Effort: Medium.
3. **`on_session_close` hooks event + prompt discipline.** Minimal code but ZERO
   blocking power — does not resolve #9. Discard as enforcement (optional audit).

## Recommendation

Combine 1 + 2 in two cuts: first 2 (thin wrapper, no duplicated logic), then 1
(extend `session_stop` in `biggz-tool-interception.js`, unify the duplicate in
`biggz-extension-api.js`). Explicit scope decision needed on the project filter
and fallback-file validity.

## Risks

- No-extension / non-TUI pi modes bypass `session_stop`.
- Slow/corrupt BigMem must degrade to `session-fallback.md`, never trap the agent.
- Triplicated `session_stop` handlers must be unified or blocks diverge.
- Review budget 400 lines; split into chained PRs if needed (`prs=auto-chain`).
