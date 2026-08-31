# Exploration — sdd-parity-rescope-grant-ledger

> **Change:** `sdd-parity-rescope-grant-ledger` — Close critical SDD parity gaps vs gentle-ai (2026-08-31)
> **Preflight:** interactive, artifact_store=openspec, delivery=auto-chain, review_budget=800, sdd-init DONE
> **Scope:** single-worktree, file-backed `openspec`, no linked-worktree orchestration

## Context
Comparative audit of `biggz-ai` @ `C:/Users/USER/Desktop/biggz-ai` vs `gentle-ai` @ `C:/Users/USER/Desktop/herramientas/gentle-ai` (2026-05-17) identified 7 parity gaps. The 5 high-ROI gaps are: legacy ledger duplication, rescope narrowing, grant instance scoping (cosmetic), runtime topology guard, and strict passive-risk proof. Hybrid equal-bytes is already compliant. Attestation/handoff are correctly discarded for biggz's single-worktree, review-free architecture.

## Current Architecture & Files

| File | Lines / Symbol | Gap | Role |
|------|---------------|-----|------|
| `internal/sdd/attempt.go:12` | `AttemptState{MaxAttempts=3, MaxLines=400}` → `.attempt.json` | 1 duplicate ledger | Legacy file ledger; callers outside tests: none |
| `internal/sddattempt/sddattempt.go:1992` | `Rescope()` + `runtimeObjectiveRescopeStructurallyPermitted:2078` wedge-only | 2 rescope | CAS ledger `<git-common-dir>/biggz/sdd-runtime/v1/<change>/record-<sha>.json + HEAD + LOCK` |
| `internal/sddattempt/cas_store.go:86` | `Store` + `grantedRootsFor:108` + `Grant:1728` + `StatusWithInstance:306` | 3 ForInstance | `RuntimeGrant{Instance}` + instance-scoped `grantedRootsFor`; tests `TestArchivedNameReuseDoesNotResurrectGrantedRoots` pass |
| `internal/sdd/status.go:340` | `readGrantedRoots` + `applyEditAuthorityBlock:473` (V2 no-op) + `engram_status.go:319` | 3,4 wiring | Edit-authority + topology insertion point |
| `internal/sdd/edit_authority.go:19/233` | `pathLikeTokens` / `detectUnauthorizedEditRoots` | 4,7 topology+readOnly | No `readOnlyMarkerAfterToken` nor `cross_common_dir` check |
| `internal/review/risk.go:165` | `ClassifyRisk` + `HighRiskChangedLines=400` + `isDocumentationPath` | 5 passive | Extension-only; gentle proves tier-0 by content (8 MiB cap, shebang, MDX, git grep) |
| `internal/sdd/research.go:39` | `HybridResearchEqual` + `EvaluateResearchHybrid:54` + `preproposal.go:IsPreproposalReady:42` | 6 hybrid | Already requires `rev>0 && revEqual && len>0 && lenEqual && bytesEqual` |
| `cmd/biggz/cli_sdd.go:667` | `GrantedRoots` + `sddAttemptRun` grant flags | 3 CLI | Explicit `ChangeInstance` param already covers containment |

## Gap-by-Gap Findings

### 1 — Ledger duplicated (`internal/sdd/attempt.go` vs `internal/sddattempt/`)
`attempt.go:12 AttemptState` persists `.attempt.json` with fixed budgets; `sddattempt.go:RuntimeStore` + `cas_store.go:Store` persists CAS `record-<sha>.json + HEAD + LOCK` under `git-common-dir`. Only legacy tests call `AttemptBegin`. No caller uses both.

**Options:**
- *Guard fail-closed (recommended):* make `AttemptBegin/Finish/Reset` return `ErrLegacyRetired` with pointer `run biggz sdd-attempt status/acquire/settle` — 15 LOC, no migration risk.
- *Hard delete:* cleaner but breaks any external script importing `internal/sdd` — unnecessary now.

**Decision:** guard fail-closed, not delete.

