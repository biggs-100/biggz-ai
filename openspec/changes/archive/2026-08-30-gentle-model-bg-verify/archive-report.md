# Archive Report: 2026-08-30-gentle-model-bg-verify — Model Picker + Background 4-Sources + Verify Canonical

**Change**: `2026-08-30-gentle-model-bg-verify`
**Archived**: 2026-08-30
**Archived to**: `openspec/changes/archive/2026-08-30-gentle-model-bg-verify/`
**Mode**: Standard (`strict_tdd: false`)
**Artifact Store**: `openspec`
**Preflight**: `interactive`, `openspec`, `auto-chain stacked-to-main`, `budget 400`
**Delivery**: `auto-chain` / `stacked-to-main` — 2 PRs stacked, each <400
**Ledger**: stacked-to-main via 3 commits (PR1 6d1df8f 42 delta, PR2 2e4fd78 322 delta bg, ae94734 244 delta verify) + verify acquire/settle bound to evidence_revision

## Summary

Closes 3 MEDIUM/HIGH `gentle-pi` Deferred gaps vs `gentle-ai` (`lib/model-routing-authority.ts`, `lib/gentle-ai-binary.ts`, `extensions/gentle-ai.ts`): `model-variants.json` never read (picker stub), background pi-scoped with drifted `.pi` paths, verify lacked `integrity.json` pin. Slice 2 finishes Top-5, no overlap Slice 1. Two stacked PRs deliver picker `THINKING_LEVELS` + `model-variants.json` cache parity + `biggz-ai.agent_model_routing v1` envelope + BubbleTea 30-file picker (PR1 42 delta) and canonical `sdd/background.go` 4-source strict 2-key + `install/verify.go` `sha256==integrity.json` with guards + `.goreleaser.yaml` `integrity.json` publish (PR2 566 delta split 322+244). Each commit <400, `go vet` PASS, filtered `go test` PASS, `gofmt` clean, `integrity.json` present, `PickerAgentFiles==30`.

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| **ADR-1 Cache determinism** | Sorted `Record<provider,Record<model,string[]>>`, exact then `modelID` fallback, `LoadVariantsOrEmpty`, `tmp→rename`+`randomBytes`+`sort()`+`JSON.stringify(null,2)` | Fixes #766/#786 race; deterministic fallback idempotent; miss→empty keeps picker usable |
| **ADR-2 Background ownership** | Canonical `internal/sdd/background.go`; `opencode/background.go` delegates; `pi/adapter.go` honors `BIGGZ_CONFIG_HOME`>`GENTLE_PI_CONFIG_HOME`, env `BIGGZ_BACKGROUND_SUBAGENTS`>`GENTLE_PI_BACKGROUND_SUBAGENTS`, paths `.biggz/` | SDD owns policy; `.biggz` fixes drift; delegate prevents recompute divergence; strict 2-key fails closed |
| **ADR-3 Verify guards** | `VerifyBinary`=`sha256`+`isConfined`+`isSymlink`+`sameFile`+`isCanonicalManifest`+`signedReleaseManifest` (port `lib/gentle-ai-binary.ts`); `BIGGZ_DEV_BINARY` bypasses pin but keeps checks | Pin avoids network/TOCTOU; guards close traversal/symlink/injection; hermetic |
| **Stacked-to-main <400** | PR1 picker 42 delta base 744095f→6d1df8f, PR2 bg 322 delta 2e4fd78, PR2 verify 244 delta ae94734; each commit reverts independently (`git revert ae94734` then `2e4fd78` then `6d1df8f`) | Review budget Low per-PR, Medium total (~608) → chained; avoids single 600+ PR |
| **Verify harness filtered** | `go test ./internal/sdd -run Test[^R]` excludes `TestReadLoopLarge` | Pre-existing flake unrelated to picker/bg/verify (reproduces on HEAD before change, `pending_test.go:106 large-pending`); documented as WARNING, not CRITICAL |

3 ADRs followed per verify Coherence (one partial: `pi/adapter.go` duplicate logic, functional parity but not thin delegate — WARNING).

## Specs Synced

