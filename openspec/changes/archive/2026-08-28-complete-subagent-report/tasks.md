# Tasks: Complete Subagent Report

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 650–900 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 → PR2 → PR3 stacked-to-main |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

## Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Template + strict gate + read-loop + alias | PR1 base `main` | `go test ./internal/assets/biggz -run TestOrchestrator` + `node --test biggz-synthesis-gate.test.mjs` | Pi: missing→isError, rich→pass | Revert `biggz-orchestrator.md`, `biggz-synthesis-gate.js`, `synthesis.go` |
| 2 | Failure synthesis + envelope validation + ownership | PR2 base PR1 | `go test ./internal/sdd -run TestValidate\|TestSynthesis` | Pi: thin+advise→concern, envelope reject→fallback, general→bypass | Revert `question.go` + Failure add in `synthesis.go` |
| 3 | Pending dual-write + fallback + E2E | PR3 base PR2 | `go test ./internal/sdd -run TestPending` + `go vet && go test` | Pi: compaction reload→fallback re-emit | Revert `pending.go` + `state.yaml` entry; delete `sdd/*/pending-question` |

## Phase 1: PR1 — Template, Gate, Read-Loop, Alias

- [x] 1.1 Modify `internal/assets/biggz/biggz-orchestrator.md` — 4 markers + `INVALID` + `REMINDER≥12`; add 6 optional omit-empty `Preview/Diff/Decisions/Commands/Validation/Failure`
- [x] 1.2 Modify `internal/assets/pi/biggz-synthesis-gate.js` — `hasSynthesis` 4-marker strict; `checkSynthesisPrecondition` uses `currentTurnMarkdown` only for block `isError:true`; `getCurrentTurnSynthesis` fallback for advise only; add `isCheckpointAsk`, `isThinSynthesis(<2‖<50)`, `PI_SUBAGENT_CHILD` bypass
- [x] 1.3 Create `internal/sdd/synthesis.go` — `RenderSynthesis` 4+6 omit-empty compat; `ReadLoop` paginated `read(offset,limit)` verify >50KB retry once
- [x] 1.4 Enforce `engram==bigmem` alias in template and `internal/sdd/*.go`; drift must fail `orchestrator.test.go`
- [x] 1.5 Tests PR1 — `orchestrator.test.go` markers+Preview+INVALID+REMINDER+alias; `gate.test.mjs` missing→isError no-handler, rich→pass

## Phase 2: PR2 — Failure, Validation, Ownership

- [x] 2.1 Create `internal/sdd/question.go` — `ValidateQuestionEnvelope` limits `header≤16, label≤60, qs≤4, opts∈[2,4]` reject `isError:true`; `FormatFallback` plain markdown
- [x] 2.2 Enhance `synthesis.go` `RenderSynthesis` — failure JSON → human `**Failure:**` summary (not raw JSON)
- [x] 2.3 Single ownership — gate `isCheckpointAsk` blocks sub-agent/Pi checkpoint asks when synthesis missing; only orchestrator emits them
- [x] 2.4 Tests PR2 — `TestValidate` rejects 17/61/5/1-opt and allows 12/≤60/3×3; `TestSynthesis` humanized Failure; JS envelope reject→fallback + thin advise/general bypass

## Phase 3: PR3 — Pending Dual-Write, Compaction

- [x] 3.1 Create `internal/sdd/pending.go` — `PendingQuestion` `biggz-ai.pending-question/v1`; `SavePendingDualWrite` dual-write BigMem+`state.yaml` equality retry; `VerifyEquality`, `LoadOnCompaction`
- [x] 3.2 Wire orchestrator — persist `SavePendingDualWrite` before ask; reload `LoadOnCompaction` and re-emit fallback markdown when UI unavailable
- [x] 3.3 Tests PR3 — `TestPending` dual-write equality + compaction fallback (temp store); `TestReadLoop` >50KB

## Phase 4: Verification & CI

- [x] 4.1 CI `go vet ./...` + `go test ./...` + `node --check` + `node --test biggz-synthesis-gate.test.mjs` green (covers loop/envelope/pending/alias) — `go vet` PASS (hash e3b0c44), `go test $(go list ./... | grep -v e2e)` PASS (38 pkgs, hash c516c4a), `node --test` 20/20 PASS (hash ba3120f), combined 81d5af7; e2e WARNING duplicate binary excluded
- [x] 4.2 E2E Pi harness — thin/rich/failure+truncated→fallback re-emit end-to-end — gate.test.mjs covers missing→isError, thin+advise→concern, general bypass, envelope reject→fallback, ownership; pending fallback re-emit verified via TestPendingCompactionFallback + LoadPendingFallback
- [x] 4.3 Verify slices — `git diff --stat` each PR <400 lines; bases PR1→main PR2→PR1 PR3→PR2 diff clean — PR1 381 (b8a3d1d→a5b1afd) <400, PR2 400 (a5b1afd→867d54b) =budget, PR3 334 (867d54b→dff57bd) <400 stacked-to-main

*Threat Matrix: N/A — in-process JS interception + BigMem/state.yaml only.*
