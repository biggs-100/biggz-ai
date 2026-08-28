# Archive Report: testing-guidance — Contract-First Testing Filter + Lint Guards

**Change**: `testing-guidance` → `2026-08-27-testing-guidance`
**Archived**: 2026-08-27
**Archived to**: `openspec/changes/archive/2026-08-27-testing-guidance/`
**Previous location**: `openspec/changes/testing-guidance/` (active)
**Mode**: `interactive`, `openspec`, `auto-chain`, `800 lines`, `single PR 300-400`, `strict_tdd off`, `go test ./... -count=1 -timeout 180s`
**Artifact Store**: `openspec` — `openspec/changes/testing-guidance` → `openspec/changes/archive/2026-08-27-testing-guidance/` + `openspec/specs/testing-guidance/spec.md` source of truth
**Preflight**: `interactive` / `openspec` / `auto-chain` / `800` — single PR under budget, no split needed

## Summary

Completed `testing-guidance` — docs + linter + CI contract-first testing filter. `docs/testing-guidance.md` codifies Good vs Bad filter (Good: failure mode, transformation/branch, external contract, regression; Bad: static echo, passthrough, wording, duplicate) with bans (`mock.module` `oven-sh/bun#12823` global leak, source-grep `expect(src).toContain` / `os.ReadFile`+`Contains` on `*.go`/`*.md`) and `bench:guard` (`go test ./bench` never proves driven execution). Custom analyzer `tools/nosourcegrep` (AST `*ast.CallExpr`, `*_test.go` scope, `testdata` allowlist) enforces bans via primary `go vet -vettool=./tools/nosourcegrep` + `golangci-lint` custom `nosourcegrep` (`testdata` + `shim_test.go` excludes) + `rg` fallback. CI adds `lint-no-source-grep` (vet primary + golangci fallback warning + `rg` + `mock.module` ban) and `rapid` (`go test -run TestRapid ./... -count=1 -timeout 180s`). Pinned anchors `TestBlob_ConcurrentSameBytes` (`-race`, failure mode + external contract) and `TestBranch_Traversal` (`TestBranchCreateChild`/`TestBranchListGetChain` via `modernc.org/sqlite`) with `// Good:` comments link docs to DB-contract tests versus Bad `TestExpectSrcContains`.

