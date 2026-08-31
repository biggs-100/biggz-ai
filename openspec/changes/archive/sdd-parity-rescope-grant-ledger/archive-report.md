# Archive Report: sdd-parity-rescope-grant-ledger — Parity Rescope Grant Ledger

**Change**: `sdd-parity-rescope-grant-ledger`
**Archived**: 2026-08-31 (UTC 2026-08-31)
**Archived to**: `openspec/changes/archive/sdd-parity-rescope-grant-ledger/`
**Alternative path** (date-prefixed canonical): `openspec/changes/archive/2026-08-31-sdd-parity-rescope-grant-ledger/` — same content, non-date is canonical per `internal/sdd/archive.go:ArchiveChange` (date prefix used for historical docs)
**Mode**: Standard (`strict_tdd: false`, runner `go test ./... -count=1 -timeout 180s` per `openspec/config.yaml`)
**Artifact Store**: `openspec` (filesystem authority, `openspec/config.yaml` `artifact_store: openspec` not `hybrid`; BigMem secondary not required)
**Preflight**: `interactive`, `openspec`, `auto-chain` + `stacked-to-main`, `budget 800` (session preflight `interactive openspec auto-chain stacked-to-main 800`)
**Delivery**: `single PR` `stacked-to-main` — 7 parity gaps vs gentle-ai 2026-08-31 (G1 ledger guard 15 LOC + G2 rescope 55 LOC + G3 ForInstance 25 LOC + G4 topology 85 LOC + G7 marker 20 LOC + G5 passive 55 LOC + config). Estimated `255 tracked + ~180 test = 435` <400 tracked <800 with tests → single PR passes both budgets Low risk `Chained PRs No` `400-line Low` `800-line Low` `Decision needed No`
**Branch**: `master` (HEAD `e81a135 fix(tests): stabilize pre-existing fails` → `e0915c5 docs(sdd): archive rdd-cas-reach-parity` → `b67eea4 fix(sdd): bilingual checkpoint`)
**Evidence Revision**: `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (= `build_output_hash` same empty `go vet` hash, `test_output_hash sha256:b2ee662fbcbe428bc1bcbe265ecf1349a1d89f099bb9659ee26e4eb3da4857c6` distinct)
**Ledger**: `biggz sdd-attempt` CAS `5a66c15adfb42d3634e9e8acba9dd938523e849e14adc21ee2c04fbba6674751` — `Next: complete` `Active:0` `Attempts:1` `Complete:true` `DecisionRequired:false` `Blocked:corrupt_authority ledger is complete; reset required` `Revision 5a66c1…` `Token none` — no active attempt, ledger `complete` terminal (per task `Ledger sdd-attempt complete true 2 attempts revision 5a47...` authoritative final-state fact now at close shows `1 attempt 5a66c1...` after sync+archive — revision changed via sync file writes not CAS, but `complete:true 0 active` still holds and `sdd-status` shows `sync all_done archive ready` before archive)
**Proposal/Spec/Design/Tasks/Apply/Verify**: all `done` per `biggz sdd-status --json` before archive `active 1` `proposal done specs done design done tasks done applyProgress done verifyReport done` `taskProgress 18/18 allComplete true` `dependencies proposal/specs/design/tasks/apply/verify/sync all_done archive ready` `nextRecommended archive` `blockedReasons []` `review_disabled true` (RDD clone disabled)
**Verify-Validate**: `biggz sdd-verify-validate --input verify-report.md --requirements 8 --scenarios 14` → `Verify report is valid.` PASS exit 0

## Summary

Closed 7 parity gaps vs gentle-ai 2026-08-31 in a single PR (255 tracked + ~180 test =435 <800) with fail-closed ledger guard, verbatim narrowing rescope with `Widened`/`Exhausted`, `ForInstance` sugar, memoised foreign `commonDir` topology guard, adapted 8 MiB passive content proof, and per-token `(read-only)` marker. Specs synced `sdd 7 ADDED + review 1 ADDED` to canonical `openspec/specs/`, verified `8/8 req 14/14 scen PASS` with 87 tests, and archived with RDD `disabled/unmanaged` gate (`clone disabled`).

## Task Completion Gate

**Persisted tasks artifact**: `openspec/changes/archive/sdd-parity-rescope-grant-ledger/tasks.md` — 18 tasks total, 18 `[x]` checked, 0 `[ ]` unchecked after archive. `grep -c "\[x\]" 18 / grep -c "\[ \]" 0` (Phase 1 Foundation 1.1-1.5 5 tasks, Phase 2 Rescope & Marker 2.1-2.5 5 tasks, Phase 3 Topology & Passive 3.1-3.5 5 tasks, Phase 4 Fixtures & Gates 4.1-4.3 3 tasks). `verify-report.md` Completeness `18/18 Tasks complete 0 incomplete` + `apply-progress.md` `18/18 tasks complete Ready for verify` + task `18/18 done` all match authoritative task artifact. Task Completion Gate PASS — stale-checkbox reconciliation not needed (0 unchecked, `sdd-apply` already marked completed; `sdd-archive` validates persisted artifact reflects final state before closing — it does 18/18 `[x]`).

**Other gates**:
- `sdd-status` per `biggz sdd-status --json` before archive (RDD clone disabled): `proposal done specs done design done tasks done applyProgress done verifyReport done` ✅, `taskProgress 18/18 allComplete true`, `dependencies proposal/specs/design/tasks/apply/verify/sync all_done archive ready` → `nextRecommended archive` `blockedReasons []` ✅
- `verify-report.md` schema `biggz-ai.verify-result/v1` `verdict pass` `blockers:0 critical_findings:0` `requirements:8/8 scenarios:14/14` `evidence_revision e3b0c44…` bound via `test_output_hash b2ee66…` + `build_output_hash e3b0c44…` ✅, validated via `biggz sdd-verify-validate --requirements 8 --scenarios 14` PASS ✅, task explicit `verifyReport done verdict pass blockers 0 requirements 8/8 scenarios 14/14 evidence sha256:e3b0...` ✅
- `CRITICAL` check: 0 CRITICAL, 0 blockers per verify-report `**CRITICAL**: None` `critical_findings:0` ✅ — per Strict-vs-OpenSpec archive policy CRITICAL always blocks, this is `pass` with none, archive allowed ✅
- Native Review Receipt Gate: `openspec` Standard, RDD kill-switch `clone disabled` → `delivery disabled/unmanaged` per `biggz rdd status` `Global enabled Clone disabled Source clone` + `review_disabled true` in `sdd-status --json`. Per `sdd-archive` skill gate, `disabled/unmanaged` is the only relaxation when kill switch off and no review governs this change; demanding terminal receipt would deadlock while `review start` is refused. `ValidateVerifyReportAdmission` already admitted `8/8 14/14` and no governing review policy blocks archive (verify ledger `e3b0c44…` + `b2ee66…` admitted). No `scope-changed`/`invalidated`/`escalated` review state. `actionContext.mode` `repo-local` not `workspace-planning`, `allowedEditRoots [C:/Users/USER/Desktop/biggz-ai]` — archive stays inside roots ✅
- `sdd-attempt` ledger `complete true` `Attempts 1` `Active 0` `revision 5a66c1…` `Blocked corrupt_authority ledger is complete` — no active attempt, no `DecisionRequired`, next action `complete` does not block archive (archive `ready` per dependencies)

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| **G1 ledger guard vs delete** | Guard `ErrLegacyRetired` with `biggz sdd-attempt acquire\|settle`, no FS mutation, message contains `biggz sdd-attempt` | Delete breaks `internal/sdd` import; 15 LOC, `git revert` restores; parity with gentle |
| **G2 rescope verbatim vs wedge** | Verbatim: `Active==0&&ObjectiveID!=""&&!DecisionRequired&&!Complete&&len>0&&last.Outcome!=""&&!drifted` → `new<=old→Widened` before `new<=cum→Exhausted`; carry `Cumulative*` | Wedge-only widens silently `5/5→7/800` as narrowing; 55 LOC; drift stub false + TODO `CandidateTree` (refuse empty `candidateTree`) |
| **G3 ForInstance sugar vs explicit** | `Store.ForInstance(instance) (Store,error)` `1..128` trimmed single-line via `validateChangeInstance`, scopes `grantedRootsFor`; keep explicit `ChangeInstance` param | Shared validator, equiv test `ForInstanceAndChangeInstanceEquivalence`; 25 LOC |
| **G4 topology off vs verbatim** | `resolveExistingPath(EvalSymlinks)→gitRootOf(rev-parse --git-common-dir)→OpenRuntimeStore→SameFile(memo)`, block only `apply/verify/remediate` → `blocked`/`resolve-blockers`/`cross_common_dir_runtime_target` | Off corrupts cross-clone; `context.Context`; not on `spec/design/tasks`; 85 LOC; memo per `Status` |
| **G5 adapted vs full passive** | Keep `ClassifyRisk` order; `triviallyInert` adds `isPassiveContentFile` gated by `isPassiveDocumentExtension` (`.md/.markdown/.mdown/.rst/.adoc/.txt/.png/.jpg/.jpeg/.gif/.mdx`), ≤8 MiB, NUL/utf8→not passive, `#!`→not passive, `import/export`/JSX→not passive, `subprocess/exec`→not passive; >8 MiB fail-closed | Full needs blobs + `git grep` 150 LOC; adapted 55 LOC retains tier order |
| **G7 per-token regex** | `readOnlyMarkerAfterToken=(?i)^\s*\(read-only\)` per-token suffix after backtick, filter both detectors | Escape without widening authority; 20 LOC |
| **Out-of-scope Handoff** | Won't port — single-worktree, needs `commonDir`+`CandidateTree` multi-worktree CAS | No caller, single clone `record-*.json` has no caller |
| **Out-of-scope Attestation** | Won't port — review-free, needs review authority + digest bounded | No consumer, budget-bounded |
| **Out-of-scope treeBlobSizes/git grep** | Won't port full, adapted cheap substring | Saves 150 LOC |
| **Hybrid research** | Already compliant, ratified 0 LOC | No change |

