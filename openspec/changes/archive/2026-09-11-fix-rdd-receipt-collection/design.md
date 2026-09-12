# Design: fix-rdd-receipt-collection

## Technical Approach

Port gentle-ai's verified shape (`review/rdd-receipt-collection-reference-map`): Go materializes the reviewer task from frozen trees (read-only, 4 MiB refusal-not-truncation), hosts transport bytes verbatim to a tool-less reviewer, and one derivation `review-<sha256(worktree+target)[:16]>` serves start, offer and gate. Legacy lineages stay readable read-path-only; events are never rewritten.

## Architecture Decisions

**D1 — Single lineage identity (highest risk).** `internal/review/lineage_identity.go` derives `review-<hex(sha256("biggz-ai.review-start-lineage/v1\x00"+json{worktree_identity,target_identity}))[:16]>`; worktree identity = canonical git common dir (store scope), target = full SHA via `rev-parse <raw>^{commit}`. `review start` canonicalizes `subject.CommitSHA`, rejects unresolvables (persists nothing), defaults to the derived id. Offer: `biggz review start --subject '<changeRoot>/review-subject.json'` (pathquote.Quote), no id. Gate (`verify.go`) calls `review.ResolveCandidateLineage(repo, HEAD^{commit})`: derived id if present; else read-only newest-first scan of `.git/biggz/review-transactions/*` matching the rev-parsed genesis subject (legacy abbreviated/UUIDv7 readable — never rewritten); else fail-closed typed refusal. Rejected: bare change name (bug #60), offer-embedded id.

**D2 — Materializer.** `internal/review/materialize.go` + `frozen_inspector.go` compose: `GENTLE_AI_REVIEW_BINDING {json}`, `GENTLE_AI_REVIEW_CONTEXT {preflight json}`, `GENTLE_AI_REVIEW_NAME_STATUS`, `GENTLE_AI_REVIEW_NUMSTAT`, then per path `GENTLE_AI_REVIEW_PATCH <path>` + frozen-tree diff bytes (canonical `--patch --full-index --no-ext-diff --no-textconv --unified=3 <baseTree> <candidateTree> -- :(literal)<path>` flags, mirroring `frozen_candidate_context.go:355-400`), through an isolated temp GIT_DIR (objects via `GIT_OBJECT_DIRECTORY`, config/attributes neutralized), binary-safe (`--text` forbidden on patch). Whole-task cap `ArtifactResultLimit` (4 MiB); exceeding → typed refusal naming the cap. `capture-result --materialize` prints exactly those bytes, captures nothing; exclusive with `--input`/`--preflight` (unchanged). S2 hosts transport these bytes verbatim, replacing binding+context injection; the `deriveLensHunks` placeholder (cli_review.go:48) is replaced by inspector-backed hunks feeding `lens.NewLensInput`. Small helpers for the CI 15/20 complexity gate.

**D3 — Non-vacuity.** Fixture test: parser-error Go candidate → non-empty readability findings over materialized hunks; empty manifest or empty patch for a content-changing path → typed `materialize_vacuous` refusal; admission also rejects an empty inspection path set — no all-clear over nothing.

**D4 — Parity guard.** `internal/review/producers.go` holds `SupportedReviewHosts = {"opencode","pi"}`, `BlockingReviewSurfaces()` (RDD gate kinds + sdd verify preflight) and `ProducerManifest()` (surface×host → exact commands); refusals consume it. Guard fails on any missing producer; a check asserts the plugin wires capture.

**D5 — Surfacing.** Obligation via existing `blockedReasons` (`rdd_receipt_missing: …; run <producer>`) plus `PhaseInstructions.Verify/Apply`; `nextRecommended` untouched, no new keys. Unproducible receipts report explicit `rdd_unproducible`. `sdd-verify` writes `<changeRoot>/review-subject.json` (`{"repository":"<ws>","commit_sha":"HEAD"}`) so the offer invocation is runnable.

**D6 — PR slicing.** Auto-chain: S1a materializer, S1b identity+gate, S1c surfacing+guard, S2 plugin/overlay/doc. The expected two-slice split is HIGH risk; split slice 1 as above.

## Data Flow

```
offer (no id) ─► review-subject.json ─► start (rev-parse → full SHA; derived id)
collect ─► capture-result --materialize ─► lineage store
   │ bytes verbatim ─► tool-less reviewer ─► raw JSON
capture-result --input ─► lens_result ─► finalize ─► receipt
sdd verify/status ─► ResolveCandidateLineage(repo, HEAD) ────┘
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/review/lineage_identity.go` | Create | Derivation + canonicalization |
| `internal/review/lineage_resolve.go` | Create | Derived-first + scan; refusal |
| `internal/review/frozen_inspector.go` | Create | Isolated GIT_DIR reads |
| `internal/review/materialize.go` | Create | Marker composition |
| `internal/review/producers.go` | Create | Hosts, surfaces, commands |
| `cmd/biggz/cli_review.go` | Modify | `--materialize`; start canonicalization |
| `internal/sdd/verify.go` | Modify | Resolve lineage before gate |
| `internal/sdd/{status.go,engram_status.go}` | Modify | Offer; obligation surfaced |
| `internal/review/*_test.go`, `internal/sdd/rdd_parity_test.go` | Create | Materialize, identity, resolution, guard |
| `internal/assets/opencode/{plugins/review-result-artifacts.ts,sdd-overlay-*.json}` | Modify (S2) | Verbatim transport; tool-less |
| `internal/assets/skills/_shared/review-ledger-contract.md` | Modify (S2) | Materialize route |

## Interfaces / Contracts

```go
// internal/review
func CanonicalSubjectSHA(repo, raw string) (string, error)
func DeriveLineageID(repo, subjectSHA string) (string, error)           // "review-"+16hex
func ResolveCandidateLineage(repo, candidateSHA string) (string, error) // typed refusal
func MaterializeReviewerTask(b CaptureBinding) ([]byte, error)          // read-only
var SupportedReviewHosts = []string{"opencode", "pi"}                   // + marker consts
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Derivation determinism; abbrev→full; sections, binary, cap/vacuous refusals | fixtures |
| Unit | derived-first, legacy readable, none→typed; surfaces×hosts guard | fixtures |
| Integration | non-empty findings; offer≡gate≡receipt; obligation surfaced | CLI/sdd fixture |
| E2E | start→materialize→capture→finalize→archive unblocked | CLI flow |

## Threat Matrix

| Boundary | Applicability | Design response + RED test |
|---|---|---|
| Documentation-like paths | Applicable | Manifest paths always materialize (never executed/skipped); git classifies binary, `--text` forbidden. RED: executable `.md` + binary. |
| Git repository selection | Applicable | Repo resolved once; `-C` + isolated GIT_DIR; unresolvable → refusal. RED: foreign cwd → identical bytes. |
| Commit state | Applicable | Frozen trees only, never index/worktree. RED: dirty/staged edits → byte-identical output. |
| Push state | N/A | No push/refspec logic added. |
| PR commands | N/A | No PR command composition introduced. |

Reviewer process: host-owned (PiAdapter test-only).

## Migration / Rollout

Read-path-only: events never rewritten; explicit `--lineage` accepted; abbreviated-subject lineages readable. Auto-chain: S1a (over budget), S1b, S1c, S2. Fail-closed untouched. Rollback: revert the diff.

## Open Questions

- [ ] Confirm the S1a/S1b/S1c chain at tasks planning (S1a ~500 lines > 400 budget).
