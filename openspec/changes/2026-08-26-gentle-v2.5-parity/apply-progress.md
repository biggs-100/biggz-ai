# Apply Progress: Gentle v2.5 Parity — PR1+PR2 Status v2 + Research + Burn/Budget/Lock

## Status

- Mode: Standard (strict_tdd: false)
- Delivery: auto-chain stacked-to-main, PR2 slice of 4 (stacked on PR1)
- Progress: 12/24 tasks complete (Phase 1 Foundation + Phase 2 Core)
- Change: 2026-08-26-gentle-v2.5-parity
- Slice: PR2 — Last-event burn + cumulative budget + edit-authority guard + filecoord BusyError

## Completed Tasks

- [x] 1.1 RED: `internal/sdd/status_v2_test.go` — v1→fail `unsupported contract` + rerun v2; fails before impl
- [x] 1.2 `internal/sdd/status.go`: `SchemaVersion=2`, `StatusContractV2`, `ProjectStatusV2` allowlist
- [x] 1.3 `internal/sdd/status.go` CLI: default v2, reject v1 read-only
- [x] 1.4 Create `internal/sdd/research.go` + `preproposal.go`: hybrid equal revision+bytes, one-sided replay, missing→blocked
- [x] 1.5 Create `internal/agents/researchcapability/*`: closed `biggz-ai.sdd-research-capability/v1`, exact grants, else deny
- [x] 1.6 Create `internal/assets/skills/sdd-research/*` + `_shared/*`: port lifecycle, status v2, burn docs
- [x] 2.1 RED: `internal/review/compact_burn_test.go` — twice→not-found, concurrent→timeout, residue→incomplete
- [x] 2.2 RED: `internal/sdd/research_test.go` — divergent→blocked, one-sided→both, missing→blocked
- [x] 2.3 `internal/review/compact_burn.go` + `store.go` + `receipt.go`: `BurnApprovedCompactAuthority` lock+lease, delete 3 paths, retire receipts
- [x] 2.4 `internal/sdd/edit_authority.go`: explicit `apply to <path>` only; `investigate|if possible` read-only
- [x] 2.5 `internal/sddattempt/cas_store.go`: cumulative never reset; rescope `5/5→3` vs 5
- [x] 2.6 `internal/filecoord/lock.go` + `lock_backend.go`: `Acquire(ctx,target,root)` non-blocking → `BusyError`, `no-follow`

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/status.go` | Modified | Bump `StatusSchemaVersion` to 2, add `StatusContractV2`/`V1`, wire `ArtifactStore`/`Relationships`/`ReviewOffer` fields, set v2 defaults in derive (PR1) |
| `internal/sdd/status_v2.go` | Created | `StatusContractV2`, `ArtifactStore`, `Relationships`, `ReviewOfferBlock`, `StatusV2Projection`, `ParseCommandArgs` (default v2, reject v1 with fresh instruction), `ProjectStatusV2` allowlist, helpers (PR1) |
| `internal/sdd/status_v2_test.go` | Created | RED clean-break tests: default v2, v1 fails with rerun instruction, projection key allowlist, rejection of unknown values (PR1) |
| `internal/sdd/research.go` | Created | `ResearchSchemaV1`, `ResearchRecord`, `ParseResearchRecord`, `IsResearchComplete`, `HybridResearchEqual`, `EvaluateResearchHybrid`, `RecoverHybridResearch` — hybrid equal revision+bytes, one-sided replay, missing→blocked (PR1) |
| `internal/sdd/preproposal.go` | Created | `PreproposalSchemaV1`, `PreproposalRecord`, `ParsePreproposalRecord`, `IsPreproposalReady`, `RecoverHybridPreproposal` — hybrid same-bytes gate, blocked until done+confirmed+refs+equal (PR1) |
| `internal/sdd/engram_status.go` | Modified | Set `ArtifactStore: openspec/engram`, `Relationships: {}` and `ReviewOffer: nil` for v2 parity (PR1) |
| `internal/agents/researchcapability/contract.go` | Created | Closed `biggz-ai.sdd-research-capability/v1` with `documentation→WebFetch` and `open-web→WebSearch+WebFetch` exact, else deny; `ForAgent` defensive copy, `Admit` exact-match (PR1) |
| `internal/assets/skills/_shared/sdd-status-contract.md` | Modified | Port to `biggz-ai.sdd-status/v2` sole contract: update schemaVersion, add artifactStore/relationships/reviewOffer, note v1 retired, add pre-proposal gate section (PR1) |
| `internal/assets/skills/_shared/research-lifecycle.md` | Modified | Update gate note: native `biggz-ai.sdd-status/v2` sole, `v1` retired (PR1) |
| `internal/assets/skills/_shared/persistence-contract.md` | Modified | Add Research reconciliation section with closed readiness matrix and hybrid same-bytes recovery rule (PR1) |
| `internal/assets/skills/_shared/bigmem-convention.md` | Modified | Add Research artifacts: `sdd/{change}/research` and `preproposal` hybrid equal revision+bytes note (PR1) |
| `internal/assets/skills/_shared/openspec-convention.md` | Modified | Add Research artifacts: `research.md` `biggz-ai.sdd-research/v1` + hybrid preproposal note (PR1) |
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | Update research gate to advertise `v2` sole contract (PR1) |
| `cmd/biggz/cli_sdd.go` | Modified | Add `--contract` flag: default `biggz-ai.sdd-status/v2`, reject `v1` and unknown with fresh-v2 rerun instruction, read-only fail (PR1) |
| `internal/review/compact_burn.go` | Created | `BurnApprovedCompactAuthority` with maintenance lease (2s) + version lock, validates revision+state approved, deletes 3 paths (v2/<lineage>, effect-markers/v1/<lineage>, incidents/<lineage>), verifies absence, returns `ReviewAuthorityBurnIncompleteError` on residue, `CompactRevisionConflictError` on mismatch, `ReviewAuthorityBurnStateError` on non-approved; `storeResetRemoveTreeFn` stubbable for tests (PR2) |
| `internal/review/compact_burn_test.go` | Created | RED then GREEN: twice→not-found (second burn fails lineage-not-found), concurrent→timeout (second holds lease 2s timeout), residue→incomplete (stubbed RemoveAll leaves residue, burn returns incomplete, authority remains) (PR2) |
| `internal/review/store.go` | Modified | Update burn semantics comment to last-event closure + 3-path delete + retire receipts, note compact receipts retired (PR2) |
| `internal/review/receipt.go` | Modified | Add header comment: compact receipts retired, no compact receipt file persisted/consumed, burn-based delivery without tombstone/mirror (PR2) |
| `internal/sdd/research_test.go` | Created | RED then GREEN: divergent→blocked (different bytes/revision → blocked), one-sided→both (retained intent + canonical writes new equal rev to both → ready), missing→blocked (retained 0/nil → blocked) (PR2) |
| `internal/sdd/edit_authority.go` | Modified | Enhance `detectUnauthorizedEditRoots` with `sameRepositoryEditRoot` narrowing + canonical EvalSymlinks on allowed roots, prefix+pathidentity `withinAnyRoot`, add `HasExplicitEditIntent` (apply to <path> only, investigative `investigate|explore|check|look into` and conditional `if possible|maybe|consider|when ready` → read-only) (PR2) |
| `internal/sdd/edit_authority_explicit_test.go` | Created | Tests for `HasExplicitEditIntent`: explicit apply→true, investigate/conditional→false (PR2) |
| `internal/sddattempt/cas_store.go` | Modified | Add header comment: cumulative attempts/lines never reset on rescope, 5/5→3 measured vs 5 already consumed (PR2) |
| `internal/sddattempt/sddattempt.go` | Modified | Add `Rescope` (cumulative-preserving narrow): validates active==0, requires prior terminal attempt, must narrow (new ≤ old), refuses when cumulative > new ceiling with `ErrRuntimeRescopeWidened`, preserves `Attempts` slice (never reset), fix `Begin` ordinal derivation via `len(Attempts)+1` when no active, fix `Finish` to clear `ActiveAttempt` on failed/interrupted; add `ErrRuntimeRescopeWidened/NotAllowed` sentinels (PR2) |
| `internal/sddattempt/rescope_test.go` | Created | Tests: cumulative never reset (2/5→3 preserves len 2, next ordinal 3 not 1), 5/5→3 vs 5 (narrow that would exceed cumulative refused with `ErrRuntimeRescopeWidened`, ledger unchanged vs fresh 0) (PR2) |
| `internal/filecoord/lock.go` | Created | `ErrBusy/InvalidRoot/InvalidTarget/Operational`, `BusyError`, `Lease` with idempotent `Release`, `LockPath` (sha256 hash of cleaned absolute target), `Acquire(ctx,target,root)` validates, honors `ctx.Err()` before FS, delegates to `acquireCooperativeLock` (PR2) |
| `internal/filecoord/lock_backend.go` | Created | `acquireCooperativeLock` via `O_CREATE|O_EXCL` non-blocking, returns `BusyError` on `EEXIST` (caller owns retry pacing), `no-follow` walk rejecting symlink components (Windows skipped), `os.MkdirAll` for dir, idempotent `Lease` close (PR2) |
| `internal/filecoord/filecoord_test.go` | Created | Tests: exclusive until release, second→BusyError, release idempotent, cancelled context→Operational without FS mutation, (symlink fixture skipped on Windows) (PR2) |
| `openspec/changes/2026-08-26-gentle-v2.5-parity/tasks.md` | Modified | Mark Phase 2 2.1–2.6 as [x] (PR2) |

## Verification

### Focused test command
- `go test ./internal/sdd -run TestStatusV2 -count=1` — passed (both `TestSDDStatusV2CleanBreak` and `TestProjectStatusV2RejectsUnsupportedValues` green) (PR1)
- `go test ./internal/sdd -run TestResearch -count=1` — passed (3 tests: divergent blocked, one-sided ready, missing blocked) (PR2)
- `go test ./internal/sdd -run TestHasExplicit -count=1` — passed (explicit intent true/false matrix) (PR2)
- `go test ./internal/review -run TestBurn -count=1` — passed (burn twice→not-found, concurrent→timeout, residue→incomplete plus existing Burn_PreventsReplay, GateBecomesInformational, ReceiptEphemeral) (PR2)
- `go test ./internal/sddattempt -run TestRescope -count=1` — passed (cumulative never reset 2/5→3 preserves 2, 5/5→3 refused vs 5) (PR2)
- `go test ./internal/filecoord -count=1` — passed (exclusive until release, BusyError, cancelled context) (PR2)
- `go test ./internal/sdd -count=1` — passed
- `go vet ./...` — passed (no output)

### Runtime harness
- `biggz sdd-status --contract biggz-ai.sdd-status/v2 --json` — passed, emitted `schemaVersion: 2`, `artifactStore: openspec`, 6-key `artifactPaths` allowlist, no `runtimeStatus`/`reviewGate` keys (PR1)
- `biggz sdd-status --contract biggz-ai.sdd-status/v1` — correctly failed with `unsupported sdd-status contract "biggz-ai.sdd-status/v1". Start a fresh implementation state and rerun .../v2.` (exit 1, read-only, no mutation) (PR1)
- `biggz sdd-status` (no flag) — defaulted to v2, succeeded, no state change after prior v1 refusal (PR1)
- `go test ./internal/sddattempt ./internal/filecoord -count=1` — passed (acquire/settle + filecoord + rescope) (PR2)

### Work Unit Evidence

| Evidence | Required value |
|----------|---------------|
| Focused test command and exact result | `go test ./internal/review -run TestBurn -count=1` — `ok github.com/biggs-100/biggz-ai/internal/review 6.4s` (6 tests: 3 new burn + 3 existing) |
| Focused test command and exact result (research) | `go test ./internal/sdd -run TestResearch -count=1` — `ok github.com/biggs-100/biggz-ai/internal/sdd 0.7s` (3 tests passed) |
| Runtime harness command/scenario and exact result | `go test ./internal/sddattempt ./internal/filecoord -count=1` — `ok github.com/biggs-100/biggz-ai/internal/sddattempt 1.0s` + `ok github.com/biggs-100/biggz-ai/internal/filecoord 0.3s` |
| Rollback boundary | PR1: revert `internal/sdd/status*`, `research.go`/`preproposal.go`, `researchcapability`, 5 `_shared` docs, `cli_sdd.go`; PR2: revert `review/compact_burn*`, `review/store.go`/`receipt.go`, `sdd/edit_authority*`+`research_test.go`, `sddattempt/*`+`rescope_test.go`, `filecoord/*` — no overlap with PR3/PR4 |

## Deviations from Design

None — implementation matches design: v2 sole with `SchemaVersion=2`, `StatusContractV2`, `ProjectStatusV2` allowlist; CLI default v2 reject v1; research hybrid equal revision+bytes with one-sided retained-intent replay and missing→blocked; burn lock+lease delete 3 paths verify absence retire receipts; explicit intent `apply to <path>` only investigative/conditional read-only; cumulative never reset rescope 5/5→3 vs 5; filecoord `Acquire(ctx,target,root)` non-blocking `BusyError` no-follow.

## Issues Found

None. `gofmt -l .` clean after PR2. Tagged-test v1 pin scan (`rg "biggz-ai.sdd-status/v1|ProjectStatusV1"` in tests) is intentional: the `StatusContractV1` constant and the expected-error literal in `status_v2_test.go` are the only occurrences; shipped `sdd-status-contract.md` no longer pins v1 except in the fresh-rerun instruction, which is required by spec.

## Remaining Tasks

- [ ] 3.1–3.6 Integration: runtime/platform (opencode grouped isolation, Windows quoting/rundll32/writer, Pi manifest bound, backup hooks, TUI reduced-motion + Rose Pine)
- [ ] 4.1–4.5 Testing / Verification
- [ ] 5.1 Cleanup (gofmt, remove v1 pins from goldens, rollback note)

## Workload / PR Boundary

- Mode: stacked PR slice (auto-chain stacked-to-main, PR2 of 4)
- Current work unit: 2 — Burn + budget + lock
- Boundary: starts from PR1's `status_v2`+`research` artifacts; ends with `review/compact_burn`, `edit_authority` explicit intent, `sddattempt` Rescope cumulative, `filecoord` BusyError — no runtime/platform or TUI changes
- Estimated review budget impact: PR1 ~650 added +80 modified, PR2 ~520 added (burn 140+100, research 70+25, edit_authority 60, filecoord 200, rescope 100+20, cas_store comment) + ~90 modified (sddattempt Begin/Finish/Rescope, edit_authority, store/receipt comments), both slices within 800-line budget individually, stacked total ~1200 but split across PRs

## Status

12/24 tasks complete. Ready for next batch (PR3: Runtime/platform). `gofmt -l .` clean.
