# Tasks: gentle-safety-sealed-explorers

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated lines | 320–380 (PR1 ~230 + PR2 ~120) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | PR1 Safety → PR2 Surfaces (stacked-to-main) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Safety 6/8/5 + 3-surface parity | PR1 | `go test ./internal/policy -run TestIsDenied -count=1` | `GENTLE_PI_AUTONOMOUS_MODE=1` + `go vet ./...` | Revert `guardrails.go`, `biggz-synthesis-gate.js`, `safety.ts`, `gate.go` |
| 2 | Scout fallback + fileCount>=4 | PR2 | `go test ./internal/orchestrator -run TestReject -count=1 && go test ./internal/sdd -run TestShouldEnforce -count=1` | `worker` no surfaces → scout read-only, log fallback | Revert `surfaces.go`, `status.go` |

## Phase 1: Foundation + Threat RED

- [x] 1.1 Map specs to ADRs: `specs/policy/spec.md` 6 req 17 scen + `specs/orchestrator/spec.md` 3 req 9 scen
- [x] 1.2 RED threat git-selection: failing `IsDenied("git -C /r push --force")→true` / `IsDenied("git push")→false` in `internal/policy/guardrails_test.go`
- [x] 1.3 RED threat push-state: failing `ClassifyGuardedCommand("git push --force",{true,{gitPush:allow}})→block` in same file

## Phase 2: PR1 Safety

- [x] 2.1 `internal/policy/guardrails.go` `IsDenied` DENIED[6] — `go test -run TestIsDenied` covers rm -rf roots, reset, clean -fd, push --force, chmod/chown
- [x] 2.2 `internal/policy/guardrails.go` `EvaluateSensitivePathTool` SENSITIVE[8] — `read|write|edit` + 8 regex, blocks `.ssh/.aws/secrets/.env/.pem`
- [x] 2.3 `internal/policy/guardrails.go` `ClassifyGuardedCommand` GUARDED[5] + `Parse` + `LoadRuntimeGuardrailsConfig` — env `1` no I/O, copy-on-merge, malformed→safe
- [x] 2.4 `internal/assets/pi/biggz-synthesis-gate.js` mirror 6/8/5 verbatim — deny→block, guarded per mode, sensitive→block
- [x] 2.5 Create `internal/assets/opencode/plugins/safety.ts` (~120 lines) — `tool_call` hook, 3 checks parity
- [x] 2.6 `internal/review/gate.go` pre-check via `policy` — denied/sensitive→`Allowed=false`, guarded→confirm/block before gates

## Phase 3: PR2 Sealed Explorers

- [x] 3.1 `internal/orchestrator/surfaces.go` `IsTaskScopedRepositoryRelativePath` — `\→/`, reject empty/absolute/~, whitespace, `..`, first-segment `*?[]{}`
- [x] 3.2 `internal/orchestrator/surfaces.go` `readAllowedEditSurfaceEntries`+`HasTaskScopedAllowedEditSurfaces` — heading ci, bullet `` ` `` strip, ≥1, dedup/sort, all headings agree
- [x] 3.3 `internal/orchestrator/surfaces.go` `RejectUnscopedBoundedWriterDispatch` — `worker|gentle-ai-worker` no surfaces → `Block WRITER_EDIT_SURFACE_REJECTION`, relaunch scout read-only, log `scout_fallback`
- [x] 3.4 `internal/sdd/status.go` `ShouldEnforceScopedSurfaces`/`ValidateBoundedWriterSurfaces` — `>=4` enforces, `3→nil 4→Block`, non-writer→nil

## Phase 4: Testing / Verification

- [x] 4.1 `go test ./internal/policy -count=1 -timeout 180s` — 6 DENIED incl. `git -C`, 8 SENSITIVE incl. `.env` variants/array/exec→nil, GUARDED denied>allow/defaults/!auto→confirm
- [x] 4.2 `go test ./internal/orchestrator ./internal/sdd -count=1 -timeout 180s` — isTaskScoped rejects/accepts, heading valid/bad/missing/multi, Validate 3→nil 4→Block
- [x] 4.3 Parity harness — same `git push --force` + `read ~/.ssh/id_rsa` block on 3 surfaces; `git rebase` !auto→confirm each (Node fixture + Go)
- [x] 4.4 Gates — `gofmt -l` clean, `go vet ./...` PASS, `go test ./internal/policy ./internal/orchestrator ./internal/sdd -count=1 -timeout 180s` PASS

## Phase 5: Docs / Cleanup

- [x] 5.1 Rollback `git revert PR2→PR1`, no migration, `sdd-attempt reset` note
- [x] 5.2 Scope guard: no persona/banner/watcher/sync/CodeGraph/lenses/themes untouched
