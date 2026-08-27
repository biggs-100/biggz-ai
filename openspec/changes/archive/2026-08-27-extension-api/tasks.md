# Tasks: extension-api

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 850-1050 |
| 400-line budget risk | High |
| 800-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 Foundation → PR2 Core → PR3 Migration |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | ExtensionAPI + middleware + Fake | PR1 → main | `go test ./internal/extension -count=1 -timeout 60s` | N/A — unit | Delete `api.go`+`testutil/` |
| 2 | Runner + shim + JS | PR2 → main | `go test ./internal/extension -run TestRunner -count=1` | `BIGGZ_TOOL_CONSENT` allow/deny + `PI_SUBAGENT_CHILD=1` | Revert `runner.go`/`shim.go`/`biggz-extension-api.js` |
| 3 | Readability + Deploy | PR3 → main | `go test ./internal/review/lens/... ./internal/install -count=1 -timeout 60s` | `t.TempDir` Deploy + `grep RegisterLens` | Revert `readability/register.go` + `install.go` |

## Phase 1: Foundation — ExtensionAPI + testutil

- [x] 1.1 RED: `internal/extension/api_test.go` — On order, block/revise short-circuit, tool_result no-mutate. Done: `go test` red.
- [x] 1.2 `internal/extension/api.go` — `ExtensionAPI` (On/RegisterLens/RegisterCommand/RegisterTool/RegisterFileWriteFallback/InvokeTool) + middleware chain. Done: 1.1 green, `go vet`.
- [x] 1.3 RED: `internal/extension/testutil/fake_test.go` — Fake records lenses/tools/fallback/On, InvokeTool triggers, t.Setenv. Done: red.
- [x] 1.4 `internal/extension/testutil/fake.go` — FakeExtensionAPI in-mem. Done: 1.3 green, `go test ./internal/extension/testutil`.
- [x] 1.5 `plugintest/agent.go` — compat alias to testutil. Done: `go test ./plugintest` pass.

## Phase 2: Core — Runner + Shim + JS

- [x] 2.1 RED: `internal/extension/runner_test.go` — `TestRunner_BlocksInjectedBash` (rm -rf/mkfs/forkbomb → block), Revise, ConsentDeny/Allow, ToolResultNoMutate, Fallback, SubagentBypass. Done: red.
- [x] 2.2 `internal/extension/runner.go` — `Runner{API, Interceptor}` wraps `pi.on`, delegates to `PolicyInterceptor`, handles `PI_SUBAGENT_CHILD=1` + `BIGGZ_TOOL_CONSENT` v3, fallback. Done: 2.1 green, `go vet`.
- [x] 2.3 RED: `internal/extension/shim_test.go` — delegates RegisterTool to Fake, `// Deprecated: use ExtensionAPI`, `type LensPlugin`=0. Done: red.
- [x] 2.4 `internal/extension/shim.go` — Deprecated shim forwards hooks → RegisterTool. Done: 2.3 green.
- [x] 2.5 `internal/assets/pi/biggz-extension-api.js` — pi.on wiring, PI_SUBAGENT_CHILD guard, block/revise, After no-mutate. Done: js lints.

## Phase 3: Integration — Readability + Deploy

- [x] 3.1 `internal/review/lens/readability/register.go` — `Register(api ExtensionAPI)` wiring; remove init from `lens.go`. Done: Analyze pure, no extension import.
- [x] 3.2 `internal/review/lens/registry.go` — keep Ordered/ResetRegistry. Done: `go test ./internal/review/lens`.
- [x] 3.3 `internal/agents/pi/adapter.go` + `internal/install/install.go` — Deploy via ExtensionAPI + `filemerge.WriteFileAtomic` + JS asset. Done: `go test ./internal/install` + t.TempDir.
- [x] 3.4 Invariants: `rg "type LensPlugin"`=0, `internal/lens/` absent, `RegisterLens` count=1, Analyze snapshot. Done: grep + `go vet`.

## Phase 4: Verification

- [x] 4.1 `go vet ./...` + `go test ./... -count=1 -timeout 180s`. Done: pass, no regressions.
- [x] 4.2 Validate deltas: extension-api, plugin-system Deprecated, review-lenses pure, tool-interception Runner reuses PolicyInterceptor. Done: checklist green.
- [x] 4.3 Evidence: changed-files, tests-added, commands-run, no-staged-files. Done: acceptance-report complete.

Deps: 1.1→1.2→1.3→1.4→1.5→2.1→2.2→2.3→2.4→2.5→3.1→3.2→3.3→3.4→4.1→4.2→4.3. RED before GREEN (1.1>1.2, 2.1>2.2, 2.3>2.4).
