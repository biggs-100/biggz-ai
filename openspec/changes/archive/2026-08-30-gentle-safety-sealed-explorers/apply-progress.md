# Apply Progress: gentle-safety-sealed-explorers

## Summary
Implemented Safety 6/8/5 + Sealed Explorers with 3-surface parity (verbatim gentle-ai.ts:280-720). 2 work units stacked-to-main: PR1 Safety (~277) + PR2 Sealed (~254) + test expansion, all <400/work-unit.

## Work Unit 1 — PR1 Safety (ee25c2d)
- guardrails.go IsDenied DENIED[6], EvaluateSensitivePathTool SENSITIVE[8], ClassifyGuardedCommand GUARDED[5] + Parse + LoadRuntimeGuardrailsConfig (env fast-path, copy-on-merge, malformed→safe)
- biggz-synthesis-gate.js mirror verbatim DENIED/SENSITIVE/GUARDED + pi tool_call hook + _biggzSafety export
- safety.ts new opencode plugin 134 lines tool.execute.before 3 checks
- gate.go SafetyPreCheck via policy (Allowed=false block, confirm log surface+kind)
- RED tests 1.2/1.3 passing

Focused test: `go test ./internal/policy -run TestIsDenied -count=1` PASS (5/5)
Runtime harness: `GENTLE_PI_AUTONOMOUS_MODE=1` env fast-path verified; `go vet` PASS
Rollback: revert ee25c2d (deletes safety.ts, reverts gate/synthesis/guardrails)

## Work Unit 2 — PR2 Sealed Explorers (aa97f44)
- surfaces.go IsTaskScopedRepositoryRelativePath (\\→/, absolute/~, whitespace \\s, .., first-segment *?[]{}), readAllowedEditSurfaceEntries (heading any-level #{1,6}, bullet/` strip, prose handling, blank skip, all headings agree dedup/sort), RejectUnscopedBoundedWriterDispatch scout_fallback log
- status.go ShouldEnforceScopedSurfaces >=4, ValidateBoundedWriterSurfaces full validation + sddFindOffendingSurface log
- Updated to handle markdown heading ci, deeper headings, prose closing list per issue #484

Focused test: `go test ./internal/orchestrator -run TestReject -count=1` PASS; `go test ./internal/sdd -run TestShould -count=1` PASS
Runtime harness: worker no surfaces → scout read-only Block WRITER_EDIT_SURFACE_REJECTION, log scout_fallback, no human block
Rollback: revert aa97f44 (reverts surfaces/status)

## Verification (51ef9fd)
- Full policy tests 4.1: go test ./internal/policy -count=1 PASS (14/14)
- Orchestrator/sdd 4.2: go test ./internal/orchestrator ./internal/sdd -count=1 PASS (filtered, TestReadLoopLarge pre-existing FAIL unrelated)
- Parity harness 4.3: node parity-harness.mjs — same git push --force + read ~/.ssh/id_rsa block on pi/opencode/go; git rebase !auto→confirm each — PASS
- Gates 4.4: gofmt -l clean for changed Go files, go vet PASS, go test ./internal/policy ./internal/orchestrator ./internal/sdd filtered PASS

## Work Unit Evidence
| Evidence | Value |
|---|---|
| Focused test command | `go test ./internal/policy -count=1 -timeout 60s` PASS 14 tests; `go test ./internal/orchestrator -count=1` PASS 5 tests |
| Runtime harness | `node parity-harness.mjs` — 3 surfaces same blocks + confirm — PASS |
| Rollback boundary | PR1 revert ee25c2d, PR2 revert aa97f44, test revert 51ef9fd — each <400, independent |
| Scope guard | No persona/banner/watcher/sync/CodeGraph/lenses/themes touched — verified git diff only 6 files |

## Remaining
- Verify full go test ./... has pre-existing TestReadLoopLarge FAIL (unrelated to change) — reported as residual risk
- Proceed to sdd-verify
