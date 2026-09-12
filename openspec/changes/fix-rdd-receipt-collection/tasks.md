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

- [ ] 3.1 RED: `lineage_identity_test.go` — derivation deterministic; abbrev → full SHA persisted; unresolvable → typed reject
- [ ] 3.2 GREEN: `internal/review/lineage_identity.go` — `CanonicalSubjectSHA`, `DeriveLineageID`
- [ ] 3.3 GREEN: `review start` canonicalizes `CommitSHA`; defaults to derived id; `go test ./internal/review -run TestLineageIdentity`

## Phase 4: Resolution + gate (S1b2)

- [ ] 4.1 RED: `lineage_resolve_test.go` — derived-first; legacy abbreviated readable (read-only scan, no rewrite); none → typed refusal
- [ ] 4.2 GREEN: `internal/review/lineage_resolve.go` — `ResolveCandidateLineage`
- [ ] 4.3 GREEN: `internal/sdd/verify.go` — resolve `HEAD^{commit}` before gate; replaces bare-change lookup; gate tests

## Phase 5: Surfacing + parity guard (S1c)

- [ ] 5.1 GREEN: `internal/review/producers.go` — `SupportedReviewHosts`, `BlockingReviewSurfaces()`, `ProducerManifest()`
- [ ] 5.2 RED: `rdd_parity_test.go` — missing producer fails guard; coverage passes; plugin wires capture
- [ ] 5.3 GREEN: `internal/sdd/{status.go,engram_status.go}` — offer without lineage id (`pathquote.Quote`); obligation+producer in `blockedReasons`; `nextRecommended` untouched
- [ ] 5.4 GREEN: refusals name exact producer command; unproducible → `rdd_unproducible`
- [ ] 5.5 Verify: offer ≡ gate ≡ receipt one lineage; obligation clears (receipt/disabled); `go test ./internal/review ./internal/sdd`

## Phase 6: Verify-side subject writer (S1d)

- [ ] 6.1 `sdd-verify/SKILL.md`: verify writes `<changeRoot>/review-subject.json` (`{"repository","commit_sha":"HEAD"}`); `sdd-status` stays read-only — writer cannot live there
- [ ] 6.2 Marker test `sdd_verify_writer_marker_test.go` proves instruction present; `go test ./internal/assets -run TestSDDVerifySubjectWriter`

## Phase 7: OpenCode plugin + overlays + doc (S2)

- [ ] 7.1 `opencode/plugins/review-result-artifacts.ts` — verbatim transport; tool-less reviewer; caller prompt discarded
- [ ] 7.2 `opencode/sdd-overlay-{single,multi}.json` — review step uses `--materialize`
- [ ] 7.3 `skills/_shared/review-ledger-contract.md` — materialize route documented
- [ ] 7.4 Extend `review_plugin_contract_test.go` markers; `go test ./internal/assets`

## Phase 8: Dogfooding + downstream close

- [ ] 8.1 Own receipt: start → `--materialize` → reviewer → `--input` → finalize → `review gate` allows
- [ ] 8.2 Retro-collect `fix-bigmem-recall-friction`: full-SHA subject → materialize → capture → finalize
- [ ] 8.3 `sdd-sync` then `sdd-archive` `fix-bigmem-recall-friction` with RDD enabled
- [ ] 8.4 Comment issue #60 with reference mapping; close once verified
- [ ] 8.5 Fix every defect surfaced, each with covering test
