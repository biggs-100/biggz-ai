# Apply Progress: fix-rdd-receipt-collection

## Cumulative Summary

| Slice | Tasks | Status | PR |
|-------|-------|--------|----|
| S1a1 | Phase 1 (1.1–1.6): frozen inspector + real hunk derivation | done | PR 1 (base: tracker `fix/rdd-receipt-collection`; merged as 362c4c18) |
| S1a2 | Phase 2 (2.1–2.5): materializer + `--materialize` | done | PR 2 (base: `fix/rdd-receipt-collection-2-materializer`, off tracker 362c4c18) |
| S1b1 | Phase 3 (3.1–3.3): lineage identity + start canonicalization | done | PR 3 (base: `fix/rdd-receipt-collection-2-materializer`; head `fix/rdd-receipt-collection-3-identity`) |
| S1b2 | Phase 4 (4.1–4.3): candidate-lineage resolution + verify gate (bug #60) | done | PR 4 (base: `fix/rdd-receipt-collection-3-identity` @ cc26de2a; head `fix/rdd-receipt-collection-4-resolve-gate`) |
| S1c | Phase 5 (5.1–5.5): surfacing + parity guard | done | PR 5 (base: `fix/rdd-receipt-collection-4-resolve-gate` @ 5ea72fd7; head `fix/rdd-receipt-collection-5-surfacing`) |
| S1d | Phase 6 (6.1–6.2): verify-side `review-subject.json` writer + marker test | done | PR 6 (base: `fix/rdd-receipt-collection-5-surfacing` @ 429a7984; head `fix/rdd-receipt-collection-6-verify-subject`, uncommitted) |

Progress: **24/33 tasks** (Phases 1–6). Remaining: Phases 7–8 (S2, dogfood close).

---

## Batch S1d — Verify-side subject writer (current)

| Field | Value |
|-------|-------|
| Work unit | `phases-6-8` (ledger attempt `tok-d9ab9305907d0e0c14b75d2c`) |
| Slice | S1d = Phase 6 only (tasks 6.1–6.2) |
| Mode | Standard (`strict_tdd: false`) |
| PR | PR 6 of the feature-branch-chain (base: `fix/rdd-receipt-collection-5-surfacing` @ 429a7984; head `fix/rdd-receipt-collection-6-verify-subject`, uncommitted) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

### Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 6.1 `sdd-verify/SKILL.md`: verify writes `<changeRoot>/review-subject.json` (`{"repository","commit_sha":"HEAD"}`); `sdd-status` stays read-only — writer cannot live there | done | One identical instruction paragraph added to each renderable model variant (`model-capable` + `model-small`) of `internal/assets/skills/sdd-verify/SKILL.md` and its byte-identical mirror `skills/sdd-verify/SKILL.md`; `internal/sdd` untouched (status stays read-only) |
| 6.2 Marker test `sdd_verify_writer_marker_test.go` proves instruction present | done | `internal/assets/sdd_verify_writer_marker_test.go` → `TestSDDVerifySubjectWriter` PASS; overlay-stripped negative proof FAILs (guard bites) |

### Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/assets/skills/sdd-verify/SKILL.md` | Modify (writer instruction in both model variants) | +2/−0 |
| `skills/sdd-verify/SKILL.md` | Modify (mirror, byte-identical in the same commit) | +2/−0 |
| `internal/assets/sdd_verify_writer_marker_test.go` | Create (marker test via `assets.FS`, nosourcegrep-safe) | +47/−0 |
| `openspec/changes/fix-rdd-receipt-collection/tasks.md` | Modify (6.1–6.2 `[x]`) | +2/−2 |

Slice total: **53 insertions + 2 deletions across 4 files** (well under the 400-line budget).

### Focused Test Command + Result

```
go test ./internal/assets -run TestSDDVerifySubjectWriter -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/assets 0.439s (exit 0)
  === RUN   TestSDDVerifySubjectWriter
  --- PASS: TestSDDVerifySubjectWriter (0.00s)
  PASS
```

Negative proof (build overlay substitutes a copy of the skill with the two instruction lines removed; repo files untouched):

```
go test -overlay "$TEMP/ovl/overlay.json" ./internal/assets -run TestSDDVerifySubjectWriter -count=1 -v
→ FAIL (exit 1):
  sdd_verify_writer_marker_test.go:36: sdd-verify/SKILL.md missing the review-subject writer instruction "Before the RDD gate runs, write `<changeRoot>/review-subject.json`"
  --- FAIL: TestSDDVerifySubjectWriter (0.00s)
```

### Package Suite Command + Result

```
go test ./internal/assets ./internal/sdd ./internal/review ./cmd/biggz -count=1 -timeout 300s
→ ok internal/assets   0.790s
→ ok internal/sdd     37.652s
→ ok internal/review 173.078s
→ ok cmd/biggz        87.285s
```

### Static Checks

| Check | Result |
|-------|--------|
| `biggz sdd-apply fix-rdd-receipt-collection` (edit-authority guard) | exit 0; allowed roots = `C:\Users\USER\Desktop\biggz-ai` |
| `go build ./...` | OK (exit 0) |
| `go vet ./...` | OK (exit 0, no findings) |
| `gofmt -l internal/assets/sdd_verify_writer_marker_test.go` | clean |
| `node scripts/check-skill-lint.mjs` | exit 0; both mirrors WARN 1101 tokens (ideal 450, hard 3200) — token WARNs acceptable, no FAILs |
| `cmp skills/sdd-verify/SKILL.md internal/assets/skills/sdd-verify/SKILL.md` | MIRROR-IDENTICAL |
| `use-modern-go list --go-version 1.25` + `list --file-path internal/assets/sdd_verify_writer_marker_test.go` | consulted; no applicable modernization (marker test: no context, goroutines, slices, maps, or manual loops) |
| CI complexity gate (cyclomatic ≤15 / cognitive ≤20) | no non-test `internal/sdd`/`internal/review` code touched — no additions possible |

### Runtime Harness Evidence (raw)

Phase 6's declared harness "offered subject invocation runs": fresh `go build -o <temp>/biggz-harness.exe ./cmd/biggz`; an external temp-module program (`github.com/biggs-100/biggz-ai/harness`, `replace` → repo, so it can import `internal/review` + `internal/sdd`) creates a temp git repo with isolated `HOME`, writes the subject file EXACTLY as the new instruction dictates, runs the real offered CLI invocation, resolves the gate lineage for `HEAD`, then captures + finalizes that same lineage and re-runs the real verify gate. Raw output:

```
harness: repo=C:\Users\USER\AppData\Local\Temp\rdd-harness-repo-4133046029
harness: head=b1210cf602f16cbdf0ae6a1e65e6e8a3b6ea85be
harness: subject_file=C:\Users\USER\AppData\Local\Temp\rdd-harness-repo-4133046029\openspec\changes\fix-rdd-receipt-collection\review-subject.json
harness: subject_bytes={"repository":"C:/Users/USER/AppData/Local/Temp/rdd-harness-repo-4133046029","commit_sha":"HEAD"}
harness: start exit=0 output="Review started: review-268a31d339558e06 (correction budget: 2 lines, base 4b36dfd79db36d8c59d1fb032de66b57f0457b65, risk tier: low, lenses: risk)"
harness: started_lineage=review-268a31d339558e06 gate_resolved=review-268a31d339558e06 equal=true
harness: preflight_before_capture=rdd_receipt_missing: missing persisted review receipt: run 'biggz review finalize <lineage>' to finalize the captured review; hint: run `biggz review start --subject "C:\Users\USER\AppData\Local\Temp\rdd-harness-repo-4133046029\openspec\changes\fix-rdd-receipt-collection\review-subject.json"` and `biggz review finalize <lineage>`
harness: capture_admission=completed receipt=sha256:a79cff8a505314cf33816119bb30fdef55fab548a4f6ed801f08a474e9ba1df5
harness: preflight_after_capture=<nil>
harness: OK — offered subject invocation runs; start lineage == gate lineage for HEAD
```

Assertions proven: (a) the instruction-dictated subject file (`repository` + `commit_sha:"HEAD"`) makes the offered `biggz review start --subject '<changeRoot>/review-subject.json'` invocation runnable (real CLI, exit 0); (b) the lineage start derived (`review-268a31d339558e06`) EQUALS the lineage the RDD gate resolves for `HEAD` (`Review.ResolveCandidateLineage(repo, "HEAD^{commit}")` — the exact call `verifyPreflightAt` makes) — `equal=true`; (c) before capture the gate fails closed naming the same subject file; (d) after capture+finalize of that same lineage the real `sdd.VerifyPreflightAt` returns `nil` (receipt `sha256:a79cff8a…` satisfied the gate started from the subject file).

### Rollback Boundary

Revert exactly this unit; no other slice consumes anything from it:

1. delete `internal/assets/sdd_verify_writer_marker_test.go`;
2. remove the two instruction bullets from `internal/assets/skills/sdd-verify/SKILL.md` and `skills/sdd-verify/SKILL.md` (revert BOTH mirrors together to keep them byte-identical);
3. revert `tasks.md` 6.1–6.2 `[x]` → `[ ]`.

→ back to 429a7984 (PR 5 head). Asset-only: zero production Go changes, zero behavior change outside the shipped skill text; `sdd-status` untouched.

### Notes / Deviations

1. **Instruction in both model variants (deliberate):** the skill file ships two renderable bodies — `<!-- section:model-capable -->` and `<!-- section:model-small -->` — and `extractModelSection`/`parseFrontmatter` render exactly one per model class. A single placement would silently drop the writer contract for the other class, so the identical paragraph lands in both variants; `TestSDDVerifySubjectWriter` locks the dual presence (count == 2) so removing either fails the build. This delivers the "one instruction paragraph" requirement per renderable variant.
2. **Harness not committed:** the dispatch's allowed edit surfaces excluded the `internal/sdd`/`cmd/biggz` test packages, so the runtime harness ran as an external temp-module program driving the real built CLI plus the exported resolution/gate functions; raw output captured above. No fixture-only shortcut: real git repo, real CLI binary, real gate function.
3. **Ledger attempt `tok-d9ab9305907d0e0c14b75d2c` NOT settled** (per dispatch — the orchestrator settles after reviewing the diff).
4. **No commit/push** (per dispatch — orchestrator commits after reviewing the diff). Working tree: 3 modified + 1 new file, all within the five allowed surfaces; the pre-existing untracked `openspec/changes/fix-rdd-receipt-collection/state.yaml` was left untouched.
5. `biggz.exe` (untracked build output at the repo root) untouched; the harness binary was built to a temp path.

---

## Batch S1c — Surfacing + parity guard (previous)

| Field | Value |
|-------|-------|
| Work unit | `phases-3-8` (ledger attempt `tok-819f1c0143a853fc247762de`) |
| Slice | S1c = Phase 5 (tasks 5.1–5.5) |
| Mode | Standard (`strict_tdd: false`) |
| PR | PR 5 of the feature-branch-chain (base: `fix/rdd-receipt-collection-4-resolve-gate` @ 5ea72fd7; head `fix/rdd-receipt-collection-5-surfacing`, uncommitted) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

### Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 5.1 GREEN: `internal/review/producers.go` — `SupportedReviewHosts`, `BlockingReviewSurfaces()`, `ProducerManifest()` | done | Single source of truth: 6 blocking surfaces (the five gate kinds + `sdd-verify`) × 2 supported hosts; `ProducerSubjectToken`; `ValidateProducerManifest` (guard); `ResolveProducer`; `CurrentProducerHost` (pi relay-gated, opencode ambient — existing `IsPiRelayAvailable`) |
| 5.2 RED: `rdd_parity_test.go` — missing producer fails guard; coverage passes; plugin wires capture | done | `internal/review/rdd_parity_test.go` — 4 subtests all PASS: `FullCoveragePasses`, `MissingProducerFailsGuard` (missing host, missing surface, empty command), `ResolveProducerConsumesManifest`, `PluginWiresCapture` |
| 5.3 GREEN: `internal/sdd/{status.go,engram_status.go}` — offer without lineage id; obligation+producer in `blockedReasons`; `nextRecommended` untouched | done | Offer is `biggz review start --subject "<ws>/openspec/changes/<change>/review-subject.json"` (`pathquote.Quote`, no id); `appendReviewObligation` lifts the `blockedReasons` obligation into `PhaseInstructions.Apply/Verify`; no new status keys; BigMem derivation runs the same RDD gate; dead `shortSHAForWorkspace` removed |
| 5.4 GREEN: refusals name exact producer command; unproducible → `rdd_unproducible` | done | verify.go `reviewProducerResolution` + `producerRefusal` consume the manifest (no hardcoded command); `classifyGateReason` hints through the same resolution; `rdd_unproducible` typed branch when the surface/host has no producer |
| 5.5 Verify: offer ≡ gate ≡ receipt one lineage; obligation clears (receipt/disabled); `go test ./internal/review ./internal/sdd` | done | Harness proofs PASS (`TestRDDParitySurfacing`: gate resolves HEAD to the derived lineage that holds the receipt; obligation clears with receipt and with RDD disabled); full three-package suite green (below) |

### Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/review/producers.go` | Create (hosts, surfaces, manifest, guard, resolution) | +154/−0 |
| `internal/review/rdd_parity_test.go` | Create (4 subtests) | +122/−0 |
| `internal/sdd/verify.go` | Modify (manifest-consuming refusals, `rdd_unproducible`, `classifyGateReason` signature) | +23/−11 |
| `internal/sdd/status.go` | Modify (subject offer, `appendReviewObligation`, dead helper removed) | +31/−13 |
| `internal/sdd/engram_status.go` | Modify (RDD gate parity + obligation wiring) | +16/−0 |
| `internal/sdd/gates_test.go` | Modify (append `TestRDDParitySurfacing`, imports) | +180/−0 |
| `internal/sdd/review_offer_test.go` | **Modify (boundary extension — see Deviations)** | +17/−11 |

Slice total: **543 insertions + 35 deletions across 7 files** (over the 400-line budget; `size:exception` accepted for the slice per the session preflight).

### Focused Test Command + Result

```
go test ./internal/review -run TestRDDParity -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/review 0.151s (exit 0)
  TestRDDParity/FullCoveragePasses ✓ | MissingProducerFailsGuard ✓ |
  ResolveProducerConsumesManifest ✓ | PluginWiresCapture ✓

go test ./internal/sdd -run 'TestRDDParitySurfacing|TestReviewOffer|TestVerifyRDDResolve' -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/sdd 9.682s (exit 0)
  TestRDDParitySurfacing (2.49s): ObligationNamesProducerAndSurfacesOffer ✓ |
  ObligationClearsWithValidReceipt ✓ | ObligationClearsWhenRDDDisabled ✓ |
  UnproducibleRefusalIsTyped ✓
  TestReviewOffer (4 subtests) ✓ | TestReviewOfferQuoting ✓ | TestReviewOfferDisabledEmitsNil ✓
  TestVerifyRDDResolve* (4 tests) ✓
```

### Package Suite Command + Result

```
go test ./internal/review ./internal/sdd ./cmd/biggz -count=1 -timeout 240s
→ ok internal/review  177.447s
→ ok internal/sdd      37.026s
→ ok cmd/biggz         93.918s
```

### Static Checks

| Check | Result |
|-------|--------|
| `go build ./...` | OK (exit 0) |
| `go vet ./...` | OK (exit 0, no findings) |
| `go vet ./internal/review ./internal/sdd` (focused) | OK (exit 0) |
| `use-modern-go list --go-version 1.25` (go.mod `1.25.0`) consulted before authoring | consulted; applied `maps.Clone` in the guard test; no `interface{}` |

### Runtime Harness Evidence (raw)

Scenario A — obligation and offer from a real derived change (RDD enabled, no receipt): the offer carries the quoted subject file and no lineage id; `blockedReasons` carries `rdd_receipt_missing` naming the exact manifest command; apply and verify instructions surface it; `nextRecommended == resolve-blockers` (existing routing).

Scenario B — real temp repository, derived lineage started → captured → finalized; raw proof from `TestVerifyRDDResolveRuntimeHarness` (manifest command, no `--lineage`):

```
harness: repo=...\TestVerifyRDDResolveRuntimeHarness3406334167\002 head=8864666cd9c26ede126d5124e620e433a6500e38 derived=review-2b8fc96dde40b099 receipt=sha256:c436a9f9... preflight_err=<nil>
harness: repo=...\003 head=c6eb27e91e87debc5cefa061850514487e3ae2fc missing_preflight_err=rdd_receipt_missing: review lineage resolution: unresolved_candidate_lineage: ...; hint: run `biggz review start --subject "...\003\openspec\changes\fix-rdd-receipt-collection\review-subject.json"` and `biggz review finalize <lineage>`
```

Assertions proven: (a) a host with no producer command fails the guard (`MissingProducerFailsGuard`); (b) the full manifest passes (`FullCoveragePasses`); (c) the OpenCode plugin’s capture wiring is asserted (`PluginWiresCapture`: `capture-result`, `--input`, `--preflight`, `tool.execute.after`, preserve budget); (d) the obligation text in `blockedReasons` names the exact manifest command (Scenario A; raw output in the focused run); (e) the obligation clears with a receipt and when RDD is disabled (Scenario B + disabled subtest); (f) `nextRecommended` unchanged (`resolve-blockers` from the pre-existing gate routing).

### Rollback Boundary

Revert exactly this unit; no other slice consumes the new symbols yet:

1. delete `internal/review/producers.go` and `internal/review/rdd_parity_test.go`;
2. revert `internal/sdd/verify.go` (+23/−11): restore `reviewProducerInvocation` + `pathquote` import, drop `producerRefusal`/`reviewProducerResolution`/`errors` import, restore `classifyGateReason(reason)`;
3. revert `internal/sdd/status.go` (+31/−13): restore the `--lineage` offer + `shortSHAForWorkspace`, drop `appendReviewObligation`/`isReviewObligationReason` and the two call-site appends;
4. revert `internal/sdd/engram_status.go` (+16): drop the RDD gate block and the two obligation appends;
5. revert `internal/sdd/gates_test.go` (+180) and `internal/sdd/review_offer_test.go` (+17/−11).

→ back to 5ea72fd7 (PR 4 head). Surfacing is additive; no data, migration, store, config, or receipt changes.

### Notes / Deviations

1. **Boundary extension (disclosed):** `internal/sdd/review_offer_test.go` was edited outside the listed allowed surfaces. Its two offer tests asserted the superseded `--lineage <change>-<shortsha>` invitation, which the change’s own sdd spec MODIFIES to the subject form (“Previously: Invocation embedded `--lineage …`”); task 5.3 and 5.5’s suite are impossible while those assertions stand. The edit is test-only (+17/−11) and reverts alone.
2. **`rdd_parity_test.go` placement:** the session’s allowed surfaces list `internal/review/rdd_parity_test.go` (design.md’s File Changes table said `internal/sdd/rdd_parity_test.go`); the sdd-side surfacing tests landed in the allowed `internal/sdd/gates_test.go` as `TestRDDParitySurfacing`.
3. **RED-first order for 5.2:** the parity test and `producers.go` were authored in the same pass (dependency-ordered), so a strict pre-implementation compile-failure transcript was not captured; the guard contract is asserted both ways (broken manifest fails, full manifest passes).
4. **`rdd_unproducible` reachability:** the typed branch fires when the manifest has no producer for the surface/host; `CurrentProducerHost` only returns guard-covered hosts, so the path is proven at the unit seam (`producerRefusal` + `ResolveProducer` unknown-host). Host detection reuses the EXISTING `IsPiRelayAvailable()` handshake — no new env var.
5. **`internal/review/gate.go` untouched:** `EvaluateGate` receives no workspace or subject path, so it cannot name a producer without inventing context; producer naming lands on the verify preflight and the status surfacing (both workspace-aware).
6. **`shortSHAForWorkspace` removed** in `status.go` (dead after the subject offer; no other callers).

---

## Batch S1b2 — Resolution + gate (previous)

| Field | Value |
|-------|-------|
| Work unit | `phases-3-8` (ledger attempt `tok-2aabb247ee2fd230cacb573e`) |
| Slice | S1b2 = Phase 4 only (tasks 4.1–4.3) |
| Mode | Standard (`strict_tdd: false`) with the mandated RED-first order for 4.1 |
| PR | PR 4 of the feature-branch-chain (base: `fix/rdd-receipt-collection-3-identity` @ cc26de2a; head `fix/rdd-receipt-collection-4-resolve-gate`) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

### Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 4.1 RED: `lineage_resolve_test.go` — derived-first; legacy abbreviated readable (read-only scan, no rewrite); none → typed refusal | done | `internal/review/lineage_resolve_test.go` — 6 tests: `DerivedFirst`, `LegacyAbbreviatedReadOnly`, `NewestFirst`, `RefusesTyped`, `UnreadableStoreRefusesTyped`, `RuntimeHarness` |
| 4.2 GREEN: `internal/review/lineage_resolve.go` — `ResolveCandidateLineage` | done | Focused suite green (exit 0); derived identity wins even against a NEWER competing legacy lineage; abbreviated candidate canonicalizes to the same resolution; legacy store bytes unchanged after resolution (`bytes_unchanged=true`) |
| 4.3 GREEN: `internal/sdd/verify.go` — resolve `HEAD^{commit}` before gate; replaces bare-change lookup; gate tests | done | `internal/sdd/verify_rdd_resolve_test.go` — 4 tests: captured receipt satisfies the gate; legacy UUID lineage resolves by genesis subject; missing receipt names the runnable producer invocation and never `--lineage`; harness (below) |

RED evidence provenance: the original apply run for this slice timed out (1200000ms, 111 turns) after writing the implementation but before persisting its console output; the RED-first test files are the unmodified artifacts (`lineage_resolve_test.go` was authored against undefined symbols — `ResolveCandidateLineage`, `LineageResolutionRefusal`, `LineageResolutionUnresolvedCode` — before 4.2 landed, so the package did not build pre-implementation). This completion re-run persisted GREEN + regression evidence only; no implementation changes were made (every required command passed on the first attempt).

### Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/review/lineage_resolve.go` | Create (`ResolveCandidateLineage`, `LineageResolutionRefusal`, read-only legacy scan) | +197/−0 |
| `internal/review/lineage_resolve_test.go` | Create (6 tests) | +355/−0 |
| `internal/sdd/verify_rdd_resolve_test.go` | Create (4 gate tests) | +227/−0 |
| `internal/sdd/verify.go` | Modify (`verifyCandidateRef`, resolve-then-gate, `reviewProducerInvocation`, `pathquote` import) | +22/−2 |

Slice total: **803 changed lines** (801 additions + 2 deletions).

### Focused Test Command + Result

```
go test ./internal/review -run TestResolveCandidateLineage -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/review 5.772s (exit 0)
  6 tests all PASS:
  DerivedFirst (0.79s) ✓ | LegacyAbbreviatedReadOnly (0.88s) ✓ | NewestFirst (1.01s) ✓ |
  RefusesTyped (0.78s) ✓ | UnreadableStoreRefusesTyped (0.49s) ✓ | RuntimeHarness (1.67s) ✓

go test ./internal/sdd -run 'TestVerifyRDDResolve|TestResolve' -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/sdd 5.411s (exit 0)
  TestVerifyRDDResolveCandidateLineagePassesWithCapturedReceipt (1.45s) ✓
  TestVerifyRDDResolveLegacyUUIDLineage (1.17s) ✓
  TestVerifyRDDResolveMissingNamesProducerInvocation (0.52s) ✓
  TestVerifyRDDResolveRuntimeHarness (2.13s) ✓
  (TestResolveExistingPathEvalSymlinks SKIP — pre-existing Windows symlink-privilege skip, unrelated)
```

### Package Suite Command + Result

```
go test ./internal/review ./internal/sdd ./cmd/biggz -count=1 -timeout 240s
→ ok internal/review  173.989s
→ ok internal/sdd      31.204s
→ ok cmd/biggz         88.955s
```

### Static Checks

| Check | Result |
|-------|--------|
| `biggz sdd-apply fix-rdd-receipt-collection` (edit-authority guard) | exit 0; allowed roots = `C:\Users\USER\Desktop\biggz-ai` |
| `go build ./...` | OK (exit 0) |
| `go vet ./...` | OK (exit 0) |
| `gofmt -l` on the four touched files | clean |

### Runtime Harness Evidence

Command (resolver): `go test ./internal/review -run TestResolveCandidateLineageRuntimeHarness -count=1 -v` → PASS (1.67s). Raw output:

```
harness: repo=C:\Users\USER\AppData\Local\Temp\TestResolveCandidateLineageRuntimeHarness1009495569\001 full=7d29652d2dc194340ad9e04859d45aeef1b62384 derived=review-a4e3f51e8147a654
harness: derived-present resolved=review-a4e3f51e8147a654 err=<nil>
harness: legacy_id=01932d0a-7f4e-7c31-9a6b-2f6f6b6c0005 full=6daada2173f7056573f9f42644ab78d1a55eaaee subject=6daada21 resolved=01932d0a-7f4e-7c31-9a6b-2f6f6b6c0005 err=<nil> bytes_unchanged=true
harness: empty-store refusal err=review lineage resolution: unresolved_candidate_lineage: no review lineage for candidate "HEAD^{commit}" (resolved commit c0f67c76b8749a15d3a2800658de5fc2b3c5a438): derived identity review-54f226fe58bce81d has no store entry and no legacy lineage genesis subject resolves to it typed=true
```

Command (gate): `go test ./internal/sdd -run TestVerifyRDDResolveRuntimeHarness -count=1 -v` → PASS (2.13s). Raw output:

```
harness: repo=C:\Users\USER\AppData\Local\Temp\TestVerifyRDDResolveRuntimeHarness2441013870\002 head=7d5f13c30f088db8a0167ae27e05856559ceb50b derived=review-e664b6f2eba20180 receipt=sha256:46cfbf803d327227ff76ac069d31434f50e0d94c38e81293fab520ab9218a0a1 preflight_err=<nil>
harness: repo=C:\Users\USER\AppData\Local\Temp\TestVerifyRDDResolveRuntimeHarness2441013870\003 head=fa58c0134042a24324aa73e58d6beadc62e0b730 missing_preflight_err=rdd_receipt_missing: review lineage resolution: unresolved_candidate_lineage: no review lineage for candidate "HEAD^{commit}" (resolved commit fa58c0134042a24324aa73e58d6beadc62e0b730): derived identity review-994c7b74512b2468 has no store entry and no legacy lineage genesis subject resolves to it; hint: run `biggz review start --subject "C:\Users\USER\AppData\Local\Temp\TestVerifyRDDResolveRuntimeHarness2441013870\003\openspec\changes\fix-rdd-receipt-collection\review-subject.json"` and `biggz review finalize <lineage>`
```

Assertions proven by the harnesses: (a) derived id present → resolves to the derived identity; (b) a legacy lineage (UUIDv7-style id, abbreviated subject `6daada21`) still resolves AND its store bytes are unchanged afterwards (`bytes_unchanged=true` — read-only scan, no migration/rewrite); (c) an empty store refuses with the typed `unresolved_candidate_lineage` (`typed=true`), and `RefusesTyped` additionally proves a lineage for a different subject never matches (no bare-name fallback); (d) the real verify gate on a change with a captured receipt no longer reports `rdd_receipt_missing` (`preflight_err=<nil>` — bug #60 fixed), and a missing receipt fails closed naming the runnable producer invocation (`biggz review start --subject <quoted path>`), never `--lineage`.

### Rollback Boundary

Revert exactly this unit, independently of S1b1 (untouched by this slice):
1. delete `internal/review/lineage_resolve.go`, `internal/review/lineage_resolve_test.go`, `internal/sdd/verify_rdd_resolve_test.go`;
2. revert the `internal/sdd/verify.go` diff (+22/−2): the `verifyCandidateRef` const, the resolve-then-gate block, the `reviewProducerInvocation` helper, and the `pathquote` import — restoring the bare-change `EvaluateGate` lookup and its bare `--lineage` hint.

→ back to cc26de2a (PR 3 head). Read-path only: the legacy scan is stat/read and never migrates, renames, or rewrites a lineage (proven by byte snapshots); the gate remains fail-closed.

### Notes / Deviations

- **Completion re-run**: the original apply run for this slice timed out (1200000ms, 111 turns) with the implementation already written but the batch unpersisted. This completion re-run verified, captured evidence, marked tasks, and merged this batch; **zero implementation changes were made**.
- **Workload overrun**: S1b2 authors 803 changed lines against the 400-line review budget; `size:exception` accepted for the slice per the session preflight (`exception-ok`), consistent with chain precedent (S1a1 ~1,031; S1a2 979; S1b1 640).
- The gate now governs the candidate at `HEAD^{commit}` (design D1). The old behavior — passing the bare change name as a lineage id — is removed; `TestVerifyRDDResolveMissingNamesProducerInvocation` asserts the refusal contains no `--lineage` fallback.
- Resolver ordering: derived-first (even against a newer competing legacy lineage), then matching lineages newest-first by most-recent event timestamp with a stable id tiebreak; a missing store root has no matches.
- Unreadable/unparsable legacy lineages are skipped, never rewritten; an unreadable store root refuses typed without touching it (sentinel byte-equality assertion in `UnreadableStoreRefusesTyped`).
- `TestResolveExistingPathEvalSymlinks` (Windows symlink privilege) skip is pre-existing and unrelated to this slice.

---

## Batch S1b1 — Identity + start canonicalization (previous)

| Field | Value |
|-------|-------|
| Work unit | `phases-3-8` (ledger attempt `tok-7d92e2825654e5077908597a`) |
| Slice | S1b1 = Phase 3 only (tasks 3.1–3.3) |
| Mode | Standard (`strict_tdd: false`) with the mandated RED-first order for 3.1 |
| PR | PR 3 of the feature-branch-chain (base: `fix/rdd-receipt-collection-2-materializer` @ 60ba4033; head `fix/rdd-receipt-collection-3-identity`) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

### Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 3.1 RED: `lineage_identity_test.go` — derivation deterministic; abbrev → full SHA persisted; unresolvable → typed reject | done | `internal/review/lineage_identity_test.go` + `cmd/biggz/review_lineage_identity_test.go` (RED: `undefined: DeriveLineageID`, `CanonicalSubjectSHA`, `LineageIdentityRefusal`, `LineageIdentityUnresolvableCode` in both packages) |
| 3.2 GREEN: `internal/review/lineage_identity.go` — `CanonicalSubjectSHA`, `DeriveLineageID` | done | `TestLineageIdentityDerivationIsExactAndDeterministic`, `TestLineageIdentityCanonicalSubjectSHA` (5 resolve + 6 refusal subtests) |
| 3.3 GREEN: `review start` canonicalizes `CommitSHA`; defaults to derived id | done | `TestReviewStartLineageIdentityCanonicalizesAbbreviatedSubject`, `...SymbolicHEADSubject`, `...UnresolvableRejectedTyped`, `...ExplicitLineageStillWins`, `TestReviewLineageIdentityRuntimeHarness` |

RED evidence captured before implementation:

- `go test ./internal/review -run TestLineageIdentity -count=1 -v` → `FAIL [build failed]`: `undefined: DeriveLineageID` (lines 91, 102, 113, 125, 138), `undefined: CanonicalSubjectSHA` (164, 186), `undefined: LineageIdentityRefusal` (193), `undefined: LineageIdentityUnresolvableCode` (197-198).
- `go test ./cmd/biggz -run TestReviewStartLineageIdentity -count=1 -v` → `FAIL [build failed]`: `undefined: review.DeriveLineageID` (56, 87, 152, 157, 175).

### Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/review/lineage_identity.go` | Create (D1 derivation, canonicalization, typed refusal) | +130/−0 |
| `internal/review/lineage_identity_test.go` | Create (formula/refusal/harness tests) | +286/−0 |
| `cmd/biggz/review_lineage_identity_test.go` | Create (CLI acceptance + harness tests) | +195/−0 |
| `cmd/biggz/cli_review.go` | Modify (canonicalize `subject.CommitSHA`, derived-id default, usage strings, `uuid` import removed) | +25/−4 |

Slice total: **640 changed lines** (636 additions + 4 deletions).

### Focused Test Command + Result

```
go test ./internal/review -run TestLineageIdentity -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/review 2.836s
  3 top-level tests + 11 subtests, all PASS:
  DerivationIsExactAndDeterministic ✓
  CanonicalSubjectSHA ✓ (full idempotent ✓ | abbreviated ✓ | symbolic HEAD ✓ | tag ✓ | HEAD^{commit} ✓ |
    refusals: missing commit ✓ | malformed ref ✓ | empty ✓ | whitespace ✓ | tree object ✓ | blob object ✓)
  RuntimeHarness ✓

go test ./cmd/biggz -run 'TestReviewStartLineageIdentity|TestReviewLineageIdentityRuntimeHarness' -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/cmd/biggz 4.562s
  TestReviewStartLineageIdentityCanonicalizesAbbreviatedSubject (0.74s) ✓
  TestReviewStartLineageIdentitySymbolicHEADSubject (0.70s) ✓
  TestReviewStartLineageIdentityUnresolvableRejectedTyped (0.35s) ✓
  TestReviewStartLineageIdentityExplicitLineageStillWins (0.75s) ✓
  TestReviewLineageIdentityRuntimeHarness (1.86s) ✓
```

### Package Suite Command + Result

```
go test ./internal/review ./cmd/biggz -count=1 -timeout 240s
→ ok internal/review  165.399s
→ ok cmd/biggz         80.601s
```

### Static Checks

| Check | Result |
|-------|--------|
| `go build ./...` | OK |
| `go vet ./...` | OK (exit 0) |
| `gofmt -l` on the four touched files | clean |
| CI complexity gate (cyclomatic ≤15 / cognitive ≤20, non-test `internal/review`) | `gocyclo -over 15 internal/review` → no non-test offender; `gocognit -over 20 internal/review` → no non-test offender |
| `use-modern-go list --go-version 1.25` + `list --file-path cmd/biggz/cli_review.go` | consulted; full output read before editing. Applicable idioms: none forced (new code carries no `interface{}`, no manual sort/loop patterns, no WaitGroup/Once usage); tests use `t.Chdir` for cwd isolation |

### Runtime Harness Evidence

Command (internal): `go test ./internal/review -run TestLineageIdentityRuntimeHarness -count=1 -v` → PASS (0.86s). Raw output:

```
harness: repo=C:\Users\USER\AppData\Local\Temp\TestLineageIdentityRuntimeHarness1339296016\001
harness: full_sha=be51024218573552760763bf772818022a5c6986 abbrev=be510242 symbolic=HEAD
harness: common_dir=C:\Users\USER\AppData\Local\Temp\TestLineageIdentityRuntimeHarness1339296016\001\.git
harness: derived(abbrev)=review-92e365ef771384ff derived(symbolic)=review-92e365ef771384ff derived(full)=review-92e365ef771384ff all_equal=true
harness: second_run=review-92e365ef771384ff stable=true
harness: foreign_cwd_id=review-92e365ef771384ff stable=true (cwd=C:\Users\USER\AppData\Local\Temp\TestLineageIdentityRuntimeHarness1339296016\002)
harness: linked_worktree_id=review-92e365ef771384ff stable=true
harness refusal: raw=ffffffffffffffffffffffffffffffffffffffff err=review lineage identity: unresolvable_subject_commit: subject commit "ffffffffffffffffffffffffffffffffffffffff" does not resolve to a commit object: exit status 1
harness: store_root=C:\Users\USER\AppData\Local\Temp\TestLineageIdentityRuntimeHarness1339296016\001\.git\biggz\review-transactions entries_after_refusal=0 (expect 0)
```

Command (CLI integration): `go test ./cmd/biggz -run TestReviewLineageIdentityRuntimeHarness -count=1 -v` → PASS (1.86s). Raw output:

```
harness: repo=C:\Users\USER\AppData\Local\Temp\TestReviewLineageIdentityRuntimeHarness2319633159\001 full=6f68331e3918899dbe15d85f3084349495361965 abbrev=6f68331e
harness: start(abbrev) lineage=review-d32d6d30d9f7b10d derived=review-d32d6d30d9f7b10d identical=true exit=0
harness: genesis_subject=6f68331e3918899dbe15d85f3084349495361965 full_sha_persisted=true
harness: derived_again=review-d32d6d30d9f7b10d stable=true
harness: start(HEAD) lineage=review-7812186a227628e4 derived=review-7812186a227628e4 identical=true genesis_subject=1f5a1d880aa0aeca6edd1210842a465360d5e3ef full_sha_persisted=true
harness refusal: exit=1 stderr=error: review lineage identity: unresolvable_subject_commit: subject commit "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee" does not resolve to a commit object: exit status 1
harness: store_entries_after_refusal=0 (expect 0)
```

Assertions proven by the harnesses: the exact D1 formula (independently recomputed with the canonical common dir + full SHA + `\x00` domain separator) matches the implementation for abbreviated, symbolic and full targets; the id is `review-<16 lowercase hex>`; stable across two derivations, a foreign cwd, and a linked worktree (store scope = git common dir); a real `review start` over an abbreviated subject persists the full SHA (`full_sha_persisted=true`) under the derived id; a symbolic `HEAD` subject canonicalizes the same way; an unresolvable subject exits 1 with the typed code `unresolvable_subject_commit` and zero lineage store entries.

### Rollback Boundary

Revert exactly this unit, independently of S1a2 (untouched by this slice):
1. delete `internal/review/lineage_identity.go`, `internal/review/lineage_identity_test.go`, `cmd/biggz/review_lineage_identity_test.go`;
2. revert the `cmd/biggz/cli_review.go` diff (+25/−4): the canonicalization block, the derived-id default, the restored `uuid.Must(uuid.NewV7())` default (and its import), and the usage strings.

→ back to 60ba4033 (PR 2 head). Read-path only: no events, receipts, stores, or migrations are rewritten; an explicit `--lineage` keeps its behavior. Existing lineages started under the old UUID default stay readable (resolution/legacy scan is S1b2's surface).

### Notes / Deviations

- **Interpretation of "two runs"**: derivation stability across runs is proven by the pure-function harness (two derivations + foreign cwd + linked worktree) because starting the same derived lineage twice on one repo would append a second genesis to the same store; the CLI harness instead proves one real start per repo plus a re-derivation match.
- **Repo-selection caveat (pre-existing, for S1b2/S1c awareness)**: `review start` opens the store via `NewAuthority("")` (cwd-scoped), while the identity derives from `subject.Repository`. The CLI acceptance tests `chdir` into the subject repo (matching every existing start test); a caller running from a foreign repo would still get the pre-existing cwd-scoped store. Resolution/gate surfaces (S1b2) are the natural place to reconcile this.
- `CanonicalSubjectSHA` uses `rev-parse --verify --quiet <raw>^{commit}`; a caller-supplied value already ending in `^{commit}` peels idempotently (covered by the `HEAD^{commit}` subtest).
- **Workload overrun:** S1b1 authors 640 changed lines against the 400-line review budget; `size:exception` accepted for the slice per the session preflight (`exception-ok`), consistent with this chain's precedent (S1a1 ~1,031; S1a2 979).

---

## Batch S1a2 — Materializer + `--materialize` (previous)

| Field | Value |
|-------|-------|
| Work unit | `S1a2-materializer` (ledger attempt `tok-ca728f6272a52610be37aabb`) |
| Slice | S1a2 = Phase 2 only (tasks 2.1–2.5) |
| Mode | Standard (`strict_tdd: false`) with the mandated RED-first order for 2.1/2.2 |
| PR | PR 2 of the feature-branch-chain (base: tracker `fix/rdd-receipt-collection` @ 362c4c18; head `fix/rdd-receipt-collection-2-materializer`) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

### Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 2.1 RED: `materialize_test.go` — binding, context, name-status, numstat, per-path `GENTLE_AI_REVIEW_PATCH` delimiters | done | `TestMaterializeReviewerTaskSections` (RED: build failed with undefined symbols; GREEN after 2.3) |
| 2.2 RED: >4 MiB → typed cap refusal (no truncation); empty patch on content-changing path → `materialize_vacuous` | done | `TestMaterializeReviewerTaskRefusesOverCap`, `TestMaterializeReviewerTaskRefusesVacuousEvidence` (2 subtests: empty path set real, empty patch via composition seam) |
| 2.3 GREEN: `internal/review/materialize.go` — marker composition, read-only | done | Focused suite green (5 top-level tests); read-only proven by store snapshot equality |
| 2.4 GREEN: `capture-result --materialize` prints exactly bytes, captures nothing; exclusive with `--input`/`--preflight` | done | `TestReviewCaptureResultMaterializePrintsBytesAndCapturesNothing`, `TestReviewCaptureResultMaterializeExclusiveFlags` |
| 2.5 Verify: chain events + receipts unchanged; bytes deterministic, byte-identical ×2 + foreign cwd | done | `TestMaterializeReviewerTaskReadOnlyAndDeterministic`, `TestMaterializeRuntimeHarness` (raw output below) |

RED evidence captured before implementation:

- `go test ./internal/review -run TestMaterialize -count=1 -v` → build failed: `undefined: MaterializeBindingMarker`, `MaterializeContextMarker`, `MaterializeNameStatusMarker`, `MaterializeNumStatMarker`, `MaterializePatchMarker`, `MaterializeReviewerTask` (+ more).
- `go test ./cmd/biggz -run TestReviewCaptureResultMaterialize -count=1 -v` → build failed: `undefined: review.MaterializeBindingMarker`, `review.MaterializeReviewerTask`.

### Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/review/materialize.go` | Create | +226 |
| `internal/review/materialize_test.go` | Create | +524 |
| `cmd/biggz/review_materialize_test.go` | Create | +158 |
| `cmd/biggz/cli_review.go` | Modify (`--materialize` flag, exclusivity validation, usage strings, stdout branch) | +31/−6 |
| `internal/review/frozen_inspector.go` | Modify (`NameStatus`, `NumStat`, shared `diffFactArgs`) | +34/−0 |

Slice total: **979 changed lines** (973 additions + 6 deletions).

### Focused Test Command + Result

```
go test ./internal/review -run TestMaterialize -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/review 7.407s
  5 top-level tests + 2 subtests, all PASS:
  MaterializeReviewerTaskSections ✓
  MaterializeReviewerTaskRefusesVacuousEvidence ✓ (empty path set ✓ | empty patch for a content-changing path ✓)
  MaterializeReviewerTaskRefusesOverCap ✓
  MaterializeReviewerTaskReadOnlyAndDeterministic ✓
  MaterializeRuntimeHarness ✓

go test ./cmd/biggz -run TestReviewCaptureResultMaterialize -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/cmd/biggz 2.414s
  TestReviewCaptureResultMaterializePrintsBytesAndCapturesNothing (1.87s) ✓
  TestReviewCaptureResultMaterializeExclusiveFlags (0.41s) ✓
```

### Package Suite Command + Result

```
go test ./internal/review ./cmd/biggz -count=1 -timeout 240s
→ ok internal/review  165.265s
→ ok cmd/biggz         75.493s
```

### Static Checks

| Check | Result |
|-------|--------|
| `go build ./...` | OK |
| `go vet ./...` | OK (no findings) |
| `go vet -vettool=/tmp/nosourcegrep.exe ./internal/review ./cmd/biggz` (CI primary linter) | OK (exit 0, no findings) |
| `gofmt -l` on the five touched files | clean |
| CI complexity gate (cyclomatic ≤15 / cognitive ≤20, non-test `internal/review`) | `gocyclo -over 15 internal/review/materialize.go internal/review/frozen_inspector.go` → no offender; `gocognit -over 20` → no offender |
| `use-modern-go list --file-path` on `materialize.go`, `frozen_inspector.go`, `cli_review.go` | consulted; applied `bytes.CutPrefix` (test seam), `t.Context()`; `order` intentionally NOT `omitzero` (the binding literal requires `order: 0` present — the host plugin validates a safe integer) |

### Runtime Harness Evidence

Command: `go test ./internal/review -run TestMaterialize -count=1 -v` → PASS (7.407s), raw output captured.

Scenario: real temp lineage over a candidate touching a binary blob (`blob.bin`), a modified doc (`docs/guide.md`), a deleted path (`old.txt`) and an executable Markdown file (`scripts/run.md`, mode 100755, shebang); on top of the committed candidate a staged file (`docs/HARNESS-staged.md`) and unstaged edits (`docs/guide.md`); then materialize from the lineage, a second run, and a foreign cwd. Raw output:

```
harness: lineage=materialize-harness target=4e9652cee41bf623cacf4d9180647073cc9a5e4b bytes=2898 sha256=426c5b0381eefa112aed1ec14047a30690113a537a6940962e15aa0615ba31d9
harness: section marker="GENTLE_AI_REVIEW_BINDING" arg="{"lineage":"materialize-harness",...}" (one-line JSON)
harness: section marker="GENTLE_AI_REVIEW_CONTEXT" arg="{...biggz-ai.review-capture-preflight/v1...}"
harness: section marker="GENTLE_AI_REVIEW_NAME_STATUS" body_bytes=54
harness: section marker="GENTLE_AI_REVIEW_NUMSTAT" body_bytes=62
harness: section marker="GENTLE_AI_REVIEW_PATCH" arg="blob.bin" body_bytes=175
harness: section marker="GENTLE_AI_REVIEW_PATCH" arg="docs/guide.md" body_bytes=203
harness: section marker="GENTLE_AI_REVIEW_PATCH" arg="old.txt" body_bytes=196
harness: section marker="GENTLE_AI_REVIEW_PATCH" arg="scripts/run.md" body_bytes=227
harness: second run sha256=426c5b0381eefa112aed1ec14047a30690113a537a6940962e15aa0615ba31d9 (identical=true)
harness: foreign cwd sha256=426c5b0381eefa112aed1ec14047a30690113a537a6940962e15aa0615ba31d9 (identical=true)
harness: lineage store files before=3 after=3 (unchanged=true)
```

Assertions proven by the harness: non-empty output; sectioned markers exactly as designed (binding, context, name-status, numstat, one `GENTLE_AI_REVIEW_PATCH <path>` per manifest path in order); blob.bin renders Git's binary summary (`Binary files a/blob.bin and b/blob.bin differ`, 175 bytes) with zero blob payload (`HARNESS-BINARY-SECRET-PAYLOAD` absent); `new file mode 100755` present for the executable `.md`; no `HARNESS-*` staged/unstaged marker leaks; byte-identical ×2 and from a foreign cwd; lineage store byte-for-byte unchanged.

Refusal-path raw output (same focused run):

```
harness refusal: review materialize: materialize_vacuous: the frozen inspection path set is empty
harness refusal: review materialize: the composed reviewer task is at least 6122181 bytes, exceeding the 4194304-byte whole-task cap (refused whole; never truncated)
```

CLI end-to-end (`cmd/biggz`): `--materialize` stdout byte-equal to `review.MaterializeReviewerTask` for the same binding; second invocation byte-identical; lineage store snapshot unchanged (no capture, no events, no receipts); `--materialize --preflight` and `--materialize --input -` both exit 1 naming the mutual exclusivity.

### Rollback Boundary

Revert exactly this unit, independently of S1a1 (which is untouched by this slice):
1. delete `internal/review/materialize.go`, `internal/review/materialize_test.go`, `cmd/biggz/review_materialize_test.go`;
2. revert the `NameStatus`/`NumStat`/`diffFactArgs` block in `internal/review/frozen_inspector.go` (+34);
3. revert the `--materialize` diff in `cmd/biggz/cli_review.go` (+31/−6): flag parsing, exclusivity validation, stdout branch, usage strings.

→ back to 362c4c18 (tracker with PR 1 merged). No data, migration, store, config, or receipt changes; `--materialize` is additive (no existing capture/preflight behavior changed).

### Notes / Deviations

- **Wire format defined here.** No pre-existing `GENTLE_AI_REVIEW_PATCH` reference exists in this repo (`frozen_candidate_context.go` from the launch prompt is not present on this branch); the patch flag shape was mirrored from the merged `frozen_inspector.go:patchArgs`. The binding JSON keys (`lineage`, `target`, `lens`, `order`, `revision`, `subject_hash`) mirror the accepted host-plugin `ReviewBinding` shape verified against `internal/assets/opencode/plugins/review-result-artifacts.ts`, which S2 will replace with verbatim transport of these bytes.
- **Empty-patch vacuity is unreachable from a real repository**: a frozen tree-diff entry always renders a non-empty patch (mode-only changes emit mode lines, binary emits the summary). The guard is exercised through the composition seam (`composeReviewerTask` + stubbed facts); the empty-path-set guard is exercised through a real empty-diff lineage.
- `MaterializeRefusal` (code `materialize_vacuous`, plus `materialize_tree_mismatch` for preflight/inspector tree disagreement) and `MaterializeCapRefusal` (names `ArtifactResultLimit`, 4 MiB, `Actual` is a lower bound; refusal never truncates) are the typed refusals.
- Determinism caveat: patch/name-status/numstat bytes come from the local git; deterministic per machine, not guaranteed byte-equal across git versions.
- The executable-bit fixture uses `git update-index --chmod=+x` because Windows cannot carry the mode through the filesystem.
- `--materialize` remains under the same early RDD kill-switch as the rest of `capture-result` (existing behavior, unchanged); the materialize path itself performs zero writes.
- **Workload overrun:** S1a2 authors 979 changed lines against the 400-line review budget; `size:exception` accepted for the slice per the session preflight (`exception-ok`), consistent with this chain's precedent.

---

## Batch S1a1 — Inspector + hunks (previous, merged)

| Field | Value |
|-------|-------|
| Slice | S1a1 = Phase 1 only (tasks 1.1–1.6): frozen-tree inspector + real hunk derivation |
| Mode | Standard (`strict_tdd: false`) with the mandated RED-first order for the threat-matrix cases |
| PR | PR 1 of the feature-branch-chain (base: tracker `fix/rdd-receipt-collection` @ d03bb481; branch `fix/rdd-receipt-collection-1-frozen-inspector`; merged as 362c4c18) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

### Tasks Completed

| Task | Status | Evidence |
|------|--------|----------|
| 1.1 RED: executable `.md` + binary always materialize; `--text` forbidden | done | `TestFrozenInspectorMaterializesDocumentationAndBinary` (RED: build failure before 1.4; GREEN after) |
| 1.2 RED: foreign cwd → byte-identical via `-C` + isolated `GIT_DIR`; unresolvable → typed refusal | done | `TestFrozenInspectorRepoSelectionIsFrozenAgainstCwd`, `TestFrozenInspectorUnresolvableInputsRefusedTyped` (3 subtests) |
| 1.3 RED: dirty/staged never leak — frozen-tree bytes byte-identical | done | `TestFrozenInspectorDirtyAndStagedEditsNeverLeak` (dirty worktree + staged edit + staged `.gitattributes` reclassification) |
| 1.4 GREEN: `internal/review/frozen_inspector.go` — repo resolved once; `--patch --full-index --no-ext-diff --no-textconv --unified=3` | done | Focused suite green (7 top-level tests) |
| 1.5 GREEN: `deriveLensHunks` → inspector hunks → `lens.NewLensInput`; parser-error candidate → non-empty findings | done | `TestDeriveLensHunksFeedLensFindings` (RED: signature mismatch before wiring; GREEN after) |
| 1.6 Verify: `go test ./internal/review ./cmd/biggz` | done | Suites green (see Package Suite below) |

RED evidence captured before implementation:
- 1.1–1.3: `go test ./internal/review -run 'TestFrozenInspector' -count=1 -v` → build failed with 11 undefined-symbol errors (`FrozenInspector`, `OpenFrozenInspector`, `FrozenInspectionRefusal`, `FrozenByteCapRefusal`, `MaxFrozenPathPatchBytes`, `MaxFrozenTaskPatchBytes`).
- 1.5: `go test ./cmd/biggz -run 'TestDeriveLensHunks' -count=1 -v` → build failed: `too many arguments in call to deriveLensHunks (have (string, string, string), want (string, review.RiskInput))`.

### Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/review/frozen_inspector.go` | Create | +543 |
| `internal/review/frozen_inspector_test.go` | Create | +368 |
| `cmd/biggz/cli_review.go` | Modify (placeholder `deriveLensHunks` replaced; init doc comment updated) | +15/−12 |
| `cmd/biggz/review_derive_hunks_test.go` | Create | +93 |

### Focused Test Command + Result

```
go test ./internal/review -run 'TestFrozenInspector' -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/internal/review 4.605s
  7 top-level tests (6 assertions suites + harness) + 3 subtests, all PASS:
  MaterializesDocumentationAndBinary, RepoSelectionIsFrozenAgainstCwd,
  UnresolvableInputsRefusedTyped (missing_directory | not_a_git_work_tree |
  unresolvable_candidate_commit), DirtyAndStagedEditsNeverLeak,
  ByteCapsRefuseTyped, RuntimeHarness, UnknownPathAndClosedRefused

go test ./cmd/biggz -run 'TestDeriveLensHunks' -count=1 -v
→ PASS, ok github.com/biggs-100/biggz-ai/cmd/biggz 0.814s
  TestDeriveLensHunksFeedLensFindings (0.66s)
```

### Package Suite Command + Result

```
go test ./internal/review ./internal/sdd ./cmd/biggz -count=1 -timeout 240s
→ ok internal/review  148.884s
→ ok internal/sdd      25.908s
→ ok cmd/biggz         77.605s
```

### Static Checks

| Check | Result |
|-------|--------|
| `go build ./...` | OK |
| `go vet ./...` | OK (exit 0) |
| `go vet -vettool=nosourcegrep ./internal/review ./cmd/biggz` (CI primary linter) | OK (exit 0, no findings) |
| `gofmt -l` on the four touched files | clean |
| CI complexity gate (cyclomatic ≤15 / cognitive ≤20, non-test `internal/review`) | `gocyclo -over 15 ./internal/review` → no offender in `frozen_inspector.go`; `gocognit -over 20 ./internal/review` → no offender in new files. One informational test-file offender (`TestFrozenInspectorMaterializesDocumentationAndBinary`, cyclo 17) — test offenders never block per the CI gate |
| `use-modern-go list --file-path` on all four touched Go files | consulted; applied `slices.Sort`/`slices.Clone`, `strings.Cut`, `errors.Join`, `t.Context()` |

### Runtime Harness Evidence

Command: `go test ./internal/review -run 'TestFrozenInspectorRuntimeHarness' -count=1 -v` → PASS (0.91s), raw output captured.

Scenario: real temp git repository; candidate commit changes a binary blob (`blob.bin`), an executable Markdown file (`scripts/run.md`, mode 100755, shebang) and a text doc (`docs/guide.md`); on top of the committed candidate: a staged edit (`docs/harness-staged.md`), unstaged edits (`docs/guide.md`, `blob.bin`), then three derivation runs — (1) repo cwd, (2) foreign cwd, (3) hostile inherited `GIT_DIR`/`GIT_WORK_TREE`/`GIT_INDEX_FILE` environment.

Result (raw output excerpt):
- frozen trees: base `49cbe845d5596fd54d87ca951c1b2e24d4c9fc14`, candidate `917de60c18e936bfaf8209988b60bf377db6fe12` — identical across all three runs.
- `blob.bin`: 175 bytes, `sha256:b4d91a…95fa6` in all 3 runs; payload = `Binary files a/blob.bin and b/blob.bin differ` (no blob bytes, no `--text`).
- `docs/guide.md`: 218 bytes, `sha256:b8049deb…2cfa7` in all 3 runs; payload carries `+second line`.
- `scripts/run.md`: 227 bytes, `sha256:2cc3dddf…bf44` in all 3 runs; payload carries `new file mode 100755`, `+#!/bin/sh`, `+echo one`.
- no `HARNESS-*` staged/unstaged marker appears in any hunk.
- raw per-path invocation (prefixed by `git --no-pager -C <repo>`): `git -c color.ui=false -c core.attributesFile=<tmp>/neutral-attributes -c diff.external= diff --patch --full-index --no-color --no-ext-diff --no-textconv --no-renames --diff-algorithm=myers --no-indent-heuristic --unified=3 --ignore-submodules=none <baseTree> <candidateTree> -- :(literal)<path>`.

### Rollback Boundary

Revert exactly this unit, no other slice depends on the new symbols yet:
1. delete `internal/review/frozen_inspector.go` and `internal/review/frozen_inspector_test.go`;
2. delete `cmd/biggz/review_derive_hunks_test.go`;
3. restore the `deriveLensHunks` placeholder (empty map) and the original `init()` wiring comment in `cmd/biggz/cli_review.go`.

→ back to the tracker state (d03bb481). No data, migration, store, or config changes; the isolated Git view is a throwaway temp directory removed on `Close`.

### Notes (S1a1, carried forward)

- The tasks artifact is 910 words against the skill's 530-word nominal budget — a recorded deviation (mandated forecast table + 8 work units + 8 phases with the required RED tests); the closest repo precedent is 786 words.
- S1a1 authored ~1,031 changed lines; `size:exception` accepted for that slice (PR 1 merged).
- `--no-renames` is included in the per-path patch invocation (the reference's default; keeps per-path patches aligned with the rename-disabled manifest). The task prompt's literal flag list omitted it; the deviation is deliberate and documented in the code comment.
- Pre-existing, outside the S1a1 surface: `gofmt -l .` flags `internal/review/rdd_helpers.go` locally (toolchain-version artifact; CI's go1.27 toolchain reports clean — confirmed in a later investigation, see BigMem `ci/gofmt-version-artifact-rdd-helpers`).

---

## Remaining Tasks

- Phase 7 (S2): OpenCode plugin verbatim transport + overlays + contract doc (7.1–7.4) — PR 7, base = PR 6 (`fix/rdd-receipt-collection-6-verify-subject`).
- Phase 8 (close): dogfood own receipt, retro-collect `fix-bigmem-recall-friction`, archive, issue #60 (8.1–8.5) — on the tracker after PR 7.