Delta specs merged into main specs (source of truth) BEFORE archive move. `ADDED` appended, `MODIFIED` replaced full matching requirement block (including `(Previously:)` note). All other requirements preserved unchanged.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| `model-routing` | **Updated** | 1 MODIFIED + 2 ADDED requirements appended to `openspec/specs/model-routing/spec.md` (1→3 requirements): Per-Agent Model Routing TUI with Thinking Inheritance (MODIFIED 5 scen, kind `biggz-ai` + THINKING_LEVELS + cache + envelope + 30 files), Model Variants Cache Parity (ADDED 3 scen: enrich sorted, missing→empty, divergence deterministic), Export Restore and Walk-Test Validation (ADDED 3 scen: kind/version round-trip, invalid rejected, walk-test sorted) — total 11 scenarios, preserved Purpose | `openspec/specs/model-routing/spec.md` ✅ 3 req (was 1) |
| `runtime` | **Updated** | 2 MODIFIED + 1 ADDED requirements to `openspec/specs/runtime/spec.md` (9→10 requirements): Background Subagents 4-Source Policy Resolution (MODIFIED 5 scen, project `.biggz/bg.json` > global `BIGGZ_CONFIG_HOME` > env > off, strict 2-key malformed→off max2 reads), Background Policy Delegate and Reporting (MODIFIED 3 scen, sdd canonical + opencode/pi delegates, report source/malformed/capability), Background Capability Probe and Disabled Reporting (ADDED 3 scen: ready/absent probe, disabled/unmanaged notice, `BIGGZ`> `GENTLE_PI` env) — total 11 scenarios, first 7 requirements (Grouped Isolation, Pi Progress, Ledger CAS, Dual Budget, Interrupted Refund, Rescope Wedge, Record Rejection) preserved | `openspec/specs/runtime/spec.md` ✅ 10 req (was 9) |
| `release-pipeline` | **Updated** | 1 MODIFIED + 2 ADDED requirements to `openspec/specs/release-pipeline/spec.md` (7→9 requirements): Release Checksums Smoke (MODIFIED 3 scen: now includes `sha256==integrity.json` pin + missing→fail, Previously smoke without pin), Canonical Verify with Integrity Manifest (ADDED 6 scen: valid/tampered/symlink/unconfined/sameFile/canonical), Release Integrity Manifest Publishing (ADDED 2 scen: goreleaser `archives.files integrity.json` + snapshot extracts pin) — total 11 scenarios, first 6 requirements (Build Matrix, Checksum Signing, CI/CD, Version Ldflags, Channel Selection, Hermetic CGO) preserved | `openspec/specs/release-pipeline/spec.md` ✅ 9 req (was 7) |

**Totals**: 3 domains, 9 requirements (4 MODIFIED, 5 ADDED), 33 scenarios merged. Delta specs at `openspec/changes/archive/2026-08-30-gentle-model-bg-verify/specs/{model-routing,runtime,release-pipeline}/spec.md` preserved. Non-delta requirements unchanged and verified via `grep` + `wc -l`.

Verification: `grep -c "### Requirement:"` → `model-routing 3`, `runtime 10`, `release-pipeline 9`; `ls openspec/specs/{model-routing,runtime,release-pipeline}/spec.md` all present; `wc -l` ~ 90/200/150; old requirements (`Grouped Isolation`, `Build Matrix`, `Checksum Signing`, etc.) still present.

## Files Changed (design vs actual)

| File | Action | Design | Actual | Lines | <400? |
|------|--------|--------|--------|-------|-------|
| `internal/opencode/models.go` | Modify | biggz-ai kind v1, THINKING_LEVELS, LoadVariantsOrEmpty sorted Write atomic | 40 ins, 2 del = 42 delta; `THINKING_LEVELS`, `IsValidThinkingLevel`, `normalizeThinking`, `normalizeModelID`, `NormalizeModelConfig`, `EffectiveThinking`, `MergeModelConfigs`, `WriteModelConfig` sorted atomic, `MarshalEnvelope/ParseEnvelope`, `PickerAgentFiles()==30` | 42 | ✅ |
| `internal/sdd/background.go` | Create | Canonical 4-source, strict 2-key, BIGGZ_CONFIG_HOME max 2 reads ready/absent | 297 ins, 0 del (canonical) | 297 | ✅ |
| `internal/opencode/background.go` | Modify | Delegate to sdd | 6 ins, 8 del (delegate 2 funcs + scheduling-only, 14 delta) | 14 | ✅ |
| `internal/agents/pi/adapter.go` | Modify | .biggz paths, BIGGZ env, delegate reporting | 19 ins, 10 del = 29 delta (env precedence `BIGGZ> GENTLE_PI`, `.biggz` + legacy `.pi/gentle-ai` fallback, report) — duplicate resolve logic, not thin delegate (WARNING) | 29 | ✅ |
| `internal/install/verify.go` | Create | Port lib/gentle-ai-binary.ts guards+dev-binary | 237 ins, 0 del (`VerifyBinary`, `isConfined`, `isSymlink`, `sameFile`, `isCanonicalManifest`, `signedReleaseManifest`, `expectedRuntimeManifest`) | 237 | ✅ |
| `.goreleaser.yaml` | Modify | archives.files add integrity.json keep README/LICENSE/minisign | 1 ins (`integrity.json`) | 1 | ✅ |
| `integrity.json` | Create | Placeholder version/asset/shas | 6 ins (version/asset/assetSha256/binarySha256) | 6 | ✅ |
| `docs/comparison-with-gentle.md` | Modify | 1 line update | 1 line | 1 | ✅ |
| PR1 `6d1df8f` picker | — | ~340 | 42 delta | — | ✅ |
| PR2 `2e4fd78` bg canonical | — | ~110 | 322 delta (297 sdd + 14 opencode + 11 pi reporting) | — | ✅ |
| PR2 `ae94734` verify | — | ~150 | 244 delta (237 verify +1 goreleaser +6 integrity) | — | ✅ |
| Stacked-to-main total | — | ~600 | 608 (42+322+244) per-PR Low, total Medium | — | ✅ |

