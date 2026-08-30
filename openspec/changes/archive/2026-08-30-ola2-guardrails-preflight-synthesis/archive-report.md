# Archive Report: 2026-08-30-ola2-guardrails-preflight-synthesis — Ola 2 Guardrails / Preflight / Synthesis Gate

**Change**: `2026-08-30-ola2-guardrails-preflight-synthesis`
**Archived to**: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/`
**Date**: 2026-08-30
**Status**: archived (PASS 7/7 req 21/21 scen, ledger complete, remediation included)
**Mode**: openspec / interactive / auto-chain / stacked-to-main / 400-line budget / Standard (strict_tdd false)

## Summary

Ola 2 ports gentle-pi hardening to biggz-ai: guardrails deny/classify/config/sensitive-path, preflight artifact-store normalization + canonicalize + disk persist/resolve cache, and synthesis gate 120s window. Retroactive formalization of commit `9f6c8be` (470 lines: 251 + 152 + 67) plus remediation working-tree fixes staged for archive commit. Single stacked-to-main slice, `auto-chain`, `Medium` (470 exceeds 400 by 70 documented as `size:exception-ok` lineage).

- **Policy guardrails** — `internal/policy/guardrails.go` 251 lines base: `deniedBashPatterns[6]`/`IsDenied` with indices 2 (`git clean` needs `-f`/`--force` + `-d`/`--directories`) and 3 (`git push` needs `--force`/`-f`) refinement; `guardedKeyPatterns[5]` (`gitPush`, `gitRebase`, `gitBranchDeleteForce`, `npmPublish`, `piRemove`) + `autonomousDefaultActions` (`gitPush allow`, `npmPublish block`, others `confirm`) + `ClassifyGuardedCommand` deny-first; `Parse/LoadRuntimeGuardrailsConfig` two-file merge (`global→project`, project wins, env fast-path `GENTLE_PI_AUTONOMOUS_MODE=1`, malformed→safe); `sensitivePathPatterns[8]` (`\.ssh`, `\.credentials`, `library/keychains`, `\.aws/credentials`, `\.config/gh/hosts.yaml`, `secrets/`, `\.env`, `.(pem|key|p12|pfx)$`) + `isSensitivePath` normalize + `collectPathInputs` recursive + `EvaluateSensitivePathTool` Block.
- **Preflight** — `internal/sdd/preflight.go` 152 lines base: `NormalizePreflightArtifactStore` alias folding (`both/hybrid/engram/bigmem→hybrid`, `openspec→openspec`, `none→""`), `canonicalizePrefs` defaults (`interactive`/`stacked-to-main`/`400`), `SddPreflightDiskPath` (`home[0]`>`GENTLE_PI_CONFIG_HOME`>`UserHomeDir/.pi/gentle-ai` + `sdd-preflight.json`), `Write/ReadSddPreflightToDisk` (`MkdirAll 0755` + `MarshalIndent` + `0644` `\n`) + cache `Resolve` (`cache>disk>defaults`), `ValidatePreflightQuestionEnvelope` enums + `SessionRecallMarkdown`.
- **Synthesis gate** — `internal/sdd/synthesis_gate.go` 67 lines: `synthesisMarkers[4]` (`## Sub-agent Result:`, `**What was done:**`, `**Artifacts/Paths:**`, `**Next Recommended:**`), `HasSynthesis` all-4, `HasSessionRecall`, `IsChildBypass` (`PI_SUBAGENT_CHILD=1`), `IsCheckpointAsk` (`proceed|adjust|stop|continue|correct`), globals `currentTurnMarkdown/currentTurnTime` via `SetCurrentTurnMarkdown`, `ShouldBlock` 120s + bypasses (`false` if child/recall/notCheckpoint/`now-sub>120s` else `!HasSynthesis`), `CheckSynthesisPrecondition` message.