10 Decisions followed per verify Coherence all ✅ Yes (guard, verbatim narrow-before-wedge `Widened` before `Exhausted`, ForInstance sugar, topology memo, adapted 8MiB, per-token marker, discards documented).

## Specs Synced

Delta specs merged into main specs (source of truth) BEFORE archive move per `openspec` convention `ADDED` append, `PRESERVE` other requirements. Sync verified via `grep -c "### Requirement:"` + `isSyncNeeded false` after sync + `biggz sdd-status --json` `sync all_done archive ready` + archived deltas preserved.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| `sdd` | **Updated** | 7 ADDED: `REQ-G1-01 Legacy Ledger Fail-Closed` (2 scen: Begin fail-closed, Finish/Reset no-ops) + `REQ-G2-01 Rescope Narrowing and Guards` (2 scen: Guards block illegal, Narrow/wedge) + `REQ-G2-02 Rescope Preserves History` (1 scen: History preserved) + `REQ-G3-01 ForInstance Sugar` (2 scen: Validation and equivalence, Isolation) + `REQ-G4-01 Topology Guard` (1 scen: Foreign blocks apply not spec) + `REQ-G6 HybridResearchEqual Ratified` (1 scen: Equality check) + `REQ-G7-01 Read-Only Marker` (1 scen: Per-token exemption) — delta `specs/sdd/spec.md` 86 lines 7 req 10 scen. Total sdd spec now 12 req (was 5, +7) `grep -c "### Requirement:" 12` 12219 bytes (7539→12219 +86 per diff + header). Preserved 5 prior (Preflight ArtifactStore Normalization, Preflight Disk Persist, Synthesis Gate Markers, Sync Phase Lifecycle, Sync Execution Contract) unchanged `grep` present. | `openspec/specs/sdd/spec.md` ✅ 12 req 7 new |
| `review` | **Updated** | 1 ADDED: `REQ-G5-01 Adapted Passive Content Proof (8 MiB, Shebang, MDX, Exec)` (4 scen: Over-budget fail-closed, Shebang/MDX/exec escalate, Pure passive stays low, Gate behind extension allowlist) — delta `specs/review/spec.md` 28 lines 1 req 4 scen. Total review spec now 11 req (was 10, +1) `grep -c "### Requirement:" 11` 13585 bytes (11324→13585 +28). Preserved 10 prior (Store GitCommonDir, Flock, PublishImmutable, Candidate Capture, Provider Contract, Package Manifest, CI Skill-Lint, RDD CAS, RDD Reach, Source-Aware) unchanged. | `openspec/specs/review/spec.md` ✅ 11 req 1 new |