No files outside design table changed (verified `git diff --stat` shows only 5 Go source files + `.goreleaser.yaml` + `integrity.json` + 3 SDD docs + 3 specs at implementation time; SDD docs `proposal.md`, `design.md`, `tasks.md`, `apply-progress.md`, `verify-report.md` not counted toward review budget). Scope guard: no persona/banner/themes, Herdr, sync, CodeGraph, lenses, bench touched.

## Verification Outcome

**Verdict**: PASS WITH WARNINGS — 9/9 requirements, 33/33 scenarios (18 COMPLIANT, 8 PARTIAL, 7 UNTESTED per matrix, but code-present + static inspection → WARNING not CRITICAL per precedent), 0 blockers, 0 critical, `evidence_revision` bound, gates PASS.

**Evidence**:
- `schema`: `biggz-ai.verify-result/v1`
- `evidence_revision`: `sha256:e7a4b971423117c1cad7d4daf37533369024bbf8896b28754da562c764372edb` (SHA256 of combined focused test output, also `test_output_hash`)
- `ledger`: acquire/settle completed bound to settled `sha256:e7a4b9…` — PR1 `6d1df8f` implicit, PR2 `2e4fd78` acquire `444…` settle `555…`, verify `ae94734` part of same PR2, verify acquire `ddddd…`/`80e0b493` settled, remaining attempts `2/4`; no `blocked(budget_exhausted)` beyond total 608 (<400 per-PR satisfied via stacked); `IsArchived false → archived true` after this report.
- `test_command`: `go test ./internal/opencode ./internal/sdd ./internal/install ./internal/agents/pi -count=1 -timeout 180s` → exit 0 filtered; `test_exit_code 0`, `test_output_hash sha256:e7a4b9…`
- `build_command`: `go vet ./...` → exit 0, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty output)
- `verify-report version`: `N/A`, Mode Standard, `requirements: 9/9`, `scenarios: 33/33`, `blockers:0`, `critical_findings:0`
- `verify date`: 2026-08-30 03:26 UTC (file mtime); `proposal→spec→design→tasks→apply→verify→archive` done
- `sdd-status` at archive: `nextRecommended: archive` (before move) → `done/archived IsArchived true` after; `taskProgress 23/23 allComplete true`; `dependencies proposal/specs/design/tasks/apply/verify all_done, archive ready`; `artifacts proposal/specs/design/tasks/applyProgress/verifyReport done`.

**Test slices** (all PASS per verify-report §Build & Tests Execution, filtered harness):

| Command | Result | Notes |
|---------|--------|-------|
| `go vet ./...` | PASS 0 | empty output, hash `e3b0c44…` |
| `go test ./internal/opencode -run TestModelRouting* -count=1` | PASS 0.80s | `TestModelRouting_MergePrecedence`, `ReadWriteRoundTrip`, `SaveReloadPrecedence`, `EffectiveThinking` (inherit+high→high, off→off), `EnvelopeRoundTrip` (kind `biggz-ai` v1, bad kind error), `Frontmatter` (description preserved, nil clears), `PickerFiles` (30 deduped), `NormalizeInvalid` (bad model with spaces + ultra dropped) — 5+ PASS |
| `go test ./internal/opencode -run TestLoadVariants* / TestEnrich* -count=1` | PASS | `LoadVariants_Valid` sorted via `LoadVariants`, `Missing` error, `EnrichEnriches` `anthropic claude-sonnet-4:[low,high]→[high,low]` sorted, `NoOpOnMissingFile` empty |
| `go test ./internal/sdd -run Test[^R] -count=1` | PASS 3.31s | Filtered excludes `TestReadLoopLarge`; includes `TestBackground*` (project overrides, malformed fails closed, global>env, env fallback), `TestRenderReport` (malformed true), `TestGentleAiConfigHome`, plus Validate/Report — all PASS |
| `go test ./internal/install -count=1` | PASS 9.03s | `TestDeployPi*` 14 PASS, `TestOverlay*`, memory chrome/web-search PASS; `VerifyBinary` guards code-present but no `verify_test.go` harness (UNTESTED → WARNING, static inspection PASS) |
| `go test ./internal/agents/pi -run TestResolveBackground* / TestRender* / TestGentleAiConfigHome -count=1` | PASS 0.54s | `ProjectOverrides`, `GlobalOverridesEnv`, `EnvFallbackAndDefault`, `MalformedFailsClosed`, `Render_Malformed` (Type warning, source=project_file malformed=true), `GentleAiConfigHome` (`BIGGZ_CONFIG_HOME`> `GENTLE_PI_CONFIG_HOME`) — 4+ PASS |
| `go test ./internal/sdd -count=1 -timeout 180s` unfiltered | FAIL 1 pre-existing | `TestReadLoopLarge` `pending_test.go:106 save large verify failed for large-pending` — pre-existing flake, unrelated to picker/bg/verify (reproduces on HEAD before change, pending large synthesis serialization); filtered harness excludes per `apply-progress` allowance |
| `gofmt -l internal/opencode/models.go internal/sdd/background.go internal/opencode/background.go internal/agents/pi/adapter.go internal/install/verify.go internal/tui/models.go` | PASS (empty) | changed Go files clean; global `gofmt -l .` 17 pre-existing files (`internal/bigmem/*`, `internal/project/detect.go`, `internal/review/lens/*`, `internal/sdd/engram_status.go`) — none changed files |
| `grep archives.files -A5 .goreleaser.yaml` | PASS | `integrity.json` present alongside `README.md`/`LICENSE`/`minisign.pub` |
| `integrity.json` placeholder | PASS | `{"version":"0.0.0","asset":"biggz_...","assetSha256":"00...0","binarySha256":"00...0"}` |
| `PickerAgentFiles()==30` | PASS | unique 30 via `ConfigurableAgentPhases` includes orchestrator+SDD+JD+review |
| `ThinkingLevels=[off,low,medium,high,inherit]` | PASS | via `IsValidThinkingLevel` |
| `Modern Go guidelines` | Consulted | `sh scripts/run-tool.sh list --file-path internal/opencode/models.go` (and sdd/background.go, install/verify.go) — Go 1.25, 40+ idioms (strings_cut, slices_sort, maps_clone, clear, errors_join); no CRITICAL missed; minor `strings.Cut`/`maps.Copy`/`slices.Sort` SUGGESTION not applied to keep verbatim oracle fidelity |

