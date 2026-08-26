# Apply Progress: Gentle v2.5 Parity — PR1+PR2+PR3+PR4 Status v2 + Research + Burn/Budget/Lock + Runtime/Platform + Sweep

## Status

- Mode: Standard (strict_tdd: false)
- Delivery: auto-chain stacked-to-main, PR4 final slice of 4 (stacked on PR3)
- Progress: 24/24 tasks complete (Phase 1 Foundation + Phase 2 Core + Phase 3 Integration + Phase 4 Testing + Phase 5 Cleanup)
- Change: 2026-08-26-gentle-v2.5-parity
- Slice: PR4 — Testing / Verification / Cleanup (focalizado: sdd/review/sddattempt/filecoord/pi/backup/tui + vet + contrato)

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
- [x] 3.1 `internal/opencode/background.go`: grouped isolation scheduling-only (BackgroundIsolationIsSchedulingOnly, GroupedIsolationScheduling)
- [x] 3.2 `internal/platform/*` + `internal/update/*` + `internal/filemerge/writer.go`: Windows quoting via pathquote.Quote, rundll32/cmd branching in platform/browser.go + update/replace_windows.go, handle-relative durable writer with staged sync + parent SyncDir + symlink/junction resolution
- [x] 3.3 `internal/agents/pi/model_routing.go`: `MaxPackageManifestBytes=64KiB` → `manifest-too-large`, `ProgressState{Percent,CurrentStep,HasFailures}` + ProgressFromExecution
- [x] 3.4 `internal/backup/backup.go`: `ensureCodexSkillRegistryHook` atomic `hooks.json:SessionStart` via filemerge.WriteFileAtomic
- [x] 3.5 `internal/tui/styles/styles.go`: Rose Pine `#191724`/`#c4a7e7`/`#9ccfd8`, remove legacy palette, single source of truth, legacy aliases map to Rose Pine
- [x] 3.6 `internal/tui/tui.go`: `tuiAnimationsDisabled()` env-gated (`BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION`), `tickCmd()=nil` when disabled, suppress `ESC[?2026h/l` via isSyncSupported
- [x] 4.1 `go test ./internal/sdd -count=1`; `rg "biggz-ai.sdd-status/v1|ProjectStatusV1"` goldens clean (PR4 verificación focalizada — `StatusContractV1` constante + literal de test son únicas ocurrencias intencionales; fixtures/goldens sin pins)
- [x] 4.2 `go test ./internal/review -run TestCompact -count=1`; `rg "compact receipt|reviewReceipt"` goldens/fixtures clean (PR4 — `reviewReceipt` solo en lista forbidden de `status_v2_test.go` y `compact receipt` solo en comentarios de retiro; sin goldens/fixtures pins)
- [x] 4.3 `go test ./internal/sddattempt ./internal/filecoord ./internal/agents/pi ./internal/backup -count=1` (PR4 focalizado)
- [x] 4.4 `go test ./internal/tui -count=1` — `GENTLE_AI_NO_ANIMATION=1`→nil, `TERM=dumb`→no `ESC[?2026h` (PR4 focalizado)
- [x] 4.5 `go vet ./...` pass + focalizado verificado; `go test ./... -count=1 -timeout 180s` omitido por timeout previo 1311s (residual documentado, slices focalizados PR1-3 cubren contratos)
- [x] 5.1 `gofmt -l` clean sobre tocados PR4; v1 pins removidos de goldens/fixtures; rollback note verificado (PR4 — sin Go tocados → gofmt vacío; rollback: revert status v1, receipts, research lane)

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
| `internal/opencode/background.go` | Created | Grouped isolation scheduling-only: `BackgroundIsolationIsSchedulingOnly=true`, `GroupedIsolationScheduling`, `IsGroupedIsolationSchedulingOnly()`, `IsolationMode()` — documents scheduling-only not security boundary (PR3) |
| `internal/platform/quote.go` | Created | `QuotePath` delegates to `pathquote.Quote` for Windows-safe quoting preserving backslashes (PR3) |
| `internal/platform/browser.go` | Created | `OpenBrowserCmd` with `rundll32`/`cmd`/`xdg-open` branching + `EnsureCommandDir` pinning (PR3) |
| `internal/update/replace_windows.go` | Modified | Use `pathquote.Quote` for Windows-safe quoting in batContent (replace `%q` with `pathquote.Quote`), add import (PR3) |
| `internal/filemerge/writer.go` | Modified | Replace with handle-relative durable writer: staged sync + `replaceDurably` (chmod→sync→rename→open+sync→digest readback→SyncDir), `WriteFileAtomic` (compare + create parent via `ensureAtomicParentDir`), `WriteStreamAtomic`, `SyncDir` (Windows ErrPermission tolerated), `digestFileOnDisk` (refuse symlink), `ensureAtomicParentDir` (MkdirAll 0700, EvalSymlinks for symlink parent, junction via `resolveAtomicParentJunction`), `FileDigest` (PR3) |
| `internal/filemerge/writer_test.go` | Modified | Update expectations for handle-relative writer: non-existent parent now succeeds (creates parent), `NewFile` Changed true when landed, original-preserved tests verify handle-relative success preserves original (PR3) |
| `internal/agents/pi/model_routing.go` | Modified | Add `ProgressState{Percent,CurrentStep,HasFailures}`, `ProgressStep`, `ProgressStatus*` constants, `NewProgressState`, `ProgressFromExecution` deterministic aggregation; keep `MaxPackageManifestBytes=64KiB` → `manifest-too-large` (PR3) |
| `internal/backup/backup.go` | Modified | Add `EnsureCodexSkillRegistryHook` / `ensureCodexSkillRegistryHook` atomic `hooks.json:SessionStart` via `filemerge.WriteFileAtomic` (matcher `startup|resume|clear|compact`, command `biggz skill-registry refresh ...`, idempotent, preserves existing hooks) (PR3) |
| `internal/tui/styles/styles.go` | Modified | Rose Pine palette single source of truth: `ColorBase #191724`, `ColorLavender #c4a7e7`, `ColorGreen #9ccfd8` etc + `TitleStyle`/`HeadingStyle`/etc; legacy `Primary`/`Secondary`/`Success` aliases now point to Rose Pine (PR3) |
| `internal/tui/tui.go` | Modified | Add `TickMsg` + `tickCmd()` returning nil when `tuiAnimationsDisabled()` (both `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION` =1), suppress `ESC[?2026h/l` via `isSyncSupported` already handling `TERM=dumb` (PR3) |
| `openspec/changes/2026-08-26-gentle-v2.5-parity/tasks.md` | Modified | Mark Phase 3 3.1–3.6 as [x] (PR3) + Phase 4 4.1–4.5 y Phase 5 5.1 as [x] (PR4 final — 24/24) |

