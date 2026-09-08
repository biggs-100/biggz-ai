# Apply Progress: pi-footprint-slim — Port Deployed 429 Relief into Sources

## Status

- Mode: Standard (strict_tdd: false)
- Delivery: single-pr stacked-to-main (Medium risk, ~220–300 lines, no chained PRs)
- Progress: 8/8 tasks complete (Phase 1 1.1–1.2 + Phase 2 2.1–2.3 + Phase 3 3.1–3.3)
- Change: pi-footprint-slim
- Slice: single PR (base main)
- Budget: ~200 changed lines in tracked files, within 400-line budget
- Previous progress: none (fresh apply, no merge needed)

## Completed Tasks

- [x] 1.1 Trim `piDirectTools` in `internal/agents/pi/adapter.go` from 20 to the 10-tool allowlist (`save, search, get_observation, context, session_summary, save_prompt, update, timeline, review, judge` with `biggz_mem_` prefix, deployed order); verify `go build ./internal/agents/pi/...`
- [x] 1.2 Switch `mergePiDirectTools` to allowlist-prune: new `removedPiDirectTools` set drops exactly the 10 removed names, preserves foreign entries, keeps atomic write; verify `go test ./internal/agents/pi/...`
- [x] 2.1 Trim `biggz-orchestrator.md` (5 standalone REMINDER dupes → x1; rule-2 inline REMINDER → Note wording, test needles kept), `biggz-orchestrator-workflow.md` (6 synthesis REMINDER dupes dropped, Session-Recall REMINDER kept), `biggz-orchestrator-delegation.md` (4 end REMINDER dupes dropped, BAD/GOOD example condensed to GOOD-only); markers/template/tokens verbatim; verify `go test ./internal/assets/biggz/...`
- [x] 2.2 Verify `biggz-persona.md`, `bigmem-protocol.md`, `web-tools.md`: zero REMINDER/dupes found → no change per task scope; markers/tokens untouched
- [x] 2.3 Sync delta into `openspec/specs/pi-integration/spec.md`: provisioning requirement → 10-tool allowlist + allowlist-prune + server-stays-20 (5 scenarios); add Slim APPEND_SYSTEM + Reinstall Convergence requirements (4 scenarios)
- [x] 3.1 Update `internal/agents/pi/adapter_test.go`: fresh == exactly 10 allowlist; new `TestMergePiDirectTools_PrunesStale10PreservesForeign` (fat-20 + foreign → 10 + foreign, idempotent); new `TestProvisionBigMemMCP_ReinstallPrunesStale10` (fat mcp.json → 10, `other` preserved); verify `go test ./internal/agents/pi/...`
- [x] 3.2 Run `go test ./internal/agents/pi/... ./internal/install/... ./cmd/biggz-mcp/...` green; `go vet` clean; server profile stays 20 (untouched, asserted by `cmd/biggz-mcp` tests)
- [x] 3.3 Reinstall `go run ./cmd/biggz install --agent pi`, verify live `mcp.json` `directTools` == 10 allowlist, `APPEND_SYSTEM.md` single REMINDER + 8 marker lines + 1 template block, `doctor pi-mcp-adapter` PASS

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/agents/pi/adapter.go` | Modified | `piDirectTools` 20→10 allowlist + `removedPiDirectTools` set + allowlist-prune in `mergePiDirectTools` |
| `internal/agents/pi/adapter_test.go` | Modified | Fresh==10 assertion; 2 new tests (prune+foreign+idempotent, reinstall-prunes-stale-10) |
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | REMINDER dupes → x1 (rule-2 inline → Note wording) |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | 6 synthesis REMINDER dupes dropped, Session-Recall REMINDER kept |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modified | 4 end REMINDER dupes dropped, BAD/GOOD → GOOD-only |
| `openspec/specs/pi-integration/spec.md` | Modified | 10-tool + prune + server-20 provisioning; 2 new requirements |
| `openspec/changes/pi-footprint-slim/tasks.md` | Modified | Marked 8/8 `[x]` |
| `openspec/changes/pi-footprint-slim/apply-progress.md` | Created | This progress artifact |

## Verification

### Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./internal/agents/pi/... ./internal/install/... ./cmd/biggz-mcp/...` — PASS (`ok internal/agents/pi`, `ok internal/install 22.94s`, `ok internal/install/steps 12.78s`, `ok cmd/biggz-mcp 6.49s`); `go test ./internal/assets/biggz/...` — PASS; `go vet` on all four packages — exit 0 |
| Runtime harness command/scenario and exact result | `go run ./cmd/biggz install --agent pi` — success; live `~/.pi/agent/mcp.json directTools` == 10 allowlist in deployed order; `APPEND_SYSTEM.md` REMINDER count == 1, `biggz:` marker lines == 8, template blocks == 1; `doctor pi-mcp-adapter` — `[ok] pi-mcp-adapter` PASS, Summary 0 CRITICAL 0 WARNING |
| Rollback boundary | Revert the 7 tracked files above + reinstall restores 20-tool promotion and full prompt; no migration |

## Deviations from Design

- Asset trim ported as targeted dupe/example cuts, NOT verbatim deployed APPEND_SYSTEM text: the live deploy predates test-pinned source sentences (`orchestrator_test.go` requires exact routing sentences, tool-param exclusion, concise Language Boundary header), so verbatim port would regress sources and fail guards. REMINDER convergence (x1) and marker/template verbatim achieved as specified.
- `bigmem-protocol.md` / `web-tools.md` / `biggz-persona.md` left unchanged: zero REMINDER/dupes per task 2.2 scope; deployed prose condensation deferred to avoid unverified semantic drift (single-REMINDER criterion met via orchestrator trim).

## Issues Found

- None. Pre-existing working-tree dirt in unrelated files (CHANGELOG, docs, pi extensions, pi-deploy-list spec) left untouched; no commits made per instructions.

## Remaining Tasks

None. 8/8 complete. Ready for verify.