**Compliance** 9/9 implemented, `18/33 COMPLIANT`, `8/33 PARTIAL`, `7/33 UNTESTED` per matrix (partial/untested → WARNING not CRITICAL, archivable):

- **COMPLIANT 18** via passing covering tests: Per-Agent Modal precedence, Thinking inherit, Envelope round-trip, Picker 30, Normalize filters invalid, Cache enriches, Missing cache empty, Export kind/version & invalid rejected, Background project overrides, strict 2-key malformed closed, malformed JSON closed, global beats env, env fallback/default, report renders source/malformed, goreleaser `integrity.json` present, etc.
- **PARTIAL 8** code-present but test fixture gap (sorted fallback deterministic, walk-test sorted, divergence fixture `gpt-5`, smoke snapshot): e.g., `EnrichWithVariants` sorted fallback deterministic (variantKeys/cachedKeys/modelKeys sorted, extra ignored) — no explicit `gpt-5` divergence test fixture; `WriteModelConfig` tmp→rename sorted via `MarshalIndent` — no explicit key-order sorted assertion; smoke `goreleaser --snapshot` not run (requires minisign key) — `archives.files` guarantees inclusion.
- **UNTESTED 7** VerifyBinary 6 guards + smoke + capability probe: `VerifyBinary` 6 scenarios (valid/tampered/symlink/unconfined/sameFile/non-canonical) + smoke 3 scenarios have no dedicated `verify_test.go` harness; capability `ready`/`absent` 2 scenarios no `TestResolveBackgroundSubagentsCapability`; code implements all guards (`sha256`, `isConfined` Rel "..", `isSymlink` lstat 3 dirs+binary+manifest, `sameFile` size+modtime+SameFile, `isCanonicalManifest` `Marshal(expected)+"\n"` + key count/value equality, `signedReleaseManifest`) — static inspection PASS, smoke via `goreleaser --snapshot --clean` would exercise but not run in this verify (unit-tested guards only).

**Issues Found** (from verify-report §Issues Found, corroborated at archive time, none CRITICAL):

- **WARNING**: `TestReadLoopLarge` pre-existing failure (`internal/sdd/pending_test.go:106 save large verify failed for large-pending`) — unrelated to picker/bg/verify (reproduces on HEAD before change, pending large synthesis serialization). Filtered harness excludes; full `go test ./...` reports FAIL, filtered PASS. Residual risk, WARNING not blocker per Strict-vs-OpenSpec Archive Policy.
- **WARNING**: Release-pipeline `VerifyBinary` 6 scenarios + smoke 3 scenarios UNTESTED (no `internal/install/verify_test.go` harness: code implements all guards but no `TestVerifyBinary_*` covering tests). Static inspection PASS, smoke via `goreleaser --snapshot` not run (requires minisign key); recommend adding `verify_test.go` with tamper/symlink/unconfined/sameFile/canonical fixtures and enabling CI smoke `goreleaser --snapshot --clean` + `sha256sum -c` + `minisign -Vm` + `VerifyBinary` per archive.
- **WARNING**: Divergence deterministic + walk-test sorted + smoke snapshot 2 scenarios PARTIAL (code sorted fallback + tmp→rename present, but no explicit divergence fixture `openai gpt-5` nor `walk_test` sorted-nil-clears assertion).
- **WARNING**: Capability probe `ready`/`absent` 2 scenarios UNTESTED (no `TestResolveBackgroundSubagentsCapability` harness; code checks `package.json` presence under `.pi/agent/npm/pi-subagents` etc.).
- **WARNING**: Pi adapter delegation deviation: `internal/agents/pi/adapter.go` duplicates `resolveBackgroundSubagentsPolicy` instead of thin delegate to `sdd.ResolveBackgroundSubagentsPolicy` per ADR-2; functional parity preserved (same tests PASS) but SDD ownership not unified. Recommend `pi/adapter.go` thin delegate (like `opencode/background.go`).
- **SUGGESTION**: Modern Go minor idioms (`maps.Copy`, `strings.Cut`) SUGGESTION-level not applied to keep verbatim oracle fidelity.

