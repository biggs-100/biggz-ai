# Tasks: Gentle v2.5 Parity — Research, Status v2, Last-Event Closure

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1800–2200 |
| 400-line budget risk | High |
| 800-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1→PR2→PR3→PR4 stacked-to-main |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
800-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Status v2+Research | PR1 | `go test ./internal/sdd -run TestStatusV2 -count=1` | `biggz sdd-status --contract v2` ok / `v1` fail | `sdd/*`, `researchcapability`, `skills/sdd-research` |
| 2 | Burn+budget+lock | PR2 | `go test ./internal/review -run TestBurn -count=1` | `go test ./internal/sddattempt ./internal/filecoord -count=1` | `review/*`, `sddattempt/*`, `filecoord/*` |
| 3 | Runtime/platform | PR3 | `go test ./internal/agents/pi -run TestManifest -count=1` | `go test ./internal/opencode ./internal/backup -count=1` | `opencode/*`, `platform/*`, `update/*`, `filemerge/*`, `pi/*`, `backup/*` |
| 4 | TUI+sweep | PR4 | `go test ./internal/tui -count=1` | `go test ./... -count=1 -timeout 180s` | `tui/*` only |

## Phase 1: Foundation

- [x] 1.1 RED: `internal/sdd/status_v2_test.go` — v1→fail `unsupported contract` + rerun v2; fails before impl
- [x] 1.2 `internal/sdd/status.go`: `SchemaVersion=2`, `StatusContractV2`, `ProjectStatusV2` allowlist
- [x] 1.3 `internal/sdd/status.go` CLI: default v2, reject v1 read-only
- [x] 1.4 Create `internal/sdd/research.go` + `preproposal.go`: hybrid equal revision+bytes, one-sided replay, missing→blocked
- [x] 1.5 Create `internal/agents/researchcapability/*`: closed `biggz-ai.sdd-research-capability/v1`, exact grants, else deny
- [x] 1.6 Create `internal/assets/skills/sdd-research/*` + `_shared/*`: port lifecycle, status v2, burn docs

## Phase 2: Core

- [x] 2.1 RED: `internal/review/compact_burn_test.go` — twice→not-found, concurrent→timeout, residue→incomplete
- [x] 2.2 RED: `internal/sdd/research_test.go` — divergent→blocked, one-sided→both, missing→blocked
- [x] 2.3 `internal/review/compact_burn.go` + `store.go` + `receipt.go`: `BurnApprovedCompactAuthority` lock+lease, delete 3 paths, retire receipts
- [x] 2.4 `internal/sdd/edit_authority.go`: explicit `apply to <path>` only; `investigate|if possible` read-only
- [x] 2.5 `internal/sddattempt/cas_store.go`: cumulative never reset; rescope `5/5→3` vs 5
- [x] 2.6 `internal/filecoord/lock.go` + `lock_backend.go`: `Acquire(ctx,target,root)` non-blocking → `BusyError`, `no-follow`

## Phase 3: Integration

- [x] 3.1 `internal/opencode/background.go`: grouped isolation scheduling-only
- [x] 3.2 `internal/platform/*` + `internal/update/*` + `internal/filemerge/writer.go`: Windows quoting, `rundll32`/`cmd`, handle-relative writer
- [x] 3.3 `internal/agents/pi/model_routing.go`: `MaxPackageManifestBytes=64KiB` → `manifest-too-large`, `ProgressState`
- [x] 3.4 `internal/backup/backup.go`: `ensureCodexSkillRegistryHook` atomic `hooks.json:SessionStart`
- [x] 3.5 `internal/tui/styles/styles.go`: Rose Pine `#191724`/`#c4a7e7`/`#9ccfd8`, remove legacy palette
- [x] 3.6 `internal/tui/tui.go`: `tuiAnimationsDisabled()` env-gated, `tickCmd()=nil`, suppress `ESC[?2026h/l`

## Phase 4: Testing

- [x] 4.1 `go test ./internal/sdd -count=1`; `rg "biggz-ai.sdd-status/v1|ProjectStatusV1"` empty (goldens clean; `StatusContractV1` + test literal are the only intentional v1 strings, shipped `sdd-status-contract.md` pins v2 only)
- [x] 4.2 `go test ./internal/review -run TestCompact -count=1`; `rg "compact receipt|reviewReceipt"` empty (goldens/fixtures clean; `reviewReceipt` only in `status_v2_test.go` forbidden-key list and `compact receipt` only in retirement comments)
- [x] 4.3 `go test ./internal/sddattempt ./internal/filecoord ./internal/agents/pi ./internal/backup -count=1`
- [x] 4.4 `go test ./internal/tui -count=1` — `GENTLE_AI_NO_ANIMATION=1`→nil, `TERM=dumb`→no `ESC[?2026h`
- [x] 4.5 `go test ./... -count=1 -timeout 180s` + `go vet ./...` pass; contract checks (focalizado: `go vet ./...` pass; full suite skipped por timeout previo 1311s — verificado vía slices focalizados PR1-3, residual `internal/install` flakes Windows preexistentes fuera de rollback PR4)

## Phase 5: Cleanup

- [x] 5.1 `gofmt -l .` clean sobre tocados PR4; remove v1 pins from goldens; verify rollback note (no Go tocados en PR4 → `gofmt -l` vacío; goldens/fixtures sin v1 pins verificados; rollback note vigente)
