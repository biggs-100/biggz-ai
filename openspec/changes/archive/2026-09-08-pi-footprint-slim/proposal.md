# Proposal: pi-footprint-slim

## Intent

Reinstall regenerates the fat Pi footprint (20 promoted `directTools` + verbose APPEND_SYSTEM), discarding the proven deployed 429 relief. Port the deployed state into sources so `biggz install --agent pi` converges every install to it.

## Scope

### In Scope
- Trim `piDirectTools` in `internal/agents/pi/adapter.go` from 20 to the proven 10-tool allowlist
- Allowlist-prune merge: drop the 10 removed BigMem tools on reinstall, preserve foreign entries
- Trim APPEND_SYSTEM assets (REMINDER x5→x1, cut verbose examples, zero semantic change)
- Update mirror tests + `pi-integration` spec delta
- Reinstall + verify (`doctor pi-mcp-adapter`, diff `mcp.json` == 10 tools)

### Out of Scope
- Server `ProfileAgent` stays at 20 tools — no `cmd/biggz-mcp` change (promotion-only trim)
- Gate markers, gate template, `{{BIGGZ_BACKGROUND_POLICY}}` and other template tokens untouched
- Archive docs (`openspec/changes/*/archive.md`) untouched even where they pin "20 tools"
- No APPEND_SYSTEM logic changes in `install.go` / `overlay.go` (asset text only)

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `pi-integration`: `directTools` requirement changes from full 20-tool promotion to 10-tool allowlist + allowlist-prune merge semantics; APPEND_SYSTEM generation trims duplicated REMINDER/examples with markers intact

## Approach

Approach 1 (per exploration): replace `piDirectTools` with the 10 (`save, search, get_observation, context, session_summary, save_prompt, update, timeline, review, judge`); make `mergePiDirectTools` authoritative for BigMem entries (prune removed 10, keep foreign); trim asset prose/examples only; update mirror tests; reinstall + verify. Server still exposes all 20 via `--tools=agent`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agents/pi/adapter.go` | Modified | `piDirectTools` 20→10; `mergePiDirectTools` allowlist-prune |
| `internal/agents/pi/adapter_test.go` | Modified | Mirror tests to 10-tool shape |
| `internal/assets/biggz/*.md` | Modified | REMINDER dedupe, example trim; markers/tokens kept |
| `openspec/specs/pi-integration/spec.md` | Modified | Delta spec for new `directTools` requirement |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Dropped 10 referenced by prompts/skills | Med | Grep before cutting; server keeps all 20 available |
| Marker/token damage during trim | Low | Trim prose only; `pi_subagents_test.go` marker assertion guards |
| Prune drops user-added tools | Low | Prune only the 10 removed BigMem names, preserve foreign |

## Rollback Plan

Revert source changes and rerun `biggz install --agent pi` — restores 20-tool promotion and full prompt. No migration; this phase ships no source edits.

## Dependencies

- None

## Success Criteria

- [ ] `~/.pi/agent/mcp.json` `directTools` == proven 10 after reinstall (fresh and existing installs)
- [ ] APPEND_SYSTEM has single REMINDER, all `<!-- biggz:* -->` markers + gate template intact
- [ ] `go test ./internal/agents/pi/... ./internal/install/... ./cmd/biggz-mcp/...` green
- [ ] `biggz doctor pi-mcp-adapter` clean; 429 pressure reduced (halved function surface)