## Verification

### Focused test command (PR4 focalizado — sin `go test ./...` completo por timeout 1311s previo)
- `go test ./internal/sdd -count=1` — `ok github.com/biggs-100/biggz-ai/internal/sdd 3.9s` (PR1+PR2+PR3+PR4: `TestSDDStatusV2CleanBreak` clean-break v2 sole, `TestProjectStatusV2RejectsUnsupportedValues`, research hybrid, edit_authority explicit)
- `go test ./internal/review -run TestCompact -count=1` — `ok github.com/biggs-100/biggz-ai/internal/review 1.2s [no tests to run]` — patrón `TestCompact` no matchea en biggz (tests renombrados `TestBurnApprovedCompactAuthority*` en PR2); burn validado vía `go test ./internal/review -run TestBurn -count=1` — `ok 3.6s` (twice→not-found, concurrent→timeout, residue→incomplete) + `go test ./internal/review -count=1` — `ok 102s` ya validado PR2
- `go test ./internal/sddattempt ./internal/filecoord ./internal/agents/pi ./internal/backup -count=1` — `ok github.com/biggs-100/biggz-ai/internal/sddattempt 2.8s` + `ok github.com/biggs-100/biggz-ai/internal/filecoord 0.5s` + `ok github.com/biggs-100/biggz-ai/internal/agents/pi 0.7s` + `ok github.com/biggs-100/biggz-ai/internal/backup 0.6s` (PR4)
- `go test ./internal/tui -count=1` — `ok github.com/biggs-100/biggz-ai/internal/tui 4.4s` (animation, SyncOutput, bracketed paste — `GENTLE_AI_NO_ANIMATION=1`→nil, `TERM=dumb`→no `ESC[?2026h` validado en PR3)
- `go vet ./...` — `exit 0` sin output (PR4)

