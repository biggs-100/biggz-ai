# Proposal: 2026-08-30-gentle-safety-sealed-explorers — Safety + Sealed Explorers

## Intent

Close 2 ALTO gaps vs gentle-pi (S each). Safety only in `guardrails.go`; pi/opencode/gate lack verbatim deny. Writers without surfaces can over-write — no scout fallback.

## Scope

### In Scope
- **Slice 1 Safety:** verbatim `gentle-ai.ts:280-720` to `biggz-synthesis-gate.js` + `opencode/plugins/safety.ts` (new) + `review/gate.go`. Hardcode literal; override via `runtime-guardrails.json` + `autonomousMode`.
  - `DENIED[6]`→block: `rm -rf /|~|$HOME|..`, `git reset --hard`, `git clean -fd`, `git push --force` (incl. `git -C`), `chmod -R 777`, `chown -R`.
  - `SENSITIVE[8]` on `read/write/edit`: `.ssh`, `.credentials`, `keychains`, `.aws/credentials`, `gh/hosts.yaml`, `secrets/`, `.env`, `.pem/.key/*`.
  - `GUARDED[5]` (`gitPush`, `rebase`, `branchDeleteForce`, `npmPublish`, `piRemove`): denied→block, `!auto→confirm`, `auto→defaults`/override.
- **Slice 2 Sealed explorers:** `isTaskScoped...`, `readAllowed...`, `hasTask...`, `reject...` (420-520,580-720). Writer without surfaces → scout read-only, no human block.

### Out of Scope
- Persona rioplatense, banner, Herdr watcher, sync, CodeGraph, lenses, themes, BigMem, `watcher 20 roots`.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `policy`: verbatim safety + `runtime-guardrails.json` + autonomousMode (3 surfaces).
- `orchestrator`: sealed-explorer + scout fallback.

## Approach

Verbatim copy, 2 PRs stacked-to-main (`auto-chain`, 400 budget, `openspec`):

- **PR1 Safety ~250:** share `guardrails.go`; new `safety.ts`; `gate.go` pre-check `IsDenied`/`ClassifyGuardedCommand`/`EvaluateSensitivePathTool`. Reuse global→project merge, `GENTLE_PI_AUTONOMOUS_MODE=1` fast-path, malformed→safe.
- **PR2 Surfaces <150:** reuse `surfaces.go` + `sdd/status.go:ShouldEnforceScopedSurfaces (>=4)`; `reject→block` relaunches scout.

## Affected Areas

| Area | Impact | Desc |
|------|--------|------|
| `internal/policy/guardrails.go` | Modified | Expose to 3 callers |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Deny/confirm |
| `internal/assets/opencode/plugins/safety.ts` | New | Opencode plugin |
| `internal/review/gate.go` | Modified | Pre-check |
| `internal/orchestrator/surfaces.go` | Modified | Scout fallback |
| `internal/sdd/status.go` | Modified | Guard |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `rm -rf ./scoped` over-block | Low | Roots only |
| Config merge mutates global | Low | Copy-on-merge |
| Scout silent | Low | Log; test no write |

## Rollback Plan

`git revert` PR2 then PR1. No migration. Deletes `safety.ts`, reverts `gate.go`/`biggz-synthesis-gate.js`/`surfaces.go`. `sdd-attempt reset` if needed.

## Dependencies

- Oracle `gentle-ai.ts:280-720` verbatim; existing `guardrails.go`, `surfaces.go`, `status.go`; `Go 1.25`, `go test -count=1 -timeout 180s`, `go vet`.

## Success Criteria

- [ ] `IsDenied` blocks 6; `git clean` both flags; `push` needs force.
- [ ] `ClassifyGuardedCommand` denied→block, `!auto→confirm`, auto defaults + override.
- [ ] `LoadRuntimeGuardrailsConfig` merge + env fast-path + malformed→safe; `EvaluateSensitivePathTool` blocks 8.
- [ ] Same 3 checks via `biggz-synthesis-gate.js`, `safety.ts`, `gate.go`.
- [ ] Writer without surfaces → scout read-only; valid passes.
- [ ] `go vet` PASS, `go test ./internal/policy ./internal/orchestrator ./internal/sdd` PASS, <400/PR.

## Alternatives Considered

- Single PR: rejected — two S slices revertible, each <400.
- Central service: rejected — runtimes disjoint; `internal/policy` is source.
- Block human: rejected — gentle fallback non-blocking.