All 12 tasks complete, 7 requirements 21 scenarios compliant, verification PASS with ledger-bound remediation evidence.

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| **Deny IsDenied 6-pattern slice** | 6 patterns with indices 2/3 flag refinement via `\s-[^\s]*f` / `\s-[^\s]*d` and `--force`/`--directories` fallback, not greedy regex | Greedy would block `git clean`/`git push` without flags (no lookahead in Go); combined `-fd` must be detected via `\s-[^\s]*f` and `\s-[^\s]*d` — remediation fixes prior `strings.Contains("-f")` over-narrow check that missed `git clean -fd` |
| **Classify denied-first** | `ClassifyGuardedCommand` calls `IsDenied(command)` refinement first → `block`, then 5 keys with `!auto→confirm`, else `guardedCommands` override else `autonomousDefaultActions` (`gitPush allow`, `npmPublish block`) else `not-guarded` | Prevents `confirm` for `push --force`; prior loop over `deniedBashPatterns` without refinement over-blocked plain `git push` via prefix match; `IsDenied` centralizes flag logic |
| **Config parse/merge allowlist+merge** | `ParseGuardrailsConfigFile` filters `validActions {allow,confirm,block}` × `validKeys {5}`; `LoadRuntimeGuardrailsConfig` env fast-path `GENTLE_PI_AUTONOMOUS_MODE=1` → `{autonomous:true,empty}`, else read `global` then `project` (`globalPath=home/runtime-guardrails.json`, `projectPath=cwd/.pi/gentle-ai/...`), malformed→`safeGuardrailsConfig`, merge project wins, nil→map guard | Verbatim gentle-pi `guardrails.ts`; fail-closed safe defaults; copy-on-merge avoids mutating global |
| **Sensitive path 8 regexes normalized** | 8 regexes `ToLower+TrimSpace+\→/+~→HOME` + `collectPathInputs` recursive `map[string]any`/`[]any` keyed `path/paths/file/files/filePath/filePaths` plus direct `[]string` extraction; only `read/write/edit` guarded; `Block` with reason | Suffix alone misses `~/.ssh`, `secrets/.env`; recursion covers nested/array inputs |
| **Preflight canonicalize alias folding** | `both/hybrid/engram/bigmem→hybrid`, `none→""`, `openspec→openspec`, else lower passthrough; `canonicalizePrefs` fills `interactive/400` | Matches `sdd-preflight.ts`; single alias avoids `BigMem` drift; `none→""` disables planning I/O per sdd-status contract |
| **Preflight persist home+cache** | `SddPreflightDiskPath` `home[0]`>env>`UserHomeDir/.pi/gentle-ai` + `sdd-preflight.json`; `Write` `canonicalize`+`MkdirAll 0755`+`MarshalIndent`+`0644` `\n`; `Resolve` `cache>disk>defaults {interactive,openspec,stacked-to-main,400}` | Repo-file would pollute; cache enables session without disk read; injectable `home` for `TempDir` isolated tests |
| **Synthesis gate 120s+bypass** | 4 markers required, `ShouldBlock` `!child&&!recall&&checkpoint&&≤120s&&!HasSynthesis`; `CheckSynthesisPrecondition` wraps with `synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)`; injectable `now` prod `time.Now` only wrapper | Immediate would block late/child; bypass `Session Recall` and `PI_SUBAGENT_CHILD=1` keeps subagent autonomy; 120s window injectable for test determinism |
| **Remediation via IsDenied+Classify** | Fix `IsDenied` combined `-fd` via `\s-[^\s]*f`/`\s-[^\s]*d` and `Classify` via `IsDenied` call; `gofmt -w` alignment for `Guard*` consts and struct tags | Remediates prior FAIL 3 critical (`git clean -fd` false-negative, `Classify` over-block plain push); `gofmt` clean required for `verify-report` PASS |

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| `policy` | Created | 4 ADDED requirements (Bash Destructive Pattern Deny 3 scenarios, Guarded Command Classification 3 scenarios, Guardrails Config Parse and Two-File Merge 3 scenarios, Sensitive Path Tool Guard 3 scenarios) → new `openspec/specs/policy/spec.md` (0→4 requirements, 12 scenarios). Delta at `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/specs/policy/spec.md` preserved. |
| `sdd` | Created | 3 ADDED requirements (Preflight ArtifactStore Normalization 3 scenarios, Preflight Disk Persist and Resolve 3 scenarios, Synthesis Gate Markers 3 scenarios) → new `openspec/specs/sdd/spec.md` (0→3 requirements, 9 scenarios). Instruction noted `sdd` as existing domain but filesystem has no `openspec/specs/sdd/spec.md`, so created as new domain (verbatim delta copy). Delta at `sdd/spec.md` preserved. |

**Totals**: 2 domains, 7 requirements, 21 scenarios. Delta specs preserved under archive.

## Files Changed

