# Exploration — rdd-auto-enabled-post-verify

## Context
User requests: 1) biggz must do RDD automatically when blocked, 2) must be enabled by default, 3) biggz-ai must NOT disable RDD unless human asks. Verified gentle-ai is DISABLED by default opt-in (rdd_mode.go:681-712), reviewOffer post-verify only offers, SDD never disables. For biggz we need to invert: enabled by default + auto-run post-verify + never auto-disable. Current biggz-ai: RDD enabled (Global enabled, Clone empty, effective enabled after `rdd enable`), hook pre-push fixed for space bug but still picks first lineage alphabetically (019fbb3a smoke test Jul 31), 3 ghost lineages remain, last SDD did `rdd disable --scope=clone` as workaround (now re-enabled).

## Current Architecture & Files

| File | Lines | Gap |
|------|-------|-----|
| `internal/review/rdd.go:280-322` | effective := Enabled default | Default ON already correct, must not change to OFF |
| `internal/review/rdd.go:360-373` RDDEnable | clearing generations | Already correct |
| `internal/review/rdd.go:380-600` RDDDisableWithRevision | CAS+LOCK off-only | Off-only correct |
| `internal/review/gate.go:46-62,642-680` | 5 gates post-apply/pre-commit/pre-push/pre-pr/release | Sound, no change except hook input |
| `cmd/biggz/cli_rdd.go:19-128` | enable/disable/status --scope | Correct |
| `internal/sdd/status.go:177,523` + `engram_status.go:246,342` | ReviewOfferBlock{Available,Invocation} but hardcoded nil | Stub, needs wiring to emit when verify done+passing && RDD enabled |
| `internal/sdd/status_v2.go:48-53` | ReviewOffer allowlist | Must stay minimal, no lineage |
| `internal/sdd/archive.go:12-40` | os.Rename only | Must keep never auto-disable invariant |
| `.git/hooks/pre-push:8-28` | for d in ...; break | Naive picks first lineage alphabetically (ghost 019fbb3a), needs lineage-aware |
| `internal/install/install.go:410-560` ensureRDDEnabled | clears stale clone/worktree and writes ~/.biggz/rdd-mode.json enabled | Defense-in-depth, keep |

## Gaps
- ReviewOffer stub: sdd-status never emits reviewOffer even when verifyReport done + RDD enabled (contract sdd-status-contract.md:184-227)
- Hook lineage naive: picks smoke test lineage not tied to HEAD
- Ghost lineages 019fbb3a-* confuse `biggz review list`
- Previous SDD workaround disabled clone (orchestrator guard prohibits)

## Approaches

### 1. Default ON core + Offer-only + Hook fix + Ghost cleanup (minimal)
- Keep rdd.go default ON, keep ReviewOffer offer-only, fix hook to most-recent lineage (ls -t + ancestor check), manual ghost rm
- Pros: Low risk, gentle parity, no auto-run loops, <400
- Cons: Does NOT satisfy user auto-run when blocked (still manual `review start`)
- Effort: ~80+40 LOC

### 2. Installer auto-enable only + Auto-run eager + Hook fix
- Rely on install.ensureRDDEnabled only, make deriveChangeStatus auto-run `review start` when verify done+passing
- Pros: Satisfies verbatim, reversible
- Cons: Fresh clone without install disabled, couples SDD to review, violates sdd-status-contract Planning and apply never auto-launch 4R, fails without install
- Effort: ~120 LOC + risk

### 3. Hybrid (default ON code + installer defense + Offer that auto-runs on block, parameterized) — RECOMMENDED
- Keep core default ON + ensureRDDEnabled fallback. Wire ReviewOffer properly: applyState==all_done && verifyReport==done && verify Passing && RDDStatus==enabled → ReviewOffer{Available:true, Invocation:"biggz review start --lineage <change>-<shortsha>"}. Orchestrator auto-runs only when gate would block (unresolved findings / pre-push denial) and delivery_strategy auto-chain, otherwise just surfaces offer. Fix hook lineage-aware: ls -t + git merge-base --is-ancestor check or biggz review list max. Ghost cleanup documented manual rm -rf .git/biggz/review-transactions/019fbb3a-* (verify payload contains Temp before delete). Archive must never auto-disable (guard comment/test).
- Pros: Satisfies all 3 wants without breaking contract, default ON code truth, never auto-disable enforced, auto-run opt-in via auto-chain, hook eliminates phantom
- Cons: Slightly more LOC, needs orchestrator doc update
- Effort: ~180-260 LOC (status 40 + hook 30 + docs 40 + tests 100), single PR stacked-to-main Low

## Recommendation
GO with Approach 3 Hybrid. Core default ON already correct, gentle offer-only contract preserved, biggz product auto-run on block via auto-chain, hook lineage fix mandatory, ghosts cleaned via documented manual rm.

## Estimation
180-260 prod <400 single PR stacked-to-main Low. SDD touches hook template via install.

## Risks
- Auto-run without ancestor check replays stale lineage → lineage-aware mandatory
- RDDEnable clearing all overrides could surprise user who intentionally disabled globally before install → mitigate with warning
- ReviewOffer invocation quoting → use pathquote.Quote
- Ghost rm deleting valid lineage → limit to exact 019fbb3a-* with Temp payload
- Status ReviewOffer must stay allowlist, not add lineage

## Go/No-Go
GO — preflight interactive/openspec/auto-chain/800 cached, sdd-init DONE. Confirm auto-run should be on block only (not every verify PASS) to avoid loops.

## References
- internal/review/rdd.go:280-322,360-373,380-600
- internal/review/gate.go:46-62,642-680
- cmd/biggz/cli_rdd.go:19-128
- internal/sdd/status.go:177,523
- internal/sdd/status_v2.go:48-53
- internal/sdd/archive.go:12-40
- .git/hooks/pre-push:8-28
- internal/install/install.go:410-560
- gentle internal/reviewtransaction/rdd_mode.go:681-712
- gentle internal/sddstatus/status.go:209-219
