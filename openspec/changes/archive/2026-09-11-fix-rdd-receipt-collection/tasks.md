# Tasks: fix-rdd-receipt-collection

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1,900–2,400; largest slice ≤~420 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | S1a→S1a1+S1a2 and S1b→S1b1+S1b2 (both exceed 400). Order S1a1→S1a2→S1b1→S1b2→S1c→S1d→S2 |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain (PR1→tracker; child→previous; tracker→master) |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

CI complexity gate (`internal/review`/`internal/sdd`, non-test): cyclomatic ≤15, cognitive ≤20 — small helpers only.

### Suggested Work Units

| Unit | Goal | Likely PR | Base branch | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|-------------|----------------------|-----------------|-------------------|
| S1a1 | Inspector + hunk derivation (`internal/review/frozen_inspector.go`) | PR 1 | tracker | `go test ./internal/review -run TestFrozenInspector` | temp-repo: frozen trees, isolated GIT_DIR, foreign cwd, dirty index, binary+`.md` | revert; placeholder restored |
| S1a2 | Materializer + `--materialize` (`internal/review/materialize.go`, `cmd/biggz/cli_review.go`) | PR 2 | PR 1 | `go test ./internal/review -run TestMaterialize` | temp lineage: non-empty, identical ×2, foreign cwd; 4 MiB refusal | revert; additive flag |
| S1b1 | Identity + start canonicalization (`internal/review/lineage_identity.go`) | PR 3 | PR 2 | `go test ./internal/review -run TestLineageIdentity` | abbrev subject→full SHA persisted; unresolvable rejects | revert; read-path only |
| S1b2 | Resolution + gate (`internal/review/lineage_resolve.go`, `internal/sdd/verify.go`) | PR 4 | PR 3 | `go test ./internal/review -run TestLineageResolve` | start→gate one lineage; legacy abbrev readable | revert; gate fail-closed |
| S1c | Surfacing + parity guard (`internal/review/producers.go`, `internal/sdd/status.go`, `engram_status.go`) | PR 5 | PR 4 | `go test ./internal/sdd -run TestRDDParity` | status fixture: obligation+producer; offer≡gate | revert; surfacing additive |
| S1d | Verify-side `review-subject.json` writer (`internal/assets/skills/sdd-verify/SKILL.md` + marker test) | PR 6 | PR 5 | `go test ./internal/assets -run TestSDDVerifySubjectWriter` | offered subject invocation runs | revert; asset-only |
| S2 | Plugin verbatim transport + overlays + contract doc (`internal/assets/opencode/`) | PR 7 | PR 6 | `go test ./internal/assets` | OpenCode host: bytes→tool-less reviewer→capture | revert; transport only |
| close | Dogfood; retro-collect+archive `fix-bigmem-recall-friction`; close #60 | tracker | tracker (post PR 7) | `go test ./... -count=1` | both receipts: materialize→reviewer→capture→finalize→gate allow | untracked receipts; reopen #60 |

## Phase 1: Inspector + hunks (S1a1)

- [x] 1.1 RED: `frozen_inspector_test.go` — executable `.md` + binary always materialize; `--text` forbidden
- [x] 1.2 RED: foreign cwd → byte-identical via `-C` + isolated `GIT_DIR`; unresolvable → typed refusal
- [x] 1.3 RED: dirty/staged never leak — frozen-tree bytes byte-identical
- [x] 1.4 GREEN: `internal/review/frozen_inspector.go` — repo resolved once; `--patch --full-index --no-ext-diff --no-textconv --unified=3`
- [x] 1.5 GREEN: `deriveLensHunks` (`cmd/biggz/cli_review.go:48`) → inspector hunks → `lens.NewLensInput`; parser-error candidate → non-empty findings
- [x] 1.6 Verify: `go test ./internal/review ./cmd/biggz`

## Phase 2: Materializer + `--materialize` (S1a2)

- [x] 2.1 RED: `materialize_test.go` — binding, context, name-status, numstat, per-path `GENTLE_AI_REVIEW_PATCH` delimiters
- [x] 2.2 RED: >4 MiB → typed cap refusal (no truncation); empty patch on content-changing path → `materialize_vacuous`
- [x] 2.3 GREEN: `internal/review/materialize.go` — marker composition, read-only
- [x] 2.4 GREEN: `capture-result --materialize` prints exactly bytes, captures nothing; exclusive with `--input`/`--preflight`
- [x] 2.5 Verify: chain events + receipts unchanged; bytes deterministic, byte-identical ×2 + foreign cwd

## Phase 3: Identity + start canonicalization (S1b1)

- [x] 3.1 RED: `lineage_identity_test.go` — derivation deterministic; abbrev → full SHA persisted; unresolvable → typed reject
- [x] 3.2 GREEN: `internal/review/lineage_identity.go` — `CanonicalSubjectSHA`, `DeriveLineageID`
- [x] 3.3 GREEN: `review start` canonicalizes `CommitSHA`; defaults to derived id; `go test ./internal/review -run TestLineageIdentity`

## Phase 4: Resolution + gate (S1b2)

- [x] 4.1 RED: `lineage_resolve_test.go` — derived-first; legacy abbreviated readable (read-only scan, no rewrite); none → typed refusal
- [x] 4.2 GREEN: `internal/review/lineage_resolve.go` — `ResolveCandidateLineage`
- [x] 4.3 GREEN: `internal/sdd/verify.go` — resolve `HEAD^{commit}` before gate; replaces bare-change lookup; gate tests