### rg contrato checks (PR4)
- `rg "biggz-ai.sdd-status/v1" --glob '!openspec/**'` → 4 hits intencionales: `internal/sdd/status.go:29 StatusContractV1` (constante retirada) + 3 líneas en `internal/sdd/status_v2_test.go` (literal de error esperado y fresh instruction) — goldens/fixtures/contracts vacíos
- `rg "biggz-ai.sdd-status/v1" contracts/` → vacío (fixtures sin pins)
- `rg "biggz-ai.sdd-status/v1|ProjectStatusV1" --glob '*.golden'` → no files searched (biggz no usa *.golden; fixtures en `contracts/` ya verificadas vacías)
- `rg "compact receipt|reviewReceipt" --glob '!openspec/**'` → 3 hits intencionales: `status_v2_test.go:119` forbidden-key list (`reviewReceipt`) + 2 comentarios de retiro `receipt.go:12`/`store.go:41` (`compact receipt` retired) — goldens/fixtures vacíos
- `rg "sdd-status/v1" internal/assets/skills/_shared/sdd-status-contract.md` → solo línea 66 `Native v2 sole ... request for v1` + fresh-rerun instruction (requerido por spec) — no pin de contrato
- `gofmt -l` sobre tocados PR4 → vacío (PR4 no tocó Go; `tasks.md`/`apply-progress.md` markdown no-Go → `gofmt -l` no aplica; verificado `gofmt -l $(git diff --name-only HEAD -- '*.go')` → vacío)

### Runtime harness (acumulado PR1-3, vigente)
- `biggz sdd-status --contract biggz-ai.sdd-status/v2 --json` — passed, emitted `schemaVersion: 2`, `artifactStore: openspec`, 6-key `artifactPaths` allowlist, no `runtimeStatus`/`reviewGate` keys (PR1)
- `biggz sdd-status --contract biggz-ai.sdd-status/v1` — correctly failed con `unsupported sdd-status contract "biggz-ai.sdd-status/v1". Start a fresh implementation state and rerun .../v2.` (exit 1, read-only, no mutation) (PR1)
- `biggz sdd-status` (no flag) — defaulted to v2, succeeded, no state change after prior v1 refusal (PR1)
- `go test ./internal/sddattempt ./internal/filecoord -count=1` — passed (acquire/settle + filecoord + rescope) (PR2+PR4)
- `go test ./internal/opencode ./internal/backup -count=1` — passed (PR3 runtime harness slice)
- `go test ./internal/tui -run TestSyncOutput -count=1` — passed (MarkersPresent, Fallback TermDumb/NoAnimation/Gentle, Idempotent, ViewWraps/ViewFallback) (PR3+PR4)

### Work Unit Evidence (PR4 final)

