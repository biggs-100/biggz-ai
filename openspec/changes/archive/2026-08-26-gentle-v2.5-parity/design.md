# Design: Gentle v2.5 Parity — Research, Status v2, Last-Event Closure

## Technical Approach

Port gentle-ai v2.5.0-rc.1: v2 sole, research hybrid same-bytes, last-event burn, explicit intent, runtime (grouped isolation, Windows beta, Pi, lock, hooks), TUI reduced-motion + Gentleman-Cute.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| Status v2 | Compat / reject v1 | Compat hides drift | Reject `biggz-ai.sdd-status/v1` with `rerun --contract v2`; `SchemaVersion=2` sole; allowlist planning/tasks/verification |
| Research | Open / closed | Open admits Bash | Closed `biggz-ai.sdd-research-capability/v1`: `documentation`+`WebFetch`, `open-web`+`WebSearch+WebFetch`; else deny |
| Hybrid | Prefer / byte-equal | Prefer hides drift | Equal revision+bytes; one-sided replays retained intent to both then re-reads; missing→blocked |
| Pre-proposal gate | Proposer / orchestrator | Proposer leaks choices | Orchestrator offers research after `explore`; selected blocks `propose` until `done`+`confirmed`+refs+ready |
| Last-event | Receipt / burn | Receipt resurrects | Terminal capture burns lineage under lock+lease (3 paths); no receipt; reuse→not-found |
| Intent | Fuzzy / explicit | Fuzzy causes edits | Only explicit `apply to <path>` permits edit; `investigate`→read-only |
| Isolation | Security / scheduling | Security illusion | Grouped isolation=scheduling only; `filecoord` `Acquire`→`BusyError` without mutation |
| Windows | Unix rename / branch | Rename fails on lock | Windows-safe quoting, `rundll32`/`cmd`, handle-relative writer |
| Pi | Unbounded / bounded | Unbounded DoS | `MaxPackageManifestBytes=64KiB`→`manifest-too-large`; `ProgressState{Percent,CurrentStep,HasFailures}` |
| Codex hooks | Append / atomic | Corrupt config | `ensureCodexSkillRegistryHook` atomically edits `hooks.json:SessionStart` |
| TUI motion | Always / env-gated | Breaks dumb terms | `GENTLE_AI_NO_ANIMATION=1`/`BIGGZ_NO_ANIMATION=1`/`TERM=dumb`→`tickCmd()=nil`, no `ESC[?2026h/l` |

## Data Flow

```
explore → [research offer] → research(admission→sources→claims)
              │ bypass if unselected
              ↓
       pre-proposal gate (done+confirmed+refs+store-ready)
              ↓
       status v2 (cumulative Attempts/Lines)
              ↓
       orchestrator intent guard
              ↓
       review last-event (START freeze → collect → burn)
              ↓
       runtime (CAS acquire/settle → filecoord → Windows/hooks)
              ↓
       TUI (motion gate → palette → ProgressState)
```

Rescope: `Cumulative=5, Max 5→3` measures against 5.

## File Changes

| File | Action | Description | Est. |
|------|--------|-------------|------|
| `internal/sdd/status.go` | Modify | v2 sole, reject v1, allowlist `ProjectStatusV2` | M |
| `internal/sdd/research.go`, `preproposal.go` | Create | `sdd-research/v1`+`preproposal/v1`, hybrid recovery | M |
| `internal/agents/researchcapability/*` | Create | Closed admission exact-grant | S |
| `internal/assets/skills/_shared/*`, `sdd-research/*` | Create/Modify | Port lifecycle, status v2, ledger burn, `SKILL.md` | S |
| `internal/review/compact_burn.go`, `receipt.go`, `store.go` | Modify | Burn lock+lease+verify; retire receipts | M |
| `internal/sdd/edit_authority.go` | Modify | Explicit intent guard | S |
| `internal/sddattempt/cas_store.go` | Modify | Cumulative budget, anti-inherit on rescope | S |
| `internal/filecoord/lock*.go` | Modify | `BusyError` cooperative, `no-follow` | S |
| `internal/opencode/background.go` | Modify | Grouped scheduling-only | S |
| `internal/platform`, `update`, `filemerge` | Modify | Windows quoting/rundll32/writer | M |
| `internal/agents/pi/model_routing.go` | Modify | Bounded manifest, `ProgressFromExecution` | S |
| `internal/backup/*` | Modify | Codex `hooks.json` delegation | S |
| `internal/tui/model.go`, `styles/styles.go` | Modify | Motion gate + Rose Pine palette | S |

## Interfaces / Contracts

```go
const StatusContractV2 = "biggz-ai.sdd-status/v2"
func ProjectStatusV2(Status) (StatusV2Projection, error) // allowlist only
// V2 keys: schemaName, artifactStore, planningHome, changeRoot, artifactPaths, contextFiles,
// artifacts, taskProgress, dependencies, applyState, actionContext, relationships,
// remediationState, reviewOffer, consent, nextRecommended, blockedReasons
// Removed: reviewGate, reviewTransaction, runtimeStatus, lineageId, generation, fixBatch
func AdmitResearch(cap string, grants []string) error
func BurnApprovedCompactAuthority(ctx context.Context, repo, lineageID, rev string) error
const MaxPackageManifestBytes = 64 << 10
func tuiAnimationsDisabled() bool; func tickCmd() tea.Cmd // nil when disabled
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | v1 rejected w/ fresh instruction, default v2, allowlist | `TestSDDStatusV2CleanBreak` |
| Unit | docs/open-web allow, Bash/MCP deny; hybrid divergence/recovery | Table tests |
| Integration | Burn all paths, burned→not-found, receipts retired | Port `compact_last_event_closure_test.go` |
| Integration | Rescope 5/5→3, lock `BusyError`, manifest too-large, hooks atomic | Ledger/filecoord/pi/backup tests |
| E2E | `go test ./... -count=1 -timeout 180s` pass; `biggz sdd-status --contract v1` fails | CI |

## Threat Matrix

| Boundary | Applicable | Design Response | Planned RED Tests |
|----------|------------|-----------------|-------------------|
| Review burn | **Applicable** — no second decision | Lock+lease, delete 3 paths, verify; reuse→not-found | Burn twice→not-found; concurrent→timeout; residue→incomplete |
| Research hybrid | **Applicable** — byte-equal | Retained intent replay to both, equal check; missing→blocked | Divergent→blocked; one-sided→both; missing→blocked |
| Docs-like paths | N/A — no exec classification | — | — |
| Git selection | N/A — filecoord relative | — | — |
| Commit/Push/PR | N/A — no VCS automation | — | — |

Applicable rows → tasks.md. Hooks: `biggz sdd-status --contract v2` pass / `v1` fail; admission, burn, guard tests.

## Migration / Rollout

No migration. Clean break v1 rejected. Rollback: revert `status.go`, restore receipts, disable research. `auto-chain` slices testable.

## Open Questions

- None.