| File | Action | What Was Done | Lines |
|------|--------|---------------|-------|
| `internal/policy/guardrails.go` | Created (base commit `9f6c8be`) + remediated (staged) | 251 lines base: `deniedBashPatterns[6]`, `Guard*` consts, `guardedKeyPatterns[5]`, `autonomousDefaultActions`, `RuntimeGuardrailsConfig`, `IsDenied` with indices 2/3 refinement, `ClassifyGuardedCommand`, `Parse/LoadRuntimeGuardrailsConfig`, `gentlePiConfigHome`, `sensitivePathPatterns[8]`, `isSensitivePath`, `collectPathInputs`, `EvaluateSensitivePathTool`; remediation: `IsDenied` now detects combined `-fd` via `\s-[^\s]*f` and `\s-[^\s]*d` (vs `strings.Contains("-f")`), `ClassifyGuardedCommand` now calls `IsDenied(command)` refinement first (vs loop over `deniedBashPatterns` without refinement that over-blocked plain `git push`), `gofmt -w` aligned `Guard*` consts spacing | +251 base, remediation +16 / -18 (net -2, 34 lines touched) |
| `internal/sdd/preflight.go` | Created (base commit `9f6c8be`) + remediated (staged) | 152 lines base: `PreflightPrefs` + `preflightCache`, `SddPreflightDiskPath`, `NormalizePreflightArtifactStore`, `canonicalizePrefs`, `Write/ReadSddPreflightToDisk`, `Set/Get/Clear/ResolvePreflightPrefs`, `ValidatePreflightQuestionEnvelope`, `SessionRecallMarkdown`; remediation: `gofmt -w` aligned struct tags (`ExecutionMode` `ArtifactStore` `ChainedPrStrategy` `ReviewBudgetLines` and `PreflightQuestionEnvelope` `Pace` `PRs` `Review`) | +152 base, remediation +14 / -14 (net 0, 28 lines touched, gofmt clean) |
| `internal/sdd/synthesis_gate.go` | Created (base commit `9f6c8be`) | 67 lines: `synthesisMarkers[4]`, globals `currentTurnMarkdown/currentTurnTime`, `SetCurrentTurnMarkdown`, `HasSynthesis`, `HasSessionRecall`, `IsChildBypass`, `IsCheckpointAsk`, `ShouldBlock` (120s), `CheckSynthesisPrecondition` | +67 base, no remediation delta (already gofmt clean) |
| **Base total** |  | 3 files, 470 insertions (+) base `9f6c8be` (`251+152+67`) `Medium` stacked-to-main | 470 |
| **Remediation total** |  | 2 files modified, 30 insertions / 32 deletions staged (guardrails 16/18 + preflight 14/14) → 62 lines touched, net -2; instruction forecasts 32 lines — within tolerance (counts `gofmt` whitespace vs `IsDenied/Classify` logic) | ~32 logic + gofmt |
| **Combined staged diff vs HEAD** |  | `internal/policy/guardrails.go` + `internal/sdd/preflight.go` remediation + `openspec/specs/policy/spec.md` + `openspec/specs/sdd/spec.md` new + archive move (deletes + adds under `archive/`) |  |

No files outside design table were changed. Retroactive formalization + remediation fits single PR `stacked-to-main` `auto-chain` `400` budget nominated `size:exception-ok` (470 exceeds 400 by 70 = 17.5% documented overage for already-merged commit; remediation +32 is within same PR boundary per verify-report).

## Verification Outcome

**Verdict**: PASS — 7/7 requirements, 21/21 scenarios COMPLIANT, 0 blockers, 0 critical_findings, 0 warnings (after remediation). Build `go vet` PASS, `gofmt -l` clean (0 files), focused tests PASS, manual harness 82/82 PASS (21 scenarios mapped). Previous FAIL remediated.

