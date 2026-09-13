# Tasks: wire-pi-review-relay

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~650–750 (250–290 tracked + 390–470 test); per-slice ≤~340 |
| 400-line budget risk | High |
| 800-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 → PR2 (core → CLI) → PR3 (output contract, ~100 lines → Low risk, no size:exception) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Core executor + typed failures (`internal/review`) | PR1 | `go test ./internal/review -run 'ReviewerExecute\|PiRelay' -count=1` | fake `ReviewerRunner`: raw stdout → `Capture`, deadline kill, refusals | revert `reviewer_execute.go` + adapter delta |
| 2 | CLI `--execute` flags/routing (`cmd/biggz`) | PR2 | `go test ./cmd/biggz -run ReviewExecute -count=1` | `pi` shim on PATH: start → execute → finalize → gate | revert `cli_review.go`; other modes intact |

## Phase 1: Core reviewer execute (PR1)

- [x] 1.1 RED `reviewer_execute_test.go`: `ComposeReviewerPrompt` = `shared` ⊕ role ⊕ separator ⊕ `MaterializeReviewerTask`; task segment byte-identical, caller body discarded (REQ-VERBATIM, ~40 LOC)
- [x] 1.2 RED: lens map `risk→r1-risk` … `resilience→r4-resilience` (`shared` first); `performance`/`dependencies` → `role-unavailable`, zero launch/capture (REQ-BOUND, ~30 LOC)
- [x] 1.3 RED: fake `ReviewerRunner` — raw stdout reaches `Capture` unmodified; empty, nonzero exit, > `ArtifactResultLimit`, timeout kill, cancelation → typed kind naming stage, zero capture, no slot (REQ-BOUND, `artifact.go:35`, ~70 LOC)
- [x] 1.4 RED `pi_relay_test.go`: adapter kinds `launch|empty-output|nonzero-exit`; texts preserved (`:96,159,169`) (REQ-BOUND, ~25 LOC)
- [x] 1.5 GREEN `reviewer_execute.go`: `DefaultReviewerTimeout=10m`, `MaxReviewerTimeout=2h`, `ReviewerRunner`, `ComposeReviewerPrompt`, `ExecutePiReview`, cap refusal (REQ-EXEC, ~110 LOC)
- [x] 1.6 GREEN `pi_adapter.go`: `ReviewerFailure{Kind,Stage,Elapsed,ExitCode,StdoutBytes,StderrBytes,Cause}`; `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE` override (REQ-BOUND, ~55 LOC)
- [x] 1.7 Verify: `go test ./internal/review -count=1`

## Phase 2: CLI surface (PR2)

- [x] 2.1 RED `cmd/biggz/review_execute_test.go`: `--execute` requires `--agent pi`; pairwise usage errors with `--input`/`--preflight`/`--materialize`; `--timeout` only with `--execute`, integer 1..7200 (REQ-EXEC, `cli_review.go:1046-1103`, ~60 LOC)
- [x] 2.2 RED: `pi` shim on PATH (`.cmd`/POSIX) replays strict-JSON golden keyed by `--preflight` `subject_hash`; `start → execute → finalize → gate` advances lineage; capture == shim bytes (REQ-VERBATIM, ~90 LOC)
- [x] 2.3 RED: shim nonzero/empty/over-cap or admission-rejected payload → typed refusal, no event, no partial slot; no handshake/RDD refuses pre-materialize, nothing launches (REQ-EXEC, `capture.go:417`, ~65 LOC)
- [x] 2.4 GREEN `cli_review.go`: flags, exclusivity, routing to `ExecutePiReview` after handshake (`:1056`) + RDD (`:1068`), usage/help (`:191`), exit 1 on `ReviewerFailure` (REQ-EXEC, ~75 LOC)
- [x] 2.5 Verify: `go test ./cmd/biggz -run Review -count=1`

## Phase 3: Apply-time verification

- [x] 3.1 Real-`pi` smoke (Windows first): `where pi` + `pi --version`, one real `--execute` run in a disposable repo → exit 0 + artifact; `kind=launch` → retry with `BIGGZ_PI_REVIEW_RELAY_EXECUTABLE`; no real `pi` → SMOKE NOT RUN (REQ-EXEC)
- [x] 3.2 Regressions: `review_materialize_test`, `capture_test`, `rdd_parity` `PluginWiresCapture` green; full `go test ./... -count=1 -timeout 180s` (REQ-VERBATIM)

## Phase 4: Output contract remediation (PR3)

- [x] 4.1 RED `reviewer_execute_test.go`: `composeReviewerPromptWant` to 4 segments; order `shared`<`role`<`contract`<`sep`<`task`; task byte-identical at the end (REQ-VERBATIM, ~25 LOC)
- [x] 4.2 RED: contract read via `assets.FS`; markers `subject_hash`/`inspection`/`findings`/`evidence`/`evidence_class`/`causal_disposition`/one JSON object; asset free of `{{` (REQ-VERBATIM, ~20 LOC)
- [x] 4.3 GREEN `internal/assets/prompts/review-output-contract.md`: D5 shape — `subject_hash` echo, `inspection.status:"completed"` + manifest `paths`, explicit `findings`, non-empty concrete `evidence`, severe findings with `evidence_class`+`causal_disposition`, exactly one JSON object (REQ-VERBATIM, ~45 L)
- [x] 4.4 GREEN `reviewer_execute.go`: `ReviewerOutputContractAsset` const + contract segment before the separator; rollback = revert segment + delete asset (REQ-VERBATIM, +12 LOC)
- [x] 4.5 OPTIONAL `cmd/biggz/cli_review.go:1197-1199`: classify admission refusals instead of plain `error:` lines, only if ≤PR3 budget (REQ-BOUND, ~8 LOC)
- [x] 4.6 Smoke real `pi`, fresh lineage, disposable `%TEMP%\biggz-smoke-relay` (`review-97c628c61d06e295` spent): expect exit 0 + admitted artifact (`--timeout 180`); rejection → design diagnosis ladder (REQ-EXEC)
- [x] 4.7 Regressions `go test ./... -count=1 -timeout 180s`; on 4.6 pass, close 3.1 and run 3.2 (REQ-VERBATIM)
