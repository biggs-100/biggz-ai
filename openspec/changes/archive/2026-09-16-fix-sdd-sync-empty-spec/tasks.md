# Tasks: fix-sdd-sync-empty-spec

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~375 authored (+360/−15): `sync_helpers.go` ~+100/−15; new `sync_helpers_test.go` ~+260 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (one work unit) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

Arithmetic: 100 + 15 + 260 = 375 ≤ 400 (margin ~25, ~6%). `sync.go` needs no change (propagation already correct), so the diff is two files. Auto-chain proceeds; no ask. Swing factor is the test file: if T1–T5 + the 6-row unit table exceed ~285 lines, measure at apply and split the test file into a stacked slice before review. Change-dir SDD artifacts (proposal/design/spec/tasks) are pipeline documents, excluded from the code-diff count.

### Suggested Work Units

| Unit | Goal (start → finish) | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|----------------------|-----------|----------------------|-----------------|-------------------|
| 1 | RED tests reproducing the OBSERVED 0-byte false-green → writer rules 1–3 in `sync_helpers.go`; no contentless write possible, `applied` honest | PR 1 | `go test ./internal/sdd -count=1` | `exploration.md` Appendix A overlay reproducer (scratch `-overlay`, zero repo writes): mixed change → non-empty byte-identical spec; gate-bypass → `blocked`, no 0-byte file | Revert `sync_helpers.go`; delete `sync_helpers_test.go`. Copies fire only on absent mains — no migration, nothing to heal |

## Phase 1: RED — Reproduce the observed defect (before the fix)

- [x] 1.1 Create `internal/sdd/sync_helpers_test.go` (package `sdd`): `t.TempDir()` workspace builder + passing `biggz-ai.verify-result/v1` report fixture (`syncVerifyMustPass` gate). Evidence: `go vet ./internal/sdd` clean. — complete: `newSyncFixture`/`writeChangeSpec`/`writeVerifyReport` (totals counted from the actual change-spec headings) + `livingSpecsSnapshot`; `go vet ./internal/sdd` → clean.
- [x] 1.2 RED T1 mixed change (full-spec new domain + ADDED domain): `Sync()` = `applied`; `openspec/specs/fullspec-domain/spec.md` exists, non-empty, byte-identical to its delta file; change dir still exists. Pre-fix: `go test ./internal/sdd -run TestSyncMixedFullSpecVerbatim -count=1` FAILS (observed 0 bytes under `applied`). — complete: RED on `97050fab` → `FAIL: living spec … is empty under result "applied" (false-green)`; post-fix PASS with the byte-identical copy and the change dir intact (plus the D6 re-run, still `applied`).
- [x] 1.3 Pin T2 full-spec-only change: `not-applicable` naming the change; zero files created/modified under `openspec/specs/`. Passes pre-fix (observed) — regression pin. — complete: passes pre-fix and post-fix; `result="not-applicable"`, message names `chg-fullspec`, living-spec snapshot unchanged.
- [x] 1.4 RED T3 content-without-requirement-blocks (mixed with a valid ADDED domain): `blocked` naming offending file + domain + remedy; zero files created/modified under `openspec/specs/`. Pre-fix: FAILS (observed 0-byte write under `applied`). — complete: RED on `97050fab` → `FAIL: result = "applied", want "blocked" (msg "sync applied for change chg-noblocks")` (the same run wrote the sibling domain); post-fix PASS, message names `noblocks-domain/spec.md` + remedy, snapshot unchanged (block-before-any-write).
- [x] 1.5 Pin T4 existing living spec + MODIFIED: `applied` in place; H1/`## Purpose` header and unmodified requirements preserved. Passes pre-fix (observed) — regression pin. — complete: passes pre-fix and post-fix; `SHALL be NEW` present, `SHALL be old` gone, H1/`## Purpose`/`Untouched Requirement` preserved.
- [x] 1.6 RED T5 gate-bypassed writer: direct `syncApplyDeltas` on a full-spec-only change's infos → no 0-byte file; result MUST NOT be `applied` (honest status). Pre-fix: FAILS (observed 0-byte file, `err=nil`). — complete: RED on `97050fab` → `FAIL: living spec … is 0 bytes (false-green)`; post-fix PASS: byte-identical copy, writer result `""` (not `applied`), `err=nil`.
- [x] 1.7 Capture the pre-fix RED output (T1/T3/T5 failing, T2/T4 passing) into `apply-progress.md` as the reproducer evidence. — complete: RED excerpts in `openspec/changes/fix-sdd-sync-empty-spec/apply-progress.md` (untracked) + BigMem `sdd/fix-sdd-sync-empty-spec/apply-progress`.