**Evidence**:
- `evidence_revision`: `sha256:5f285c64ff53c2b7704c416d2b5b378ad129f89cbcf4528850625ca50b3ad791` (SHA256 of combined verify output `/tmp/verify.out`)
- `previous_failed_evidence`: `sha256:a8536c6b0dceaa04f90f41841cd7634ddf2fd48550c175ba7c631bd8e4061e6b` (verdict FAIL, 3 critical: `git clean -fd` combined flag false-negative, `Classify` over-blocking plain `git push` → `block` vs expected `allow/confirm`)
- `ledger`: `4293e7098281b6f3503a8362ff00dae94bd42c3a33bc21b5f531281d2aea3003` complete (`tok-8f6efb7c6e8e0d4c1d1211f1` verify 7 req 21 scen, max-attempts 3, max-lines 400, revision `1211951952bbe62155203aced0790851c28044abf294a188d3eff96a15233ecd` → settle `4293e7098281b6f3503a8362ff00dae94bd42c3a33bc21b5f531281d2aea3003` passed with `--remediates-evidence-revision sha256:a8536c6b0dceaa04f90f41841cd7634ddf2fd48550c175ba7c631bd8e4061e6b`)
- `test_command`: `go test ./internal/policy -count=1 -v; go test ./internal/sdd -run TestSynthesis -count=1 -v; go run tmp_verify_manual.go` → exit 0
- `build_command`: `go vet ./...` → exit 0, hash `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty stdout), `gofmt -l` clean on delta after remediation (`gofmt -w` guardrails.go, preflight.go)
- `validator`: `biggz sdd-verify-validate --input tmp_candidate_verify.md --requirements 7 --scenarios 21` → **admitted** (7/7, 21/21) via verify-report workflow
- `verify date`: 2026-08-30 (report persisted `2026-08-29 20:44` verify-report.md, remediation staged at archive time)

**Remediation Evidence (Final-State Facts)**:
- `IsDenied` combined `-fd` fix: `git clean -fd` now correctly returns `true` via `regexp.MustCompile(`\s-[^\s]*f`)` + `\s-[^\s]*d` plus `--force`/`--directories` fallback; previously `strings.Contains(command, "-f")` missed combined flag shape and caused `IsDenied("git clean -fd")` false-negative (CRITICAL).
- `ClassifyGuardedCommand` `IsDenied` refinement: now `if IsDenied(command) {return "block"}` before guarded classification; previously looped `deniedBashPatterns` without index-2/3 flag refinement, causing `ClassifyGuardedCommand("git push", {auto:true})` → `block` vs expected `allow` and `ClassifyGuardedCommand("git push origin main", {auto:false})` → `block` vs `confirm` (CRITICAL over-block).
- `gofmt` clean: `gofmt -w internal/policy/guardrails.go internal/sdd/preflight.go` removes alignment drift (`Guard*` consts, struct tags `PreflightPrefs` and `PreflightQuestionEnvelope`); `gofmt -l` now lists 0 files (previously 2).
- Previous FAIL `sha256:a8536c6b0dceaa04f90f41841cd7634ddf2fd48550c175ba7c631bd8e4061e6b` remediated via new PASS `sha256:5f285c64ff53c2b7704c416d2b5b378ad129f89cbcf4528850625ca50b3ad791` with explicit `biggz sdd-attempt settle --remediates-evidence-revision` linking.
- Explicit critical checks at verify time: `IsDenied("git clean -fd")` true PASS, `Classify("git push", {auto:true})` → `allow` PASS, `Classify("git push origin main", {auto:false})` → `confirm` PASS — all in manual harness `82/82` and `R1S2-clean-fd-combo` / `R2S4-6` compliance matrix.

**Test slices** (all PASS):

| Command | Result | Evidence |
|---------|--------|----------|
| `go test ./internal/policy -count=1 -v` | PASS 7/7 | `TestPolicyInterceptor_BeforeBlocksInjectedBash`, `ReviseUsesRevisedArgs`, `AfterObserveDoesNotMutate`, `ConsentAllowAndDeny (2 sub)`, `DefaultAllow`, `NoFSMImportAndNoGodObject`, `FakeExtensionAPI` |
| `go test ./internal/sdd -run TestSynthesis -count=1 -v` | PASS 3 sub | `humanized_JSON`, `prefix_BIGGZ`, `plain_and_empty` |
| `go run tmp_verify_manual.go` (21-scenario exhaustive) | PASS 82/82 | R1S1 rm -rf/reset/chmod, R1S2 git clean `-f -d`/`--force --directories` true, `-f`/`-d`/bare false, `R1S2-clean-fd-combo` `-fd` true (remediated), R1S3 push `--force/-f/-uf` vs plain, R2S4 denied→block, R2S5 auto defaults `allow/block/confirm` + custom override, R2S6 `!auto confirm` + `not-guarded`, R3S7 parse filter `badKey/badAction` + malformed false, R3S8 env fast-path + malformed safe, R3S9 global+project merge, R4S10 ssh/aws/secrets/env Block, R4S11 pem/hosts/keychain Block, R4S12 non-sensitive nil + exec not guarded + `[]string`/`[]any` nested, R5S13 alias→hybrid + none→"", R5S14 openspec passthrough, R5S15 canonicalize empty→`interactive/""/stacked/400` + explicit `BigMem→hybrid`, R6S16 disk round-trip `0644` pretty JSON, R6S17 resolve `cache>disk>defaults`, R6S18 envelope enums + `SessionRecallMarkdown` `## Session Recall` `2 observations, 1 sessions` `Project: biggz-ai`, R7S19 all-4 markers + missing false + recall/child bypass, R7S20 checkpoint 30s block / non-checkpoint no block / 121s expiry allow / synthesis no block, R7S21 precondition message |
| `go vet ./...` / `go vet ./internal/policy ./internal/sdd` | PASS exit 0 | empty output, hash `e3b0c44…` |
| `gofmt -l guardrails.go preflight.go synthesis_gate.go` | PASS 0 listed | remediation via `gofmt -w` |
| `sh "skills/use-modern-go/scripts/run-tool.sh" list --file-path` (3 files) | Consulted | Go 1.25, 40+ idioms surfaced (`slices.Contains`, `cmp.Or`, etc.); no REQUIRED modernization missed |
| `biggz sdd-verify-validate --input tmp_candidate_verify.md --requirements 7 --scenarios 21` | admitted | 7/7 21/21 |
| `biggz sdd-attempt acquire/settle` | 1 token complete | `tok-8f6efb7c6e8e0d4c1d1211f1` rev `121195…` → `4293e70…` passed, 3 attempts max, 400 lines budget, remediates prior FAIL |