### 2 — Rescope AUDITED NARROWING (`sddattempt.go:1992`)
Current wedge only: `newMaxAttempts > cumAttempts && newMaxLines > cumLines` (line 2078). Gentle splits into `ErrRuntimeRescopeWidened` (new ≤ old → narrow only) and `ErrRuntimeRescopeExhausted` (new > cumulative wedge). Also guards `ActiveAttempt==nil && Objective!=nil && !DecisionRequired && !Complete && terminal && !drifted`.

**Options:**
- *A — Port verbatim (recommended):* allow only when active nil + terminal + not drifted/not decision-required/not complete; then check narrowing (`new <= old`), then wedge (`new > cum`); carry-forward `Cumulative*` unchanged. Stub drift as `false` initially (biggz has no `CandidateTree` yet) and document TODO; refuse when `lastAttempt.candidateTree == ""` (legacy empty).
- *B — Wedge-only:* zero change, but silently widens `5/5→7/800` as narrowing.

**Decision:** Option A, stub drift false with TODO.

### 3 — Grant scoped per change-instance (`cas_store.go:83`)
Containment proven (`TestArchivedNameReuseDoesNotResurrectGrantedRoots` + `TestRecreatedChangeNameDoesNotInheritGrantedRoots:232`). Gentle exposes `RuntimeStore.ForInstance(instance) RuntimeStore` sugar; biggz does same via explicit `ChangeInstance` param + `StatusWithInstance`.

**Options:** add `ForInstance(instance) (Store,error)` sugar on `cas_store.go:Store` that validates `1..128` trimmed single-line and stores `instance`, scoping `grantedRootsFor`; keep explicit param as canonical.

**Decision:** add sugar for parity, keep explicit param valid; share `validateChangeInstance`.

### 4 — Runtime topology guard (`cross_common_dir_runtime_target`)
Missing. Gentle `internal/sddstatus/edit_authority.go:160 applyRuntimeTopologyBlock` + `foreignRuntimeTopologyRoots:184` resolves each backticked path via `gitRootOf` → `OpenRuntimeStore` → `sameRuntimeCommonDirectory` (`os.SameFile` on `commonDir`), blocks `apply/verify/remediate` when target lives in different `git --git-common-dir`.

**Options:**
- *A — Port verbatim (recommended):* `resolveExistingPath → gitRootOf → OpenRuntimeStore → sameRuntimeCommonDirectory`, filtered by `editTargetTokens`; block only `apply/verify/remediate` via `applyRuntimeTopologyBlock` mutating `ApplyState/depencies/nextRecommended="resolve-blockers"`. Memoize per status call; add `context.Context` param.
- *B — Off:* zero lines, accepts cross-clone corruption.

**Decision:** Option A, 80 LOC lib + 30 CLI wiring.

### 5 — Classifier pasivo estricto (`internal/review/risk.go:165`)
`ClassifyRisk` uses `sensitiveDomainPath`, `executionConfigPath`, `documentationOnly` (extension check), then volume >400, then `triviallyInert`. Gentle `isPassiveDocumentContent` + `activePassiveContentPaths:214` + `processBoundaryRiskReasons:358` proves tier-0 by content: 8 MiB cap (`processBoundaryScanByteLimit = 8<<20`), `hasInterpreterDirective` (`#!` after optional BOM), `isStaticMDXDocument` (import/export, `<{}>`), and `git grep -I -l -z -i -E (^#!)|(subprocess|execute_process|exec)` for process-boundary escalation.

**Options:**
- *A — Full port* (>150 LOC, needs `SnapshotBuilder`, `treeBlobSizes`, `git grep`): exact parity, heavy, changes tier surface significantly.
- *B — Adapted bounded content check (recommended):* keep `RiskInput{Paths,ChangedLines,DiffSummary}` API; inside `triviallyInert` add `isPassiveContentFile(path)` reading ≤8 MiB, checking shebang, MDX, NUL/`utf8.Valid`, plus cheap `strings.Contains` for `subprocess/exec` as `process_boundary`; over-budget/unreadable → not passive (fail-closed). 50 LOC.

**Decision:** Option B, 50-70 LOC; gate behind `isPassiveDocumentExtension` (`.md/.mdx/.rst/.adoc/.png/.jpg/.gif`).

### 6 — Hybrid equal-bytes (`research.go:39`)
Already compliant: `HybridResearchEqual` requires `revA>0 && revB>0 && revA==revB && len>0 && bytes equal` and `EvaluateResearchHybrid` hybrid branch calls it. No change; add divergent-bytes fixture for coverage.