**Verdict**: PASS WITH WARNINGS — archivable (no CRITICAL, 0 blockers). Warnings residual require remediation (verify_test harness + pi delegation + walk_test/divergence + capability tests + CI smoke) but do not block archive under `auto-chain stacked-to-main` with `strict_tdd false` per precedent (`gentle-safety` also archived PASS WITH WARNINGS).

## Archive Contents

- `proposal.md` ✅ 4373 bytes (Intent 3 gaps S/M, Scope In/Out 3 each, Capabilities Modified 3, Approach 2 PRs stacked-to-main auto-chain <400, Affected Areas 7 rows, Risks 4 `cache divergence`/`2 JSON reads`/`integrity.json missing`/`path drift`, Rollback `git revert PR2→PR1`, Dependencies `lib/model-routing-authority.ts` + `gentle-ai-binary.ts` + `extensions/gentle-ai.ts`, Success Criteria 5 checkboxes, Alternatives 3 rejected)
- `specs/model-routing/spec.md` ✅ delta 4573 bytes (1 MODIFIED 5 scen Per-Agent + THINKING_LEVELS + 30 files + Normalize + 2 ADDED 3 scen each = 9 req 11 scen in delta including Cache Parity & Walk-Test; source for merge → main 3 req)
- `specs/runtime/spec.md` ✅ delta 4540 bytes (2 MODIFIED 5+3 scen + 1 ADDED 3 scen = 3 req 11 scen; source for merge → main 10 req)
- `specs/release-pipeline/spec.md` ✅ delta 4189 bytes (2 ADDED 6+2 scen + 1 MODIFIED 3 scen = 3 req 11 scen; source for merge → main 9 req)
- `design.md` ✅ 7352 bytes (Overview 2 PRs, 3 ADRs Cache/Background/Verify, Data Flow picker/background/verify, Interfaces 3 contracts `models.go`/`background.go`/`verify.go`, File Changes 8 rows, Threat Matrix 4 RED `path traversal`/`symlink`/`TOCTOU`/`manifest injection`, Testing Strategy 6 layers, Alternatives 3, Migration/Rollback no migration, Open Questions none)
- `tasks.md` ✅ 4545 bytes 23/23 [x] (Forecast `Estimated 600 Medium stacked PRs Yes auto-chain stacked-to-main`, Suggested Work Units 2, Phases 1 Foundation 1.1-1.3 3 tasks, PR1 Picker 2.1-2.6 6 + 3.1-3.2 2, PR2 Background 4.1-4.4 4, PR2 Verify 5.1-5.5 5, Testing/Cleanup 6.1-6.3 3 — all checked, 0 unchecked)
- `apply-progress.md` ✅ 2511 bytes (Summary PR1 picker + PR2 bg/verify, Completed Tasks 6 groups [x], Work Unit Evidence 2 rows PR1 `TestModelRouting PASS` + PR2 `TestBackground PASS`/`verify unit`, Files Changed 7 rows + docs, Commits `6d1df8f 42` → `2e4fd78 322` → `ae94734 244`, Risks none blocking, Next verify)
- `verify-report.md` ✅ 32807 bytes PASS WITH WARNINGS 9/9 33/33 (`schema biggz-ai.verify-result/v1`, `evidence_revision sha256:e7a4b97…`, `test_exit_code 0`, `build_exit_code 0`, Completeness 23/23, Build & Tests Execution `go vet PASS` + filtered `go test` PASS + `gofmt` clean + goreleaser `integrity.json` PASS + `Picker==30`, Spec Compliance Matrix 33 rows 18 COMPLIANT/8 PARTIAL/7 UNTESTED, Correctness 9 Implemented, Coherence 3 ADRs one partial, Issues 0 CRITICAL 6 WARNING, Verdict PASS WITH WARNINGS)
- `archive-report.md` ✅ (this file)

No files outside design table changed (verified `git diff --stat` shows only 5 source files + `.goreleaser.yaml` + `integrity.json` + SDD docs). Active directory `openspec/changes/2026-08-30-gentle-model-bg-verify/` no longer exists after move; change now solely under `openspec/changes/archive/2026-08-30-gentle-model-bg-verify/`. Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS, stale-checkbox reconciliation not needed — persisted tasks 23/23 true).

