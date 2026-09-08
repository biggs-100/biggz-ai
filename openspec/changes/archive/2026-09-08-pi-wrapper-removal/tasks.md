# Tasks: pi-wrapper-removal

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1300 (mostly 1231-line test deletion; net code ~80) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 code+mirrors → PR 2 delete+docs+evidence |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Deploy-list exclusion + self-heal + mirror tests | PR 1 | `go test ./internal/install/steps/ -run TestPiExtensionsGuard` | Tmp-home `Apply()` with planted stale files; `pi` starts clean | `pi_extensions.go` + guard + factory test |
| 2 | Test delete + spec retire + docs + stability evidence | PR 2 | `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | `biggz doctor`, `pi list`, `node --check` remaining | Restore deleted test + docs/specs |

## Phase 1: Core Removal

- [x] 1.1 Edit `internal/install/steps/pi_extensions.go`: drop 2 entries from `piExtensionsDeployList()`, rewrite PR2 comment
- [x] 1.2 Edit `internal/install/steps/pi_extensions.go` `Apply()`: add `os.Remove` self-heal for both stale targets (ignore errors, non-DryRun only)
- [x] 1.3 Edit `internal/install/steps/pi_extensions_guard_test.go`: drop 2 mirror entries, `12` → `10` in both count guards

## Phase 2: Test + Spec Mirror

- [x] 2.1 Edit `internal/assets/pi/biggz-pi-extensions-factory.test.mjs`: drop 2 `DEPLOY_LIST` entries, `12` → `10`, header `12 JS` → `10 JS` (done in Unit 1 as PR1 mirror)
- [x] 2.2 Delete `internal/assets/pi/biggz-synthesis-gate.test.mjs` (retires with wrapper; repurpose only if live caller found)
- [x] 2.3 Edit `openspec/specs/pi-integration/spec.md`: retire `Adapter-Aware Wrapper Fallback`, point CI/test clauses at remaining suites

## Phase 3: Stability Evidence

- [x] 3.1 Run `biggz doctor` → `pi-mcp-adapter` PASS; verify `pi list` shows `biggz_mem_*`, `pi.getTool("biggz_mem_save")` truthy
- [x] 3.2 Run `go vet ./...`, `go test ./internal/install/...`, `node --check` on remaining extensions — all green
- [x] 3.3 Verify upgrade self-heal: plant stale copies in tmp-home, run `Apply()`, assert both absent and install succeeds

## Phase 4: Docs + Cleanup

- [x] 4.1 Edit `docs/architecture.md` (§27, §230, §238, §250–253), `docs/validation-guide.md` (drop gate commands), `CHANGELOG.md` (Phase 1 entry)
- [x] 4.2 Confirm untouched: `internal/sdd/synthesis_gate.go`, `biggz-session-guard.js`, `biggz-extension-api.js`, `biggz-tool-interception.js`, `adapter.go`