## Phase 2: GREEN — Writer rules 1–3

- [x] 2.1 `internal/sdd/sync_helpers.go`: add `domainSource{path, bytes, empty, fullSpec, deltas}` and `domainInfo.sources`; rework `syncParseDomainInfos` to one read per `findSpecFiles` file (sorted order), keeping zero-delta domains and unchanged delta aggregation. — complete: one read per file, first-seen domain order (deterministic), zero-delta domains kept with `sources` populated; `fullSpec = requirementHeadingRe matches && no delta section` (`isFullSpecShaped`).
- [x] 2.2 Add `syncResolveDomainWrite(info, mainPath) ([]byte, SyncResult, string, error)` (D1–D8): absent main + exactly one non-empty `fullSpec` source → its raw bytes; empty source, content-without-blocks, ≥2 `fullSpec` candidates, differing existing target, REMOVED-against-absent-main, and empty delta result over non-empty source → `SyncBlocked` + reason; byte-equal target → nil-bytes skip; else `ApplyDeltas` result. — complete: all rules implemented; write rows return `SyncApplied` + bytes, skips nil + `""`, blocks nil + `SyncBlocked`.
- [x] 2.3 Add `syncWriteBlockedMessage(reason, info, file, remedy)` → fixed shape `sync blocked: <diagnosis> (domain <D>, file <path>); <remedy>` (path + domain + remedy always present). — complete: single builder used by every blocked branch; asserted in the unit table.
- [x] 2.4 `syncApplyDeltas`: resolve every domain first, return the first `SyncBlocked` before any write; write resolved bytes behind the existing `pathidentity.Contains` check; `applied` only follows a content write. `sync.go` unchanged (`sync.go:55-62` propagates `res != ""`). — complete: two-pass resolve→write; containment checked in the resolve pass; `sync.go` untouched.
- [x] 2.5 GREEN: T1/T3/T5 pass; T2/T4 stay green; `go test ./internal/sdd -count=1` → ok, 0 FAIL. — complete: `go test ./internal/sdd -count=1` → `ok github.com/biggs-100/biggz-ai/internal/sdd 24.354s`.

## Phase 3: Unit table — `syncResolveDomainWrite` contract

- [x] 3.1 Empty source (whitespace-only) → `SyncBlocked`, no write. — complete: subtest `empty_source_blocks` PASS.
- [x] 3.2 Non-empty content, no `### Requirement:` blocks → `SyncBlocked`. — complete: subtest `content_without_requirement_blocks_blocks` PASS.
- [x] 3.3 ≥2 `fullSpec` candidates → `SyncBlocked` naming both paths (sorted determinism). — complete: subtest `two_fullspec_candidates_block_naming_both` PASS (both source paths asserted in the message).
- [x] 3.4 Re-run over byte-equal target → skip (nil bytes, not blocked); status stays `applied`. — complete: subtest `byte-equal_target_skips` PASS (nil bytes, no block, result ≠ `applied`); the sync-level `applied` persistence is pinned in T1's second run.
- [x] 3.5 Differing existing target (full-spec delta + non-equal living spec) → `SyncBlocked` (D8, stricter than the scenario text; review task 4.6). — complete: subtest `differing_existing_target_blocks` PASS.
- [x] 3.6 REMOVED deltas against absent main → `SyncBlocked` (rule-3 backstop). — complete: subtest `removed_against_absent_main_blocks` PASS (empty `ApplyDeltas` result blocks).
- [x] 3.7 Honest-status assertion on every row: `applied` appears only where bytes were written; each "no write" row asserts result ≠ `applied`. — complete: common assertion (`res == SyncApplied && got == nil` fails the row); the single write row (`absent_main_and_one_fullspec_source_returns_raw_bytes`) asserts `SyncApplied` + raw bytes.
- [x] 3.8 GREEN: `go test ./internal/sdd -run TestSyncResolveDomainWrite -count=1` → ok. — complete: all 7 subtests PASS.