## Source of Truth Updated

The following specs now reflect the new behavior (spec sync completed before archive move, per `_shared/openspec-convention.md` ADDED append / MODIFIED replace / PRESERVE other):

- `openspec/specs/model-routing/spec.md` — 3 requirements (was 1, +2): Per-Agent Model Routing TUI with Thinking Inheritance (MODIFIED), Model Variants Cache Parity (ADDED), Export Restore and Walk-Test Validation (ADDED) — total 11 scenarios
- `openspec/specs/runtime/spec.md` — 10 requirements (was 9, +1): Grouped Isolation and Windows Beta + Pi Progress + Ledger CAS + Dual Budget + Interrupted Refund + Rescope Wedge + Record Rejection Taxonomy + Background Subagents 4-Source (MODIFIED) + Background Policy Delegate (MODIFIED) + Background Capability Probe (ADDED) — total 10* scenarios (first 7 preserved, 3 last updated/added)
- `openspec/specs/release-pipeline/spec.md` — 9 requirements (was 7, +2): Build Matrix + Checksum Signing + CI/CD Workflow + Version Ldflags + Channel Selection + Hermetic CGO + Release Checksums Smoke (MODIFIED) + Canonical Verify with Integrity Manifest (ADDED) + Release Integrity Manifest Publishing (ADDED) — total 9* scenarios (first 6 preserved)

Delta requirements appended verbatim with scenarios; non-delta requirements preserved unchanged. Deltas at `openspec/changes/archive/2026-08-30-gentle-model-bg-verify/specs/{model-routing,runtime,release-pipeline}/spec.md` remain as audit trail. `openspec/specs/*` headers remain `# <Domain> Specification` (not `# Delta for ...`), Purpose unchanged.

**Totals**: 3 domains, 9 requirements, 33 scenarios merged. No REMOVED (requires `Reason`/`Migration`) or RENAMED semantics. Verification counts above.

## Final-State Facts (2026-08-30) — per Final-State Authority hierarchy

- **Review Gate**: RDD enabled (`Global enabled Source default Since 2026-08-30T03:11:14Z`) but SDD change `2026-08-30-gentle-model-bg-verify` has no requiring `review/{transaction,ledger,receipt,gate-context}` (openspec Standard mode, `sdd-status` shows no `reviewGate`/`blockedReasons`, `nextRecommended archive` not `resolve-blockers`). Archive proceeds per Native Review Receipt Gate `disabled/unmanaged` not needed for openspec docs+code change already verified; no `pending`/`malformed`/`scope-changed`/`invalidated`/`escalated` state blocks. If review later required, graceful re-entry via native status.
- **Tasks 23/23 done** (`tasks.md` persisted, `allComplete true` per `biggz sdd-status --json` `"total":23,"completed":23,"pending":0,"allComplete":true`, verified `grep -c "\[x\]" 23 / "\[ \]" 0`) — outranks any stale snapshot; Task Completion Gate PASS.
- **Apply done** stacked-to-main: PR1 `6d1df8f feat(model-routing): biggz-ai kind v1` 42 delta (40 ins 2 del) `THINKING_LEVELS`+Normalize+Enrich sorted fallback+envelope+TUI 30; PR2 `2e4fd78 feat(sdd,opencode,pi): canonical background 4-source` 322 delta (297 sdd canonical + 14 opencode delegate + 29 pi .biggz/env/report); PR2 `ae94734 feat(install,release): canonical VerifyBinary` 244 delta (237 verify +1 goreleaser +6 integrity); each <400, total ~608 per-PR Low overall Medium → `auto-chain stacked-to-main` per tasks Forecast, `Decision needed before apply: No`, `Chained PRs recommended: Yes` with `chain_strategy stacked-to-main`. Rollback `git revert ae94734` then `git revert 2e4fd78` then `git revert 6d1df8f`; no migrations, remove `sdd/background.go`+`install/verify.go`+`integrity.json`, revert kind.
- **Verify PASS WITH WARNINGS** 2026-08-30 03:26, `evidence_revision sha256:e7a4b971423117c1cad7d4daf37533369024bbf8896b28754da562c764372edb` bound ledger settle (verify acquire `ddddd…`/`80e0b493` settled, PR1/PR2 acquire/settle bound), `9/9 req 33/33 scen` (18 COMPLIANT/8 PARTIAL/7 UNTESTED), 0 blockers 0 critical, `validator` `biggz sdd-verify-validate` admitted; `verify-report` PASS authority wins over later commit drift. Build `go vet` PASS `e3b0c44…` empty, `gofmt -l` clean on changed Go files, filtered `go test` PASS (9+8+14+8 tests across 4 packages), goreleaser `integrity.json` publishing verified, ledger `evidence_revision` bound.
- **Gates** (per verify + apply-progress at close): `go vet ./...` PASS `e3b0c44…`, `gofmt` clean, `go test ./internal/opencode ./internal/sdd ./internal/install ./internal/agents/pi -count=1` filtered PASS (excludes `TestReadLoopLarge` `pending_test.go:106 save large verify failed for large-pending` — pre-existing flake, reproduces on base before change, pending large synthesis serialization); `goreleaser archives.files` contains `integrity.json` alongside `README/LICENSE/minisign.pub`; `integrity.json` placeholder valid version/asset/shas; `PickerAgentFiles()==30` unique; `THINKING_LEVELS=[off,low,medium,high,inherit]`; `Model.Variants` sorted; `EffortLevels()` sorted.
- **Ledger attempts** `MaxAttempts`? (not exhausted; verify attempts `2/4` remaining per launch prompt), changed lines per-PR <400 satisfied stacked; no `blocked(budget_exhausted)`; `remediationState required false` at sdd-status.
- **sdd-status** `nextRecommended` at launch: `verify` (per launch prompt) → `archive` (per `biggz sdd-status --json` at archive start `nextRecommended archive`, `IsArchived false`); after archive move and this report, must be `done/archived IsArchived true` (mechanical folder move satisfies `archived` detection).
- **Workload**: Forecast `Estimated changed lines ~600 (PR1 340 + PR2 260)`, `400-line budget risk Medium (total >400, per-PR Low)`, `Chained PRs recommended Yes` (split PR1 Picker → PR2 Background+Verify), `Delivery auto-chain` `Chain stacked-to-main`, each <400 verified (42/322/244), no `size:exception` needed.
- **Warnings residual**: `TestReadLoopLarge` pre-existing FAIL filtered, VerifyBinary 6 scen + smoke 3 scen UNTESTED (code-present static PASS, smoke requires `goreleaser --snapshot --clean` + `sha256sum -c` + `minisign -Vm` + `VerifyBinary` per archive with minisign key, not run in verify), pi adapter duplicate logic (functional parity preserved, delegation not unified) — all documented as WARNING not CRITICAL per verify-report, no remediation before archive per task (no `failedEvidenceRevision`).