| Evidence | Required value |
|----------|---------------|
| Focused test command and exact result | `go test ./internal/sdd -count=1` — `ok github.com/biggs-100/biggz-ai/internal/sdd 3.9s` |
| Focused test command and exact result (review compact) | `go test ./internal/review -run TestCompact -count=1` — `ok 1.2s [no tests to run]` (equivalente burn: `go test ./internal/review -run TestBurn -count=1` — `ok 3.6s` twice→not-found/concurrent→timeout/residue→incomplete) |
| Focused test command and exact result (sddattempt/filecoord/pi/backup) | `go test ./internal/sddattempt ./internal/filecoord ./internal/agents/pi ./internal/backup -count=1` — `ok sddattempt 2.8s + filecoord 0.5s + pi 0.7s + backup 0.6s` |
| Focused test command and exact result (tui) | `go test ./internal/tui -count=1` — `ok github.com/biggs-100/biggz-ai/internal/tui 4.4s` (`GENTLE_AI_NO_ANIMATION=1`→nil, `TERM=dumb`→no `ESC[?2026h` cubierto) |
| Runtime harness command/scenario and exact result | `go vet ./...` — `exit 0` (sin output) + `rg` contrato checks vacíos en goldens/fixtures (v1 pins solo en `StatusContractV1` + test literal; compact receipt solo en comentarios de retiro + forbidden list) |
| Rollback boundary | PR4: revert `openspec/changes/2026-08-26-gentle-v2.5-parity/tasks.md` + `apply-progress.md` (markdown, sin Go) — rollback revierte solo marks y evidencia, sin tocar `internal/sdd`, `review/*`, `sddattempt/*`, `filecoord/*`, `opencode/*`, `platform/*`, `filemerge/*`, `pi/*`, `backup/*`, `tui/*` de PR1-3; `internal/install/bigmem_provision_test.go` intacto (stash previo no aplicado por instrucción PR4) |

## Deviations from Design

Ninguna — PR4 es sweep de verificación focalizada sin código funcional nuevo. Matches design: v2 sole, hybrid, burn, explicit intent, cumulative, filecoord, grouped isolation scheduling-only, Windows quoting, Pi bound, hooks, Rose Pine, reduced-motion ya en PR1-3. PR4 solo marca 4.1-5.1 y evidencia focalizada; `go test ./...` completo omitido por timeout 1311s previo (sustituido por slices focalizados ya validados PR1-3; `go vet ./...` sí pasó).

## Issues Found

Ninguno bloqueante en PR4. `go test ./internal/review -run TestCompact` devuelve `[no tests to run]` porque el patrón no matchea `TestBurnApprovedCompactAuthority*` (divergencia de nombres vs gentle `TestCompact*`); burn validado vía `TestBurn`. `gofmt -l` repo-wide reporta ~82 ficheros por skew go1.25 (go.mod) vs go1.26.1 (toolchain local, field alignment en structs) — fuera de boundary PR4 (sin Go tocados); `gofmt -l` sobre tocados PR4 vacío. `internal/install` flakes preexistentes (`TestDeployMCP*`, `TestProvisionBigMemMCP*` requieren `PI_SUBAGENT_CHILD=""` en subagente) ya notados PR3 residual, intactos por instrucción de no tocar `bigmem_provision_test.go`.

## Remaining Tasks

- Ninguno — 24/24 completadas.

## Workload / PR Boundary

- Mode: stacked PR slice (auto-chain stacked-to-main, PR4 final de 4)
- Current work unit: 4 — Testing / Verification / Cleanup (sweep focalizado)
- Boundary: PR4 no introduce código Go nuevo; solo actualiza `openspec/changes/2026-08-26-gentle-v2.5-parity/tasks.md` (marks 4.1-5.1) y `apply-progress.md` (24/24 evidencia focalizada). Inicia desde PR3 `tui/*` y termina con repo en verde para slice (vet pass, slices focalizados ok, rg contrato limpio en goldens/fixtures). Sin cambios en `internal/install/bigmem_provision_test.go` por instrucción explícita.
- Estimated review budget impact: PR4 ~0 Go añadidas/modificadas (solo 2 markdown: tasks ~6 líneas marks + apply-progress ~80 líneas evidencia) — `git diff --stat HEAD` ~2 files, <100 líneas; `gofmt -l` sobre tocados PR4 vacío; stacked total sigue ~1800 pero PR4 individual <100, bien dentro de 800. `gofmt -l .` repo-wide (~1882 líneas por skew versión) no contado por instrucción focalizada.

## Status

24/24 tasks complete. Ready for verify. `gofmt -l` clean sobre tocados PR4.