## Phase 4: Verification, `sdd` delta, close

- [x] 4.1 `go test ./internal/sdd -count=1` → ok, 0 FAIL (5 integration tests + unit table). — complete: `ok github.com/biggs-100/biggz-ai/internal/sdd 24.354s`.
- [x] 4.2 `go vet ./internal/sdd` → clean; `gofmt -l internal/sdd` → no output. — complete: `go vet` clean; `gofmt -l internal/sdd/sync_helpers.go internal/sdd/sync_helpers_test.go` → no output (the known `internal/review/rdd_helpers.go` finding is untouched).
- [x] 4.3 No new git spawns: `go build -o /tmp/gitexec.exe ./tools/gitexec/cmd/gitexec && /tmp/gitexec.exe -root .` → exit 0, 0 debt, 7 boundary entries, baseline matched (new test file adds no spawns). — complete: exit 0, `self-check 9 sites, 351 .go + 17 host targets, 0 debt, 7 boundary, baseline matched`.
- [x] 4.4 `sdd` domain delta: the change's only domain delta is `specs/sdd/spec.md` (`Sync Execution Contract`, 8 scenarios: 2 preserved + 6 new); it is the sync/archive target — no other capability touched. — complete: `openspec/changes/fix-sdd-sync-empty-spec/specs/` holds only `sdd/spec.md`.
- [x] 4.5 Scenario pins: T1 asserts `openspec/changes/{c}/` preserved after sync (scenario 1); 4.3 (zero git invocations) covers scenario 2 (no `sdd-sync` auto-commit). — complete: `TestSyncMixedFullSpecVerbatim` asserts the change dir survives; the writer adds no git surface (4.3).
- [x] 4.6 Resolve the design's D8 open question at review: `blocked` for full-spec delta + differing existing target is stricter than "MUST NOT be replaced"; accepted unless review objects. — complete: accepted at apply (subtest `differing_existing_target_blocks`); flagged for review in the PR body.
- [x] 4.7 Doc comment on `syncResolveDomainWrite` stating rules 1–3, the fixed blocked-message shape, and D8; no dead code left. — complete: doc comment present; the old unconditional `os.WriteFile` path is gone.

## Threat Matrix (verbatim from design; all rows Not applicable)

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | targets fixed (`spec.md`), no exec classification |
| Git repository selection | N/A | no git invocation |
| Commit state | N/A | sync never commits |
| Push state | N/A | no push surface |
| PR commands | N/A | no PR automation |

No matrix RED test is generated: every boundary is Not applicable — no routing/shell/subprocess/VCS/exec/process surface; the new write copies bytes already in the change tree and reuses `pathidentity.Contains`. The only process-level check is the existing `gitexec` guard (4.3), proving zero new git spawns.

## Coverage

Coverage: Sync executor without archive move → 1.2, 4.5; No commit created → 4.3, 4.5; Full-spec new domain copies verbatim → 1.2, 2.2, 2.5; Verbatim copy is scoped to absent targets → 3.4, 3.5; Content without requirement blocks fails closed → 1.4, 2.2, 3.2; Full-spec-only change remains skipped → 1.3; MODIFIED applies in place → 1.5; No empty write for any shape → 1.6, 3.1, 3.6, 3.7.

Budget note (`budget_deviations` precedent, `fix-false-green-guards`): measured size vs the sdd-tasks 530-word MUST budget — see orchestrator report; components below the mandated set were not deleted.
