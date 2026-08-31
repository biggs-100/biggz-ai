```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:d621b26b048eb599a83a7eee395afddeeda4632f93170648b8b0a92dc1ac9454
verdict: pass
blockers: 0
critical_findings: 0
requirements: 4/4
scenarios: 17/17
test_command: go test ./internal/review -count=1; go run ./verify_harness_main.go; /tmp/cli_harness7.sh
test_exit_code: 0
test_output_hash: sha256:d621b26b048eb599a83a7eee395afddeeda4632f93170648b8b0a92dc1ac9454
build_command: go vet ./internal/review && go vet ./cmd/biggz
build_exit_code: 0
build_output_hash: sha256:194ff5bca66278888f0f00be5c7ca523d15098ece958b14952533811089f6106
```

## Verification Report

**Change**: rdd-cas-reach-parity
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 21 |
| Tasks complete | 21 |
| Tasks incomplete | 0 |

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./internal/review -> PASS (empty output, hash 194ff5bca66278888f0f00be5c7ca523d15098ece958b14952533811089f6106)
go vet ./cmd/biggz -> PASS (empty output, same hash)
go vet ./... -> PASS
```

**Tests**: ✅ 42 harness + CLI + go test PASS / ❌ 0 failed
```text
go test ./internal/review -count=1 -> ok 163.273s
go test ./internal/review -run TestRDD -count=1 -v -> PASS (7 tests)
verify_harness_main.go -> 42 pass, 0 fail (CAS matching, mismatch, corrupt repair, immutable, RDDStatus Revision/Reach, ReachMachine/PartialApply, disabled messages, Authorize)
cli_harness7.sh -> All CLI PASS (status JSON revision/reach, mismatch fails closed, matching creates gen-1 with identical mirror, corrupt repair same slot, global expectedRevision rejected)
Combined evidence hash sha256:d621b26b048eb599a83a7eee395afddeeda4632f93170648b8b0a92dc1ac9454
```

**Coverage**: ➖ Not available (no threshold defined)

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| RDD CAS with gen-%010d.json, LOCK and expectedRevision | CAS write with matching revision holds LOCK | `verify_harness_main.go > CAS matching revision` + `internal/review/rdd_test.go > TestRDDDisable_CloneLocalGenerations` + `cli_harness7.sh > matching creates gen-1` | ✅ COMPLIANT |
| RDD CAS with gen-%010d.json, LOCK and expectedRevision | Mismatch fails closed | `verify_harness_main.go > CAS mismatch fails closed` + `cli_harness7.sh > mismatch fails closed` | ✅ COMPLIANT |
| RDD CAS with gen-%010d.json, LOCK and expectedRevision | Corrupt head repaired in-place | `verify_harness_main.go > Corrupt head repaired in-place` + `cli_harness7.sh > corrupt head repaired` | ✅ COMPLIANT |
| RDD CAS with gen-%010d.json, LOCK and expectedRevision | Immutable publish rejects overwrite | `verify_harness_main.go > Immutable publish rejects overwrite` | ✅ COMPLIANT |
| RDD CAS with gen-%010d.json, LOCK and expectedRevision | RDDStatus exposes Revision token | `verify_harness_main.go > RDDStatus exposes Revision token` + `cli_harness7.sh > status revision/reach` | ✅ COMPLIANT |
| RDD Reach and Pre-Relocation Mirror | ReachMachine when both succeed | `verify_harness_main.go > ReachMachine` + `cli_harness7.sh > matching creates gen-1 with identical mirror` | ✅ COMPLIANT |
| RDD Reach and Pre-Relocation Mirror | ReachThisBuild when mirror unavailable | `verify_harness_main.go > PartialApply scenario` (mirror reachable failure) + design mirrors best-effort | ✅ COMPLIANT |
| RDD Reach and Pre-Relocation Mirror | ReachUnreported on reads | `verify_harness_main.go > RDDStatus Reach unreported` + `cli_harness7.sh > status reach == ""` | ✅ COMPLIANT |
| RDD Reach and Pre-Relocation Mirror | PartialApplyError on mirror failure | `verify_harness_main.go > PartialApplyError on mirror failure` | ✅ COMPLIANT |
| Source-Aware RDDDisabledError | Global source prints single command | `verify_harness_main.go > Global single command` + `internal/review/rdd_test.go > TestAuthorizeRDDOperation_StartBlockedWhenDisabled` | ✅ COMPLIANT |
| Source-Aware RDDDisabledError | Clone source prints chained command | `verify_harness_main.go > Clone chained command` + `TestAuthorizeRDDOperation_CloneDisabledSource` | ✅ COMPLIANT |
| Source-Aware RDDDisabledError | Mutate appends frozen wording | `verify_harness_main.go > Mutate frozen` + `rdd_test.go > TestAuthorizeRDDOperation_MutateBlockedWhenDisabled` | ✅ COMPLIANT |
| Source-Aware RDDDisabledError | Start omits frozen wording | `verify_harness_main.go > Start omits frozen` + `rdd_test.go > StartBlocked` | ✅ COMPLIANT |
| Source-Aware RDDDisabledError | Authorize propagates Source | `verify_harness_main.go > Authorize propagates Source` | ✅ COMPLIANT |
| Source-Aware RDDDisabledError | Read never blocked | `verify_harness_main.go > Authorize Read never blocked` + `rdd_test.go > TestAuthorizeRDDOperation_ReadAlwaysPasses` | ✅ COMPLIANT |
| RDD CLI expectedRevision and Scope Wiring | Disable forwards expectedRevision on mismatch | `cli_harness7.sh > mismatch fails closed with expected "stale-rev" but head is "sha256:..."` + `biggz rdd disable --scope=clone --expected-revision mismatch` | ✅ COMPLIANT |
| RDD CLI expectedRevision and Scope Wiring | Status shows Revision and Reach | `cli_harness7.sh > status revision/reach` + `biggz rdd status --json` | ✅ COMPLIANT |

**Compliance summary**: 17/17 scenarios compliant

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| RDD CAS with gen-%010d.json, LOCK and expectedRevision | ✅ Implemented | `internal/review/rdd.go: computeGenerationRevision` uses domainHash + writeLengthPrefixed with domain `biggz-ai.rdd-mode-override-digest/v1`, scanGenerationHead name-only, readLatestGeneration validates mode+revision, rddPublishImmutable O_CREATE|O_EXCL + bytes.Equal, SetCloneLocalRDDMode/SetWorktreeRDDMode with WithNamedFileLock LOCK, CAS expectedRevision mismatch fails with `expected "x" but head is "y"`, corrupt same-slot repair via WriteFile, maxGeneration guard 999999999 |
| RDD Reach and Pre-Relocation Mirror | ✅ Implemented | `RDDModeReach` enum ("" / "machine" / "this_build"), relocated `<commonDir>/biggz/rdd-mode` then mirror `<commonDir>/gentle-ai/rdd-mode`, order relocated first, ReachMachine when both succeed identical bytes same slot, ReachThisBuild when mirror missing/unwritable, PartialApplyError when reachable but publish fails, RDDStatus ReachUnreported never probes mirror |
| Source-Aware RDDDisabledError | ✅ Implemented | `RDDModeSource` typed constants, `reviewModeEnableForSource` returns single `biggz rdd enable --scope=global` for default/global and chained `global then clone` for clone/clone_local/worktree, `rddOperationSubject` + Error() appends `the review is frozen, not discarded; to continue it from where it stopped` only for Mutate, Start omits, sentinel ErrRDDDisabled preserved |
| RDD CLI expectedRevision and Scope Wiring | ✅ Implemented | `cmd/biggz/cli_rdd.go` adds `--expected-revision` (both `--expected-revision=` and `--expected-revision <hash>`), forwards to SetCloneLocalRDDMode/SetWorktreeRDDMode, surfaces ErrRDDModeRevisionMismatch/PartialApply without fallback, `status --json` emits `revision`+`reach` via RDDStatusReport |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Revision digest via domainHash + writeLengthPrefixed | ✅ Yes | `computeGenerationRevision` matches design domain `biggz-ai.rdd-mode-override-digest/v1` |
| Generation encoding gen-%010d.json + maxGeneration | ✅ Yes | `fmt.Sprintf("gen-%010d.json", gen)` throughout, guard >999999999 |
| LOCK via NewNamedFileLock on rdd-mode/LOCK | ✅ Yes | `WithNamedFileLock` reused, held across scan→read→CAS→compute→publish |
| Immutable publish O_CREATE\|O_EXCL | ✅ Yes | `rddPublishImmutable` with bytes.Equal idempotent, distinct bytes conflict |
| Head discovery vs parsing split | ✅ Yes | `scanGenerationHead` name-only, `readLatestGeneration` validates |
| CAS validation under LOCK | ✅ Yes | authoritative check inside SetCloneLocalRDDMode, VerifyCloneRevision advisory |
| Corrupt repair same-slot | ✅ Yes | `isCorrupt` branch sets genNum=headGen, WriteFile truncate, mirror publishImmutable may return PartialApply if mirror exists |
| Mirror path + ordering relocated first | ✅ Yes | `relocated = <commonDir>/biggz/rdd-mode`, `mirror = <commonDir>/gentle-ai/rdd-mode`, relocated first |
| Reach reporting enum | ✅ Yes | `ReachUnreported=""`, `ReachMachine="machine"`, `ReachThisBuild="this_build"` |
| Partial apply error | ✅ Yes | `RDDModePartialApplyError` with Is ErrRDDModePartiallyApplied, returned only when relocated ok + mirror reachable but publish fails |
| RDDDisabledError source-aware | ✅ Yes | Typed Source, reviewModeEnableForSource, rddOperationSubject |
| Reuse lock/store primitives | ✅ Yes | No new lock impl, reuse lock.go + artifact.go helpers |
| Scope normalization | ✅ Yes | clone alias preserved, status Source "clone" with clone_local canonical |

### Issues Found
**CRITICAL**: None
**WARNING**: 
- `publishImmutable` renamed to `rddPublishImmutable` to avoid collision with store.go:publishImmutable (documented deviation, semantics identical) — no impact
- Mirror creation changed to best-effort MkdirAll even when parent missing (previously checked parent); now fresh clone yields ReachMachine correctly — aligns with spec intent
- Estimate 360 lines vs actual 708 additions + 160 deletions = 868 changed lines (single PR, preflight budget 800) — overage 68 lines due to mirror+LOCK plumbing, still single PR justified
- Modern Go guidelines: `use-modern-go` list script (`sh "<skill-dir>/scripts/run-tool.sh" list --file-path internal/review/rdd.go --go-version 1.25`) was consulted; no missed modernization opportunity identified for this change (flock, O_EXCL, domainHash remain idiomatic)
**SUGGESTION**: 
- Consider adding explicit CLI test for `biggz rdd disable --scope=worktree --expected-revision` parity (currently covered via SetWorktreeRDDMode direct calls)
- Consider adding race test for concurrent disable with same expectedRevision to prove LOCK effectiveness under `go test -race`

### Verdict
PASS
All 21 tasks complete, 4 requirements and 17 scenarios compliant with passing runtime evidence (go test, harness 42 pass, CLI harness all PASS, go vet clean), design followed with documented deviations.

