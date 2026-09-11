# Apply Progress: fix-rdd-receipt-collection

## Cumulative Summary

| Slice | Tasks | Status | PR |
|-------|-------|--------|----|
| S1a1 | Phase 1 (1.1–1.6): frozen inspector + real hunk derivation | done | PR 1 (base: tracker `fix/rdd-receipt-collection`; merged as 362c4c18) |
| S1a2 | Phase 2 (2.1–2.5): materializer + `--materialize` | done | PR 2 (base: `fix/rdd-receipt-collection-2-materializer`, off tracker 362c4c18) |

Progress: **11/33 tasks** (Phase 1 + Phase 2). Remaining: Phases 3–8 (S1b1, S1b2, S1c, S1d, S2, dogfood close).

---

## Batch S1a2 — Materializer + `--materialize` (current)

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

- Phase 3 (S1b1): identity + start canonicalization (3.1–3.3) — PR 3, base = PR 2.
- Phases 4–8 (S1b2, S1c, S1d, S2, dogfood close) untouched.
