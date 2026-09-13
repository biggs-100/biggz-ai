# Apply Progress: wire-pi-review-relay

## Slice PR1 — Phase 1: Core reviewer execute (`internal/review`)

**Status**: tasks 1.1–1.7 implemented and verified green. **Budget exception required** — see Budget.
**Work unit**: `slice-pr1-core` · ledger token `tok-aeb8cc848376470a00e49d56` · request-id `req-pr1-core`.
**Mode**: Standard (no `strict_tdd`); repo convention honored — RED test first, then GREEN.
**Store**: hybrid — this file + BigMem mirror `sdd/wire-pi-review-relay/apply-progress` (orchestrator).
**PR boundary**: PR1 = `internal/review` core only; `cmd/biggz` CLI, CLI tests, opencode assets/plugins and spec/design/tasks artifacts untouched (PR2 / frozen inputs).

### Tasks

- [x] 1.1 RED `reviewer_execute_test.go`: `ComposeReviewerPrompt` == `shared` ⊕ role ⊕ separator ⊕ `MaterializeReviewerTask`; task segment byte-identical (`HasSuffix` + full byte equality); caller-authored body discarded (exact binary composition, no other channel).
- [x] 1.2 RED: lens map `risk→r1-risk`, `readability→r2-readability`, `reliability→r3-reliability`, `resilience→r4-resilience` (`shared` first); `performance`/`dependencies` → typed `role-unavailable` (stage `role`) with zero launch (runner calls `0`) and zero capture (store byte-unchanged + head pinned).
- [x] 1.3 RED: fake `ReviewerRunner` — raw stdout reaches `Capture` unmodified (runner prompt equality + `Capture` replay idempotency proof); empty stdout, non-zero exit, `> ArtifactResultLimit`, timeout and cancelation each typed naming the stage, zero capture, no partial slot.
- [x] 1.4 RED `pi_relay_test.go`: adapter kinds `launch|empty-output|nonzero-exit` (+ `timeout` in the POSIX deadline test); existing error texts preserved (`:96,:159,:169` — assert at shifted lines).
- [x] 1.5 GREEN `reviewer_execute.go`: `DefaultReviewerTimeout = 10m`, `MaxReviewerTimeout = 2h`, `ReviewerRunner`, `ComposeReviewerPrompt`, `ExecutePiReview`, over-cap refusals (stdout cap typed; over-max timeout refused pre-launch).
- [x] 1.6 GREEN `pi_adapter.go`: `ReviewerFailure{Kind,Stage,Elapsed,ExitCode,StdoutBytes,StderrBytes,Cause}` + kind/stage constants; `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE` absolute-path override executed directly (never a shell; relative value refuses typed).
- [x] 1.7 Verify: `go test ./internal/review -count=1` green.

