# Archive Report: fix-rdd-receipt-collection

**Change**: fix-rdd-receipt-collection
**Archived**: 2026-09-11 → `openspec/changes/archive/2026-09-11-fix-rdd-receipt-collection/`
**Store mode**: openspec (filesystem artifacts; key phases also mirrored to BigMem under `sdd/fix-rdd-receipt-collection/*` per project practice)
**Verdict at close**: PASS WITH WARNINGS — 33/33 tasks, 0 CRITICAL, 0 blockers, 12/12 requirements, 30/30 scenarios. The dogfood receipt was collected end-to-end on the new surface (lineage `review-4544f89b9074a985`, receipt `sha256:10e3b269…`, post-apply gate `allowed: true`); archive was reached with RDD enabled and no kill switch. Branch `fix/rdd-receipt-collection-9-close` (HEAD `22e691ea`); chained PRs #63 (merged → tracker `fix/rdd-receipt-collection`) and #65–#72 (open, stacked; #72 = this branch); issue #60 commented with the defect → fix → PR mapping and closed after the dogfood landed.

## Final State (terminal record — outranks intermediate snapshots)

Issue #60 ("RDD receipts are unattainable on Pi — collector is OpenCode-only, gate and offer disagree on the lineage id, review start keeps an abbreviated subject SHA") is fixed end-to-end:

| # | Defect (proposal / issue #60) | Fix | Proof |
|---|-------------------------------|-----|-------|
| 1 | Gate looked up the bare change name; offer lineage ≠ gate lookup — the receipt was never found at archive | Single lineage identity derivation (`internal/review/lineage_identity.go`) + `ResolveCandidateLineage(repo, HEAD^{commit})` in the verify preflight (`internal/sdd/verify.go`): derived-first, read-only legacy scan, typed refusal | `TestReviewOffer` (no `--lineage`), `TestVerifyRDDResolve*`, S1d harness (offer ≡ gate ≡ receipt); live dogfood gate `allowed:true` |
| 2 | No Go materialization: CLI hunk derivation was a placeholder (vacuous receipts) | Frozen-tree inspector + materializer (`internal/review/frozen_inspector.go`, `materialize.go`) + `capture-result --materialize` printing the full reviewer task (binding/context/name-status/numstat/per-path patches); 4 MiB refusal-not-truncation; non-vacuity refusals | `materialize_test.go`, `frozen_inspector_test.go`; dogfood forwarded 12,818 bytes verbatim |
| 3 | `review start` kept an abbreviated subject SHA — abbreviated subjects never captured | Start canonicalizes via `rev-parse <raw>^{commit}`; absent/blank organic subjects bind to `HEAD` (S5) | `TestLineageIdentityCanonicalSubjectSHA`, `TestLineageIdentityAbsentSubjectBindsToCurrentHead`, `e2e/TestOrganicReviewStart` |
| 4 | Obligation invisible until archive; kill switch the only escape | Typed actionable refusals + `blockedReasons`/phase instructions naming the exact producer command; producer manifest + parity guard (`internal/review/producers.go`; 6 surfaces × 2 hosts) | `TestRDDParitySurfacing`, `rdd_parity_test.go`, `TestReviewRefute_StandsFindingBlocksGate` (exit-1 path) |
| 5 | Collection OpenCode-only | OpenCode plugin transports the materialized bytes verbatim to a tool-less reviewer; overlays deny all tools (S2); Pi production relay explicitly deferred (W-1) | `review_plugin_contract_test.go` markers; S2 harness hash-in == hash-out, `caller_body_discarded=true` |
| 6 | Dogfood-surfaced: a lens-less candidate dead-ended `review finalize` behind the same RDD gate | Finalize accepts an empty frozen selection and persists a valid slot-less receipt (no invented lens, no fabricated approval) | `TestFinalize_AcceptsLensLessReview` + medium-tier regression `TestFinalize_MediumTierStillRequiresItsLens` |

