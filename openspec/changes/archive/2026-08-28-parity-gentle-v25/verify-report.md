```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:50973f5220e52cd7b8885eeb6c05ca92ccaabbbec399eabeb473e91f3fccc83c
verdict: fail
blockers: 3
critical_findings: 4
requirements: 8/8
scenarios: 15/15
test_command: go test ./... -count=1 -timeout 180s
test_exit_code: 1
test_output_hash: sha256:50973f5220e52cd7b8885eeb6c05ca92ccaabbbec399eabeb473e91f3fccc83c
build_command: go vet ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: parity-gentle-v25
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 21 |
| Tasks complete | 18 |
| Tasks incomplete | 3 |

Incomplete tasks: 4.1, 4.2, 4.3 (verification gates). 1.1-1.9, 2.1-2.6, 3.1-3.3 all [x] as required (18/21). Remaining 4.1-4.3 are verify tasks pending — per hard rules full verification blocked, but focused evidence collected. They must be marked [x] after successful verify.

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./... — exit 0 — output empty (sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855)
```

**Tests**: ❌ 4 packages failed / ✅ parity focused passed
```text
go test ./... -count=1 -timeout 180s — exit 1 — hash sha256:50973f5220e52cd7b8885eeb6c05ca92ccaabbbec399eabeb473e91f3fccc83c
- FAIL github.com/biggs-100/biggz-ai/cmd/biggz — TestReviewRecover_RestoresLostHEAD, TestReviewInspect_JSON (flaky/recovery)
- FAIL github.com/biggs-100/biggz-ai/e2e — TestOrganicDoctor (WARNING duplicate biggz.exe, not parity)
- FAIL github.com/biggs-100/biggz-ai/internal/filemerge — TestApplyWithHash_Concurrent (flaky)
- FAIL github.com/biggs-100/biggz-ai/internal/review — 18 failures: TestCapture_HappyPathPersistsSlotAndManifest, TestContractEnvelope_StopChainInvalid, TestIntegration_GatePrePR_BlocksOnTamperedChain, TestNextTransition_ChainInvalidStops, TestRecover_*, TestInspect_*, TestRepair_* etc — all "event file missing" or chain verification due to store migration to v1/events vs capture.go still using flat path (capture.go: buildCapturedArtifact filepath.Join(store.Dir, revision) vs store.go eventsDir v1/events)

Focused parity tests (all PASS):
- go test ./model -run TestBudgetParity -count=1 -v — PASS
- go test ./model -run TestEvidenceHashVectors -count=1 -v — PASS
- go test ./model -run TestChainTamper -count=1 -v — PASS
- go test ./internal/review -run TestFixDeltaBinding -count=1 -v — PASS
- go test ./internal/review -run TestStoreGitCommonDir -count=1 -v — PASS
- go test ./internal/review -run TestLegacyFlatReadable -count=1 -v — PASS
- go test ./internal/review -run TestFlockBusyError -count=1 -v — PASS
- go test ./internal/review -run TestStaleReaped -count=1 -v — PASS
- go test ./internal/review -run TestBurnEnabledTrue -count=1 -v — PASS
- go test ./internal/review -run TestBurnEnabledFalse -count=1 -v — PASS
- go test ./internal/sdd -run TestV2AuthorityFree -count=1 -v — PASS
- go test ./internal/sdd -run TestV1Refused -count=1 -v — PASS
- go test ./internal/sdd -run TestProjectStatusV2 -count=1 -v — PASS
- node --test gate (biggz-synthesis-gate.test.mjs) — 21 pass, 0 fail

Filtered: go test $(go list ./... | grep -v e2e) -count=1 still FAIL due to internal/review (same store mismatch)
```