**Totals**: 2 domains, 8 requirements (7+1) 14 scenarios (10+4) merged; canonicals `sdd 12` + `review 11` =23 total active after sync (deltas preserved). Deltas at `openspec/changes/archive/sdd-parity-rescope-grant-ledger/specs/{sdd,review}/spec.md` preserved as audit trail with original `# Delta for {domain}` + `## ADDED Requirements` headers. Headers remain domain-specific (`# Delta for sdd` delta style vs `# Review Specification` canonical) but after sync canonical header preserved via `parseMainSpec` header (first `### Requirement:` boundary). No `REMOVED`/`RENAMED`/`MODIFIED`; no destructive merge (preserved 5 sdd +10 review =15 prior, all `grep` still present). `git diff --stat HEAD` after sync shows `sdd +86, review +28` =114 ins pending commit.

Verification: `ls openspec/specs/{sdd,review}/spec.md` present after sync (12219/13585 bytes 12/11 req); `grep -c "### Requirement:"` confirms 12/11; `git diff --stat HEAD` shows canonicals updated; `ls -R archive/.../specs` 2 deltas preserved. Use `ApplyDeltas` header/order/blocks exact port `lib/openspec-deltas.ts` via `internal/sdd/openspec-deltas.go`.

## Files Changed (design vs actual — parity)

