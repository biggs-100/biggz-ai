```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:5acf8ad659b96d6b98d1de65b625b2493e6adb85709d41ed6992f36a90922049
verdict: pass
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 9/9
test_command: go test ./internal/sdd/ ./internal/bigmem/ -count=1 -timeout 180s
test_exit_code: 0
test_output_hash: sha256:5acf8ad659b96d6b98d1de65b625b2493e6adb85709d41ed6992f36a90922049
build_command: go vet ./internal/sdd/ ./internal/bigmem/
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: fix-bigmem-status-bypass
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 10 |
| Tasks complete | 10 |
| Tasks incomplete | 0 |

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./internal/sdd/ ./internal/bigmem/
EXIT:0 (empty output, sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855)
```

**Tests**: ✅ Passed
```text
go test ./internal/sdd/ ./internal/bigmem/ -count=1 -timeout 180s
ok github.com/biggs-100/biggz-ai/internal/sdd 14.766s
ok github.com/biggs-100/biggz-ai/internal/bigmem 8.568s
Focused: TestCollectBigMemChanges_* (8/8 incl. Hybrid, NoStore, PersonalExcluded, ProjectOverride, VisibleOnlyHydration, CorruptDB, CancelledCtx, StatusCtxCancel) PASS; TestListByTopicPrefix_* (6/6) PASS
Ledger settle revision 6d23b41d31e7f0ad4aba169ad0f3d95df0e0fb4f4cb30202f69456effb1742fc, evidence sha256:5acf8ad659b96d6b98d1de65b625b2493e6adb85709d41ed6992f36a90922049
```

**Coverage**: ➖ Not available / threshold: N/A → ➖ Not available

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| BigMem Status via Store Ctx API | Store-sourced collection | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_Hybrid` + `rg sql.Open\|db.Query\|modernc internal/sdd/engram_status.go` empty | ✅ COMPLIANT |
| BigMem Status via Store Ctx API | Absent DB falls back explicitly | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_NoStoreFallsBackToNil` | ✅ COMPLIANT |
| SQL-Side Visibility Filtering | Predicates in SQL | `internal/bigmem/topic_prefix_test.go > TestListByTopicPrefix_Predicates + TestListByTopicPrefix_DeletedExcluded + TestListByTopicPrefix_PersonalExcluded + TestListByTopicPrefix_ProjectMatchCaseInsensitive` | ✅ COMPLIANT |
| Minimal Hydration | Visible-only hydration | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_VisibleOnlyHydration` (100 rows / 2 visible, task counts + artifact states match) | ✅ COMPLIANT |
| Caller Context With Timeout | Cancellation fails fast | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_CancelledCtxFailsFast + TestStatusCtxCancel` | ✅ COMPLIANT |
| Caller Context With Timeout | No Background at hot spots | `rg context.Background internal/sdd/status.go` wrappers only + both `IsSessionSummaryBlocked` sites take caller `ctx` | ✅ COMPLIANT |
| Visible BigMem Failures | Query error surfaces | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_CorruptDBSurfacesError` | ✅ COMPLIANT |
| Project Visibility Parity | Personal excluded | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_PersonalExcluded` + `TestListByTopicPrefix_PersonalExcluded` | ✅ COMPLIANT |
| Project Visibility Parity | Project match and override | `internal/sdd/engram_status_test.go > TestCollectBigMemChanges_ProjectOverrideDisablesFilter` + `TestListByTopicPrefix_ProjectMatchCaseInsensitive + TestListByTopicPrefix_EmptyProjectBypass` | ✅ COMPLIANT |

**Compliance summary**: 9/9 scenarios compliant

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| BigMem Status via Store Ctx API | ✅ Implemented | `engram_status.go` uses `bigmem.Open` + `ListByTopicPrefixCtx` + `GetCtx`; no `sql.Open`/`db.Query`/`modernc` import; absent-DB logs warning and returns `(nil,nil,nil)` |
| SQL-Side Visibility Filtering | ✅ Implemented | `topic_prefix.go` SQL has `topic_key LIKE ?`, `deleted_at IS NULL`, `project COLLATE NOCASE`, personal exclusion; ordered, uncapped, served by `idx_obs_topic_lookup` |
| Minimal Hydration | ✅ Implemented | Collector parses keys first via `parseTopicKey`, hydrates only pattern survivors via `GetCtx` with per-row ctx check |
| Caller Context With Timeout | ✅ Implemented | `StatusCtx`/`StatusWithOptionsCtx`/`applyStoreRoutingCtx`/`collectBigMemChangesWithArchiveCtx` + `bigmem.WithTimeout` 5s; entry ctx.Err check; hot spots pass caller ctx |
| Visible BigMem Failures | ✅ Implemented | Resolve/open/list/get errors logged + wrapped `bigmem sdd-status <op>: %w`; never silent; engram routing propagates errors |
| Project Visibility Parity | ✅ Implemented | `project=""` disables filter (test override); production infers project; `scope=personal` excluded `COLLATE NOCASE` |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Key-only sweep + hydrate visible via GetCtx (Q1/Q3) | ✅ Yes | `ListByTopicPrefixCtx` in `internal/bigmem/topic_prefix.go` (~79 lines) + `GetCtx` hydration, no cap raise, no pagination |
| New *Ctx overloads, old funcs delegate via Background() (Q2) | ✅ Yes | `StatusCtx`/`StatusWithOptionsCtx`/routing/collect/read/derive `*Ctx`; wrappers only hold `Background` |
| Timeout via bigmem.WithTimeout 5s | ✅ Yes | Collector + status entry both wrap; caller deadline wins |
| project="" override convention + COLLATE NOCASE | ✅ Yes | Preserves `SetBigMemStoreRootForTest` semantics |
| Filesystem-wins merge untouched | ✅ Yes | `mergeFilesystemAndBigMem` unchanged |
| No migration, per-file rollout | ✅ Yes | Additive `topic_prefix.go` first; no schema change |

### Issues Found
**CRITICAL**: None
**WARNING**: Review budget overrun — ~775 changed lines (prod +243/-126 net +117, tests ~406) vs 400-line budget and 300–360 forecast; single-PR per orchestrator directive, clean split available at Unit 1/Unit 2 boundary; recommend parent confirm `size:exception`. Pre-existing workspace dirt outside this change left untouched (`openspec/changes/fix-bigmem-store-ctx/` deletions, `openspec/specs/bigmem/spec.md` modification, `openspec/changes/archive/2026-09-04-fix-bigmem-store-ctx/`).
**SUGGESTION**: None

Modern Go guidelines: consulted `sh "internal/assets/skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/bigmem/topic_prefix.go` (exit 0, full list read); code already modern (`slices.SortFunc`+`cmp.Compare`, `errors.Is` wrapping, `any` args); no missed modernization, no warning.

### Verdict
PASS WITH WARNINGS
All 10 tasks complete, 6/6 requirements and 9/9 scenarios compliant with passing runtime evidence; warnings are budget/dirt only.
