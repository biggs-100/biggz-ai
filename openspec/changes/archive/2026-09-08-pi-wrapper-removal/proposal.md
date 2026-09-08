# Proposal: pi-wrapper-removal

## Intent

Remove `biggz-memory-chrome.js` + `biggz-synthesis-gate.js` from Pi deploy once native `/mcp` (`pi-mcp-adapter@^2`) proves stable. Wrappers are already no-ops when `pi.getTool("biggz_mem_save")` is truthy; keeping them ships 67KB of dead fallback and a duplicate fail-closed gate.

## Scope

### In Scope
- Phase 1: drop 2 entries from `piExtensionsDeployList()`, self-heal stale `~/.pi/agent/extensions/` copies via `os.Remove`.
- Update `biggz-pi-extensions-factory.test.mjs` (drop entries, fix count guard); delete/repurpose `biggz-synthesis-gate.test.mjs`.
- Retire `Adapter-Aware Wrapper Fallback` requirement in `pi-integration/spec.md`.
- Phase 2 (next release, after soak): delete JS sources — only if stability gate passes.

### Out of Scope
- `internal/sdd/synthesis_gate.go` untouched (canonical Go gate stays).
- `biggz-session-guard.js` factory + `biggz-extension-api.js` + `biggz-tool-interception.js` untouched.
- No changes to `doctor/pi_mcp_adapter.go` or `adapter.go` provisioning logic.
- No wrapper source edits in this phase (apply does that).

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `pi-integration`: retire `Adapter-Aware Wrapper Fallback` (fallback no longer required); keep provisioning/annotations.
- `pi-deploy-list`: deploy list shrinks by 2 entries + stale self-heal rule.

## Approach

Phased removal per exploration Option 2. Phase 1 excludes from deploy list but keeps sources (one-line revert); Phase 2 deletes sources after one-release soak with zero fallback reports. Stability gate (ALL required before Phase 2): `doctor pi-mcp-adapter` PASS; `pi list` shows `biggz_mem_*` and `pi.getTool("biggz_mem_save")` truthy; `settings.json`+`mcp.json` provisioned (command/args/type/imports/directTools); `go vet`+`go test`+`node --check` green.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/install/steps/pi_extensions.go` | Modified | Remove 2 deploy entries, add `os.Remove` self-heal |
| `internal/assets/pi/biggz-memory-chrome.js` | Removed (P2) | Delete after soak |
| `internal/assets/pi/biggz-synthesis-gate.js` | Removed (P2) | Delete after soak |
| `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | Modified | Drop entries, fix guard |
| `openspec/specs/pi-integration/spec.md` | Modified | Retire fallback requirement |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Pi loses in-harness fail-closed enforcement until native replacement | Med | Document gap; Go gate unchanged; soak before P2 |
| Stale deployed copies keep enforcing | Med | `os.Remove` self-heal on upgrade |
| Pretty lines revert to raw tool names | High | Accept; user-visible only |

## Rollback Plan

Single-commit revert restores deploy list + assets; `biggz install --agent pi` re-deploys; `doctor` Remedy `pi install npm:pi-mcp-adapter@^2` re-provisions.

## Dependencies

- `pi-mcp-adapter@^2` published; `biggz-mcp` binary resolvable via `BiggzMCPPath()`.

## Success Criteria

- [ ] Fresh install deploys without the 2 JS files; stale copies self-removed.
- [ ] `doctor pi-mcp-adapter` PASS + `pi.getTool("biggz_mem_save")` truthy.
- [ ] One-release soak with zero fallback-firing reports.
- [ ] `go vet`/`go test`/`node --check` green.