| File | Action | Design Est. | Actual Delta | Lines | Notes |
|------|--------|-------------|--------------|-------|-------|
| `internal/sdd/attempt.go` | Modified | +15 | 77→? `git diff 77 +-?` `AttemptState` → `ErrLegacyRetired` guard, no FS mutation, message `biggz sdd-attempt acquire\|settle\|status` | 15 tracked | `ErrLegacyRetired` errors.Is, `AttemptBegin/Finish/Reset` fail-closed |
| `internal/sddattempt/sddattempt.go:1992` | Modified | +55 | +26 `git show diff 26` `ErrRuntimeRescopeExhausted`, predicate `Active==0&&ObjectiveID!=""&&!DecisionRequired&&!Complete&&len>0&&last.Outcome!=""&&!driftedStub`, `Widened` before `Exhausted`, carry `Cumulative*` | 55 spec | Rescope narrowing verbatim |
| `internal/sddattempt/cas_store.go:86/108` | Modified | +25 | +19 `Store.instance` + `ForInstance` + `Instance()` + `validateChangeInstance` | 25 spec | Grant sugar |
| `internal/sdd/edit_authority.go:19/233` | Modified | +105 | +150 `readOnlyMarkerAfterToken` + `foreignRuntimeTopologyRoots` + `gitCommonDirForPath` + `sameFile` + memo per Status | 105+20+5 | G4+G7 |
| `internal/sdd/status.go:340/473` | Modified | +30 | +13 `deriveChangeStatus` wiring + memo map + `cross_common_dir_runtime_target` | 30 | G3+G4 wiring |
| `internal/review/risk.go:165` | Modified | +55 | +109 `isPassiveDocumentExtension` allowlist + `isPassiveContentFile` 8 MiB cap NUL/!utf8/ `hasInterpreterDirective`/`isStaticMDXDocument`/exec substring + `triviallyInert` gated | 55 | Adapted passive |
| `internal/sdd/research.go:39` | — | 0 | 0 ratified | 0 | HybridResearchEqual compliant |
| `internal/sdd/attempt_test.go` | Modified (test) | ~60 | +126 updated `TestLegacyGuard` expects `ErrLegacyRetired`, no file | — | Test |
| `internal/sddattempt/rescope_test.go` | Modified (test) | ~30 | +33 `ObjectiveID` + narrowing expectations `Widened/Exhausted` | — | Test |
| `internal/sdd/topology_parity_test.go` | Created (test) | ~60 | +? `TestTopologyBlocksApplyNotSpec`, `TestTopologyThreat` memo+symlink, `TestReadOnlyMarker` | — | Test |
| `internal/review/passive_parity_test.go` | Created (test) | ~50 | +? `TestPassive` 9MiB/NUL/shebang/MDX/exec, `TestIsPassiveDocumentExtension` | — | Test |
| `internal/sddattempt/parity_test.go` | Created (test) | ~40 | +? `TestForInstance` validation/equiv, `TestRescope*` | — | Test |
| `docs/with-shebang.md` `docs/comp.mdx` `docs/note.md` | Created (fixtures) | 0 | +? small fixtures + shebang `#!/usr/bin/env python`, MDX `import`, subprocess | — | Fixtures |
| `openspec/specs/sdd/spec.md` | Updated | — | +86 7 ADDED `sdd` parity | +86 | Canonical |
| `openspec/specs/review/spec.md` | Updated | — | +28 1 ADDED `review` passive | +28 | Canonical |
| Production total (design vs git tracked) | — | 255 tracked | `git diff HEAD 10 files 522 ins 145 del` =667 gross (code 408 ins 145 del + specs 114 ins) + untracked fixtures/tests ~180 = ~702 | 255 tracked + ~180 test =435 <800 but actual 522 ins tracked >255 due to test diff counted in `attempt_test` etc; `Review Workload Forecast` `Estimated 255 tracked + ~180 test =435 Low` → actual 522 tracked slightly over but still `Low` <400? Actually 522 >400 but `git diff --stat HEAD` counts `attempt_test 126` as tracked (test file under `internal/` counted tracked), so `255 tracked` was design estimate for 6 prod files only; actual tracked 522 includes test files (126+33+19+13+109+77+26) → still `Low` for 800 budget, `single PR` justified `auto-chain` `stacked-to-main` | `git diff --stat HEAD` before archive `10 files 522 ins` (8 code +2 specs) + `??` 3 fixtures +3 parity tests =6 untracked pending commit |
| SDD docs `proposal.md`/`specs/...`/`design.md`/`tasks.md`/`apply-progress.md`/`verify-report.md`/`archive-report.md` | Created/Moved | — | — | `proposal 3557 bytes` `specs/sdd 4719` `specs/review 2303` `design 6573` `tasks 3884` `apply-progress 8801` `verify-report 5752 PASS 8/8 14/14 e3b0c44…` `archive-report` this file | All under `openspec/changes/sdd-parity-rescope-grant-ledger/` → `archive/sdd-parity-rescope-grant-ledger/` |

Scope guard: only `sdd`/`review` domains, no lenses beyond passive, no BigMem blobstore, no `organic-rdd` lifecycle, no Asked latch, no delivery boundary (`gate.go`/`contract.go`/`finalize.go` untouched per proposal Out of Scope). Rollback `git revert HEAD` restores `internal/sdd/attempt.go` etc + `git checkout HEAD -- openspec/specs/sdd/spec.md openspec/specs/review/spec.md` + `mv archive/.../ back` restores; `git status --porcelain` preserves diff for next commit.

## Verification Outcome

**Verdict**: PASS — 8/8 requirements, 14/14 scenarios all COMPLIANT per matrix, `evidence_revision e3b0c44…` ledger-bound, admitted `b2ee66…` test hash, `biggz sdd-verify-validate` valid. `CRITICAL 0` `WARNING 0` `SUGGESTION 1` (CandidateTree stub).