## Phase 5: Surfacing + parity guard (S1c)

- [x] 5.1 GREEN: `internal/review/producers.go` — `SupportedReviewHosts`, `BlockingReviewSurfaces()`, `ProducerManifest()`
- [x] 5.2 RED: `rdd_parity_test.go` — missing producer fails guard; coverage passes; plugin wires capture
- [x] 5.3 GREEN: `internal/sdd/{status.go,engram_status.go}` — offer without lineage id (`pathquote.Quote`); obligation+producer in `blockedReasons`; `nextRecommended` untouched
- [x] 5.4 GREEN: refusals name exact producer command; unproducible → `rdd_unproducible`
- [x] 5.5 Verify: offer ≡ gate ≡ receipt one lineage; obligation clears (receipt/disabled); `go test ./internal/review ./internal/sdd`

## Phase 6: Verify-side subject writer (S1d)

- [x] 6.1 `sdd-verify/SKILL.md`: verify writes `<changeRoot>/review-subject.json` (`{"repository","commit_sha":"HEAD"}`); `sdd-status` stays read-only — writer cannot live there
- [x] 6.2 Marker test `sdd_verify_writer_marker_test.go` proves instruction present; `go test ./internal/assets -run TestSDDVerifySubjectWriter`

## Phase 7: OpenCode plugin + overlays + doc (S2)

- [x] 7.1 `opencode/plugins/review-result-artifacts.ts` — verbatim transport; tool-less reviewer; caller prompt discarded
- [x] 7.2 `opencode/sdd-overlay-{single,multi}.json` — review step uses `--materialize`
- [x] 7.3 `skills/_shared/review-ledger-contract.md` — materialize route documented
- [x] 7.4 Extend `review_plugin_contract_test.go` markers; `go test ./internal/assets`

## Phase 8: Dogfooding + downstream close

- [x] 8.1 Own receipt: start → `--materialize` → reviewer → `--input` → finalize → `review gate` allows — candidate `872a1acf` (tier medium, lens `risk`); consent relayed losslessly and granted; `capture-result --materialize` (12,818 bytes) → reviewer in a fresh locked-down pi process (`--print --no-session --no-tools --no-extensions …`, PiAdapter shape) → `--input` admission `completed` (`result_hash sha256:f0803b74…`) → `finalize` receipt `sha256:5734126e…` → `review gate post-apply` `{"allowed":true,"delivery":"burned/unmanaged"}`
- [x] 8.2 Retro-collect `fix-bigmem-recall-friction`: full-SHA subject → materialize → capture → finalize — satisfied by the finalized receipt on HEAD (`review-d2d540998a64beba`, receipt `sha256:7c0320f1…`): the RDD gate resolves `HEAD^{commit}`, so the obligation cleared and the dispatcher flipped `verify: blocked → all_done` with `sync`/`archive: ready`
- [x] 8.3 `sdd-sync` then `sdd-archive` `fix-bigmem-recall-friction` with RDD enabled — sync with `allow-destructive` approved for domains `bigmem`/`orchestrator` (bigmem 50→53 reqs, +13 scenarios, +92/−10; orchestrator +3 scenarios, +18/−4; all 27 delta scenarios matched verbatim); archive moved to `openspec/changes/archive/2026-09-11-fix-bigmem-recall-friction/` (9 renames + `archive-report.md`), tasks 21/21, RDD gate `allowed:true`, `session-close --check-only` `{"verified":true}`
- [x] 8.4 Comment issue #60 with reference mapping; close once verified — commented with the defect → fix → PR mapping (PRs #63, #65–#72) plus the dogfood evidence (`issues/60#issuecomment-5642454702`) and closed `biggs-100/biggz-ai#60` after the archive landed and was verified (state `CLOSED`)
- [x] 8.5 Fix every defect surfaced, each with covering test — lens-less finalize defect: a documentation-only candidate (tier low) froze an empty lens selection and `review finalize` refused with `finalize: no captured lens slots; nothing to finalize`, dead-ending the RDD gate behind a command that could not succeed; `validateFinalizeSelection` now accepts the empty frozen selection + zero captures case, persists a valid slot-less receipt (empty selection, frozen candidate manifest, validated chain — no invented lens, no fabricated approval) and the gate resolves. Covered by `TestFinalize_AcceptsLensLessReview` (end-to-end over a real temp repo: low/no lenses → finalize → receipt validates → gate allowed; durable + production-burn paths) and the medium-tier regression `TestFinalize_MediumTierStillRequiresItsLens` — **plus a second defect surfaced by the verify phase**: `CanonicalSubjectSHA` had become stricter than the design and rejected the legacy organic subject (no `commit_sha`) with `unresolvable_subject_commit: the subject commit is empty`, breaking `e2e/TestOrganicReviewStart` in the full-repository suite; an absent/blank subject commit now binds to `HEAD` (the documented legacy contract in `DeriveRiskInput`) and only non-empty unresolvable values stay refused typed. Covered by `TestLineageIdentityAbsentSubjectBindsToCurrentHead` (absent → full HEAD SHA, stable derived id, id follows HEAD, non-empty unresolvable still refused) and by the previously failing `e2e/TestOrganicReviewStart`
