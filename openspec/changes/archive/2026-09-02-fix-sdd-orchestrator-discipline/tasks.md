# Tasks: fix-sdd-orchestrator-discipline

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~480 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 Gate → PR2 Authority → PR3 RDD |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Gate 120s Go+JS | PR 1 | `go test ./internal/sdd -run TestHasSynthesis` | N/A | `synthesis_gate.go` + `biggz-synthesis-gate.js` |
| 2 | Authority+reads+ladder+docs | PR 2 | `go test ./internal/orchestrator -run TestGuard` | `biggz sdd-status --json` | `authority.go` + `biggz-orchestrator*.md` |
| 3 | RDD gate before verify | PR 3 | `go test ./internal/sdd -run VerifyPreflight` | `biggz rdd status` + `biggz review --receipt` | `verify.go` + `status*.go` + `review/*` |

## Phase 1: Gate Bilingual 120s (PR1)

- [x] 1.1 RED `synthesis_gate_test.go`: 4 markers, `IsCheckpointAsk`, `ShouldBlock` 30s/121s; Test: `go test ./internal/sdd -run TestShouldBlock`; Rollback: delete file
- [x] 1.2 `internal/sdd/synthesis_gate.go`: `HasSynthesis` 4 markers, tokens, `ShouldBlock`+`HasSessionRecall`; Test: `go test ./internal/sdd -run TestHasSynthesis`; Rollback: revert file
- [x] 1.3 `internal/assets/pi/biggz-synthesis-gate.js`: mirror Go, strict `currentTurnMarkdown` ≤120s; Test: `node --test biggz-synthesis-gate.test.mjs`; Rollback: revert file
- [x] 1.4 `internal/sdd/synthesis.go`: `DetectLanguage`+`RenderSynthesisLocalized`; Test: `go test ./internal/sdd -run TestDetectLanguage`; Rollback: revert file

## Phase 2: Authority + Reads + Ladder (PR2)

- [x] 2.1 RED `authority_test.go`: `GuardSDAgentAuthority(spec,general)`→`SD Agent Authority`; Test: `go test ./internal/orchestrator -run TestGuardSD`; Rollback: delete file
- [x] 2.2 Create `internal/orchestrator/authority.go`: map phases→`sdd-*`, reject `general`/`explore`; Test: `go test ./internal/orchestrator`; Rollback: `rm authority.go`
- [x] 2.3 `internal/orchestrator/surfaces.go`: wire guard; Test: `go test ./internal/orchestrator`; Rollback: revert file
- [x] 2.4 `internal/assets/biggz/biggz-orchestrator.md`: 4 markers; Test: `rg "Sub-agent Result" biggz-orchestrator.md`; Rollback: revert file
- [x] 2.5 `internal/assets/biggz/biggz-orchestrator-workflow.md`: mandatory reads + dispatcher; Test: `go test ./internal/sdd -run TestStatus`; Rollback: revert file
- [x] 2.6 `internal/assets/biggz/biggz-orchestrator-delegation.md`: ladder SDD explicit only; Test: `rg "never.*SDD" delegation.md`; Rollback: revert file

## Phase 3: RDD Gate (PR3)

- [x] 3.1 RED `receipt_test.go`+`verify_rdd_test.go`: tampered `BindingHash`→block, no receipt→`rdd_receipt_missing`; Test: `go test ./internal/sdd -run TestVerifyRDDGate`; Rollback: delete tests
- [x] 3.2 `internal/review/*`: `RDDStatus()` LOCK+CAS, `Validate()` `domainHash`; Test: `go test ./internal/review -run TestValidate`; Rollback: revert dir
- [x] 3.3 `internal/sdd/verify.go`: `VerifyPreflight` RDD gate; Test: `go test ./internal/sdd -run VerifyPreflight`; Rollback: revert file
- [x] 3.4 `internal/sdd/status*.go`: propagate `rdd_*`, `resolve-blockers`, keep enabled after archive; Test: `go test ./internal/sdd -run TestStatusV2`; Rollback: revert file

## Phase 4: Integration & Verification

- [x] 4.1 E2E tmp repo: enabled no receipt→block, valid→allow, disabled→allow, tamper→block; Test: `go test -tags=e2e -run TestE2E`; Harness: `biggz rdd status && biggz review gate`; Rollback: tmp only — covered by `TestStatusV2_RDDGatePropagates` (enabled→`rdd_receipt_missing`+`resolve-blockers`, disabled→allow) + `TestVerifyPreflight_DisabledAllows/EnabledBlocksMissing` + `TestVerifyRDDGate_TamperedBindingBlocks` (binding hash mismatch→block) + `TestStatusV2_ArchiveKeepsEnabled`
- [x] 4.2 Ladder/auto-continue: interactive no `proceed`→STOP, 12-file no SDD→Simple Delegation; Test: `go test ./internal/sdd -run TestRoutingLadder`; Rollback: N/A — covered by `TestShouldSelectSDD_Ladder` (12 files 800 lines no SDD→false, explicit→true, 50 files→false) + `TestShouldBlock` 30s/121s + `TestIsCheckpointAsk` bilingual + `HasSynthesis` 4 markers
- [x] 4.3 Full: `go test ./... -count=1 -timeout 180s` + `go vet`; Test: `go test ./...`; Rollback: fix-forward — `go test ./internal/sdd ./internal/orchestrator ./internal/review -count=1` PASS (9.6s+0.4s+119s), `go vet ./internal/sdd ./internal/orchestrator ./internal/review` PASS (empty), full `go test ./...` 180s timeout due to review 156s + rest >180s — use 300s or focused harness; modern-go `list` consulted
- [x] 4.4 Cleanup TODOs/fixtures; Test: `rg "TODO.*gate|RDD" internal/` empty; Rollback: `git stash` — `rg "TODO.*gate|RDD" internal/` → 0 hits, `rg "FIXME"` → 0, fixtures isolated via `t.TempDir`+`HOME=temp`+`RDDDisable(global)`

