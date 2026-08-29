# Proposal: 2026-08-29-ola3-gentle-final-hardening — Gentle Final Hardening Ola 3

## Intent

Final gaps, no 313K, no banner. Ola1 439 lines PASS, Ola2 19 tasks PASS. Ola3: RO view, model TUI, doctor RO. Auto-chain 400.

## Scope

### In Scope
- **C1 RO+manifest**: `internal/review/candidate_view.go` `0444`/`0555`, `digestChangedPathManifest` SHA256, `GIT_LITERAL_PATHSPECS=1`, `--raw -z` rename/modeOnly/typeChanged, `../../etc/passwd` block, `GOOS=windows` skip.
- **C2 Model TUI**: `internal/tui/models.go` modal agents→user>builtin, `~/.biggz/models.json` v1 `{"sdd-design":{"model":"claude-sonnet-4","thinking":"high"}}`, per-agent `model`+`thinking(off/low/medium/high/inherit)`, envelope `gentle-pi.agent_model_routing v1`, picker 30 files.
- **C3 Doctor RO**: `biggz doctor` RO `sddGlobalAssetDriftCount`+`sddLocalAgentOverrideCount` vs `managed-assets.json` SHA256, `warn: Global SDD asset drift N`; no `--fix`.

### Out of Scope
- `startup-banner.ts` `pink/cyan/yellow/green` + `/gentle:banner` — REJECTED.
- Authority `reclaim/reconcile/quarantine` bindings.
- Watcher 20 roots, `GOSUMDB=off`.
- Background/style-guide/provider-contract/guardrails/preflight/synthesis/assets — Ola1/2.

## Capabilities

### New Capabilities
- `candidate-view-ro`: RO `0444/0555` + SHA256 + symlink guard
- `model-routing-tui`: per-agent `model+thinking` v1 + picker

### Modified Capabilities
- `system-diagnostics`: drift RO — delta `openspec/specs/system-diagnostics/spec.md`
- `managed-assets`: `managedAssetHash` — delta `openspec/specs/managed-assets/spec.md`
- `tui`: picker — delta `openspec/specs/tui/spec.md`

## Approach

- **C1** `review-candidate-view.ts:579-588` `chmod 0444/0555`, `464-477` `digest` `sha256:hex(JSON)`, `426-450` `--raw -z` rename/modeOnly/typeChanged, `361-366` `isWithin` block, `827` `GIT_LITERAL_PATHSPECS=1`, `GOOS=="windows"` skip.
- **C2** `gentle-ai.ts:1236` `→~/.biggz/models.json`, `1243-1244` `MODEL_EXPORT_KIND/VERSION`, `1334-1346` envelope, `1377-1381` frontmatter, `2124-2154` `setThinking(high/low/medium/off/inherit)`, Bubbles 30 files.
- **C3** `gentle-ai.ts:156-220` drift counts, `5477-5510` `gentle:doctor` `pass/warn` RO, `diagnostics/doctor.go` `Runner` isolated, `assets/managed.go:ManagedAssetHash` v1 skip.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/review/candidate_view.go` | New | RO+manifest+symlink |
| `internal/tui/models.go` | New | Modal+picker |
| `internal/opencode/models.go` | Modified | `~/.biggz/models.json` v1 |
| `internal/diagnostics/doctor.go` | Modified | Drift RO |
| `internal/assets/managed.go` | Modified | Hash expose |
| `openspec/specs/system-diagnostics/spec.md` | Modified | Delta drift |
| `openspec/specs/tui/spec.md` | Modified | Delta picker |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `chmod` Windows | Medium | `GOOS!="windows"` |
| Symlink false-pos | Low | `resolve+isWithin` tests |
| Routing builtin | Medium | `user>builtin`, `inherit` |
| Drift noise | Low | `warn` not `fail` |

## Rollback Plan

`git revert` per C1→C3 (`C1→main, C2→C1, C3→C2`). No migration. Partial safe. No loss, `<400`/slice.

## Dependencies

- Refs: `lib/review-candidate-view.ts:361-366,426-477,579-588,827`, `extensions/gentle-ai.ts:156-220,1236-1346,1377-1381,2124-2154,5477-5510`, `lib/native-review-cli.ts:21`
- Stack: Go 1.25, `go test ./... -count=1 -timeout 180s`, `go vet`, `gofmt -l`, 400 auto-chain
- Needs Ola1 `provider-contract.lock.json` + Ola2 `managed-assets.json v1`

## Success Criteria

- [ ] C1 `0444`/`0555` `stat`; `digest`→`sha256:<hex>`; `GIT_LITERAL_PATHSPECS=1`; rename/modeOnly/typeChanged; `../../etc/passwd` blocked; `GOOS=windows` skip
- [ ] C2 modal `agents→user>builtin`; `~/.biggz/models.json` v1 `model+thinking(high/low/medium/off/inherit)`; `gentle-pi.agent_model_routing` v1 round-trip; picker 30 files 4 modes
- [ ] C3 `biggz doctor` RO drift vs SHA256, no `--fix`; `warn` if `>0`
- [ ] `go vet`+`go test` green; no regression; `<400` stacked
- [ ] Banner/authority/watcher absent

## Proposal question round

*Assumptions skip/correct/second-round:* `GOOS` skip? `tui/models.go` vs `opencode/models.go`? `inherit`? `warn` not `fail`? Banner excluded?