- **Dogfood evidence (apply-progress S4/S5)**: 8.1 own receipt end-to-end — start on candidate `872a1acf` (tier medium, lens `risk`, lineage `review-b22910da23db328f`), `--materialize` 12,818 bytes, reviewer in a fresh locked-down pi process (`--print --mode text --no-session --no-tools --no-extensions …`, `PiAdapter` shape), `--input` admission `completed` (`result_hash sha256:f0803b74…`), finalize receipt `sha256:5734126e…`, gate `allowed:true` (`burned/unmanaged`). 8.2 retro-collect — the finalized HEAD receipt (`review-d2d540998a64beba`, receipt `sha256:7c0320f1…`) cleared `fix-bigmem-recall-friction` (verify `blocked → all_done`). 8.3 that change's destructive sync + archive executed with RDD active (sibling now at `openspec/changes/archive/2026-09-11-fix-bigmem-recall-friction/`). 8.4 issue #60 commented with the reference mapping (PRs #63, #65–#72) and closed. S5 — the verify-surfaced organic-subject fix (previously failing `e2e/TestOrganicReviewStart` now PASS, plus durable unit coverage).
- **Final verification (full change)**: `pass_with_warnings` — 12/12 requirements, 30/30 scenarios, 0 blockers, 0 CRITICAL; `evidence_revision = sha256:3ce4661a340852ad9f37a833ac78ade5013db07b4a4e0a658d77d020b3c7b92a`; whole-repository suite green at `8c21d4b6` (three bounded segments), focused S5 runs, node gates (`check-provider-contract`, `verify-package-files`, `check-skill-lint`) exit 0.
- **Recorded workload deviations**: slices S1a1 (1031), S1a2 (979), S1b1 (640), S1b2 (803), S1c (543) exceed the 400-line review budget; each carries an accepted `size:exception` recorded in apply-progress (forecast: `400-line budget risk: High`, `Chained PRs recommended: Yes`, chain strategy feature-branch-chain).
- **Warnings at close (non-blocking, carried)**: W-1 — no production `biggz-pi`/`gentle-pi` relay in this runtime (`CurrentProducerHost()` reports the ambient `opencode` host under pi; the 8.1 dogfood relay was orchestrator-driven; deferred by the approved scope). W-2 — pre-canonicalization lineages are readable but not collectable (spec-conformant migration caveat).
- **Snapshot attribution**: `verify-report.md` was authored against HEAD `8c21d4b6`; its own dispatcher snapshot ("sync: ready", `nextRecommended: sync`) has since expired — sync completed after it — while its "done" claims remain true. The archived HEAD `22e691ea` is a docs-only successor that adds the verify report itself. No unrankable contradictions were found.
- **Cycle context**: this run moved only change artifacts (`git mv`) — no archive-time code changes; the pending sync edits, the move, and this report remain uncommitted for orchestrator review (delegation constraint).

## Gates at close (native dev build `./biggz.exe`; RDD enabled)

| Gate | Result | Evidence |
|------|--------|----------|
| Task Completion | ✅ PASS — 33/33 checked, 0 unchecked; no reconciliation needed | persisted `tasks.md` (`- [x]` 33 / `- [ ]` 0); dispatcher `taskProgress.allComplete: true` |
| CRITICAL issues | ✅ PASS — 0 CRITICAL / 0 blockers | `verify-report.md` (PASS WITH WARNINGS; 2 WARNING, 4 SUGGESTION) |
| Native review receipt (RDD) | ✅ PASS — post-apply `allowed: true`, `delivery: "burned/unmanaged"` ("receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"); chain valid | `review gate post-apply review-4544f89b9074a985 --json`; `review status` → `chain_valid: true`, `integrity_verdict.valid: true` |
| Session-summary guard | ✅ PASS — `{"verified":true,"reason":"","fallback":""}` | `session-close --check-only --json` |
| Native dispatcher (final authority) | ✅ at archive time: `archive: ready`, `apply/verify/sync: all_done`, `nextRecommended: archive`, zero blockers | `sdd-status --json` (no `blockedReasons` key — empty list, `omitempty`); `sdd-continue fix-rdd-receipt-collection` → "Next phase: archive" |
| Action-context guard | ✅ repo-local; archive edits confined to `openspec/changes/**` | `actionContext.allowedEditRoots: [repo root]`, mode `repo-local` |

**Review receipt trail** (governing lineage for the verified candidate `8c21d4b6`): `review-4544f89b9074a985` — `start_review → in_review → complete_review → burn_review` (2026-09-11T21:14:09→10 local; head revision `d93864c9…`); receipt `sha256:10e3b2697faf73c22d6f9487fdb697a8cc8c38ef8f752f0f69046f2f4388e754`, burned per the ephemeral lifecycle (`burned.json` records the same hash); budget counters `fix_rounds: 0`, `scoped_validations: 0`; correction budget 36/72 (1 attempt, risk tier low). The archived HEAD `22e691ea` (docs-only successor) resolves through the store's legacy organic `HEAD`-subject transaction `01a092ef-…` (chain valid; burned → gate `allowed:true`) — no new lineage was required and nothing was rewritten. Earlier dogfood lineages remain untouched in the store: `review-b22910da23db328f` (S3 candidate `872a1acf`), `review-d2d540998a64beba` (sibling retro-collect candidate `4f6646cf`), `review-7251569bfa504050` (candidate `8a9c4a2f`).

## Specs synced (Step 2 — applied by the native sync phase; verified here, not re-applied)

| Domain | Delta action | Main spec | Numbers |
|--------|-------------|-----------|---------|
| rdd | 2 MODIFIED — Gate Blocking Semantics (2 sc), REQ-RDD-002 (5 sc) | `openspec/specs/rdd/spec.md` | +19/−3 |
| review | 5 ADDED — materialization, fidelity/cap, non-vacuity, transport, parity guard (10 sc) | `openspec/specs/review/spec.md` | +80/−0 |
| review-authority | 2 ADDED — single lineage identity, subject canonicalization (5 sc) | `openspec/specs/review-authority/spec.md` | +38/−0 |
| sdd | 1 MODIFIED — ReviewOffer Post-Verify Wiring (3 sc); 1 ADDED — Pre-Publication Review Obligation (2 sc) | `openspec/specs/sdd/spec.md` | +23/−3 |
| sdd-status | 1 ADDED — Outstanding Review Obligation Projection (3 sc) | `openspec/specs/sdd-status/spec.md` | +22/−0 |