**Modern Go guidelines**: consulted for all 3 files; files use idiomatic Go (`regexp.MustCompile`, `strings.Contains`, maps for allowlists) — considered, no REQUIRED change.

**Coverage**: manual harness covers 21/21 scenarios, existing suite covers interceptor + synthesis; not threshold-gated per `strict_tdd false` (deferred table-driven `*_test.go` remains Open Question per design.md).

## Archive Contents

- `proposal.md` ✅ (Intent Ola 2 port `9f6c8be` 470 lines, Problem 4 bullets, Solution 3-file, Scope `openspec` `strict_tdd false` `interactive/auto-chain/400`, Success 9 checkboxes, Risks, Rollback `git revert 9f6c8be`, Dependencies gentle-pi)
- `specs/policy/spec.md` ✅ (Delta 4 requirements, 12 scenarios — deny 3, classify 3, config 3, sensitive 3)
- `specs/sdd/spec.md` ✅ (Delta 3 requirements, 9 scenarios — canonicalize 3, persist 3, gate 3)
- `design.md` ✅ (7 architecture decisions, spec refs 2 deltas, data flow 4 sections, file changes 8 rows, interfaces 11 funcs, testing 7 layers, threat matrix 6 → mitigation, migration `git revert 9f6c8be` isolated)
- `tasks.md` ✅ (12/12 [x] — Phase1 1.1-1.3, Phase2 2.1-2.3, Phase3 3.1-3.2, Phase4 4.1-4.4; 0 unchecked; workload `~470` `Medium` single PR `stacked-to-main`)
- `apply-progress.md` ✅ (12/12 complete, commit `9f6c8be` 470 lines base, files-changed table 9 rows, work-unit evidence `go vet/test` + `git show --stat`, deviations zero, workload/boundary `revert 9f6c8be`)
- `verify-report.md` ✅ (yaml frontmatter `pass` 0 blockers 0 critical 7/7 21/21, evidence `sha256:5f28…`, remediates `sha256:a853…`, ledger `4293e70…` passed, remediated FAIL, completeness, build & tests execution, 21/21 compliance matrix, correctness, coherence, issues, verdict PASS)
- `archive-report.md` ✅ (this file)

Active directory `openspec/changes/2026-08-30-ola2-guardrails-preflight-synthesis/` no longer exists; change now solely under `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/` (date prefix `2026-08-30`).

## Source of Truth Updated

The following specs now reflect the new behavior (spec sync completed before archive move):

- `openspec/specs/policy/spec.md` — new, 4 requirements (12 scenarios) — bash deny + classify + config merge + sensitive path
- `openspec/specs/sdd/spec.md` — new, 3 requirements (9 scenarios) — store canonicalize + persist/resolve + synthesis gate

