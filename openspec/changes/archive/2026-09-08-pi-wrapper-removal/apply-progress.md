# Apply Progress: pi-wrapper-removal — Unit 1 + Unit 2 (MERGED)

**Mode**: Standard (strict_tdd false from sdd-init; no TDD module loaded)
**Delivery**: auto-chain, stacked-to-main
**Ledger Unit 1**: token tok-0b52f01383e64666939c7fb9, revision ccf1fca83784c9e2e386533b068182e4056ce50a82ab414e2e98bbfe5bbb6f29, request-id req-apply-u1-20260908
**Ledger Unit 2**: token tok-446d39a37286cb3ab8753517, revision 71da99fb71546a160680abd4000660aa765772d19df38cf3b92abfd6e719a10e, request-id req-apply-u2-20260908
**Scope Unit 2**: Phase 2 (2.2–2.3) + Phase 3 (3.1–3.3) + Phase 4 (4.1–4.2). Unit 1 section preserved below (merge, no overwrite).

## Completed Tasks — Unit 1 (PR1: code + mirrors, preserved)

- [x] 1.1 `pi_extensions.go`: dropped `biggz-memory-chrome.js` + `biggz-synthesis-gate.js` from `piExtensionsDeployList()` (now 10 JS + 3 TS); rewrote PR2 comment (fallback retired, native-only `/mcp` via pi-mcp-adapter@^2, sources kept on disk pending Phase-2 gate)
- [x] 1.2 `Apply()`: added non-DryRun `os.Remove` self-heal loop for both stale wrapper targets (errors ignored, absent = silent no-op)
- [x] 1.3 `pi_extensions_guard_test.go`: dropped 2 mirror entries, `12` → `10` in both count guards and related comments
- [x] 2.1 `biggz-pi-extensions-factory.test.mjs`: dropped 2 `DEPLOY_LIST` entries, `12` → `10`, header `12 JS` → `10 JS`

## Completed Tasks — Unit 2 (PR2: delete + spec retire + docs + evidence)

- [x] 2.2 Deleted `internal/assets/pi/biggz-synthesis-gate.test.mjs` (1231 lines; retires with wrapper; no live caller — no CI/workflow references it)
- [x] 2.3 `openspec/specs/pi-integration/spec.md`: removed `Adapter-Aware Wrapper Fallback` requirement + 2 scenarios; CI/test clauses now point at Go canonical suites + factory mirror
- [x] 3.1 `biggz doctor` → `pi-mcp-adapter` PASS (`pi-mcp-adapter@^2`, `mcpServers.bigmem` reachable, `tools/list` has `biggz_mem_*`, `/mcp` live); Summary 0 CRITICAL, 0 WARNING, 19 INFO
- [x] 3.2 `go vet ./internal/install/steps/` clean; `go test ./internal/install/...` PASS (both pkgs); `go test ./internal/sdd/` PASS; `node --test biggz-pi-extensions-factory.test.mjs` 11/11; `node --check` on 4 remaining JS extensions OK
- [x] 3.3 Upgrade self-heal proven in Unit 1 (tmp-home `Apply()` with planted stale copies → both absent, session-guard deployed); self-heal code unchanged since, still valid
- [x] 4.1 `docs/architecture.md` (§assets + §227 Layer intro + §238 Layer 2 + §250 Layer-3 JS + §253 CI), `docs/validation-guide.md` (gate commands → factory suite), `CHANGELOG.md` (Phase 1 entry, Go canonical unchanged)
- [x] 4.2 Untouched confirmed: working-tree diff touches none of `internal/sdd/synthesis_gate.go`, `biggz-session-guard.js`, `biggz-extension-api.js`, `biggz-tool-interception.js`, `adapter.go`

## Files Changed — Unit 2 (working tree vs HEAD)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-synthesis-gate.test.mjs` | Deleted | 1231-line wrapper suite retired |
| `openspec/specs/pi-integration/spec.md` | Modified | Fallback retired, CI clauses repointed |
| `docs/architecture.md` | Modified | 5 spots: wrapper not-deployed, source-only mirror, retired coverage, factory CI |
| `docs/validation-guide.md` | Modified | Gate commands → factory suite |
| `CHANGELOG.md` | Modified | +1 Phase 1 entry |
| `internal/install/steps/pi_extensions.go` | Modified (Unit 1, in tree) | Exclusion + comment + self-heal; line-167 stale comment refreshed to Go canonical |
| `internal/install/steps/pi_extensions_guard_test.go` | Modified (Unit 1, in tree) | Mirror 12→10 |
| `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | Modified (Unit 1, in tree) | Mirror 12→10 |
| `openspec/changes/pi-wrapper-removal/tasks.md` | Modified | All 11 boxes checked |
| `openspec/changes/pi-wrapper-removal/apply-progress.md` | Modified | This merged artifact |

## Work Unit Evidence — Unit 2

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/install/steps/ -run TestPiExtensions -count=1` → ok PASS, exit 0; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → 11 pass, 0 fail |
| Broader sweep | `go test ./internal/install/... -count=1` → both pkgs ok; `go test ./internal/sdd/ -count=1` → ok |
| Vet + syntax | `go vet ./internal/install/steps/` → clean; `node --check` on thinking-wrap, tool-interception, extension-api, session-guard → all OK |
| Runtime harness command/scenario and exact result | `go run ./cmd/biggz doctor` → `[ok] pi-mcp-adapter: pi-mcp-adapter@^2 and biggz-mcp healthy: mcpServers.bigmem reachable, tools/list has biggz_mem_*, /mcp live`; 19 `[ok]`, Summary 0 CRITICAL, 0 WARNING, 19 INFO |
| Self-heal (from Unit 1, code unchanged) | Tmp-home `Apply()` with planted stale copies → both absent after Apply, install succeeds |
| Stale-ref check | `rg synthesis-gate .github/` → no hits (CI never referenced deleted test); deleted file confirmed absent; line-167 comment refreshed to Go canonical |
| Rollback boundary | Restore deleted test file + revert docs/spec/CHANGELOG; Unit 1 files revert separately (`pi_extensions.go` + guard + factory test) |

## Work Unit Evidence — Unit 1 (preserved)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/install/steps/ -run 'TestPiExtensions' -v` → PASS (5/5), exit 0 |
| Focused test command and exact result (JS) | `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → 11 pass, 0 fail |
| Vet + syntax | `go vet ./internal/install/steps/` → clean; `node --check` on factory test + 4 remaining JS extensions → all OK |
| Runtime harness command/scenario and exact result | Tmp-home `Apply()` with planted stale copies → PASS: both absent, `biggz-session-guard.js` deployed |
| Rollback boundary | Revert the 3 modified files; `biggz install --agent pi` redeploys wrappers |

## Deviations from Design

None — implementation matches design.md.

## Issues Found

- OUT OF SCOPE (not touched, flag for verify/follow-up): `internal/install/install.go` (~L1486–1534) contains standalone `DeployPiSynthesisGate` / `DeployPiMemoryChrome` helpers that read the wrapper sources from embedded FS and deploy them directly, independent of `piExtensionsDeployList()`. If either helper is still called on the install path, stale wrappers could be redeployed despite the deploy-list exclusion + self-heal. Verify should confirm they are dead code or file a follow-up change; this unit's allowed surfaces did not include `install.go`.

## Modern Go

Skill `use-modern-go`: Unit 1 ran `list --file-path internal/install/steps/pi_extensions.go`, no applicable guideline. Unit 2 edited no Go files.

## Status

11/11 tasks complete. Ready for verify (full).