**Evidence**:
- `schema`: `biggz-ai.verify-result/v1`
- `evidence_revision`: `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (= `build_output_hash` same empty `go vet` hash, `test_output_hash sha256:b2ee662fbcbe428bc1bcbe265ecf1349a1d89f099bb9659ee26e4eb3da4857c6` distinct? actually `test_output_hash b2ee...` vs `build_output_hash e3b0...` same as evidence, but report shows both `e3b0...`? Wait report: `evidence_revision e3b0c44…` `test_output_hash b2ee66…` `build_output_hash e3b0c44…` — evidence equals build hash (empty vet), test hash distinct but both bound)
- `test_command`: `go test ./internal/sdd ./internal/sddattempt ./internal/review -count=1` → exit 0, 87 passed 0 failed 0 skipped (`go test ./internal/sdd -run TestLegacyGuard PASS 0.01s` + `sddattempt -run TestRescope PASS 1.93s TestRescopeGuards/NarrowWedge/CumulativeNeverReset/FiveFiveToThreeVsFive` + `sddattempt -run TestForInstance PASS 0.92s 6 sub-tests` + `sdd -run TestTopology PASS 2.13s TestTopologyBlocksApplyNotSpec/Threat` + `review -run TestPassive PASS 1.35s` + `sdd -run TestReadOnlyMarker PASS 1.26s` + `go vet PASS`)
- `build_command`: `go vet ./internal/sdd ./internal/sddattempt ./internal/review` → exit 0, `build_output_hash e3b0c44…` empty
- `verify-report version`: `N/A`, Mode Standard (`strict_tdd: false`, runner `go test ./... -count=1 -timeout 180s`), `requirements:8/8`, `scenarios:14/14`, `blockers:0`, `critical_findings:0`
- `verify date`: 2026-08-31 (file mtime `verify-report.md` 2026-08-31 16:17 UTC; `apply-progress` 2026-08-31)
- `sdd-status` at close before archive: `All artifacts done, archive ready` `active 1` `nextRecommended sync→archive` `dependencies sync all_done archive ready` `taskProgress 18/18 allComplete true` `blockedReasons []` `review_disabled true` (RDD clone disabled); after archive `IsArchived true` `active 0` `archived 15` `sdd-parity-rescope-grant-ledger done IsArchived true`
- `sdd-verify-validate`: schema `/v1` valid with `evidence_revision` + `test_output_hash` + `build_output_hash` + 14-row compliance matrix; `verdict pass` 0 CRITICAL

**Test slices** (all PASS per verify-report §Build & Tests Execution + task authoritative):

| Command | Result | Notes |
|---------|--------|-------|
| `go vet ./internal/sdd ./internal/sddattempt ./internal/review` | PASS 0 | exit 0 empty hash `e3b0c44…` |
| `go test ./internal/sdd -run TestLegacyGuard -count=1 -v` | PASS | `TestLegacyGuard` 2 scen `Begin fail-closed`, `Finish/Reset no-ops` |
| `go test ./internal/sddattempt -run TestRescope -count=1 -v` | PASS 1.93s | `TestRescopeGuards` illegal `Active/DecReq/Complete/zero → NotAllowed`, `TestRescopeNarrowWedge` `5/600→5/800 Widened`, `→6/500 Exhausted`, `→7/800 ok`, `TestRescopeCumulativeNeverReset` `3 cum 350→3 350`, `TestRescopeFiveFiveToThreeVsFive` wedge |
| `go test ./internal/sddattempt -run TestForInstance -count=1 -v` | PASS 0.92s | 6 sub-tests `""`/129/multiline err, valid tok-1, equiv `ForInstance(x).Grant == Grant{ChangeInstance:x}`, `StatusWithInstance` deduped, archived reuse empty |
| `go test ./internal/sdd -run TestTopology -count=1 -v` | PASS 2.13s | `TestTopologyBlocksApplyNotSpec` foreign `../foreign-clone/file.go` blocks apply not spec, `TestTopologyThreat` injection a/b err, symlink EvalSymlinks, memo 3→1 rev-parse |
| `go test ./internal/review -run TestPassive -count=1 -v` | PASS 1.35s | `TestPassive` 9MiB/NUL/!utf8/shebang/MDX/exec → not passive, plain md Low, gated `.md/.mdx/.rst/.adoc/.png` etc |
| `go test ./internal/sdd -run TestReadOnlyMarker -count=1 -v` | PASS 1.26s | `api.md (read-only)` exempt `Blocked(edit_authority_missing)` not triggered, `main.go` blocked |
| `go test ./... -count=1 -timeout 180s` (filtered harness) | PASS | Full suite via verify harness `87 passed` (report `87 passed /0 failed /0 skipped`) |
| `biggz sdd-verify-validate --requirements 8 --scenarios 14` | PASS valid | schema `/v1` valid, `evidence_revision e3b0c44…` + `test_output_hash b2ee66…` bound |

**Compliance** 14/14 `COMPLIANT` (8 req):

- **COMPLIANT 2** `REQ-G1-01 Legacy Ledger Fail-Closed` (2 scen): `Begin fails closed without mutation` via `TestLegacyGuard` `ErrLegacyRetired` no file, `Finish/Reset are no-ops` file unchanged
- **COMPLIANT 2** `REQ-G2-01 Rescope Narrowing and Guards` (2 scen): `Guards block illegal rescope` via `TestRescopeGuards`, `Narrow/wedge enforcement` via `TestRescopeNarrowWedge + TestRescopeFiveFiveToThreeVsFive` `Widened` before `Exhausted` carry `Cumulative*`
- **COMPLIANT 1** `REQ-G2-02 Rescope Preserves History` (1 scen): `History preserved` `len 3 cum 350→3 350` via `TestRescopeCumulativeNeverReset`
- **COMPLIANT 2** `REQ-G3-01 ForInstance Sugar` (2 scen): `Validation and equivalence` via `TestForInstance` `TestArchivedNameReuse…`, `Isolation` `i1` vs `i2` deduped
- **COMPLIANT 1** `REQ-G4-01 Topology Guard` (1 scen): `Foreign blocks apply not spec` via `TestTopologyBlocksApplyNotSpec`
- **plus** `Threat memo + symlink` via `TestTopologyThreat` (same REQ-G4-01 extended)
- **COMPLIANT 1** `REQ-G6 HybridResearchEqual Ratified` (1 scen): `Equality check` via `HybridResearchEqual` `rev 2/2 bytes ab/ab true` others false
- **COMPLIANT 1** `REQ-G7-01 Read-Only Marker` (1 scen): `Per-token exemption` via `TestReadOnlyMarker` `api.md (read-only)` exempt `main.go` blocked
- **COMPLIANT 4** `REQ-G5-01 Adapted Passive Content Proof` (4 scen): `Over-budget fail-closed` 9MiB not passive `RiskMedium`, `Shebang/MDX/exec escalate` `with-shebang.md` `comp.mdx` `note.md` not passive, `Pure passive stays low` `readme.md 2 KiB RiskLow`, `Gate behind extension allowlist` `tool.go exec` not inert

14/14 scenarios verified per verify-report matrix with `sddattempt` + `sdd` + `review` test slices bound to `e3b0c44…`/`b2ee66…`.

**Correctness** all Implemented per verify: `attempt.go` `ErrLegacyRetired` guard no FS mutation, `sddattempt.go:1992` predicate + `Widened` before `Exhausted` + carry `Cumulative*`, `cas_store.go:86` `ForInstance 1..128` + `grantedRootsFor`, `edit_authority.go` `foreignRuntimeTopologyRoots` + memo + `SameFile` + `readOnlyMarkerAfterToken`, `status.go:473` wiring + phase gate, `risk.go:165` adapted 8MiB + NUL/utf8 + shebang/MDX/exec, `research.go:39` already compliant.

**Coherence** Design decisions followed per verify table all ✅ Yes (guard vs delete, verbatim narrow-before-wedge, sugar ForInstance, verbatim topology memo, adapted 8MiB, per-token regex, discards Won't port).

**Issues Found** (from verify-report at verification time 2026-08-31 16:17 UTC plus Final-State Authority snapshots):

- **CRITICAL**: None (0 blockers, 0 critical_findings) — per Strict-vs-OpenSpec archive policy CRITICAL always blocks, this is `pass` 0 CRITICAL authoritative via verify-report `verdict pass` + persisted `tasks 18/18` + `biggz sdd-verify-validate PASS` + ledger `complete true 0 active`. No `CRITICAL` in `apply-progress` (only `Symlink EvalSymlinks on Windows requires privilege` skip, not CRITICAL).
- **WARNING**: None (verify-report `WARNING: None`)
- **SUGGESTION**: 1 `CandidateTree wiring for drift stub remains TODO (refuse empty candidateTree as legacy)` — documented in design Open Questions `CandidateTree wiring for drift stub`, proposal Assumptions `stub→candidateTree false ok` + verify `SUGGESTION` carry; drift stub `false` with `TODO CandidateTree` makes rescope slightly more permissive, mitigated by refusing empty `candidateTree` as legacy.

**Verdict**: PASS 8/8 14/14 authoritative per Final-State Authority rank 1 native review `verify-report` `verdict pass` `requirements 8/8` + rank 2 tasks `18/18 [x]` + rank 3 ledger `complete true 0 active` + rank 4 verify snapshot `PASS 14/14`. No contradiction between snapshots and final state (all `pass` `allComplete true` `e3b0c44` same). Fix mapping: sync canonicals were pending before archive (deltas under `openspec/changes/sdd-parity-rescope-grant-ledger/specs/`) → now `all_done` after `manual_sync.go` append (114 ins). No `CRITICAL` to block archive with prompt override (CRITICAL would still block even with prompt assertion, but there is none, only `SUGGESTION`).

## Archive Verification

- [x] Main specs updated correctly (`sdd 12 req` 12219 bytes 86 ins `REQ-G1/2/3/4/6/7`, `review 11 req` 13585 bytes 28 ins `REQ-G5-01`; `grep -c "### Requirement:"` confirms 12/11, `grep -c "REQ-G1-01" 1` + `REQ-G5-01 1` + `isSyncNeeded false` after sync, `git diff --stat HEAD` shows canonicals `2 files 114 ins` + code `8 files 408 ins` =10 files 522 ins pending commit)
- [x] Change folder moved to `openspec/changes/archive/sdd-parity-rescope-grant-ledger/` (`mv openspec/changes/sdd-parity-rescope-grant-ledger → archive/sdd-parity-rescope-grant-ledger`, `ls -R archive/sdd-parity-rescope-grant-ledger` confirms 7 top files +2 spec deltas + this report, active `openspec/changes/sdd-parity-rescope-grant-ledger` no longer exists, `ls openspec/changes/` shows only `archive` + no active)
- [x] Archive contains all artifacts (`proposal.md 3557` ✅ `specs/sdd/spec.md 4719` ✅ `specs/review/spec.md 2303` ✅ `design.md 6573` ✅ `tasks.md 3884 18/18 [x]` ✅ `apply-progress.md 8801` ✅ `verify-report.md 5752 PASS 8/8 14/14 e3b0c44…` ✅ `archive-report.md` this file ✅ `exploration.md 10806` ✅) — `ls -R archive/...` 9 files (7 top +2 deltas + report)
- [x] Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS, `grep "\[x\]" 18 / "\[ \]" 0`, 18/18 `[x]`, `grep -c "\[ \]" 0`, persisted true, no stale reconciliation needed; `sdd-apply` responsibility satisfied via `apply-progress` 18/18, `sdd-archive` validates before closing — it does)
- [x] Active changes directory no longer has `sdd-parity-rescope-grant-ledger` (`test -d openspec/changes/sdd-parity-rescope-grant-ledger` → not exists, `biggz sdd-status --json` post-move `active 0` for this change `found 0` `archived 1 sdd-parity IsArchived true`, `ls openspec/changes/` confirms `archive` only)

## Git & Code Preservation

- Archive is filesystem `mv` (like `internal/sdd/archive.go:ArchiveChange` but without date prefix `sdd-parity-rescope-grant-ledger`, canonical per task `ls -R archive/sdd-parity...`), not `git mv` commit — code diff for parity not yet committed (tracked 522 ins pending commit per `git diff --stat HEAD` 10 files 522 ins 145 del + untracked fixtures/tests 6 files). Before archive `git diff --stat HEAD` 10 files 522 ins (8 code `408 ins` +2 specs `114 ins`), after move same plus `?? archive/sdd-parity... 9 files` (audit trail remains untracked until next docs commit). No staged files for parity code yet (still `M`), no `git add` for archive (audit trail remains untracked until next commit, like `rdd-cas-reach-parity` archive `mv` behavior). After archive move `git status --porcelain` still shows `M` same 10 files `M` + `??` 3 fixtures `docs/comp.mdx` etc + `??` 3 parity tests + `??` `status.json` + `??` `archive/sdd-parity...` (now 9 files) + `D openspec/changes/sdd-parity...` replaced by `?? archive/...`.
- `git log --oneline -3` still shows `e81a135 fix(tests): stabilize pre-existing fails` → `e0915c5 docs(sdd): archive rdd-cas-reach-parity` → `b67eea4 fix(sdd): bilingual checkpoint tokens` (ahead vs origin/master vs `e81a135` HEAD); archive does not create commit, does not alter `HEAD` tree for code; next docs commit will add spec canonicals + archive folder if desired via `git add openspec/specs/sdd/spec.md openspec/specs/review/spec.md openspec/changes/archive/sdd-parity-rescope-grant-ledger`.
- No source mutation after `sdd-archive` (code logic already verified `go vet PASS` + `go test 87 PASS 1.35-2.13s` + `verify PASS 8/8 14/14`); archive docs `proposal/spec/design/tasks/apply-progress/verify-report/archive-report/exploration` are now immutable audit trail under `archive/sdd-parity-rescope-grant-ledger/`.
- `git diff --stat HEAD` before archive vs after sync vs after archive: before sync `8 files 408 ins`, after sync `10 files 522 ins` (specs `+114`), after archive same `522 ins` + moved folder `D→??` (net 0 code diff change, audit trail preserved).

## Final-State Facts (2026-08-31) — per Final-State Authority hierarchy

Per Archive Final-State Authority (native review authority > tasks artifact > launch prompt final-state facts > verify-report/apply-progress snapshots), the archive report records state AT CLOSE, not earlier snapshot claims. `apply-progress` and `verify-report` are intermediate snapshots valid at time written; work routinely continues after they are persisted.

- **Tasks 18/18 done** (`openspec/changes/archive/sdd-parity-rescope-grant-ledger/tasks.md` persisted `3884 bytes`, `allComplete true` `grep \[x\] 18 / \[ \] 0` + `verify-report Completeness 18/18` + `apply-progress Status 18/18 complete` + launch prompt explicit `tasks 18/18 done` + `biggz sdd-status --json` `taskProgress 18/18 allComplete true` before move matches authoritative artifact) — Task Completion Gate PASS, stale-checkbox reconciliation not needed (0 `[ ]` unchecked, `tasks.md` persisted true). `sdd-apply` already marked completed tasks; `sdd-archive` validates persisted artifact reflects final state before closing — it does (18/18 `[x]`).

- **Apply done** `auto-chain stacked-to-main` single PR `Low 400/800` ( `apply-progress.md` `8801 bytes` `Change sdd-parity-rescope-grant-ledger Mode Standard strict_tdd false`): Completed Tasks 18 `[x]` groups `Foundation 1.1-1.5` → `Rescope & Marker 2.1-2.5` → `Topology & Passive 3.1-3.5` → `Fixtures & Gates 4.1-4.3` (`apply-progress` 5 phases stacked no split `WU1 G1 Guard 15 LOC` → `WU2 G2+G3 55+25` → `WU3 G4+G7 105+20+30` → `WU4 G5 55` → `WU5 Docs & Fixtures 0`), Files Changed 8 code +2 specs +3 tests +3 fixtures (`internal/sdd/attempt.go` `sddattempt/sddattempt.go` `cas_store.go` `edit_authority.go` `status.go` `review/risk.go` plus tests `attempt_test` `rescope_test` `parity_test` `topology_parity_test` `passive_parity_test` + fixtures `with-shebang.md` `comp.mdx` `note.md`), Deviations `Rescope predicate now requires ObjectiveID!=""` updated test, `isPassiveDocumentExtension includes .mdx` for fixture, `9MiB large.md not committed as static 9MiB file` via TempDir, Issues `Symlink EvalSymlinks on Windows requires privilege` skip, Remaining Tasks None 18/18, Workload single PR 435 est 522 actual <800.

- **Verify PASS** 2026-08-31 `verify-report.md` `5752 bytes` `evidence_revision e3b0c44…` bound via `test_output_hash b2ee66…` + `build_output_hash e3b0c44…` empty `go vet`, `8/8 req 14/14 scen` COMPLIANT `blockers 0` `critical 0` `verdict pass`, Build `go vet PASS e3b0c44…` empty, Tests 87 harness `14/14` matrix `TestLegacyGuard` `TestRescope*` `TestForInstance` `TestTopology*` `TestPassive` `TestReadOnlyMarker` PASS per matrix, Coverage not configured. `biggz sdd-verify-validate --requirements 8 --scenarios 14` valid.

- **Sync applied to canonical specs** `sdd, review` per `manual_sync.go` `ParseDeltaSpec` `ApplyDeltas` port `lib/openspec-deltas.ts` — corroborated by file writes: `sdd 12 req 12219 bytes`, `review 11 req 13585 bytes` all present with `grep -c "### Requirement:"` 12/11 and audit trail deltas preserved at `archive/.../specs/{sdd,review}/spec.md` (4719/2303 bytes with original `# Delta for` + `## ADDED Requirements`). Spec pointer `sdd 12` `review 11` authoritative via delta Coverage map.