All delta requirements preserved verbatim with scenarios; no REMOVED/RENAMED; non-delta requirements unchanged (new domains have no prior requirements). Files created via direct copy of delta (provisional `Delta for` header retained per OpenSpec change spec sync instruction for new domains; purpose wording preserved from delta premise).

## Final-State Facts (2026-08-30 at archive)

- **Verify PASS (remediation)**: 2026-08-30, evidence `sha256:5f285c64ff53c2b7704c416d2b5b378ad129f89cbcf4528850625ca50b3ad791`, ledger `4293e7098281b6f3503a8362ff00dae94bd42c3a33bc21b5f531281d2aea3003` complete, admitted validator. Previous FAIL `sha256:a8536c6b0dceaa04f90f41841cd7634ddf2fd48550c175ba7c631bd8e4061e6b` (3 critical) remediated via `IsDenied` combined `-fd` + `Classify IsDenied` + `gofmt`.
- **Compliance**: 21/21 scenarios COMPLIANT (0 PARTIAL/UNTESTED/FAILING) per spec compliance matrix with covering manual harness + focused suites.
- **Remediation staged for commit**: `internal/policy/guardrails.go` `IsDenied` `\s-[^\s]*f`/`\s-[^\s]*d` + `Classify` `IsDenied` call, aligned `Guard*` consts; `internal/sdd/preflight.go` `gofmt` aligned struct tags; `internal/sdd/synthesis_gate.go` unchanged (already clean). Working-tree fixes included in archive commit diff (uncommitted at verify time, now staged).
- **gofmt clean**: 0 files listed after remediation (`internal/policy/guardrails.go`, `internal/sdd/preflight.go`, `internal/sdd/synthesis_gate.go`); Windows perm `0666` vs `0644` lenient per spec (not flagged).
- **Ledger**: `4293e7098281b6f3503a8362ff00dae94bd42c3a33bc21b5f531281d2aea3003` complete via `biggz sdd-attempt acquire/settle` bounded verify with remediates linkage.
- **Tasks**: 12/12 `[x]`, 0 unchecked, `applyState all_done`, `nextRecommended verify` → archived (now `done`). Task Completion Gate PASS at archive time.
- **Pre-existing outside change**: Full `go test ./... -short 180s` has 2 pre-existing failures outside ola2 scope (`TestReadLoopLarge`, `TestOrchestratorSynthesisTemplateInvariant` per ola1/ola3 verify-reports, base `9c73f6f`) — not introduced by ola2; verify-report scoped harness is authoritative for this change.
- **No migration**: `git revert 9f6c8be` removes 3 files (470) isolated from ola1 (`2ff2737`, `d0c527e`) and ola3 (`f6d636d`); remediation diff is within same rollback boundary.

## Commits

- **Base**: `ce708fb` docs(sdd): archive ola1 gentle hardening — verify PASS 10/10 32/32 (now `HEAD` before archive commit)
- **Base prod**: `9f6c8be` feat: recreate ola2 guardrails, preflight, synthesis gate — 470 lines (`internal/policy/guardrails.go 251`, `internal/sdd/preflight.go 152`, `internal/sdd/synthesis_gate.go 67`) base `main` — already on `HEAD`, remediation is follow-up diff on top
- **Remediation (staged, not yet committed)**: `internal/policy/guardrails.go` (16 ins / 18 del) IsDenied combined `-fd` via `\s-[^\s]*f`/`\s-[^\s]*d` + `ClassifyGuardedCommand` `IsDenied` refinement; `internal/sdd/preflight.go` (14 ins / 14 del) `gofmt -w` alignment — total ~32 logic+gofmt lines within same PR boundary per instruction
- **Archive sync (staged)**: `openspec/specs/policy/spec.md` (new 92 lines, 4 req 12 scen), `openspec/specs/sdd/spec.md` (new 70 lines, 3 req 9 scen)
- **Archive move (staged)**: `openspec/changes/2026-08-30-ola2-guardrails-preflight-synthesis/` → `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/` (proposal, design, tasks, apply-progress, verify-report, 2 spec deltas) + new `archive-report.md` (this file)
- **Ahead**: `HEAD` is `ce708fb`; `origin/master` is behind by at least `f6d636d` ola3 + `9f6c8be` ola2 + `2ff2737`/`d0c527e` ola1 stacked (exact ahead count per `git log --oneline origin/master..HEAD`)

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. `biggz sdd-status` after commit will report `IsArchived: true`, `nextRecommended: done`, `HasProposal/HasSpecs/HasDesign/HasTasks/HasApply/HasVerify: true`, `TasksTotal: 12 TasksDone: 12`. No `CRITICAL` issues (previous 3 remediated). Archive audit trail preserved under `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/` (proposal, 2 specs, design, tasks, apply-progress, verify-report, archive-report). Source of truth synced (`openspec/specs/policy/spec.md`, `openspec/specs/sdd/spec.md`). Remediation fixes staged for commit. Ready for next change.