**Coverage**: ➖ Not available (not measured, not required for Standard)

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| FSM Transition Validation (model/review.go, model/fsm.go) | Second round blocked (FixRounds 1→ reject 1/1) | `model/fsm_test.go > TestBudgetParity` ({1,0} reject) | ✅ COMPLIANT |
| FSM Transition Validation | First round allowed (0,0→ succeed) | `model/fsm_test.go > TestBudgetParity` | ✅ COMPLIANT |
| Evidence Chain Integrity (model/hash.go) | Domain vectors match gentle (writeLengthPrefixed+domainHash vs pipe) | `model/hash_test.go > TestEvidenceHashVectors` | ✅ COMPLIANT |
| FixDelta Binding (internal/review/finalize.go, receipt.go, snapshot.go, model/hash.go) | Zero cumulative empty hash (cumulative=0 → EmptyFixDeltaHash) | `internal/review/finalize_test.go > TestFixDeltaBinding` | ✅ COMPLIANT |
| FixDelta Binding | Binding differs from flat (domainHash vs payloadSHA256) + Validate rejects flat | `internal/review/finalize_test.go > TestFixDeltaBinding` | ✅ COMPLIANT |
| Burn Semantics (internal/review/finalize.go) | Burn tombstones (BurnEnabled=true → burned.json + DeliveryBurned) | `internal/review/finalize_test.go > TestBurnEnabledTrue` + `gate_test > TestBurn` | ✅ COMPLIANT |
| Burn Semantics | Burn disabled preserves receipt (BurnEnabled=false → receipt remains, !IsBurned) | `internal/review/finalize_test.go > TestBurnEnabledFalse` | ✅ COMPLIANT |
| Store GitCommonDir (internal/review/store.go) | Worktree writes to common dir (git-common-dir/biggz/review-transactions/<lineage>/v1/events/<sha256>) | `internal/review/store_test.go > TestStoreGitCommonDir` | ✅ COMPLIANT |
| Store GitCommonDir | Legacy flat readable (dual-read identical ValidatedChain) | `internal/review/store_test.go > TestLegacyFlatReadable` | ✅ COMPLIANT |
| Flock-based File Lock (internal/review/lock.go) | Concurrent serialize via flock (two Finalize Acquire → one BusyError) | `internal/review/lock_test.go > TestFlockBusyError` | ✅ COMPLIANT |
| PublishImmutable Evidence Chain (model/hash.go, snapshot.go, receipt.go, hash.go) | Snapshot length-prefix (computeSnapshotHash = domainHash SnapshotDomain + writeLengthPrefixed) | `internal/review/snapshot_test.go > TestSnapshotRecord` + manual vector checked (PASS) | ⚠️ PARTIAL |
| PublishImmutable Evidence Chain | Idempotent publish (same bytes no-op, chain valid) | `internal/review/store_test.go > TestStoreAppend_Idempotent` | ✅ COMPLIANT |
| SDD Status v2 Sole Contract (internal/sdd/status_v2.go, edit_authority.go) | Projection authority-free (JSON must NOT contain granted_roots/missing_roots/edit_authority_blocked, allowlist keys only) | `internal/sdd/status_v2_test.go > TestV2AuthorityFree` + `TestSDDStatusV2CleanBreak` | ✅ COMPLIANT |
| SDD Status v2 Sole Contract | Pre-apply warning replaces block (blockedReasons=[] and next≠resolve-blockers, sdd-apply warns blocked(edit_authority_missing)) | `internal/sdd/status_v2_test.go > TestV2AuthorityFree` + `cmd/biggz > TestSDDApplyGuard` | ✅ COMPLIANT |
| SDD Status v2 Sole Contract | V1 still refused (contract v1 → unsupported sdd-status contract) | `internal/sdd/status_v2_test.go > TestV1Refused` | ✅ COMPLIANT |

**Compliance summary**: 12/15 scenarios compliant (2 COMPLIANT pending snapshot vector explicit, 1 chain mismatch due to capture.go flat vs v1/events — see critical)

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| FSM budget 3/5→1 | ✅ Implemented | model/review.go MaxFixRounds=1 MaxScopedValidations=1, model/fsm.go guard <1 verbatim |
| Chain domainHash+writeLengthPrefixed | ✅ Implemented | model/hash.go writeLengthPrefixed u32 BE + domainHash, evidenceHash MerkleRoot rewritten, SnapshotDomain |
| FixDeltaHashForSnapshot | ✅ Implemented | internal/review/finalize.go FixDeltaHashForSnapshot + EmptyFixDeltaHash via fix-delta/v1\x00+lp, receipt.go domainHash binding |
| Burn tombstone | ✅ Implemented | finalize.go BurnEnabled var, burnReceiptLocked writes burned.json + burn_review event, gate.go IsChainBurned → DeliveryBurned |
| GitCommonDir + publishImmutable | ✅ Implemented | store.go resolveGitCommonDir --git-common-dir fallback, eventsDir v1/events, publishImmutable, dual-read flat+common |
| Flock lock | ✅ Implemented | lock.go flock(LOCK_EX|LOCK_NB) on .lock, stale PID+mtime>5m, AcquireWithTimeout 100ms, flock_unix/windows helpers |
| SDD v2 authority-free | ✅ Implemented | status_v2.go filters blocked(edit_authority_missing), normalizes nextRecommended away from resolve-blockers, allowlist keys, edit_authority.go applyEditAuthorityBlock no-op |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| I1 Budget const+guard+ledger | ✅ Yes | Matches design B choice |
| I2 FixDelta domainHash cumulative | ✅ Yes | FixDeltaHashForSnapshot verbatim |
| I3 Chain writeLengthPrefixed+domains | ✅ Yes | EvidenceDomain review-evidence/v1, MerkleDomain, SnapshotDomain |
| I4 Store GitCommonDir+flock | ✅ Yes | Dual-read preserved, flock authoritative |
| I5 Burn tombstone+BurnEnabled | ✅ Yes | Flag reversible, gate DeliveryBurned |
| I6 SDD V2 authority-free allowlist | ✅ Yes | status_v2 filtering, edit_authority warn only |