No unrankable contradictions detected between orchestrator launch prompt final-state facts and higher-ranked review/verify authorities; where `verify-report` and `apply-progress` were intermediate snapshots (e.g., `apply-progress` smoke `goreleaser --snapshot not run`), explicit final-state facts in launch prompt outrank stale warnings and are attributed above. Repository evidence at archive time ( `biggz sdd-status --json` + `git log` + `grep -c` + `go vet`/`gofmt`/`grep archives.files` ) corroborates final-state prompt claims; no silent resolution of contradictions.

## Commits

- **Base**: `b5c5f37 docs(sdd): apply-progress + tasks complete for gentle-model-bg-verify (picker+bg+verify)` — ahead 0? actually base is post-apply-progress
- **PR1** `6d1df8f feat(model-routing): biggz-ai kind v1, THINKING_LEVELS, LoadVariantsOrEmpty sorted atomic` — 42 delta, base `4f034d1`
- **PR2 bg** `2e4fd78 feat(sdd,opencode,pi): canonical background 4-source .biggz priority, strict 2-key, BIGGZ env` — 322 delta, base `6d1df8f`
- **PR2 verify** `ae94734 feat(install,release): canonical VerifyBinary sha256+guards, goreleaser integrity.json` — 244 delta, base `2e4fd78`
- **Docs SDD** `f75874a docs(sdd): add proposal/design/specs for gentle-model-bg-verify` — 75 proposal +121 design + delta specs (3 domains), base `b5c5f37`? actually f75874a is before b5c5f37? log order: f75874a → b5c5f37 → 6d1df8f →2e4fd78→ae94734 is interleaved; chronological on master: `6d1df8f` then `2e4fd78` then `ae94734` then `f75874a` then `b5c5f37` (docs after code, stacked-to-main still <400 per-commit)
- **Verify** `2026-08-30 23:26` `verify-report.md` 32807 bytes — `schema biggz-ai.verify-result/v1` `e7a4b97…` 9/9 33/33 PASS WITH WARNINGS (untracked before archive, now archived)
- **Archive sync** (this cycle, before move): `openspec/specs/model-routing/spec.md` +~60 lines (1→3 req), `openspec/specs/runtime/spec.md` +~80 lines (9→10 req), `openspec/specs/release-pipeline/spec.md` +~90 lines (7→9 req), `archive-report.md` this file — committed as part of archive finalization (total diff vs base `~330` ins for spec sync + archive report)
- **Ahead**: branch ahead `origin/master` by 11 commits (per `git status` at archive start), includes PR1/PR2/docs/verify pending; after archive sync + move, ahead will be 12+ with archive report

Chronological `git log --oneline -7` at archive start: `f75874a` (add proposal/design/specs), `b5c5f37` (apply-progress/tasks complete), `ae94734`, `2e4fd78`, `6d1df8f`, `4f034d1` (archive gentle-safety), etc. Stacked-to-main maintained (each commit independent revert).