### Files changed

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/review/reviewer_execute.go` | create | 143 | 0 | 143 |
| `internal/review/pi_adapter.go` | modify | 107 | 11 | 118 |
| `internal/review/reviewer_execute_test.go` | create | 230 | 0 | 230 |
| `internal/review/pi_relay_test.go` | modify | 87 | 4 | 91 |
| **Tracked subtotal** | | 250 | 11 | **261** |
| **Test subtotal** | | 317 | 4 | **321** |
| **Total changed** | | **567** | **15** | **582** |

`apply-progress.md` (this artifact) is a phase doc, not counted in the code changed-line numbers.

### Budget

Slice budget: **400** changed lines (`additions + deletions`). Actual: **582** (261 tracked + 321 test) — **over by 182 (≈45%)**.

The `tasks.md` forecast (`~40/30/70/25/110/55`) under-counted the cohesive RED coverage that REQ-BOUND/REQ-VERBATIM demand (five typed-failure cases each with a zero-capture proof, the replay-based unmodified-bytes proof, the override matrix, and the shared fixtures/helpers). Requested resolution (orchestrator/maintainer decision, not resolved by the executor):

1. `size:exception` for PR1 (recommended — the slice is a single coherent, verified unit), or
2. re-slice (e.g. move the override + adapter typed-kind tests to a micro follow-up; the `ReviewerFailure` type itself must stay with PR1 because the executor emits `role-unavailable`/`output-over-cap`).

### Evidence

| Evidence | Command | Result |
|---|---|---|
| RED (test-first) | `go test ./internal/review -run 'ComposeReviewerPrompt\|ExecutePiReview\|PiAdapter' -count=1` | `FAIL [build failed]` — undefined: `ReviewerFailureKind`, `ReviewerFailureEmptyOutput`, `ReviewerFailure`, `ReviewerStagePi`, `ReviewerFailureTimeout`, `ReviewerFailureLaunch` (+more) |
| Focused GREEN | `go test ./internal/review -run 'TestComposeReviewerPrompt\|TestExecutePiReview\|TestPiAdapter\|TestPiRelayHandshake' -count=1 -v` | `ok` 7.562s — 17/17 top-level PASS |
| Full package (1.7) | `go test ./internal/review -count=1 -timeout 300s` | `ok` 140.452s (baseline before changes: `ok` 131.537s) |
| Static | `go vet ./internal/review` · `go build ./...` | OK · OK |
| Runtime harness | fake `ReviewerRunner` + real lineage store (`captureFixture`): valid payload → `Capture` persists slot and advances head; empty/over-cap/nonzero/timeout/cancel → typed failure, store byte-unchanged, head pinned at expected revision | PASS — `TestExecutePiReview_RawStdoutReachesCapture`, `TestExecutePiReview_TypedFailuresCaptureNothing` |
| Rollback boundary | delete `internal/review/reviewer_execute.go` + revert the `pi_adapter.go` delta (plain transport errors) + revert the two test files' deltas — additive only, no schema/state change, no other caller depends on the new symbols | n/a |

Focused PASS list: `TestPiRelayHandshake_*` (×6), `TestPiAdapter_Review_WithFakeBinary`, `TestPiAdapter_Review_ScratchDirIsolation`, `TestPiAdapter_Review_MissingBinary`, `TestPiAdapter_Review_NonzeroExitIsTyped`, `TestPiAdapter_Review_ExecutableOverride`, `TestPiAdapter_Review_FlagsAreComplete`, `TestComposeReviewerPrompt_SharedThenRoleThenByteIdenticalTask`, `TestExecutePiReview_RoleUnavailableRefusesWithoutLaunchOrCapture`, `TestExecutePiReview_RawStdoutReachesCapture`, `TestExecutePiReview_TypedFailuresCaptureNothing`, `TestExecutePiReview_BoundsAndOverMaxTimeout`.

### Deviations

- `tasks.md` checkboxes intentionally untouched: spec/design/tasks artifacts were frozen for this slice by the orchestrator; per-task state lives here and the ledger is settled by the orchestrator.
- `pi_relay_test.go` line numbers shifted (+5) after the new imports; preserved error texts assert at their new lines.
- `MaxReviewerTimeout` is enforced by the executor as defense-in-depth (plain error, no typed kind); PR2's CLI usage validation (`--timeout` integer `1..7200`) remains the user-facing gate.
- Adapter timeout/canceled typed kinds are additionally covered cross-platform through the executor tests; the adapter's own deadline test remains POSIX-only (pre-existing skip on Windows).

## Slice PR2 — Phase 2: CLI execute surface (`cmd/biggz`)

**Status**: tasks 2.1–2.5 implemented and verified green. **Budget**: 430 changed lines vs the 400 slice budget (over by 30) → `size:exception` needed at delivery, same as PR1.
**Work unit**: `slice-pr2-cli` · ledger token `tok-e9493607df1cde8dab0f2d61` · request-id `req-pr2-cli`.
**Provenance (honest)**: the delegated apply run was cut off by the harness timeout (20 min) while it was capturing evidence, after the code was written and its tests had gone green. Its own captured output was lost with the timeout, so **the orchestrator finished the bookkeeping** (the 2.1–2.5 checkboxes in `tasks.md` and this section) and re-ran every command below; those orchestrator runs are the evidence of record for this slice.

### Tasks

- [x] 2.1 RED `cmd/biggz/review_execute_test.go`: `--execute` requires `--agent pi`; pairwise usage errors with `--input`/`--preflight`/`--materialize`; `--timeout` only with `--execute`, integer 1..7200 (`TestReviewExecuteUsageMatrix`).
- [x] 2.2 RED: `pi` shim on PATH replays the strict-JSON golden keyed by the `--preflight` `subject_hash`; `start → execute → finalize → gate` advances the lineage; captured bytes equal the shim stdout (`TestReviewExecuteShimPipeline`).
- [x] 2.3 RED: shim non-zero / empty / over-cap, and an admission-rejected payload → typed refusal, no appended event, no partial slot; missing handshake or disabled RDD refuses before materialization with nothing launched (`TestReviewExecuteRefusals` with its four subtests).
- [x] 2.4 GREEN `cmd/biggz/cli_review.go`: flags, usage exclusivity validated immediately after parsing (before the handshake and RDD gates, so conflicting flags never depend on the environment), `--timeout` parsing against `MaxReviewerTimeout`, routing to `ExecutePiReview` after the handshake + RDD gates, `signal.NotifyContext` cancelation, typed-failure printing (`kind/stage/elapsed/exit_code/stdout_bytes/stderr_bytes` + "no capture was performed") with exit 1, usage/help updates.
- [x] 2.5 Verify: `go test ./cmd/biggz -run Review -count=1` green.

### Files changed

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `cmd/biggz/cli_review.go` | modify | 87 | 13 | 100 |
| `cmd/biggz/review_execute_test.go` | create | 330 | 0 | 330 |
| **Tracked subtotal** | | 87 | 13 | **100** |
| **Test subtotal** | | 330 | 0 | **330** |
| **Total changed** | | **417** | **13** | **430** |

### Evidence

| Evidence | Command | Result |
|---|---|---|
| CLI slice tests (2.5) | `go test ./cmd/biggz -run Review -count=1` | `ok` 29.139s |
| Static | `go vet ./cmd/biggz` · `go build ./...` | OK · OK |
| Change surface | `git status --short` | only `cmd/biggz/cli_review.go` (modified) and `cmd/biggz/review_execute_test.go` (new) for this slice; `internal/review/**` untouched after PR1 |
| Rollback boundary | revert the `cli_review.go` delta and delete `review_execute_test.go` — `--preflight`/`--materialize`/`--input` paths and their help text semantics intact |

### Deviations

- Orchestrator-completed bookkeeping (see Provenance): `tasks.md` checkboxes and this section were written by the orchestrator, not the interrupted apply run.
- Budget: 430 vs 400 — the CLI test file (330 lines) carries the shim harness; no re-slice proposed because PR1 already carries the same `size:exception` decision and both slices are coherent units.
- The real-`pi` smoke and the full-repo regression sweep remain Phase 3 (3.1/3.2).

## Slice PR3 — Phase 4: Output contract remediation (`internal/review` + asset)

**Status**: tasks 4.1–4.5 implemented and verified green. **Budget**: 145 changed lines vs the 200-line slice cap (design estimated ≈100) — within budget.
**Work unit**: `slice-pr3-output-contract` · ledger token `tok-b27761657dc85e07c93db222` · request-id `req-pr3-contract`.
**Mode**: Standard (no `strict_tdd`); repo convention honored — RED test first, then GREEN.
**Store**: hybrid — this file + BigMem mirror `sdd/wire-pi-review-relay/apply-progress`.
**PR boundary**: PR3 = output contract (asset + prompt segment) + CLI admission classification. `specs/**`, `design.md`, `proposal.md`, `internal/review/artifact.go`, opencode assets/plugins and the PR1/PR2 sections remain untouched. No real-`pi` smoke (4.6) and no commit — orchestrator-owned.

### Tasks

- [x] 4.1 RED `reviewer_execute_test.go`: `composeReviewerPromptWant` now composes 4 segments (shared ⊕ role ⊕ contract ⊕ separator ⊕ task); the test asserts the order `shared@0 < role < contract < separator < task` via `bytes.Index`, keeps `HasSuffix` + full byte equality, and pins the task at the very end (`taskAt = len(prompt) − len(task)`).
- [x] 4.2 RED: `reviewerContract(t)` reads the asset through `assets.FS` (`ReviewerOutputContractAsset`); `TestComposeReviewerPrompt_CarriesOutputContract` asserts the markers (`subject_hash`, `inspection`, `findings`, `evidence`, `evidence_class`, `causal_disposition`, `exactly one JSON object`), rejects `{{` (never a Go template), and proves the composed prompt carries the contract bytes verbatim.
- [x] 4.3 GREEN `internal/assets/prompts/review-output-contract.md` (created, 42 lines / 3,620 B): D5 shape — verbatim `subject_hash` echo from `GENTLE_AI_REVIEW_BINDING`, `inspection.status:"completed"` over every frozen manifest path (each once, ascending), explicit `findings` (`[]` = clean), non-empty concrete `evidence`, stable `R<n>-NNN` ids with in-manifest `path:line` locations, severe findings requiring `evidence_class` + `causal_disposition`, unknown keys rejected, exactly one JSON object with no prose/fences, one canonical JSON example.
- [x] 4.4 GREEN `reviewer_execute.go`: `ReviewerOutputContractAsset` const; the contract is read via `assets.FS` and appended between the role text and the separator (`shared ⊕ "\n" ⊕ role ⊕ contract ⊕ "\n---\n" ⊕ task`); timeout bounds, typed failures and the over-cap refusal are unchanged.
- [x] 4.5 OPTIONAL `cmd/biggz/cli_review.go`: the execute branch classifies `*review.ArtifactAdmissionError` as `error: reviewer artifact admission refused: decision=… diagnostic=… no capture was performed` instead of the plain `error:` line; wrapped admission errors from `captureValidatePayload` keep the plain line the design's diagnosis ladder expects.

### Files changed

| File | Action | + | − | Changed |
|---|---|---|---|---|
| `internal/assets/prompts/review-output-contract.md` | create | 42 | 0 | 42 |
| `internal/review/reviewer_execute.go` | modify | 18 | 7 | 25 |
| `cmd/biggz/cli_review.go` | modify | 7 | 1 | 8 |
| **Tracked subtotal** | | 67 | 8 | **75** |
| `internal/review/reviewer_execute_test.go` | modify | 61 | 9 | 70 |
| **Test subtotal** | | 61 | 9 | **70** |
| **Total changed** | | **128** | **17** | **145** |

Delta vs the PR1/PR2 baseline was measured with `git diff --no-index --numstat` against a reconstructed pre-slice copy (the two Go files are still untracked; `cli_review.go` was diffed against a copy with the 4.5 edit reverted). No other lines changed.

### Evidence

| Evidence | Command | Result |
|---|---|---|
| RED (test-first) | `go test ./internal/review -run 'TestComposeReviewerPrompt' -count=1` | `FAIL [build failed]` — `undefined: ReviewerOutputContractAsset` (`reviewer_execute_test.go:54:38`) |
| Focused GREEN | `go test ./internal/review -run 'TestComposeReviewerPrompt' -count=1 -v` | `ok` 3.481s — 2/2 top-level PASS (4 lens subtests + `TestComposeReviewerPrompt_CarriesOutputContract`) |
| Package | `go test ./internal/review -count=1 -timeout 300s` | `ok` 150.790s |
| CLI slice + shim runtime | `go test ./cmd/biggz -run Review -count=1` | `ok` 29.357s — includes `TestReviewExecuteShimPipeline` (real `pi` shim on PATH: start → execute → finalize → gate; shim stdin closes with the byte-identical task) and the admission-rejection case of `TestReviewExecuteRefusals` |
| Blast radius (template counts) | `go test ./internal/review/lens ./internal/skillregistry ./internal/components ./internal/assets/... -count=1` | all `ok` |
| Static | `go vet ./internal/review ./cmd/biggz` · `go build ./...` | OK · OK |
| Runtime harness (allowed scope) | `pi` shim replaying a golden keyed by the preflight `subject_hash` — `TestReviewExecuteShimPipeline` | PASS — admission `completed` with the contract-carrying prompt; the real-`pi` smoke (4.6) stays with the orchestrator |
| Rollback boundary | Revert the const + segment in `reviewer_execute.go` (+18/−7), delete the asset (42 L), revert the test delta (+61/−9) and the CLI classification (+7/−1) — additive only, no schema or state change, no other caller depends on the new symbol | n/a |

### Deviations

- 4.5 was included (optional) within budget; the typed line keeps the `admission`/`binding_mismatch` substrings asserted by `TestReviewExecuteRefusals` (its file `cmd/biggz/review_execute_test.go` is outside the allowed surface and was not touched).
- Wrapped admission errors from `captureValidatePayload` (`reviewer artifact admission incomplete: reviewer payload contains no complete JSON object`) still print as a plain `error:` line — that is the current failure signature the design diagnosis ladder expects; only the `*ArtifactAdmissionError` from `Admit` is typed.
- 4.6 (real-`pi` smoke) and 4.7 (full-repo regressions) were NOT run: orchestrator-owned.
- Final asset size is 3,620 B (design estimated ≈1.2–1.8 KB); the smoke prompt moves from 2,936 B to ≈6.5 KB (+0.23% over the 1.58 MB field datapoint) — D3's timeout decision is unchanged.

## Phase 4 verification — real-`pi` smoke (orchestrator, task 4.6)

**Result: PASS.** `capture-result --agent pi --execute` completed against the real `pi` reviewer and the artifact was admitted.

| Step | Command | Observed |
|---|---|---|
| Prerequisites | `where pi` · `pi --version` | `C:\Users\USER\AppData\Roaming\npm\pi` (+ `pi.cmd`) · `0.85.1` |
| Start | `review start --agent pi --subject subject.json --consent granted` | lineage `review-d8e94d2af673b09d`, candidate `a29f2fa`, tier medium, lens `risk`, `expected_revision ced96708…` |
| Execute | `review capture-result --agent pi --execute --timeout 180 …` | **exit 0** — `biggz-ai.review-result-artifact/v1`, `admission_decision: completed`, `revision ae997825…`, `canonical_sha256 sha256:3f46ec57…` |
| Finalize | `review finalize review-d8e94d2af673b09d` | receipt `receipts\0ee34e0f…json`, hash `sha256:f0ee681b…`, revision `f8e2745d…` |
| Gate | `review gate pre-pr review-d8e94d2af673b09d --json` | `allowed: true`, `delivery: burned/unmanaged` (ephemeral receipt burned by policy) |

**C33 resolved empirically (no override needed)**: `exec.LookPath("pi")` resolves the Windows npm shim (`pi.cmd`) and `PiAdapter` spawns it correctly — the `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE` escape hatch was not required.

### Diagnostic trail (why the first post-PR3 smoke failed)

The first re-run of `capture-result --execute` after PR3 still returned `reviewer artifact admission incomplete: reviewer payload contains no complete JSON object`. The failure was **not** in the code: `./biggz.exe` had been built at 17:57 while `internal/assets/prompts/review-output-contract.md` was created at 19:54, and the reviewer prompt is composed from the **embedded** asset FS (`assets.FS`), so the stale binary composed the pre-PR3 prompt (no contract) and the reviewer answered without the admitted shape. Rebuilding the CLI resolved it.

Independent evidence collected before the rebuild: 3 direct `pi --print …` invocations plus 5 real `PiAdapter.Review` calls (via a throwaway diagnostic test, deleted afterwards) all returned a single valid JSON object (`json.Valid=true`, `ExtractBoundedSingleJSONObject` → `completed`), proving the adapter, the composition and the contract asset were correct while the binary was stale.

**Lesson for apply/verify**: any change under `internal/assets/**` requires rebuilding the CLI binary before an end-to-end run — the embedded FS is frozen at build time.

### Phase 4 regressions (task 4.7)

| Evidence | Command | Result |
|---|---|---|
| Named regressions | `go test ./internal/review -run 'TestMaterialize|TestCapture|TestPluginWiresCapture|TestRDDParity' -count=1` | `ok` 13.075s |
| Full repo sweep | `go test ./... -count=1 -timeout 900s` | **exit 0** — 60 packages `ok`, 0 `FAIL`, 0 panics |
| Timeout note (pre-existing) | `go test ./... -count=1 -timeout 180s` | panics with `test timed out after 3m0s` in `internal/review` (`TestStoreGitCommonDir`) because that package alone runs ~150s; the 900s ceiling is the honest setting on this machine. The same flake is documented from earlier sessions (Windows CI reruns). |