Shipped as single PR, **~300-350 net** (tracked `97 insertions` in 10 files + untracked `docs/testing-guidance.md` + `tools/nosourcegrep/` + `.golangci.yml` ≈ 250-350), under `800` budget (`400`-line Low risk, `800 Low`, `Chained PRs recommended: No` per tasks forecast). All **12/12 tasks** (4 phases) complete, **6/6 requirements, 9/9 scenarios** verified PASS, `go vet` + `go vet -vettool` + `gofmt -l` clean, `go test ./...` + `go test -run TestRapid` green, `mock.module` empty, `bench:guard` pinned.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 12/12 marked `[x]` — `total:12 completed:12 pending:0 allComplete:true` (`biggz sdd-status --json` `total:12 completed:12` before archive, `dependencies.tasks: all_done`) |
| Verify verdict | ✅ PASS — 0 blockers, 0 CRITICAL, 6/6 req 9/9 scen (`biggz sdd-verify-validate --requirements 6 --scenarios 9` PASS) |
| Spec compliance | ✅ 9/9 scenarios COMPLIANT, 6/6 requirements satisfied |
| Build | ✅ `go vet ./...` exit 0 (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` empty), `go vet -vettool=/tmp/nosourcegrep ./...` clean on valid code, flags 3 diagnostics on `bad.go` fixture, `gofmt -l .` empty, `go build -o /tmp/nosourcegrep ./tools/nosourcegrep/cmd/nosourcegrep` 0 |
| Tests | ✅ `go test ./tools/nosourcegrep -count=1 -v` → `TestAnalyzer` PASS (bad flagged, good passes, 1.72s, `analysistest.Run`), `go test -run TestRapid ./... -count=1 -timeout 180s` → 11 `TestRapid_*` PASS (`compact_state_rapid_test.go` 6 + `lifecycle_rapid_test.go` 5, internal/review 3.65s), `go test ./... -count=1 -timeout 180s` → PASS (65+ packages, 0 failures, evidence_revision `sha256:8286d0e1d6f7809054e52406fdc2f9d3e8760679b09ec1e101a8ef9b03343f59`) |
| Evidence | `evidence_revision sha256:8286d0e1d6f7809054e52406fdc2f9d3e8760679b09ec1e101a8ef9b03343f59` (test_output_hash), `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `test_command go test ./... -count=1 -timeout 180s` `test_exit_code 0`, `build_command go vet ./...` `build_exit_code 0` |
| Review gate | N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` emits no `reviewGate` field for `openspec` changes; prior to archive `nextRecommended: archive`, `dependencies.archive: ready`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done}`, `artifactStore: openspec`, `applyState: all_done` — gate PASS (consistent with archived precedent `bigmem-blobstore`, `tui-sanitize`) |
| Task gate | PASS — persisted `openspec/changes/archive/2026-08-27-testing-guidance/tasks.md` shows 12/12 `[x]`, 0 `[ ]` pending. `taskProgress: {total:12, completed:12, pending:0, allComplete:true}` |
| Ledger | `sdd-attempt status` at verify time reported `complete:true corrupt_authority ledger is complete; reset required` vs `sdd-status --json` `nextRecommended: archive` + `taskProgress.allComplete:true` + `verifyReport done`. Verification proceeds on file-backed evidence per `openspec` store (preflight interactive openspec auto-chain). Not blocking per archived precedent; file-backed evidence is authority for openspec archive readiness. |

## Spec Compliance

**Verdict**: PASS (per `verify-report.md`, `evidence_revision sha256:8286d0e1d6f7809054e52406fdc2f9d3e8760679b09ec1e101a8ef9b03343f59`, `biggz sdd-verify-validate --requirements 6 --scenarios 9` PASS)

| Metric | Value |
|--------|-------|
| Requirements | 6/6 compliant |
| Scenarios | 9/9 compliant (0 UNTESTED) |
| Tasks | 12/12 complete (Phase 1:2, Phase 2:4, Phase 3:3, Phase 4:3) |
| Blockers / Critical | 0 / 0 |
| Warnings at verify | 4 warnings (golangci plugin fallback, shim exempt, wg.Go, ledger corrupt_authority) — reconciled at archive as non-blocking intentional (see Final-State Reconciliation) |
| Production net | ~300-350 (single PR, <800) |

**Detailed matrix** (from `verify-report.md`, each COMPLIANT):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Test Filter — Good vs Bad Classification | Good test passes filter | `tools/nosourcegrep/testdata/src/good/good.go` (Good: DB query via `modernc.org/sqlite`) + `internal/bigmem/blobstore_test.go > TestBlob_ConcurrentSameBytes` (failure mode + external contract, -race) + `go test ./tools/nosourcegrep -v > TestAnalyzer` (good passes) | ✅ COMPLIANT |
| Test Filter — Good vs Bad Classification | Bad test is rejected | `tools/nosourcegrep/testdata/src/bad/bad.go > Bad()` (os.ReadFile+Contains, expect(src).toContain) + `go vet -vettool=/tmp/nosourcegrep ./tools/nosourcegrep/testdata/src/bad/...` flags 3 diagnostics (`os.ReadFile on source file`, `strings.Contains/bytes.Contains`, `expect(src).toContain`) + `analysistest.Run` | ✅ COMPLIANT |
| Ban — mock.module | mock.module is rejected | `tools/nosourcegrep/analyzer.go` (BasicLit + SelectorExpr + CallExpr for `mock.module`, `isExemptFile`, `isTestFile`) + `rg -n "mock\.module" --glob '*_test.go' .` empty + negative: temp `mock_check_test.go` flagged `mock.module is banned (oven-sh/bun#12823)` + CI `Ban mock.module` job `.github/workflows/ci.yml:264-273` | ✅ COMPLIANT |
| Ban — Source-Grep Assertions | Source-grep is flagged | `tools/nosourcegrep/testdata/src/bad/bad.go` (`os.ReadFile("internal/foo.go")` + `strings.Contains`) + `go vet -vettool` flags + `golangci-lint` custom `nosourcegrep` config (fallback) + `rg -n "os\.ReadFile.*Contains|expect\(src\)" --glob '*_test.go'` parity (empty on valid) | ✅ COMPLIANT |
| Ban — Source-Grep Assertions | Valid contract assertion passes | `tools/nosourcegrep/testdata/src/good/good.go` (`sql.Open` + `QueryRow parent_id/leaf_id`) passes vet + `internal/bigmem/branch_test.go > TestBranchCreateChild / TestBranchListGetChain` (DB state via `modernc.org/sqlite`) + `go vet -vettool ./...` clean | ✅ COMPLIANT |
| Bench Guard | Bench success does not prove driven | `docs/testing-guidance.md:77-84` `## bench:guard` — `go test ./bench -count=1 never proves driven execution` + doc anchors `bench:guard` searchable + CI does not use bench as proof (only `go test -run TestRapid` + `lint-no-source-grep` are gates) | ✅ COMPLIANT |
| Guidance Documentation | Doc completeness | `docs/testing-guidance.md` (105 lines, 8.1K) Good/Bad table (4 Good + 4 Bad), bans (`mock.module` #12823, source-grep), `bench:guard`, 3+ pinned examples: `TestBlob_ConcurrentSameBytes` (-race 200KiB wg) with `// Good:` anchor, `TestBranch_Traversal` (`TestBranchCreateChild`/`TestBranchListGetChain`/`TestGetLeafPathChain` with `modernc.org/sqlite`), `TestExpectSrcContains` Bad + rapid FSM + `ledger_regression_test` context | ✅ COMPLIANT |
| CI Enforcement — Lint and Rapid | CI blocks source-grep and runs TestRapid | `.github/workflows/ci.yml:220-293` job `lint-no-source-grep` (build nosourcegrep, `go vet -vettool=/tmp/nosourcegrep ./...` primary, `rg` fallback, `mock.module` check) + job `rapid` (`go test -run TestRapid ./... -count=1 -timeout 180s` without `-short`) + `go test -run TestRapid ./...` PASS 11 funcs | ✅ COMPLIANT |
| CI Enforcement — Lint and Rapid | CI passes on valid Good test | `go vet -vettool=/tmp/nosourcegrep ./...` passes on `TestBlob_ConcurrentSameBytes` + `go test -run TestRapid` passes + `go test ./...` + `go vet` + `gofmt -l` gates remain clean per `ci.yml:jobs.test/format/complexity` | ✅ COMPLIANT |

## Final-State Reconciliation (per Final-State Authority hierarchy)

`verify-report` and `apply-progress` are intermediate snapshots valid at their write time, not evidence of final state. Final-state authority at close ranks: (1) native `sdd-status` + `reviewGate` (none for openspec), (2) persisted `tasks.md`, (3) explicit final-state facts in launch prompt (none beyond preflight), (4) `verify-report`. No orchestrator-provided post-verify fix commits were reported, so verify warnings remain final state at close and are non-blocking by design:

- **golangci plugin fallback**: `verify-report` W: `golangci-lint run ./...` → `Error: build linters: unable to load custom analyzer "nosourcegrep": plugin: not implemented`. At archive, `.golangci.yml` (992B) correctly declares custom `nosourcegrep` (`path: ./tools/nosourcegrep`) for both v2 `linters.settings.custom` and v1 `linters-settings`, and `.github/workflows/ci.yml:251-252` degrades correctly: `go vet -vettool=/tmp/nosourcegrep ./...` is primary gate, `golangci-lint run` is optional with `::warning` fallback (`if command -v golangci-lint`). Parity verified via `go vet` ≈ `rg` on fixtures (both flag `bad.go` 3 diagnostics, both pass `good.go`/`TestBlob_ConcurrentSameBytes`). No contradiction; warning is intentional runner limitation already handled in CI design decision (chosen: custom via `golangci-lint` primary + `rg` fallback).

- **shim exempt**: `verify-report` W: `tools/nosourcegrep/analyzer.go` uses `strings.Contains(val, "/")` slash-path guard and `isExemptFile` for `internal/extension/shim_test.go` (contains `os.ReadFile("shim.go")` + `strings.Contains` without slash). At archive, `internal/extension/shim_test.go:TestShim_DeprecatedAnnotation`/`TestAgentAdapterShim_Deprecated` etc. still contain source-grep (read `shim.go` + `Contains`) but proposal explicitly marks `extension-api` Out of Scope, `.golangci.yml` `issues.exclude-rules` lists exactly this path, and CI `rg` fallback `grep -v "internal/extension/shim_test.go"` mirrors the analyzer exempt. Analyzer comment documents this as initial-rollout allowlist; future slice should migrate to DB-contract style. Intentional, not a gap.

- **wg.Go**: `verify-report` W/info: `use-modern-go` suggests `sync_waitgroup_go: Use wg.Go` for `TestSetLeafRace` / `TestBlob_ConcurrentSameBytes` (`wg.Add(1)/go func/defer wg.Done`). At archive, both tests remain `wg.Add`/`Done` (correct and `-race`-safe). `wg.Go` (Go 1.25) is optional modernization, not a correctness blocker per verify `No CRITICAL modernization missed without justification; recorded as WARNING only if missed wg.Go is considered material, but current implementation is idiomatic`. No fix required.

- **ledger corrupt_authority**: `verify-report` W: `biggz sdd-attempt status` reports `complete:true corrupt_authority ledger is complete; reset required to continue` while `biggz sdd-status --json` reports `verify ready` / `archive ready` and `taskProgress.allComplete:true`. At archive (2026-08-27), `biggz sdd-status --json` (captured pre-move) still shows `nextRecommended: archive`, `dependencies.archive: ready`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done}`, `artifactStore: openspec`, `applyState: all_done`, `taskProgress {total:12, completed:12, pending:0, allComplete:true}`, `artifacts {proposal:done, specs:done, design:done, tasks:done, verifyReport:done}`. Per `sdd-status-contract.md` divergences, `sdd-status` is authoritative for `openspec` file artifacts and `biggz has no review authority` on this path, so `dependencies.archive: ready` governs archive readiness. `evidence_revision` remains bound to `go test ./...` output hash (`sha256:8286d0e1...`) via `verify-report` and `biggz sdd-verify-validate` PASS with `--requirements 6 --scenarios 9`. Ledger `corrupt_authority` would require explicit maintainer `biggz sdd-attempt reset` only for next runtime-bearing attempt, not for file-backed verify/archive which is already `ready`. No contradiction to resolve; authority hierarchy places file-backed `sdd-status` above ledger complete for this store. If strict ledger binding were required for a successor, that reset is a distinct future decision.

No CRITICAL issues exist at close (`critical_findings: 0`, `verify-report` `No blockers`). No orchestrator final-state facts contradict intermediate snapshots. Numbers carried from highest-ranked source (`sdd-status` + persisted `tasks.md`): `12/12` tasks, `6/6` req, `9/9` scen, `go test ./...` PASS, `go vet ./...` 0, `gofmt -l` empty, `rapid` 11 funcs PASS.

## Spec Sync

Delta specs merged into main specs (source of truth) BEFORE archive move. In `openspec` mode `openspec/specs/` is the audit authority; filesystem wins on conflict. New domain — full spec copy, no delta append semantics needed. Preserved artifactStore `openspec`.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| testing-guidance | **Created (new domain)** | 6 requirements, 9 scenarios — Test Filter Good vs Bad (2), Ban mock.module (1), Ban Source-Grep (2), Bench Guard (1), Guidance Documentation (1), CI Enforcement Lint and Rapid (2). Full spec copied verbatim, no preservation of OTHER requirements needed (new domain). | `openspec/specs/testing-guidance/spec.md` ✅ 85 lines, 3.9K |

- **Pre-sync check**: `openspec/specs/testing-guidance/spec.md` did not exist (new capability per proposal `Modified Capabilities: None` except new domain). Checked `ls openspec/specs/testing-guidance` → not found before copy.
- **Copy**: `openspec/changes/testing-guidance/specs/testing-guidance/spec.md` → `openspec/specs/testing-guidance/spec.md` via `cp` (diff identical, `diff -u` 0). No REMOVED (requires `Reason`/`Migration`) or RENAMED or MODIFIED (no prior `testing-guidance` domain).
- **Verification**: `ls openspec/specs/testing-guidance/spec.md` present 85 lines, `grep -c "### Requirement"` → 6, `grep -c "#### Scenario"` → 9, `biggz sdd-verify-validate --requirements 6 --scenarios 9` PASS (verify-report already PASS). Subsequent consumers read from `openspec/specs/testing-guidance/spec.md`.
- **Preservation**: Existing `openspec/specs/tui/spec.md`, `openspec/specs/bigmem/spec.md`, etc. unchanged as required (proposal Out of Scope `internal/*` logic, `hashline`, `tui`, etc.). Only new domain added.
- **Rules**: No `rules.archive` in `openspec/config.yaml` to apply; `strict_tdd: false`, `testing.runner: go test`, phases all required — archive not destructive.

## Implementation Summary

Single PR (`interactive`, `auto-chain`, `800 lines`, `300-400` estimate `Low` risk, `strict_tdd off`), 9/9 design file changes present (extra `tools/nosourcegrep/main.go` benign duplication of `cmd/nosourcegrep/main.go`):

- **Docs**: `docs/testing-guidance.md` 105 lines (8.1K) — Good/Bad table (4 Good criteria: failure mode, transformation/branch, external contract, regression; 4 Bad: static echo, passthrough, wording, duplicate), bans (`mock.module` `oven-sh/bun#12823` rationale + allowed fakes, source-grep banned patterns `os.ReadFile` on `*.go`/`*.md` + `strings/bytes.Contains` + `expect(src).toContain`/`ToContain`), `bench:guard` anchor, 3+ pinned examples (`TestBlob_ConcurrentSameBytes` `-race` 200KiB `sync.WaitGroup` `t.TempDir` `isolatedHome`, `TestBranch_Traversal` via `modernc.org/sqlite` `parent_id`/`leaf_id`, Bad `TestExpectSrcContains` with fix to contract), `Good overrides Bad` rule, CI data flow diagram, references (`pgregory.net/rapid`, `modernc.org/sqlite`, `ledger_regression_test.go`), linter/Bun leak links.

- **Analyzer**: `tools/nosourcegrep/analyzer.go` (7.9K) `var Analyzer = &analysis.Analyzer{Name: "nosourcegrep", Doc: "bans source-grep in *_test.go", Run: run, Requires: [inspect.Analyzer]}` — `run` tracks per-file `hasSourceRead` map, first pass detects `isOsReadFile`+`hasSourceLiteral` (`*.go`/`*.md` slash-path), second `insp.Preorder` on `CallExpr`/`BasicLit`/`SelectorExpr` reports `os.ReadFile on source file *.go/*.md banned`, `strings.Contains/bytes.Contains` on source text banned, `expect(src).toContain` banned, `mock.module` literal/selector/call banned (`oven-sh/bun#12823`), scoped to `*_test.go` via `isTestFile` + `isAnalyzerTestdata` + `isRealTestdata` allowlist, `isExemptFile` for `internal/extension/shim_test.go`; `tools/nosourcegrep/analyzer_test.go` (293B) `analysistest.Run(t, Analyzer, "bad", "good")` with `testdata/src/bad/bad.go` (3 expected diagnostics) and `testdata/src/good/good.go` (DB `sql.Open`+`QueryRow`, passes); `tools/nosourcegrep/cmd/nosourcegrep/main.go` + `tools/nosourcegrep/main.go` (204B) `singlechecker.Main(Analyzer)` vet entries (redundant, harmless).

- **Config**: `.golangci.yml` 992B version 2 + v1 compat `linters-settings.custom.nosourcegrep {path: ./tools/nosourcegrep}` + `linters.enable: [nosourcegrep]` + `issues.exclude-rules` for `testdata/.*` and `internal/extension/shim_test.go`; `tools.go` adds `_ "github.com/biggs-100/biggz-ai/tools/nosourcegrep"` + `cmd/nosourcegrep` + `analysis` pins, `go mod tidy` clean (`go list -m all` clean, `go vet ./...` unchanged, no prod import in `internal/*`).

- **CI**: `.github/workflows/ci.yml` `lint-no-source-grep` job (lines 220-273) `needs: format` `runs-on: ubuntu-latest` steps: `Build nosourcegrep vet tool` (`go build -o /tmp/nosourcegrep ./tools/nosourcegrep/cmd/nosourcegrep`), `Run nosourcegrep vet (primary)` (`go vet -vettool=/tmp/nosourcegrep ./...` fails on `::error`, then optional `golangci-lint run` with `::warning` fallback), `Fallback rg guard` (`rg -n "os\\.ReadFile.*Contains|expect\\(src\\)" --glob '*_test.go'` excluding `testdata`/`shim_test.go`, `::error` on hit), `Ban mock.module` (`rg -n "mock\\.module" --glob '*_test.go'`, `::error`); `rapid` job (lines 276-293) `needs: format` `go test -run TestRapid ./... -count=1 -timeout 180s` without `-short` (10 funcs in `internal/review` per spec, actually 11 at verify: 6 compact_state +5 lifecycle), existing `format` (`gofmt -l`)/`test`/`complexity` gates unchanged, YAML valid, `rg` parity.

- **Anchors**: `internal/bigmem/blobstore_test.go:112` `// Good: failure mode + external contract — races PutBlob with -race vs TestExpectSrcContains (Bad)` traceable via `rg -n "Good:"` + reference to `docs/testing-guidance.md`; `internal/bigmem/branch_test.go:160` `// Good: transformation/branch + external contract via modernc.org/sqlite — TestBranch_Traversal anchor`; both comment-only (`git diff --stat` shows comment-only for these two + `tools.go`/`ci.yml`/`golangci` only, zero `internal/*` logic except formatting via `gofmt`).

- **Commits/PR**: Single PR within `800` budget per `tasks.md` Review Workload Forecast (`Estimated 300-400`, `400 Low`, `800 Low`, `Chained PRs No`, `Decision needed before apply: No`). Rollback: `git revert` one commit — delete `docs/testing-guidance.md`, `tools/nosourcegrep/`, `.golangci.yml`, revert `tools.go`/`go.mod`/`go.sum`/`ci.yml`, remove anchors — <5 min per proposal.

- **Design** (799w, 4 decisions): Doc location `docs/testing-guidance.md` vs `architecture.md` (chosen separate), Linter `golangci-lint` custom vs `rg` (chosen primary + fallback), Rapid `go test -run TestRapid` vs build tag (chosen explicit), Example pinning anchored `// Good:` vs inline (chosen grep-able). Data flow `*_test.go → go vet -vettool → golangci-lint → CI lint-no-source-grep (analyzer || rg) → go test -run TestRapid → go test + vet + gofmt → docs` verified end-to-end.

## Archive Contents

| Artifact | Status | Path (archived) | Notes |
|----------|--------|-----------------|-------|
| proposal.md | ✅ | `openspec/changes/archive/2026-08-27-testing-guidance/proposal.md` | 3.3K, Intent codify Good vs Bad + bans, Scope docs+linter+CI, Approach docs+analyzer+CI, Risks `testdata` allowlist + fallback, Success criteria 5 checks |
| specs/testing-guidance/spec.md | ✅ | `openspec/changes/archive/2026-08-27-testing-guidance/specs/testing-guidance/spec.md` | 85 lines 3.9K delta (source for main sync), 6 req 9 scen |
| design.md | ✅ | `openspec/changes/archive/2026-08-27-testing-guidance/design.md` | 5.9K 799w, 4 decisions + data flow + 9 file changes + Testing Strategy + Threat Matrix (test-flake) |
| tasks.md | ✅ | `openspec/changes/archive/2026-08-27-testing-guidance/tasks.md` | 12/12 `[x]` (Phase1 1.1-1.2 2/2, Phase2 2.1-2.4 4/4, Phase3 3.1-3.3 3/3, Phase4 4.1-4.3 3/3), 0 `[ ]` at archive |
| verify-report.md | ✅ | `openspec/changes/archive/2026-08-27-testing-guidance/verify-report.md` | 14K, `verdict: pass`, 6/6 9/9, `evidence_revision sha256:8286d0e1...`, `build_output_hash e3b0c442...`, spec matrix 9/9 COMPLIANT, Issues WARNING only |
| archive-report.md | ✅ | `openspec/changes/archive/2026-08-27-testing-guidance/archive-report.md` | This file — sync + move + final-state reconciliation |
| Main spec (source of truth) | ✅ | `openspec/specs/testing-guidance/spec.md` | Post-archive source of truth outside archive (85 lines, 6 req 9 scen, diff identical) |

Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS). Active changes directory no longer contains `testing-guidance` (verified `ls openspec/changes/` → only `archive/`). Archive preserves exact delta spec for audit trail; main spec is authority for consumers.

## Task Completion Gate

- **Persisted tasks artifact**: `openspec/changes/archive/2026-08-27-testing-guidance/tasks.md` (also pre-move `openspec/changes/testing-guidance/tasks.md`)
- **Check**: `grep -c "^- \[x\]"` → 12, `grep -c "^- \[ \]"` → 0 (via `rg -n "^- \[ \]"` → no matches). All 12 `[x]` across Phase1 Foundation (1.1 doc +1.2 stub), Phase2 Core (2.1 Analyzer.Run +2.2 main.go +2.3 `.golangci.yml` +2.4 `tools.go`), Phase3 Integration (3.1 `ci.yml` lint+rapid +3.2 blobstore anchor +3.3 branch anchor), Phase4 Verification (4.1 unit analysistest +4.2 integration parity +4.3 E2E gates). No stale checkboxes for completed work.
- **Gate**: PASS — `sdd-apply` marked completed tasks in persisted artifact (`[x]` with `Done:` evidence per task); `sdd-archive` validated no stale unchecked tasks before sync/move, so cycle may close. No exceptional stale-checkbox reconciliation needed (all `[x]` already, `taskProgress.allComplete:true`, `applyState: all_done`).
- **Dependencies before move**: `biggz sdd-status --json --instructions` (filtered) → `nextRecommended: archive`, `dependencies.archive: ready`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done}`, `artifacts {proposal:done, specs:done, design:done, tasks:done, verifyReport:done}`, `artifactStore: openspec`.

## Verification Evidence (Final State per Authority Hierarchy)

| Evidence | Value | Authority |
|----------|-------|-----------|
| Build — `go vet ./...` | exit 0 `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty) | `verify-report` `build_output_hash` + `sdd-verify-validate` admission |
| Build — `go vet -vettool=/tmp/nosourcegrep ./...` | clean on valid code; flags 3 diagnostics on `testdata/src/bad/bad.go` (`os.ReadFile`, `strings.Contains`, `expect(src).toContain`); negative `mock.module` temp file flagged | `verify-report` Build & Tests Execution logs + `tools/nosourcegrep` |
| Build — `gofmt -l .` | empty (clean) — `go.mod`+`internal/*` whitespace formatting via `gofmt` is zero logic change | `verify-report` + `git diff --stat` at close (97 tracked insertions) |
| Build — `go build nosourcegrep` | `go build -o /tmp/nosourcegrep ./tools/nosourcegrep/cmd/nosourcegrep` 0 | `verify-report` |
| Tests — unit `TestAnalyzer` | `go test ./tools/nosourcegrep -count=1 -v` → `TestAnalyzer` PASS 1.72s (bad flagged, good passes) `hash:sha256:f3c328...` | `verify-report` test logs |
| Tests — `go test -run TestRapid` | `go test -run TestRapid ./... -count=1 -timeout 180s` 11 funcs PASS (compact_state 6 + lifecycle 5, 3.65s) `hash:sha256:916b367...` | `verify-report` |
| Tests — `go test ./...` | `go test ./... -count=1 -timeout 180s` PASS (65+ packages, 0 failures, ~143s review) `evidence_revision sha256:8286d0e1...` `test_output_hash same` | `verify-report` + `sdd-verify-validate` |
| `golangci-lint` | `plugin: not implemented` on runner — CI correctly degrades to vet primary + warning (design anticipates, not blocking) | `verify-report` W + `.github/workflows/ci.yml:251-252` |
| `mock.module` | `rg -n "mock\\.module" --glob '*_test.go'` empty — ban enforced | `verify-report` + CI job |
| `bench:guard` | `docs/testing-guidance.md:77-84` pins `go test ./bench -count=1 never proves driven execution`; CI only gates `TestRapid`+`lint-no-source-grep` | `docs/testing-guidance.md` + `verify-report` Spec Compliance 6/6 |
| Anchors | `rg -n "Good:" internal/bigmem/blobstore_test.go` → line 112, `branch_test.go` → line 160 | `verify-report` + archive check |
| Verify admission | `biggz sdd-verify-validate --input verify-report.md --requirements 6 --scenarios 9` → `Verify report is valid.` | CLI validation at archive (re-run) |
| Spec counts | 6 req / 9 scen in `openspec/specs/testing-guidance/spec.md` (`### Requirement` ×6, `#### Scenario` ×9) match verify totals | Delta spec → main spec copy, `biggz sdd-verify-validate` |
| Remediation | Not required — verify already PASS, no failed evidence revision, `remediationState {required:false, complete:false}` | `sdd-status --json` |
| Review gate | N/A — no `reviewGate` for `openspec` SDD; `biggz rdd status` `enabled` but SDD path has no review authority (divergence), `nextRecommended archive` governs | `sdd-status-contract.md` divergences + archived precedents |
| Action context | `mode: repo-local`, `workspaceRoot: C:\Users\USER\Desktop\biggz-ai`, `allowedEditRoots: [C:\Users\USER\Desktop\biggz-ai]` — all edits inside roots, no `workspace-planning` guard trip | `sdd-status --json` `actionContext` |
| ArtifactStore | `openspec` preserved — `planningHome.mode: repo-local, path: C:\Users\USER\Desktop\biggz-ai\openspec`, `changeRoot` moved to archive prefix 2026-08-27 | `sdd-status --json` + filesystem |

No unrankable contradiction at close. All WARNINGs reconciled above as intentional design/fallback; no CRITICAL open. Final numbers carried from highest-ranked source (`sdd-status` + `verify-report` PASS + persisted `tasks.md`), not from stale snapshots.

## Risks / Residual

- Low: `golangci-lint` custom plugin remains `plugin: not implemented` on some runners — CI already handles via vet primary + `::warning`, parity `vet ≈ rg` verified. Mitigation: pin `golangci-lint` via `tool` directive in `tools.go` or `go run` latest in follow-up (suggestion in verify-report, not blocking).

- Low/info: `internal/extension/shim_test.go` still exempt from `nosourcegrep` (Out of Scope per proposal). CI `rg` and analyzer both exclude it currently. Future migration to DB-contract assertions (as with `TestBranch_Traversal`) would remove exempt, but not required for this slice.

- Low/info: `wg.Go` modernization not applied (`sync.WaitGroup` + `go func` remains). Correct and `-race`-safe; optional Go 1.25 improvement, not a gap.

- Low: ledger `corrupt_authority complete:true` persists post-archive for runtime-bearing continuations only; future `biggz sdd-attempt acquire` for a new change would need no action (different change), but re-running `sdd-attempt` for this archived change would require explicit `reset` per maintainer decision (per `sdd-status-contract.md` `Reset is exceptional... never automatic`). No impact on archived `verify done` / `archive ready` state.

- Info: `go.mod`+`internal/*` `gofmt` whitespace diffs (`adapter.go`, `api_test.go`, `fake_test.go`, `interceptor.go`) are zero logic change, required to satisfy `format` gate. Net ~300-350 includes them; all within 800 budget.

## References

- Rapid FSM: `pgregory.net/rapid` — `internal/review/compact_state_rapid_test.go`, `internal/review/lifecycle_rapid_test.go` — 11 `TestRapid_*` PASS
- SQLite contract: `modernc.org/sqlite` — `internal/bigmem/branch_test.go`, `internal/bigmem/blobstore_test.go` — `TestBranch_Traversal` / `TestBlob_ConcurrentSameBytes`
- Regression pin: `internal/review/ledger_regression_test.go`, `internal/review/contract_test.go` — referenced in docs `testing-guidance.md`
- Linter: `tools/nosourcegrep/analyzer.go` (`*ast.CallExpr`, `*_test.go` scope, `testdata` allowlist), `.golangci.yml` custom `nosourcegrep`, `tools.go` pins, `tools/nosourcegrep/cmd/nosourcegrep/main.go`
- Bun leak: `oven-sh/bun#12823` — cited in docs, analyzer comment, CI job name
- Verify evidence: `openspec/changes/archive/2026-08-27-testing-guidance/verify-report.md` `evidence_revision sha256:8286d0e1...` `verdict: pass` `6/6` `9/9`
- SDD status pre-archive: `biggz sdd-status --json --instructions` (`active` `testing-guidance` `artifactStore: openspec`, `nextRecommended: archive`, `dependencies.archive: ready`, `taskProgress allComplete:true`, no `reviewGate` per divergences)

---

**SDD Cycle Complete** — change `testing-guidance` has been fully planned, implemented, verified, and archived. Source of truth is `openspec/specs/testing-guidance/spec.md`. Artifact preserved at `openspec/changes/archive/2026-08-27-testing-guidance/` (audit trail). Ready for next change.

**Skill Resolution**: `paths-injected` — `sdd-archive` + `sdd-phase-common` + `openspec-convention` (injected via orchestrator `## Skills to load before work` equivalent; fallback `sdd-archive` SKILL.md read at `internal/assets/skills/sdd-archive/SKILL.md` and `_shared/openspec-convention.md`, `_shared/sdd-phase-common.md`, `_shared/sdd-status-contract.md` verified)

## Key Learnings

1. Contract-first testing filter needs AST linter plus rg fallback because golangci custom analyzer plugin is not implemented on some runners and vet primary must carry the gate.
2. testdata allowlist and shim exempt must mirror in both analyzer and ci rg fallback, otherwise parity breaks and false positives block CI.
3. Single file spec sync as new domain is direct copy, not delta merge, and sdd-verify-validate admission must match 6 req 9 scen exactly.
4. Archive with no reviewGate is valid for biggz-ai openspec SDD path per sdd-status-contract divergences and nextRecommended archive governs.
5. Ledger corrupt_authority complete does not block file-backed archive when sdd-status shows verify done and archive ready for openspec store.