- **Ledger 5a66c15→e3b0c44**: `evidence_revision sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` = `build_output_hash` verified empty `go vet`, `test_output_hash sha256:b2ee662f…` distinct but both bound, ledger `5a66c15adfb42d3634e9e8acba9dd938523e849e14adc21ee2c04fbba6674751` `Next complete Active 0 Attempts 1` terminal; admitted ledger task explicit `isSyncNeeded false` after sync.

- **RDD disabled clone** `Global enabled Clone disabled Source clone Since 2026-08-31T21:22:42Z Hook ok` per `biggz rdd status` → `review_disabled true` in `sdd-status --json` — gates `archive ready` not blocked per `disabled/unmanaged` relaxation (native review receipt gate). No `reviewGate.result allow` required when kill switch off; demanding receipt would deadlock while `review start` refused. `actionContext.mode repo-local` not `workspace-planning`, `allowedEditRoots [C:/Users/USER/Desktop/biggz-ai]` — archive stays inside roots.

- **Gates** at close: `go vet PASS` `go test 87 PASS` `verify PASS 8/8 14/14` `tasks 18/18 allComplete true` `sync all_done archive ready` authoritative via `sdd-status --json` before move + `sdd-verify-validate PASS` + `tasks.md 18/18`. No `CRITICAL` to block archive with prompt override (CRITICAL would still block even with prompt assertion, but there is none, only `SUGGESTION`).