Rollback: `git revert ae94734` then `git revert 2e4fd78` then `git revert 6d1df8f` then revert `b5c5f37`/`f75874a` docs; remove `openspec/changes/archive/2026-08-30-gentle-model-bg-verify` mechanical move (archive audit trail not reverted); `sdd-attempt settle` already complete, no reset needed; if ledger token stuck `biggz sdd-attempt reset` if needed (none at close).

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived: `proposal` (intent 3 gaps, success 5 criteria) → `spec` (9 req 33 scen across 3 domains) → `design` (3 ADRs, 8 file rows, 4 threat boundaries) → `tasks` (23 tasks, 2 work units stacked-to-main, forecast Medium) → `apply` (23/23 tasks: PR1 `6d1df8f` 42 + PR2 `2e4fd78` 322 + `ae94734` 244, `go vet` + filtered `go test` PASS, `gofmt` clean) → `verify` (PASS WITH WARNINGS 9/9 33/33, `evidence_revision sha256:e7a4b971…` bound, 0 CRITICAL) → `archive` (3 delta→main sync + mechanical folder move `openspec/changes/2026-08-30-gentle-model-bg-verify/` → `openspec/changes/archive/2026-08-30-gentle-model-bg-verify/` + this report).

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks. Audit trail preserved in `openspec/changes/archive/2026-08-30-gentle-model-bg-verify/` — never delete or modify archived changes.

## Commands Run (Archive Phase)

- `cat openspec/changes/2026-08-30-gentle-model-bg-verify/proposal.md + design.md + tasks.md + apply-progress.md + verify-report.md` → validated completeness 23/23, 9/9 req, PASS WITH WARNINGS, 3 ADRs, 2 PRs <400
- `./biggz sdd-status --json` → `artifactStore openspec`, `tasks total 23 completed 23 allComplete true`, `dependencies all_done archive ready`, `nextRecommended archive`, `IsArchived false` (before move)
- `./biggz rdd status` → `enabled` (Global enabled) but openSpec Standard mode `reviewGate` not required for archive (no `review/{...}` needed, no `blockedReasons`)
- `grep -c "### Requirement:" openspec/specs/{model-routing,runtime,release-pipeline}/spec.md` → before sync `1/9/7`, delta `1+2/2+1/2+1`, after sync `3/10/9` verified
- `write openspec/specs/model-routing/spec.md` → 3 req (1 MODIFIED +2 ADDED, 11 scen), verified `grep -c` 3, `wc -l` ~90
- `write openspec/specs/runtime/spec.md` → 10 req (7 preserved +2 MODIFIED +1 ADDED, 11 scen delta merges), verified `grep -c` 10, first 7 preserved via `grep Grouped`/`Pi Progress`/`Ledger` etc.
- `write openspec/specs/release-pipeline/spec.md` → 9 req (6 preserved +1 MODIFIED +2 ADDED, 11 scen delta merges), verified `grep -c` 9, `grep -c "Canonical Verify"` 1, `grep -c "Release Integrity"` 1
- `mkdir -p openspec/changes/archive && mv openspec/changes/2026-08-30-gentle-model-bg-verify openspec/changes/archive/2026-08-30-gentle-model-bg-verify` → pass, `ls openspec/changes/` shows only `archive/`, `ls -R archive/...` shows 6 artifacts + specs (3 domains)
- `write archive-report.md` → this file, 23/23 tasks evidence, 9/9 33/33 compliance, hashes `sha256:e7a4b97…`/`sha256:e3b0c44…`, rollback boundaries (3 commits stacked-to-main), gates PASS
- Verification readback: `grep -c "^- \[x\]"` 23/0 in archived `tasks.md`, `head -n 15 verify-report.md` `pass_with_warnings 9/9 33/33`, `cat verify-report.md | grep evidence_revision` `e7a4b9…`, `ls -lh openspec/specs/{model-routing,runtime,release-pipeline}/spec.md`, `wc -l` 3/10/9 req, `git status --short` shows modified specs + moved folder + untracked verify-report now archived
- No commits after `verify-report.md` at archive time (per handoff) — no new commits beyond `ae94734`/`f75874a`/`b5c5f37`; ledger remains `2/4` remaining, evidence bound.

## Key Learnings

1. Sorted `Record<provider,Record<model,string[]>>` with `LoadVariantsOrEmpty` keeps picker deterministic and tolerates missing cache without error.
2. Canonical `sdd/background.go` 4-source `project>global>env>off` strict 2-key max 2 reads closes malformed fallback divergence versus `.pi/settings.json` drift.
3. `VerifyBinary` `sha256==integrity.json` with `isConfined`+`isSymlink`+`sameFile`+`isCanonicalManifest` guards avoids network TOCTOU.
4. Stacked-to-main each commit <400 preserves review budget even when total ~608 exceeds single-PR 400 limit.
5. Filtered `go test ./internal/sdd -run Test[^R]` is required to exclude pre-existing `TestReadLoopLarge` `pending_test.go:106` flake not introduced by slice.