### 7 — Consultation (`readOnlyMarkerAfterToken`, `FinalVerifyAttestation`, `Handoff`)
- `readOnlyMarkerAfterToken` (`(?i)^\s*\(read-only\)`): **Port** — filter tokens where backticked span remainder matches marker; ergonomic escape for `` `../other/docs/api.md` (read-only)`` without widening authority (gentle #2934). 20 LOC.
- `FinalVerifyAttestation` (`verify-attestation` work-unit + `AttestedVerifyReportDigest`): **Discard consciously** — document `Won't port — review-free architecture` in proposal Alternatives. Requires review-authority + immutable candidate trees; no consumer in biggz.
- `Handoff` (`RuntimeHandoff`, `BeginWorktree/EffectiveWorktree`, removed-worktree admission): **Discard** — document `Won't port — single-worktree assumption`. No caller in biggz.

## Estimates & Chained-PR Plan

| Gap | Files | Tracked LOC |
|-----|-------|-------------|
| 1 ledger guard | `attempt.go` | 15 |
| 2 rescope narrowing+state | `sddattempt.go` | 55 |
| 3 ForInstance sugar | `cas_store.go` | 25 |
| 4 topology guard | `edit_authority.go` + `status.go` wiring | 85 |
| 5 passive adapted | `risk.go` | 55 |
| 7 readOnlyMarker | `edit_authority.go` | 20 |
| + docs | `proposal.md` Alternatives | 0 |
| **Total** | | **≈255 tracked** + ~180 test LOC (untracked) |

`git diff --stat HEAD` tracked <400 → single PR passes review budget single PR `auto-chain` with `stacked-to-main`. With tests counted conservative, still <500 <800. Alternative split (only if team prefers ultra-conservative): **Slice1 (security/correctness 160 LOC):** 1+2+3+4; **Slice2 (tier/ergonomics 95 LOC):** 5+7 + hybrid fixture. Slice1 ships first, Slice2 rebases.

## Go/No-Go
**Go — single slice (<400 tracked, <800 with tests) is sufficient.** Scope small, ROI high, deletion-free.

## Alternatives Considered (summary)
- Delete `attempt.go` vs guard — guard wins (no break).
- Rescope wedge-only vs verbatim narrowing — verbatim wins (prevents widening laundering).
- Full blob passive port vs adapted check — adapted wins (50 vs 150 LOC, no API break).
- Attestation/handoff port vs discard — discard wins (no worktree/review-authority in biggz).

## Risks & Mitigations
- Drift stub `false` makes rescope slightly more permissive — document TODO, refuse when `candidateTree==""`.
- `git rev-parse --git-common-dir` per token latency — memoize per status call; tests with `git init` temp dirs.
- Passive tier regression for `docs/large-docs-only.md` — gate behind extension allowlist; add `docs/with-shebang.md` fixture expecting escalation.
- Legacy `AttemptBegin` break — shim `ErrLegacyRetired` with migration pointer.
- Double API `ForInstance` vs `ChangeInstance` drift — share `validateChangeInstance` + test `ForInstanceAndChangeInstanceEquivalence`.

## Scope for Proposal
Next phase `sdd-propose` to sharpen:
1. Rescope narrow-only semantics + two sentinels + drift stub?
2. Confirm single-worktree → Handoff permanently out of scope?
3. Discard `FinalVerifyAttestation` — any downstream consumer needs digest?
4. Accept adapted passive 8 MiB cap vs full `git grep` blob proof?
5. Single PR (<400) vs two-slice chain given auto-chain 800?

## References
- `internal/sddattempt/sddattempt.go:1992-2135`
- `internal/sddattempt/cas_store.go:86/108/1728/306`
- `internal/sdd/status.go:340/473`, `internal/sdd/edit_authority.go:1-60`
- `internal/review/risk.go:165`, `internal/sdd/research.go:39`, `internal/sdd/preproposal.go:42`
- Gentle: `internal/sddstatus/runtime_ledger.go:108/160/184`, `internal/reviewtransaction/risk.go:178/214/358`, `edit_authority.go:19`
