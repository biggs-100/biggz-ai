```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:902b6d509c0002cec2c72c964e71a965b56e0aef786613d91400ea47c2b112fb
verdict: pass
blockers: 0
critical_findings: 0
requirements: 7/7
scenarios: 19/19
test_command: go test ./internal/codegraph -count=1 -timeout 60s -v && go test ./internal/agentbuilder -run TestAdvisoryHint -count=1 -timeout 60s -v && go test ./cmd/biggz -run TestCodeGraph -count=1 -timeout 60s -v
test_exit_code: 0
test_output_hash: sha256:902b6d509c0002cec2c72c964e71a965b56e0aef786613d91400ea47c2b112fb
build_command: go vet ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: codegraph-change-intent-full
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 16 |
| Tasks complete | 16 |
| Tasks incomplete | 0 |

All 16 tasks marked [x] in tasks.md. Apply-progress documents PR1 (core engine) and PR2 (CLI+hint) complete. No pending tasks block verification.

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./...
exit 0, hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 (empty output, no vet errors)
```

**Tests**: ✅ 27 passed / ❌ 0 failed (relevant) / ⚠️ 2 unrelated failures pre-existing
```text
go test ./internal/codegraph -count=1 -timeout 60s -v -> PASS 16 tests (3.162s)
go test ./internal/agentbuilder -run TestAdvisoryHint -count=1 -> PASS 3 tests (2.485s)
go test ./cmd/biggz -run TestCodeGraph -count=1 -> PASS 9 tests (2.277s)
Combined hash sha256:902b6d509c0002cec2c72c964e71a965b56e0aef786613d91400ea47c2b112fb

Full suite go test ./... -count=1 -timeout 180s -> FAIL 2 pre-existing unrelated:
 - internal/sdd TestReadLoopLarge (flaky, also fails on clean stash without this change)
 - internal/tui/screens TestHelpModel_ViewportRenderingWithFilter (also fails on clean stash)
Relevant packages PASS; unrelated failures not caused by this change (verified via stash test).
```

**Coverage**: ➖ Not available (no threshold configured; go test without -cover)