**Totals**: 9 requirements added, 3 modified, 0 removed; +182/−6. **Verification**: all 12 delta requirements and all 30 delta scenarios matched verbatim in the main specs; requirements outside the deltas untouched; the 6 deletions are confined to the three modified requirement bodies (replaced text plus `(Previously: …)` annotations); no REMOVED/RENAMED sections; no destructive merges. Archive made **no changes** to `openspec/specs/**` — the sync was already `all_done` before archive started (edits stay uncommitted for orchestrator review).

## Archived contents

- `proposal.md`, `specs/{rdd,review,review-authority,sdd,sdd-status}/spec.md` (deltas as authored), `design.md` ✅
- `tasks.md` (33/33), `apply-progress.md` (batches S1a1…S5 including the S4 dogfood close and the S5 verify-surfaced fix), `verify-report.md` (PASS WITH WARNINGS, 0 CRITICAL) ✅
- `state.yaml` ✅ — retained verbatim; carries the historical S2 `pending_question` checkpoint envelope (answered: "Continue with Phase 8")
- `review-subject.json`, `review-subject-s2.json`, `review-subject-s3.json` ✅ (dogfood subject files, carried as untracked)
- `archive-report.md` ✅ (this file; also mirrored to BigMem as `sdd/fix-rdd-receipt-collection/archive-report`)
- No `_meta.yaml` (native-era change; none existed). No `.biggz-instance` present in the change folder; none carried.

## Post-archive hygiene (Step 3b)

- Non-TTY (sub-agent) execution → deleted nothing (contract: non-TTY path deletes nothing and exits 0).
- Candidates check: all 10 local `fix/rdd-receipt-collection*` branches still exist on `origin` (no `[gone]` upstreams) → no prune candidates regardless; a single worktree (repo root). No `fetch --prune` was run (nothing to prune; mirrors the no-candidates state).

## Follow-ups for future work (not blockers)

- W-1: add the production `biggz-pi`/`gentle-pi` host relay (`BIGGZ_PI_REVIEW_RELAY_CONTRACT`) so `CurrentProducerHost()` reports pi and hosts drive the relay.
- W-2: pre-canonicalization lineages stay readable but cannot be captured/finalized — abandon-in-place or document the upgrade path.
- S-1: record the `use-modern-go list` consultation line in the S1b2/S5 batch notes (process hygiene).
- S-2: replace the stale Sep-7 `biggz` on PATH (predates `--materialize`); use `./biggz.exe` until then.
- S-3: `deriveNextTransition` does not offer `finalize` for unfinalized lens-less lineages (`review status --next-transition` errors); `finalize` itself is unblocked.
- S-4: add a hint in the `MODIFIED requirement … not found` error naming the retitle-by-old-name rule.
- Orchestrator: commit the sync edits + this archive move + report; then proceed with the chain delivery.

## Evidence refs (raw)

- `./biggz.exe sdd-status --json` → `dependencies {apply: all_done, verify: all_done, sync: all_done, archive: ready}`; `taskProgress 33/33`; `nextRecommended: "archive"`; no `blockedReasons` key (empty with `omitempty`)
- `./biggz.exe sdd-continue fix-rdd-receipt-collection` → `Next phase: archive`
- `./biggz.exe rdd status` → `RDD Status: enabled` (global enabled, source default)
- `./biggz.exe review gate post-apply review-4544f89b9074a985 --json` → `{"allowed":true,"delivery":"burned/unmanaged","reason":"review burned: receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"}`
- `./biggz.exe review status review-4544f89b9074a985 --json` → `chain_valid: true`, `receipt.binding_hash sha256:b906fc81…`, `receipt_artifact.hash sha256:10e3b269…`, budget counters `{fix_rounds: 0, scoped_validations: 0}`, correction budget `36/72`, `risk_tier: low`
- `./biggz.exe session-close --check-only --json` → `{"verified":true,"reason":"","fallback":""}`
- Verify: `verify-report.md` `evidence_revision sha256:3ce4661a…`; `go build ./...` / `go vet ./...` exit 0; suite segments ok; node gates exit 0
- Delivery: `gh pr list --repo biggs-100/biggz-ai` → #63 MERGED (→ tracker), #65–#72 OPEN stacked (#72 = `fix/rdd-receipt-collection-9-close`); issue #60 `CLOSED` (2026-09-12T01:13:05Z)
- Move: `git mv openspec/changes/fix-rdd-receipt-collection openspec/changes/archive/2026-09-11-fix-rdd-receipt-collection` → 10 staged renames (`R`), `git diff --cached --stat` = 0 insertions / 0 deletions; 4 untracked files carried (3 review subjects + `state.yaml`); pre/post MD5 of all 14 files identical
- BigMem: `sdd/fix-rdd-receipt-collection/archive-report` saved via `./biggz.exe bigmem save … --type architecture --topic-key sdd/fix-rdd-receipt-collection/archive-report` — the marker the merged dispatcher view reads to report the change as archived
