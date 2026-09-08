# Design: pi-footprint-slim

## Technical Approach

Port the proven deployed slim state into sources so `biggz install --agent pi` converges every install: shrink `piDirectTools` to the 10-tool allowlist, switch `mergePiDirectTools` from additive-union to allowlist-prune, trim APPEND_SYSTEM asset prose with zero semantic change. Maps to proposal Approach 1 and the `pi-integration` delta (1 modified + 2 added requirements, 9 scenarios). Server `ProfileAgent` stays at 20 — promotion-only trim.

## Architecture Decisions

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Allowlist-prune merge vs union merge | Union never removes stale tools; prune risks dropping user tools | **Prune only the 10 removed `biggz_mem_*` names, preserve all foreign entries.** Converges existing installs; no user-tool loss. |
| Promotion-only trim vs server trim | Server trim shrinks `--tools=agent` surface but breaks flows calling dropped tools | **Promotion-only.** `cmd/biggz-mcp` untouched; dropped 10 stay callable server-side, just not top-level promoted. |
| Asset prose-trim vs logic change | Logic change in `install.go`/`overlay.go` risks dual-path drift | **Text-only trim.** No `DeployPersona`/`DeployBigMemProtocol` logic change; markers, gate template, `{{BIGGZ_BACKGROUND_POLICY}}` verbatim. |

## Data Flow

```
install --agent pi ──→ ProvisionBigMemMCP ──→ mergePiDirectTools ──→ mcp.json/settings.json
        │                      │                        │ (allowlist-prune, atomic write)
        │                      └──→ DeployPersona/Protocol ──→ APPEND_SYSTEM.md (slim assets)
        └──→ doctor pi-mcp-adapter (verify: 10 tools, 1 REMINDER)
```

Non-obvious pattern — allowlist-prune (only removed-10 filtered, foreign kept):

```go
removed := map[string]bool{ /* 10 dropped biggz_mem_* names */ true }
for _, e := range existing { if !removed[e] { add(e) } }
for _, want := range piDirectTools { add(want) } // exactly 10
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/agents/pi/adapter.go` | Modify | `piDirectTools` 20→10 (`save, search, get_observation, context, session_summary, save_prompt, update, timeline, review, judge` with `biggz_mem_` prefix); `mergePiDirectTools` allowlist-prune + existing atomic write |
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | REMINDER x6→x1, cut verbose examples; markers/template/tokens verbatim |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | REMINDER x7→x1, cut verbose examples; markers verbatim |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modify | REMINDER x5→x1, cut verbose examples; markers verbatim |
| `internal/assets/biggz/biggz-persona.md`, `bigmem-protocol.md`, `web-tools.md` | Modify | Prose/example trim only if REMINDER/dupes found; markers/tokens verbatim |
| `internal/agents/pi/adapter_test.go` | Modify | Mirror tests: fresh==10, reinstall prunes stale-10, foreign preserved, idempotent |
| `openspec/specs/pi-integration/spec.md` | Modify | Sync delta spec (already drafted in change `specs/`) |
| `cmd/biggz-mcp/*`, gate JS/Go, archives, `internal/install/pi_subagents_test.go`, `cmd/biggz-mcp/main_test.go` | Untouched | Server stays 20; marker test guards trim; archives keep "20 tools" prose |

## Interfaces / Contracts

No new interfaces. Contract change: `directTools` post-merge MUST equal exactly the 10 allowlist plus any foreign entries; the 10 removed names MUST never survive merge. `APPEND_SYSTEM.md` MUST contain exactly one REMINDER, all `<!-- biggz:* -->` markers, gate template, `{{BIGGZ_BACKGROUND_POLICY}}`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Fresh provision ==10; reinstall prunes stale-10; foreign preserved; idempotent | Extend `TestProvisionBigMemMCP_*` in `adapter_test.go` |
| Integration | Marker test still green; server profile still 20 | `pi_subagents_test.go` (untouched), `main_test.go` server-20 (untouched) |
| E2E | Reinstall converges; doctor clean | `biggz install --agent pi`, diff `mcp.json`==10, `doctor pi-mcp-adapter` |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

No migration required. Fresh and existing installs converge on reinstall; rollback = revert sources + reinstall (restores 20-tool promotion + full prompt). Reinstall+verify procedure ships with tasks: `go test ./internal/agents/pi/... ./internal/install/... ./cmd/biggz-mcp/...`, reinstall, diff `directTools`==10, REMINDER count==1.

## Open Questions

None — allowlist, trim boundaries, and untouched surfaces fixed by exploration/proposal.