**Additional runtime evidence**
```text
biggz codegraph report codegraph-change-intent-full --cwd . --json openspec/changes/codegraph-change-intent-full/codegraph.json --md openspec/changes/codegraph-change-intent-full/codegraph.md
 -> exit 0, JSON 19M + Markdown 11M written, dual stdout+file verified
Custom: --json /tmp/verify_evidence.json --md /tmp/verify_evidence.md -> PASS, MkdirAll verified

biggz codegraph --help -> lists report <change> [--cwd][--json][--md] ✅
biggz codegraph report --help -> documents flags ✅
biggz codegraph report (no change) -> usage error exit 1 ✅
biggz codegraph report does-not-exist -> proposal required exit 1 ✅

biggz codegraph report codegraph-change-intent-full --cwd . (no custom) -> JSON stdout + codegraph.md under openspec/changes/<change>/ ✅

Orchestrator hint: AdvisoryHint present/absent tests PASS; LoadHint nil when absent without block ✅

Modern Go guidelines: sh "skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/codegraph/types.go|report.go|scan.go|hint.go consulted (Go 1.25 guidelines listed, 48 items, newest first). No critical modernization missed; current code uses context with 30s timeout, filepath.EvalSymlinks+Abs, parser fallback, BFS closure, MkdirAll, atomic writes — idiomatic. No WARNING for modern Go.
```

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| SDD Artifact Intent Extraction | Proposal-only extraction succeeds | `internal/codegraph > TestExtractIntent_ProposalOnly` | ✅ COMPLIANT |
| SDD Artifact Intent Extraction | Missing proposal blocks inference | `internal/codegraph > TestExtractIntent_MissingProposalFails` + `TestGenerate_ProposalRequired` + `cmd/biggz > TestCodeGraph_ReportMissingProposalFails` | ✅ COMPLIANT |
| SDD Artifact Intent Extraction | Symbol weight exceeds keyword weight | `internal/codegraph > TestExtractIntent_SymbolWeightExceedsKeyword` | ✅ COMPLIANT |
| Go Dependency and Call Graph Scan | Import and call edges discovered | `internal/codegraph > TestScanGo_ImportAndCallEdges` | ✅ COMPLIANT |
| Go Dependency and Call Graph Scan | Scan timeout enforced | `internal/codegraph > TestGenerate_TimeoutNoPartial` + `TestGenerate_30sTimeoutNoPartial` | ✅ COMPLIANT |
| Full Graph with Transitive Closure | Transitive closure expands blast radius | `internal/codegraph > TestGraph_TransitiveClosure` | ✅ COMPLIANT |
| Full Graph with Transitive Closure | Flat-list guard | `internal/codegraph > TestGraph_FlatListGuard` | ✅ COMPLIANT |
| Dual Output JSON and Markdown Emission | Default dual emission | `internal/codegraph > TestGenerate_DualEmissionDefaultPaths` + `cmd/biggz > TestCodeGraph_ReportDefaults` + runtime `biggz codegraph report --cwd .` | ✅ COMPLIANT |
| Dual Output JSON and Markdown Emission | Custom paths override | `internal/codegraph > TestEmit_MkdirAll` + `cmd/biggz > TestCodeGraph_ReportCustomPaths` + runtime custom --json/--md | ✅ COMPLIANT |
| Advisory Consumption by Human and Orchestrator | Human reads Markdown | `internal/codegraph > TestRenderMarkdown_ContainsFilesAndGraph` + runtime codegraph.md inspection (files table + graph summary) | ✅ COMPLIANT |
| Advisory Consumption by Human and Orchestrator | Orchestrator optional hint | `internal/codegraph > TestLoadHint_NilWhenAbsent` + `TestLoadHint_ReadAndNil` | ✅ COMPLIANT |
| CodeGraph Report Verb (cli) | Report emits dual output | `cmd/biggz > TestCodeGraph_ReportDefaults` (exit 0 + JSON+MD) | ✅ COMPLIANT |
| CodeGraph Report Verb (cli) | Custom output flags | `cmd/biggz > TestCodeGraph_ReportCustomPaths` (MkdirAll) | ✅ COMPLIANT |
| CodeGraph Report Verb (cli) | Missing change fails | `cmd/biggz > TestCodeGraph_ReportMissingChangeFails` + `ReportMissingProposalFails` + `UsageErrors` | ✅ COMPLIANT |
| CodeGraph Report Verb (cli) | Help documents report | `cmd/biggz > TestCodeGraph_HelpDocumentsReport` + `UsageErrors` | ✅ COMPLIANT |
| CodeGraph Report Verb (cli) | Existing init preserved | `cmd/biggz > TestCodeGraph_ReportPreservesInitAndGuidance` + `TestCodeGraph_Guidance` | ✅ COMPLIANT |
| CodeGraph Advisory Scope Hint (orchestrator) | Report present surfaces hint | `internal/agentbuilder > TestAdvisoryHint_PresentSurfacesFiles` | ✅ COMPLIANT |
| CodeGraph Advisory Scope Hint (orchestrator) | Report absent continues normally | `internal/agentbuilder > TestAdvisoryHint_AbsentContinues` | ✅ COMPLIANT |
| CodeGraph Advisory Scope Hint (orchestrator) | Advisory does not auto-apply | `internal/agentbuilder > TestAdvisoryHint_DoesNotBlock` (hint string only, no mutate) | ✅ COMPLIANT |