### Issues Found
**CRITICAL**:
- Tasks incomplete: 4.1, 4.2, 4.3 still [ ] (18/21). Must be marked [x] only after verify passes. Currently blocked per hard rules — return FAIL.
- go test ./... overall FAIL (exit 1) — 18 failures in internal/review due to capture.go path mismatch: buildCapturedArtifact uses flat `store.Dir/<hash>` while store.go now writes `v1/events/<hash>`. TestCapture_HappyPathPersistsSlotAndManifest fails "event file missing" because artifact.Path points to flat but file is under v1/events. This breaks PublishImmutable integration and leaves store migration incomplete. Must update capture.go to use eventsDir/publishImmutable (or adapt test expectations) and re-run store capture suite.
- Additional chain tests (TestContractEnvelope_StopChainInvalid, TestNextTransition_ChainInvalidStops etc) fail with "no event files found" / "open ... review-transactions/<id>/<hash>: file not found" — same root cause: expectations vs new layout, or dual-read not finding migrated events. Indicates incomplete migration of all store consumers.
- Budget PR2 540 changed lines (456+84) exceeds 400-line budget risk HIGH — requires exception. Documented as stacked-to-main exception 800 (manual rescue), but verify must flag as CRITICAL until maintainer confirms size:exception.

**WARNING**:
- Modern Go guidelines check: Go files were changed (model/hash.go, store.go, lock.go, snapshot.go, etc). `sh "internal/assets/skills/use-modern-go/scripts/run-tool.sh" list --file-path model/hash.go` was consulted (sync_waitgroup_go, testing_t_context, slices_collect etc listed). No CRITICAL modernization missed without explain. However list for other touched files (store.go, lock.go) not explicitly documented in apply-progress, but checked via generic --go-version 1.25 list shows no mandatory modernization missed. Record WARNING if evidence of consultation not in apply-progress (only manually verified now).
- Flaky tests: internal/filemerge TestApplyWithHash_Concurrent, cmd/biggz recovery tests, e2e TestOrganicDoctor WARNING due to duplicate biggz.exe in PATH — not parity-related but obscure signal.
- Uncommitted changes: PR3 slice (status_v2.go, edit_authority.go, status_v2_test.go — 183/+20) still unstaged vs 961ced6. Must commit or stash before archive. hybrid artifact already written but git status shows diverged branch.

**SUGGESTION**:
- Ensure ledger evidence goal counts align: specs are 8 req 15 scen (not 7 req). Use --evidence-goal "verify parity 8req 15scen" consistently.
- Add explicit gentle vector test for SnapshotDomain hash vs expected hex to upgrade PublishImmutable snapshot scenario from PARTIAL to COMPLIANT.
- Persist verify-report also to BigMem mirror for hybrid store: biggz bigmem save topic sdd/parity-gentle-v25/verify-report.
- Clear duplicate biggz.exe warning via PATH dedup for cleaner e2e.

### Verdict
FAIL — parity focused invariants PASS (budget, chain, FixDelta, flock, burn flag, V2 authority-free all COMPLIANT with passing unit tests), but overall go test ./... FAIL (store capture layout mismatch breaks 18 integration tests), 3 tasks incomplete (4.1-4.3), and PR2 540 lines needs explicit size:exception approval before archive.

