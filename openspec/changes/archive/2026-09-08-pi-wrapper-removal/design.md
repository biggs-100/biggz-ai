# Design: pi-wrapper-removal

## Technical Approach

Phase 1 excludes both wrappers from `piExtensionsDeployList()` while keeping JS sources on disk (one-commit revert). `Apply()` gains scoped `os.Remove` self-heal for the two stale targets after the deploy loop, mirroring the existing `gentle-ai.ts` pattern. Tests, `pi-integration` spec, and docs retire the fallback contract. Phase 2 deletes sources only after the 5-criteria stability gate + one-release soak. Maps to proposal Option 2 and both delta specs (`pi-integration`, `pi-deploy-list`).

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Deploy-list exclusion, keep sources (P1) vs hard delete now | Exclusion is one-line revertible; hard delete has no soak safety | **Exclusion first** — sources deleted in Phase 2 only |
| `os.Remove` self-heal in `Apply()` vs leaving stale files | Stale copies keep enforcing gate/duplicating pills; self-heal follows existing `gentle-ai.ts` precedent | **Self-heal** both stale targets, silent no-op when absent |
| Delete vs repurpose `biggz-synthesis-gate.test.mjs` | CI (`.github/workflows/ci.yml`) runs no `node --test`; only `docs/validation-guide.md` cites it | **Delete test + fix doc refs**; repurpose only if apply finds a live caller |

`validatePiExtensionsFactory` stays untouched — it iterates the (shrunk) list, so no logic change is needed. Legacy `DeployPiSynthesisGate`/`DeployPiMemoryChrome` in `internal/install/install.go` have no production callers (tests only); leave for apply to deprecate, do not expand scope.

## Data Flow

Fresh install and upgrade share one path; self-heal runs after deploy so a re-added stale copy can never survive `Apply()`:

```
PiExtensionsStep.Apply()
  ├─ validatePiExtensionsFactory (10 JS, shrunk list)
  ├─ deploy loop: 10 JS + 3 TS → ~/.pi/agent/extensions/
  └─ self-heal (non-DryRun only):
       os.Remove biggz-memory-chrome.js   (ignore error)
       os.Remove biggz-synthesis-gate.js  (ignore error)
       existing: biggz-pi-pretty.js, gentle-ai/quiet-tools/sdd-init/startup-banner.ts

Native path (unchanged): adapter.go ProvisionBigMemMCP → settings.json+mcp.json
  → pi.getTool("biggz_mem_save") truthy → doctor pi-mcp-adapter PASS
```

Non-obvious ordering snippet (follows existing stale-cleanup block):

```go
extDir := piExtensionsDir(p.HomeDir)
for _, stale := range []string{"biggz-memory-chrome.js", "biggz-synthesis-gate.js"} {
    _ = os.Remove(filepath.Join(extDir, stale)) // absent = silent no-op
}
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/install/steps/pi_extensions.go` | Modify | Remove 2 entries from `piExtensionsDeployList()`; extend self-heal block; rewrite PR2 comment (fallback retired) |
| `internal/install/steps/pi_extensions_guard_test.go` | Modify | Drop 2 mirror entries; `12` → `10` in both count guards |
| `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | Modify | Drop 2 `DEPLOY_LIST` entries; `12` → `10`; header comment `12 JS` → `10 JS` |
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Delete | Gate coverage retires with the wrapper; fix refs |
| `openspec/specs/pi-integration/spec.md` | Modify | Retire `Adapter-Aware Wrapper Fallback`; point CI/test clauses at remaining suites |
| `docs/architecture.md` | Modify | Remove wrapper + test refs (§27, §230 Layer 2 JS mirror, §238, §250–253) |
| `docs/validation-guide.md` | Modify | Drop `node --test`/`node --check` synthesis-gate commands |
| `CHANGELOG.md` | Modify | Phase 1 entry (exclusion + self-heal + soak gate) |
| `internal/assets/pi/biggz-memory-chrome.js` | Phase 2 delete | Only after 5-criteria gate + soak; NOT this phase |
| `internal/assets/pi/biggz-synthesis-gate.js` | Phase 2 delete | Same gate; NOT this phase |

Explicitly UNTOUCHED: `internal/sdd/synthesis_gate.go`, `internal/assets/pi/biggz-session-guard.js`, `biggz-extension-api.js`, `biggz-tool-interception.js`, `internal/agents/pi/adapter.go`, `internal/doctor/pi_mcp_adapter.go`.

## Interfaces / Contracts

No new public APIs. Contract changes: `piExtensionsDeployList()` returns 10 JS + 3 TS; `~/.pi/agent/extensions/` MUST NOT contain either wrapper after `Apply()`; missing stale files MUST NOT error. Rollback = single-commit revert + `biggz install --agent pi` redeploys.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit (Go) | Guard test passes at 10 JS; helper-drift check catches list/test skew | `go test ./internal/install/steps/ -run TestPiExtensionsGuard` |
| Unit (JS) | Factory test passes on shrunk list, every entry exports `default function(pi)` | `node --test biggz-pi-extensions-factory.test.mjs` |
| Integration | Upgrade removes stale copies; fresh install omits wrappers; pi starts (no factory crash) | Tmp-home `Apply()` with planted stale files; `go test ./internal/install/` |
| Gate | Native stability evidence before Phase 2 | `doctor pi-mcp-adapter` PASS; `pi.getTool("biggz_mem_save")` truthy; `go vet` + `go test` + `node --check` green |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. `os.Remove` targets two fixed filenames inside `piExtensionsDir` only, errors ignored; `references/threat-matrix.md` absent from repo. No RED tests required.

## Migration / Rollout

Phase 1 ships this release (exclusion + self-heal, sources kept). Soak one release with zero fallback-firing reports. Phase 2 (next release) deletes both JS sources iff ALL five gate criteria pass; any failure retains sources. No feature flags; no data migration.

## Open Questions

- [ ] Deprecate legacy `DeployPiSynthesisGate`/`DeployPiMemoryChrome` + `pi_memory_chrome_test.go` in this phase or Phase 2? (No production callers; recommend Phase 2 with source deletion.)
- [ ] None blocking — design is ready for tasks.