**Compliance summary**: 19/19 scenarios compliant

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| SDD Artifact Intent Extraction | ✅ Implemented | ExtractIntent parses proposal.md required + specs/design/tasks optional, regex symbol `[A-Z][a-zA-Z0-9_]*` weight 2 > keyword 1, `sdd` reason, proposal required fail in intent.go + report.go Generate |
| Go Dependency and Call Graph Scan | ✅ Implemented | ScanGo primary go/packages cached 30s timeout via context.WithTimeout, fallback parser+ast.Inspect, Go-only *.go filter in scan.go; tested via cached and GoOnlyFilter |
| Full Graph with Transitive Closure | ✅ Implemented | BuildGraph merges sdd+import+call, BFS transitive closure, sddReachable pruning, isolated sdd preserved via self-loop guard in graph.go |
| Dual Output JSON and Markdown Emission | ✅ Implemented | Report struct Files+Graph with reasons sdd/import/call, Generate 30s ctx + proposal required abort, Emit MkdirAll atomic tmp+rename no partial, RenderMarkdown files table + graph summary |
| Advisory Consumption by Human and Orchestrator | ✅ Implemented | RenderMarkdown lists files with reasons and graph summary; codegraph.md deletable safe, no VCS mutation; verified via Markdown inspection |
| CodeGraph Report Verb | ✅ Implemented | cli_codegraph.go reportRun router, flags --cwd/--json/--md, resolveReportRoot Abs+EvalSymlinks, help lists report, Generate+Emit dual stdout+file, custom paths with MkdirAll |
| CodeGraph Advisory Scope Hint | ✅ Implemented | hint.go LoadHint nil if absent, agentbuilder/sdd.go AdvisoryHint/FormatAdvisoryHint surfaces files advisory only, no auto-mutate/block when absent |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Intent extraction regex keyword+Symbol weight 2>1 sdd reason proposal required fail | ✅ Yes | intent.go implements symbolRe + keywordRe, WeightSymbol=2 WeightKeyword=1, proposal required error |
| Go scan go/packages primary cached 30s timeout fallback parser+ast.Inspect Go-only | ✅ Yes | scan.go ScanGo with context, packages.Load + fallback, 30s enforced in report.go Generate |
| Graph model Unified {nodes,edges} reason sdd/import/call BFS closure isolated sdd kept | ✅ Yes | graph.go BuildGraph with adjacency, BFS closure, sddReachable, isolated guard |
| Dual output JSON stdout+file default codegraph.json + MD codegraph.md MkdirAll no partial | ✅ Yes | report.go Emit, cli_codegraph.go defaults under openspec/changes/<change>/, MkdirAll, atomic writes |
| Orchestrator hook LoadHint hint.go agentbuilder/sdd.go advisory only no auto-mutate | ✅ Yes | hint.go LoadHint nil if absent, AdvisoryHint returns string without mutating tasks |
| Threat/matrix Go-only filter, cwd via Abs+EvalSymlinks, no shell interpolation | ✅ Yes | resolveReportRoot, resolveCwd, scan Go-only |

### Issues Found
**CRITICAL**: None

**WARNING**: 
- Full suite `go test ./...` contains 2 pre-existing failures unrelated to this change (internal/sdd TestReadLoopLarge flaky, internal/tui/screens TestHelpModel_ViewportRenderingWithFilter). Verified they also fail on clean stash without this change; not blockers. Relevant tests PASS.
- Ledger `sdd-attempt` for this change required reset after PR2 passed complete=true to allow verify work-unit. Reset performed with reason "verify new work-unit after apply complete" (revision aeecdcc...), then verify acquire settle passed. Not a code issue; ledger lifecycle expected.

**SUGGESTION**:
- Consider adding `go test -cover` reporting or explicit coverage threshold for future changes.
- JSON output currently includes stdlib paths due to full repo scan; consider filtering to project-relative paths only or adding config flag to suppress stdlib noise (advisory only, not failing).
- Modern Go: `use-modern-go` list suggests potential future idioms (slices.SortFunc, maps.Keys) not yet applied; no immediate action required as current implementation is idiomatic and passes vet.

### Verdict
PASS
All 7 requirements and 19 scenarios compliant with passing covering tests; build passes; dual JSON+Markdown emission and advisory hint verified at runtime; design decisions followed; no critical findings.
