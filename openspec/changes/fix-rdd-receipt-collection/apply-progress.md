# Apply Progress: fix-rdd-receipt-collection

## Batch

| Field | Value |
|-------|-------|
| Slice | S1a1 = Phase 1 only (tasks 1.1–1.6): frozen-tree inspector + real hunk derivation |
| Mode | Standard (`strict_tdd: false`) with the mandated RED-first order for the threat-matrix cases |
| PR | PR 1 of the feature-branch-chain (base: tracker `fix/rdd-receipt-collection` @ d03bb481; branch `fix/rdd-receipt-collection-1-frozen-inspector`) |
| Date | 2026-09-11 |
| Store mode | hybrid (tasks.md `[x]` + BigMem `sdd/fix-rdd-receipt-collection/apply-progress`) |

## Tasks Completed

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

## Files Changed

| File | Action | +/- |
|------|--------|-----|
| `internal/review/frozen_inspector.go` | Create | +543 |
| `internal/review/frozen_inspector_test.go` | Create | +368 |
| `cmd/biggz/cli_review.go` | Modify (placeholder `deriveLensHunks` replaced; init doc comment updated) | +15/−12 |
| `cmd/biggz/review_derive_hunks_test.go` | Create | +93 |

## Focused Test Command + Result

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

## Package Suite Command + Result

```
go test ./internal/review ./internal/sdd ./cmd/biggz -count=1 -timeout 240s
→ ok internal/review  148.884s
→ ok internal/sdd      25.908s
→ ok cmd/biggz         77.605s
```

## Static Checks

| Check | Result |
|-------|--------|
| `go build ./...` | OK |
| `go vet ./...` | OK (exit 0) |
| `go vet -vettool=nosourcegrep ./internal/review ./cmd/biggz` (CI primary linter) | OK (exit 0, no findings) |
| `gofmt -l` on the four touched files | clean |
| CI complexity gate (cyclomatic ≤15 / cognitive ≤20, non-test `internal/review`) | `gocyclo -over 15 ./internal/review` → no offender in `frozen_inspector.go`; `gocognit -over 20 ./internal/review` → no offender in new files. One informational test-file offender (`TestFrozenInspectorMaterializesDocumentationAndBinary`, cyclo 17) — test offenders never block per the CI gate |
| `use-modern-go list --file-path` on all four touched Go files | consulted; applied `slices.Sort`/`slices.Clone`, `strings.Cut`, `errors.Join`, `t.Context()` |

## Runtime Harness Evidence

Command: `go test ./internal/review -run 'TestFrozenInspectorRuntimeHarness' -count=1 -v` → PASS (0.91s), raw output captured.

Scenario: real temp git repository; candidate commit changes a binary blob (`blob.bin`), an executable Markdown file (`scripts/run.md`, mode 100755, shebang) and a text doc (`docs/guide.md`); on top of the committed candidate: a staged edit (`docs/harness-staged.md`), unstaged edits (`docs/guide.md`, `blob.bin`), then three derivation runs — (1) repo cwd, (2) foreign cwd, (3) hostile inherited `GIT_DIR`/`GIT_WORK_TREE`/`GIT_INDEX_FILE` environment.

Result (raw output excerpt):
- frozen trees: base `49cbe845d5596fd54d87ca951c1b2e24d4c9fc14`, candidate `917de60c18e936bfaf8209988b60bf377db6fe12` — identical across all three runs.
- `blob.bin`: 175 bytes, `sha256:b4d91a…95fa6` in all 3 runs; payload = `Binary files a/blob.bin and b/blob.bin differ` (no blob bytes, no `--text`).
- `docs/guide.md`: 218 bytes, `sha256:b8049deb…2cfa7` in all 3 runs; payload carries `+second line`.
- `scripts/run.md`: 227 bytes, `sha256:2cc3dddf…bf44` in all 3 runs; payload carries `new file mode 100755`, `+#!/bin/sh`, `+echo one`.
- no `HARNESS-*` staged/unstaged marker appears in any hunk.
- raw per-path invocation (prefixed by `git --no-pager -C <repo>`): `git -c color.ui=false -c core.attributesFile=<tmp>/neutral-attributes -c diff.external= diff --patch --full-index --no-color --no-ext-diff --no-textconv --no-renames --diff-algorithm=myers --no-indent-heuristic --unified=3 --ignore-submodules=none <baseTree> <candidateTree> -- :(literal)<path>`.

## Rollback Boundary

Revert exactly this unit, no other slice depends on the new symbols yet:
1. delete `internal/review/frozen_inspector.go` and `internal/review/frozen_inspector_test.go`;
2. delete `cmd/biggz/review_derive_hunks_test.go`;
3. restore the `deriveLensHunks` placeholder (empty map) and the original `init()` wiring comment in `cmd/biggz/cli_review.go`.

→ back to the tracker state (d03bb481). No data, migration, store, or config changes; the isolated Git view is a throwaway temp directory removed on `Close`.

## Notes

- The tasks artifact is 910 words against the skill's 530-word nominal budget — a recorded deviation (mandated forecast table + 8 work units + 8 phases with the required RED tests); the closest repo precedent is 786 words.
- **Workload overrun (needs a decision at PR time):** S1a1 authors ~1,031 changed lines (543 impl + 461 tests + 27 cli diff) against the tasks forecast “largest slice ≤~420” and the 400-line review budget. PR creation is the orchestrator's step, so this is flagged, not decided: either an explicit `size:exception` acceptance or a re-slice (e.g. moving the isolation/env helpers into the materializer slice) is required before opening PR 1.
- `--no-renames` is included in the per-path patch invocation (the reference's default; keeps per-path patches aligned with the rename-disabled manifest). The task prompt's literal flag list omitted it; the deviation is deliberate and documented in the code comment.
- Pre-existing, outside the allowed surface: `gofmt -l .` flags `internal/review/rdd_helpers.go` (the blob at d03bb481 is also not gofmt-clean — indentation at ~line 143). This slice did not touch it, but CI's repo-wide format job will keep failing until it is fixed on the tracker branch.
- No prior apply-progress batch existed for this change (BigMem search returned none); this file starts the record.

## Remaining Tasks

- Phase 2 (S1a2): materializer + `--materialize` (2.1–2.5) — PR 2, base = this PR.
- Phases 3–8 (S1b1, S1b2, S1c, S1d, S2, dogfood close) untouched.