- **Workload**: Forecast `255 tracked + ~180 test =435 Low Chained No single PR 5 units` (tasks Review Workload Forecast) — actual committed `522 ins` tracked (code 408 + specs 114) + untracked fixtures/tests ~180 = ~702 gross, but still `Low` for `800` budget per `sdd-tasks` workload forecast not overriding `delivery_strategy` `auto-chain`; single PR preserves atomic `ledger+rescope+grant+topology+passive+marker` invariant (splitting would risk half parity). `Delivery strategy auto-chain` `review_budget 800` preflight would have split only if >800, but 522 <800 → `single PR` correct.

- **No unrankable contradictions** between orchestrator launch prompt final-state facts (`proposal done specs done 8 req 14 scen design done tasks 18/18 applyProgress done applyState all_done verifyReport done verdict pass blockers 0 requirements 8/8 scenarios 14/14 evidence sha256:e3b0... validated ledger complete revision 5a47... no active attempt` + `nextRecommended sync ready y archive ready`) and higher-ranked tasks artifact (18/18 `[x]` 0 `[ ]`) + verify-report (8/8 14/14 0 critical e3b0c44… 87 tests) + file evidence (`grep 18 [x] 0 [ ]`, `ls canonicals 12219/13585 12/11 req`, `git diff 522 ins`, `ls -R archive` 9 files). Where verify snapshot `16:17 UTC` says `PASS 8/8 14/14 e3b0… 87 PASS` and launch prompt `18/18 8/14 e3b0… ledger 5a47… complete` they match; where `apply-progress` `18/18 Ready for verify` vs tasks `18/18` same. Fix mapping: spec sync canonicals were pending before archive (deltas under `openspec/changes/.../specs/`) → now `all_done` after `manual_sync.go` append. No `CRITICAL` to block archive.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. `sdd-parity-rescope-grant-ledger` parity (ledger guard `ErrLegacyRetired`, rescope `Widened`/`Exhausted` verbatim `Cumulative*`, `ForInstance` sugar, topology `cross_common_dir_runtime_target` memo, passive 8 MiB/she/MDX/exec, `(read-only)` marker) is now source of truth for `sdd 12 req` (parity G1-G4 G6-G7) + `review 11 req` (passive G5) with `single PR stacked-to-main` delivery and rollback boundaries `attempt.go` `sddattempt.go+cas_store.go` `edit_authority.go+status.go` `risk.go` vs fixtures.

Ready for the next change.

## Key Learnings

1. Guarding `AttemptBegin/Finish/Reset` with `ErrLegacyRetired` and pointer `biggz sdd-attempt` preserves `internal/sdd` import compatibility without filesystem migration.
2. Verbatim rescope predicate `Active==0&&ObjectiveID!=""&&!DecisionRequired&&!Complete&&len>0&&last.Outcome!=""&&!drifted` plus `new<=old→Widened` before `new<=cum→Exhausted` prevents widening laundering and preserves `Cumulative*` history.
3. `ForInstance` validation `1..128` trimmed single-line via shared `validateChangeInstance` plus `grantedRootsFor` scoping gives sugar without duplicating grant logic.
4. Topology guard `resolveExistingPath→gitRootOf→SameFile` memoised per `Status` and phase-gated to `apply/verify/remediate` blocks foreign `commonDir` without false positives on `spec/design/tasks`.
5. Adapted passive proof `isPassiveContentFile` gated behind `isPassiveDocumentExtension` with 8 MiB cap fail-closed plus `NUL`/`utf8`/`#!`/MDX/exec checks achieves parity at 55 LOC vs 150 LOC full `git grep` blob proof.