## Risks / Open Questions

- Future verify should add permanent table-driven `*_test.go` for guardrails/preflight/synthesis to prevent regression (design Open Question `[ ]`).
- Consider `slices.Contains`, `cmp.Or` modern idioms where applicable (SUGGESTION, not required).
- `sdd` spec created as new domain despite instruction noting "existing" — filesystem had no prior `openspec/specs/sdd/spec.md`; created as new to satisfy sync; if future `sdd` spec is consolidated elsewhere, merge rename migration can be recorded with Reason/Migration.
- Policy `Delta for policy` header retained verbatim per sync copy instruction; eventual purpose paragraph can be promoted to `Purpose` section without changing requirements.

## References

- proposal.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/proposal.md`
- specs/policy/spec.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/specs/policy/spec.md` (delta, 4 req 12 scen)
- specs/sdd/spec.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/specs/sdd/spec.md` (delta, 3 req 9 scen)
- design.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/design.md`
- tasks.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/tasks.md` 12/12
- apply-progress.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/apply-progress.md` (commit `9f6c8be` 470 lines + remediation)
- verify-report.md: `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/verify-report.md` PASS 7/7 21/21 evidence `sha256:5f28…` ledger `4293e70…` remediates `sha256:a853…`
- archive-report.md: this file `openspec/changes/archive/2026-08-30-ola2-guardrails-preflight-synthesis/archive-report.md`
- Specs synced: `openspec/specs/policy/spec.md` (new 4 req), `openspec/specs/sdd/spec.md` (new 3 req)
- Base commit: `9f6c8be` (3 files 470 lines)
- Remediation staged: `internal/policy/guardrails.go` IsDenied combined + Classify, `internal/sdd/preflight.go` gofmt
- Verification ledger: `4293e7098281b6f3503a8362ff00dae94bd42c3a33bc21b5f531281d2aea3003` (`tok-8f6efb7c6e8e0d4c1d1211f1` acquire rev `1211951952bbe62155203aced0790851c28044abf294a188d3eff96a15233ecd` → settle passed, remediates `a8536c6b…`)

## Key Learnings:
1. IsDenied combined `-fd` must use `\s-[^\s]*f` and `\s-[^\s]*d` not `strings.Contains("-f")` to catch `git clean -fd` while still requiring both force and directory flags (greedy would over-block).
2. ClassifyGuardedCommand must call centralized `IsDenied` with its flag refinements, not raw `deniedBashPatterns` loop, to avoid over-blocking plain `git push` without `--force`.
3. gofmt alignment drift on struct tags and const blocks (`Guard*`, `PreflightPrefs`, `PreflightQuestionEnvelope`) is caught by verify gate; `gofmt -w` at remediation time clears WARNING.
4. `GENTLE_PI_AUTONOMOUS_MODE=1` fast-path must short-circuit before any file read to avoid malformed global fallback.
5. `SddPreflightDiskPath` injectable `home[0]` > env > `UserHomeDir` ordering isolates `TempDir` tests; `MkdirAll 0755` + `0644` pretty JSON preserves round-trip canonicalization (alias `BigMem→hybrid`).
6. Synthesis gate 120s window with `SetCurrentTurnMarkdown(time.Now)` injectable `now` keeps tests deterministic; bypass `Session Recall` and `PI_SUBAGENT_CHILD=1` prevents checkpoint block in recall/child contexts.
7. OpenSpec archive for retroactive commit must stage remediation fixes alongside spec sync and folder move; `git mv` verbatim plus `git add` ofRemediation ensures single archive commit closes delta plus fix.
8. Policy `sdd` was noted as existing domain but absent on filesystem; creating as new domain with delta copy preserves audit trail — eventual consolidation can be handled via RENAMED migration.
