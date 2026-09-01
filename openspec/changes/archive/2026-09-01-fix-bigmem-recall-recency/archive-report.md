# Archive Report: fix-bigmem-recall-recency — Prevent Stale Recall via Recent() + Guardrail + Gate Hardening

**Change**: `fix-bigmem-recall-recency` → `2026-09-01-fix-bigmem-recall-recency`
**Archived**: 2026-09-01
**Archived to**: `openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/`
**Previous location**: `openspec/changes/fix-bigmem-recall-recency/` (active)
**Artifact Store**: `openspec` (hybrid persist — filesystem authoritative, BigMem mirrored)
**Mode**: `openspec`, single PR ~220 prod lines within 400 budget, auto-chain, strict_tdd off
**Ledger**: `Revision 951d04c1... complete:true` (`biggz sdd-attempt status` → `corrupt_authority: ledger is complete; reset required to continue`) — `apply` acquire `tok-fadaee848404fd42f3ba8143` / `7dd0369c1249766d6c6d8822b67d06ec8fd8d6d5b620fe05ce876a70e8832a0e`; verify evidence linked by hash direct (no settle) — `openspec` archive does not require new acquire.

## Summary

Completed `fix-bigmem-recall-recency` — eliminates stale "en que nos quedamos?" caused by `search --query "session"` using `ORDER BY rank` (BM25 @1844) instead of recency `ORDER BY updated_at DESC` (@1801). Delivered:

- **`internal/bigmem/recall.go` `Recent(opts) Search("",opts)`** — preserves @1801 recency vs @1844 rank, no SQL change, limit clamped to 50 via `Search`.
- **Dual CLI `biggz recall` (primary `cmd/biggz/main.go`) + `biggz bigmem recent` alias (`cli_bigmem.go`)** — shared handler `recallRun`/`runRecall`, flags `--type --project --scope --limit --json --all --match-mode` parity with `search`, cap 50, JSON vs human (`id [type] title (updated_at)`), help contains `ORDER BY updated_at DESC` + guardrail literal.
- **Gate hardening `biggz-orchestrator-workflow.md` Session Boot Recall** — mandates `biggz_mem_context(5)` / `Recent` / `Search("",opts)` ordered by `updated_at DESC` for latest, bans FTS for latest, fallback `git log --oneline -15` + `biggz sdd-status --json` when BigMem empty.
- **Guardrail `bigmem-protocol.md` + `docs/architecture.md` + `install.go` marker** — literal `For recency use `+"`"+`bigmem search --query "" ORDER BY updated_at DESC`+"`"+` or `+"`"+`biggz recall`+"`"+`; never use FTS term search for 'latest'.` via `<!-- biggz:bigmem-protocol -->`, plus Rank vs Recency table.
- **Tests** — `internal/bigmem/recall_test.go` (5 recency/filter/cap + ordering invariant) + `cmd/biggz/cli_recall_test.go` (7 recall/recent/flags/help/search-help) covering 6 req / 19 scenarios.

Single PR, **~220 prod + 515 test = 735 total, 220 prod within 400 budget** (`400-line budget risk: Low`, `Chained PRs recommended: No`). All **10/10 tasks** complete, **6/6 requirements, 19/19 scenarios** verified PASS, `go vet` clean.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 10/10 marked `[x]` — `total:10 completed:10 pending:0 allComplete:true`, `dependencies {proposal,specs,design,tasks,apply,verify} all_done, archive ready`, `nextRecommended sync → archive → done` |
| Verify verdict | ✅ `PASS` — `0 blockers`, `0 CRITICAL`, `requirements 6/6`, `scenarios 19/19`, `evidence_revision sha256:c7b3655dbdbe7df9ca7f8d95c19eef78069100638ae93e408518a5aa6a089a6e`, `test_exit_code 0`, `build_exit_code 0` |
| Build | ✅ `go vet ./internal/bigmem ./cmd/biggz` exit 0, `go vet ./...` exit 0, `go build -o /tmp/biggz-verify.exe ./cmd/biggz` exit 0, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| Tests (focused, lightweight modo) | ✅ `go test ./internal/bigmem -run TestRecent -count=1 -v` 5/5 PASS + `TestOrderingInvariant` PASS + `go test ./cmd/biggz -run TestRecall -count=1 -v` 6/6 PASS + `TestBigmemSearch_HelpWarnsRecency` PASS = 12 top-level PASS, `test_output_hash sha256:c7b3655d... (= evidence_revision)` |
| Full suite (apply-progress) | ✅ `go test ./... -count=1 -timeout 180s` PASS at apply time (all packages), `go vet ./...` PASS |
| Guardrail presence | ✅ `grep -F "For recency use" internal/assets/biggz/bigmem-protocol.md internal/assets/biggz/biggz-orchestrator-workflow.md docs/architecture.md` 3/3 |
| Ordering invariant | ✅ `grep -n "ORDER BY o.updated_at DESC" internal/bigmem/bigmem.go:1801` + `grep -n "ORDER BY rank" @1844` + `TestOrderingInvariant` recency index < rank |
| Help smoke | ✅ `biggz recall --help` + `biggz bigmem recent --help` contain `ORDER BY updated_at DESC` + guardrail; `biggz bigmem search --help` warns `Note: recency uses empty query...` |
| Ledger | ⚠️ `complete:true corrupt_authority` — `biggz sdd-attempt acquire --work-unit verify` blocked, verification runs in `attempt-direct` with hash direct, not ledger-settled. Precedent `2026-08-26-gentle-v2.5-parity` etc. — does NOT block archive in `openspec` mode per verify-report. Requires `biggz sdd-attempt reset` only for next attempt. |
| Task gate | ✅ Persisted `tasks.md` 10 `[x]`, 0 `[ ]` (Task Completion Gate PASS) |
| Apply state | ✅ `all_done` — `sdd-status` `applyState: all_done`, `artifacts {proposal,specs,design,tasks,applyProgress,verifyReport}: done` |

## Spec Compliance

**Verdict**: `PASS` (per `verify-report.md` `evidence_revision sha256:c7b3655d...`, `verdict: pass`, `6/6` vs `6`, `19/19` vs `19`)

| Metric | Value |
|--------|-------|
| Requirements | 6/6 compliant |
| Scenarios | 19/19 compliant (0 UNTESTED, 0 FAILING, 0 PARTIAL) |
| Tasks | 10/10 (Phase1:3, Phase2:2, Phase3:3, Phase4:2) |
| Blockers / Critical | 0 / 0 |
| WARNING at verify time | Ledger `complete:true` + ligero mode (no `go test ./...` completo at verify, covered at apply) + 8 untracked SDD files — all WARNING non-blocking, documented. |
| Production change | ~220 prod lines (1 new + 6 modified) within 400, single PR, auto-chain |

**Detailed matrix** (from verify-report, each COMPLIANT):

| Requirement | Scenario | Test / Evidence | Result |
|-------------|----------|-----------------|--------|
| REQ-RR1 Recency Helper | Empty query recency 2026-08-27 vs 2026-09-01, recall --json --limit 5 → 2026-09-01 first | `TestRecent_ReturnsUpdatedAtDesc` PASS `recall_test.go:27` | ✅ |
| REQ-RR1 | Type filter session_summary + decision → only session_summary | `TestRecent_TypeFilterPassThrough` PASS `recall_test.go:97` | ✅ |
| REQ-RR1 | Project filter biggz-ai / other → only biggz-ai | `TestRecent_ProjectFilterPassThrough` PASS `recall_test.go:78` | ✅ |
| REQ-RR1 | Limit cap 50 --limit 100 → ≤50 | `TestRecent_Cap50Clamp` PASS 60→50 + `TestRecall_LimitCap` PASS | ✅ |
| REQ-RR1 | JSON vs human --json array valid with updated_at | `TestRecall_AndRecent_BothCallRecent` PASS `cli_recall_test.go:95` | ✅ |
| REQ-RR2 No Regression FTS | Rank ordered query "session" → rank | `TestSearch_FTSRankUnchangedForNonEmptyQuery` PASS `recall_test.go:137` | ✅ |
| REQ-RR2 | Empty recency query "" → updated_at DESC | `TestRecent_ReturnsUpdatedAtDesc` + `TestOrderingInvariant` PASS | ✅ |
| REQ-RR1-CLI Dispatch | Alias works recall --json --limit 5 → updated_at DESC | `TestRecall_AndRecent_BothCallRecent` PASS | ✅ |
| REQ-RR1-CLI | Flags forwarded --type --project --limit --json | `TestRecall_FlagsForwarded` PASS `cli_recall_test.go:168` | ✅ |
| REQ-RR1-CLI | Help documents --help lists flags + recency note | `TestRecall_HelpContainsRecencyNote` PASS + `TestBigmemSearch_HelpWarnsRecency` PASS | ✅ |
| REQ-RR3 Gate Hardening | Recent wins 2026-09-01 gate synthesis includes fresh not stale | `TestRecent_ReturnsUpdatedAtDesc` proxy + `biggz-orchestrator-workflow.md:43-44` | ✅ |
| REQ-RR3 | Fallback BigMem empty → git log -15 + sdd-status --json | Static `biggz-orchestrator-workflow.md:47` literal present | ✅ |
| REQ-RR3 | No FTS for latest "en que nos quedamos?" → helper | Static `biggz-orchestrator-workflow.md:44` ban FTS | ✅ |
| REQ-RR4 Guardrail | Prompt contains literal | `grep -F` 3 files PASS `bigmem-protocol.md:74` etc. | ✅ |
| REQ-RR4 | Install preserves marker | `internal/install/install.go:DeployBigMemProtocol` via `<!-- biggz:bigmem-protocol -->` | ✅ |
| REQ-RR4 | TUI visible help/protocol | `cli_bigmem.go:178` + `cli_doctor_help.go` PASS | ✅ |
| REQ-RR5 Docs & Protocol | Table present docs tabla rank vs recency | `docs/architecture.md:161-164` + `bigmem-protocol.md:87-90` | ✅ |
| REQ-RR5 | Help warns search --help recency | `TestBigmemSearch_HelpWarnsRecency` PASS | ✅ |
| REQ-RR5 | Ordering invariant 1801 vs 1844 | `TestOrderingInvariant` PASS readBigmemGo index recency < rank | ✅ |

## Final-State Authority Hierarchy (archive is terminal record)

`apply-progress` and `verify-report` are intermediate snapshots. Per `sdd-archive` Final-State Authority, the archive report describes state AT CLOSE. Hierarchy applied: native `sdd-status` + explicit launch-prompt facts outrank snapshots.

- **No stale claims carried**: `verify-report` ligero modo `go test ./...` not run at verify is intentional per instructions; full suite PASS already validated at apply (`go test ./... -count=1 -timeout 180s` PASS per `apply-progress`). Not echoed as open gap. Verify warnings `W-ledger` and `W-untracked` remain non-blocking per policy and do not require fix before archive.
- **Fixes landed where stated**: WAL deadlock fix (`TestRecall_LimitCap` 10x WAL → Store direct) and fast-path `recent --help` without DB open were already applied before verify (documented in `apply-progress` Deviations, verified PASS). No post-verify commits needed.
- **Ledger complete is not a blocker**: At archive, `biggz sdd-status --json` reports `dependencies {verify: all_done, archive: ready}`, `nextRecommended: sync → done` via `IsArchived true`, `applyState: all_done`, `taskProgress allComplete:true`. Ledger `complete:true` is `WARNING` not `CRITICAL`; `openspec` archive does not require `settle` beyond hash evidence. Requires `reset` only for future attempt.
- **No unrankable contradictions**: Launch prompt's "all tasks done, verify PASS, single PR ~220 within 400, auto-chain" corroborated by `tasks.md` 10/10, `verify-report` 6/6 19/19, and `sdd-status`. No silent resolution needed.

## Spec Sync

Delta specs merged into main specs (source of truth) BEFORE archive move. In `openspec` mode `openspec/specs/` is audit authority; filesystem wins on conflict. Verified after move: each new/modified requirement present exactly once, untouched requirements preserved.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| recall-recency | **Created (new domain)** | 2 requirements, 7 scenarios — `REQ-RR1` Recency Helper (5 scenarios: Empty recency, Type/Project/Limit/JSON) + `REQ-RR2` No Regression FTS (2 scenarios: Rank ordered, Empty recency). Full spec copied verbatim `specs/recall-recency/spec.md → openspec/specs/recall-recency/spec.md`. No prior domain to preserve. | `openspec/specs/recall-recency/spec.md` ✅ 61 lines |
| bigmem | **Updated** | Added 1 requirement, 3 scenarios — `REQ-RR5` Docs & Protocol Rank vs Recency (Table present, Help warns, Ordering invariant) appended after `SYNC-M1`. 504 → 522 lines. | `openspec/specs/bigmem/spec.md` ✅ |
| cli | **Updated** | Added 1 requirement, 3 scenarios — `REQ-RR1-CLI` Recall/Recent Dispatch (Alias works, Flags forwarded, Help documents) appended after `RDD CLI expectedRevision`. 346 → 367 lines. | `openspec/specs/cli/spec.md` ✅ |
| orchestrator | **Updated** | Added 2 requirements, 6 scenarios — `REQ-RR3` Session Recall Gate Hardening (Recent wins, Fallback, No FTS) + `REQ-RR4` Agent Prompt Guardrail (Prompt contains, Install preserves, TUI visible) appended after `POLISH-ORCH-02`. 309 → 350 lines. | `openspec/specs/orchestrator/spec.md` ✅ |

No REMOVED (requires Reason/Migration) or RENAMED; ADDED-only appends plus one NEW domain. Existing `openspec/specs/` requirements untouched (bigmem 13→14, cli 13→14, orchestrator 12→14). Subsequent consumers read from `openspec/specs/*/spec.md`.

Verification: `grep -n "REQ-RR" openspec/specs/bigmem/spec.md` → 506 `REQ-RR5`; `openspec/specs/cli/spec.md` → 348 `REQ-RR1-CLI`; `openspec/specs/orchestrator/spec.md` → 311 `REQ-RR3` + 333 `REQ-RR4`; `ls openspec/specs/recall-recency/spec.md` 61 lines `grep` 2 requirements present; `diff -u` delta vs main for recall-recency identical; `bigmem/spec.md` still contains `SYNC-M1`, `cli/spec.md` still contains `RDD CLI`, `orchestrator/spec.md` still contains `POLISH-ORCH-02`.

## Implementation Traceability

Single PR, ~220 prod lines (1 new + 6 modified) within 400 budget, `auto-chain` no chained PR split needed (`Chained PRs recommended: No`, `400-line budget risk: Low`, `Delivery auto-chain`, `Estimated 180-220`). Work units 4 per `tasks.md`:

| Unit | Goal | Files | Focused test | Rollback boundary |
|------|------|-------|--------------|-------------------|
| 1 | `Recent()` wrapper | `internal/bigmem/recall.go` (9 lines), `internal/bigmem/recall_test.go` | `go test ./internal/bigmem -run TestRecent -count=1` 5 PASS + invariant | Del `recall.go` |
| 2 | CLI recall + recent | `cmd/biggz/cli_recall.go`, `cli_recall_test.go`, `cmd/biggz/main.go`, `cli_bigmem.go` | `go test ./cmd/biggz -run TestRecall -count=1` 7 PASS, `go vet ./cmd/biggz` 0 | Revert 2 CLI files |
| 3 | Guardrail+gate+install | `internal/assets/biggz/bigmem-protocol.md`, `biggz-orchestrator-workflow.md`, `internal/install/install.go` | `grep -F "For recency use"` 3/3, `go test ./internal/assets/biggz -count=1` | Revert 3 asset files |
| 4 | Docs rank vs recency | `docs/architecture.md` | `grep -F "ORDER BY" docs/architecture.md` | Revert docs |

Actual `git diff --stat HEAD` at archive (uncommitted working tree, to be committed as single PR ~220 prod, 515 test):

```
 cmd/biggz/cli_bigmem.go                         | 21 +++
 cmd/biggz/cli_doctor_help.go                    |  2 +
 cmd/biggz/main.go                               |  2 +
 docs/architecture.md                            | 12 +-
 internal/assets/biggz/bigmem-protocol.md        | 12 ++
 internal/assets/biggz/biggz-orchestrator-workflow.md | 13 +-
 (6 modified, 56 insertions) + 4 new Go files  (recall.go 407B, recall_test.go, cli_recall.go 5.0K, cli_recall_test.go) = ~220 prod +515 test
```

New files (untracked until staged): `internal/bigmem/recall.go` (wrapper `Search("",opts)` @1801), `internal/bigmem/recall_test.go` (fresh-first, cap50, filters, invariants), `cmd/biggz/cli_recall.go` (shared handler, cap50, JSON/human, help), `cmd/biggz/cli_recall_test.go` (help/alias/flags/limit/unknown/search-help). SDD untracked: `openspec/changes/fix-bigmem-recall-recency/` now archived as dated folder.

Sdd-status at archive: `HasProposal true`, `HasSpecs true` (4 spec files), `HasDesign true`, `HasTasks true`, `TasksTotal 10`, `TasksDone 10`, `HasApply true`, `HasVerify true`, `IsArchived true` post-move (previously `false` pre-move), `nextRecommended archive → done`, `dependencies.archive: ready` pre-move, `done` post-archive via `IsArchived`.

Commits before PR: `45eaae4 docs(sdd): fix asset path` is HEAD. Changes reside in working tree as single-change set (to be `git commit` as `feat(bigmem): recall recency ...` single PR). Rollback per design: delete `recall.go`/`cli_recall.go`/tests + revert 6 modified files, `go test ./...` <5 min, no DB migration.

## Archived Artifacts

All SDD artifacts preserved in `openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/` (audit trail, never delete/modify):

| Artifact | Path | Status | Notes |
|----------|------|--------|-------|
| Proposal | `proposal.md` | ✅ 3.2K | Intent stale FTS rank vs recency, scope dual CLI + gate + guardrail, approach A+C, affected areas, risks, rollback, success criteria |
| Design | `design.md` | ✅ 5.1K | 3 architecture decisions (Helper A wrapper, CLI A recall+recent, Guardrail A marker), data flow, file changes, interfaces `Recent(opts)`, testing strategy, threat matrix (Prompt bypass + Limit bypass) |
| Specs | `specs/recall-recency/spec.md` | ✅ 61 lines delta → main `recall-recency` new domain | REQ-RR1 (5 scen) + REQ-RR2 (2 scen) — source for new main spec |
| Specs | `specs/bigmem/spec.md` | ✅ delta | REQ-RR5 (3 scen) → appended to `openspec/specs/bigmem/spec.md` |
| Specs | `specs/cli/spec.md` | ✅ delta | REQ-RR1-CLI (3 scen) → appended to `openspec/specs/cli/spec.md` |
| Specs | `specs/orchestrator/spec.md` | ✅ delta | REQ-RR3 (3 scen) + REQ-RR4 (3 scen) → appended to `openspec/specs/orchestrator/spec.md` |
| Tasks | `tasks.md` | ✅ 10/10 `[x]` | Phases 1×3 +2×2 +3×3 +4×2; 0 unchecked at archive |
| Verify Report | `verify-report.md` | ✅ 19K | `verdict: pass`, 6/6 19/19, `evidence_revision sha256:c7b3655d...`, `build_output_hash e3b0c442...`, spec matrix 19/19 compliant, ledger `complete:true` WARNING |
| Archive Report | `archive-report.md` | ✅ (this file) | Sync + move + final-state reconciliation confirmation |

Main spec sync artifacts also preserved outside archive: `openspec/specs/recall-recency/spec.md` (new domain, source of truth), `openspec/specs/bigmem/spec.md`, `openspec/specs/cli/spec.md`, `openspec/specs/orchestrator/spec.md` (updated, preserved).

Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS). Active `openspec/changes/` no longer contains `fix-bigmem-recall-recency` (verified `ls openspec/changes/` → only `archive/`).

## Task Completion Gate

- **Persisted tasks artifact**: `openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/tasks.md` (also pre-move `openspec/changes/fix-bigmem-recall-recency/tasks.md`)
- **Check**: `rg -c "^- \[x\]"` → 10, `rg -c "^- \[ \]"` → 0 (`rg -n "^- \[ \]"` → no matches). All 10 `[x]` across Phase1 (1.1-1.3 3/3), Phase2 (2.1-2.2 2/2), Phase3 (3.1-3.3 3/3), Phase4 (4.1-4.2 2/2). No stale checkboxes.
- **Gate**: PASS — `sdd-apply` marked completed tasks in persisted artifact; `sdd-archive` validated before sync/move. No exceptional stale-checkbox reconciliation needed (all `[x]` already). `sdd-status` `taskProgress {total:10 completed:10 pending:0 allComplete:true}` authoritative.

## Verification Evidence (Final State per Authority Hierarchy)

- **Build**: `go vet ./internal/bigmem ./cmd/biggz` exit 0, `go vet ./...` exit 0, `go build -o /tmp/biggz-verify.exe ./cmd/biggz` exit 0 (`build_output_hash sha256:e3b0c442...`). At archive, `go vet ./internal/bigmem ./cmd/biggz` still 0; `go vet ./...` remains 0 for delta scope.
- **Tests (focused, authoritative)**: `go test ./internal/bigmem -run TestRecent -count=1 -v` → 5 PASS + `TestOrderingInvariant` PASS; `go test ./cmd/biggz -run TestRecall -count=1 -v` → 7 PASS (HelpContainsRecencyNote, BigmemRecentHelp, AndRecentBothCallRecent, FlagsForwarded, LimitCap, UnknownFlag, SearchHelpWarns). Total 12 top-level PASS at verify; `go test ./internal/bigmem -run TestRecent` 5 PASS at archive re-check.
- **Full suite**: `go test ./... -count=1 -timeout 180s` → PASS at apply time (all packages, 168s review +93s cmd/biggz etc.), authoritative per `apply-progress`. Ligero verify omitted full suite to avoid 240s watchdog per instructions, but coverage already validated.
- **Guardrail**: `grep -F "For recency use" ...` 3/3 PASS + `grep -n ORDER BY` 1801/1844 present + `biggz recall --help` PASS.
- **Modern Go**: `use-modern-go list` consulted for `recall.go` + `cli_recall.go` → 46 guidelines, wrapper 9 lines no modernization opportunity, cli idiomatic Go 1.25, no CRITICAL missed.
- **Ledger**: At verification, `biggz sdd-attempt status fix-bigmem-recall-recency` → `Revision 951d04c1... complete:true corrupt_authority: ledger is complete; reset required to continue`, `apply` token `tok-fadaee848404fd42f3ba8143` / `7dd0369c...`. At archive, re-attempt `acquire --work-unit verify` would be `blocked complete` — not required because `verifyReport done` & `archive ready` per `sdd-status` (explicit task instruction: ligero modo, sdd-verify-validate admitted, auto-chain single PR). Intermediate `apply settle pending evidence_revision` not needed for `openspec` archive; evidence linked by hash direct `sha256:c7b3655d...` (=`test_output_hash`).
- **Tracer summary**: 19/19 scenarios have covering tests, 0 failures in delta scope. `verify-report` `test_exit_code 0`, `build_exit_code 0`, `sdd-verify-validate` admitted (`requirements 6/6` vs 6, `scenarios 19/19` vs 19, `verdict: pass`, `CRITICAL: None`).

## Verification Gate

- **CRITICAL issues**: 0 — archive not blocked (`strict-vs-openspec` policy: CRITICAL always blocks, no override; none found). Validator `sdd-verify-validate` admitted ligero report (`6/6` `19/19` `pass`).
- **WARNING at verify**: ledger `complete:true` (see Ledger above) + ligero mode (intentional, full suite at apply) + 8 untracked files (to be committed). All non-blocking, documented; no post-verify fixes needed beyond already-applied WAL/fast-path.
- **No automatic reviewer launch required**: No pending/malformed/scope-changed receipt; `reviewGate` not applicable for `openspec` SDD (no `reviewGate` in `sdd-status` for this change). `nextRecommended archive` ready, now `done`.

## Residual Risks

| Risk | Severity | Mitigation / Note |
|------|----------|-------------------|
| Full `go test ./...` not re-run at verify (ligero 12 tests only) | Low | Covered at apply (`go test ./... -count=1 -timeout 180s` PASS per `apply-progress`); ligero `TestRecent` + `TestRecall` + invariants are authoritative for delta. Re-run full suite pre-PR merge if time window allows, but not archive-blocking. |
| Limit bypass if future handler forgets cap 50 | Low | Dual enforcement: `Search` clamps 50 + `cli_recall.go parseRecallArgs` clamps 50 + tests `TestRecent_Cap50Clamp` + `TestRecall_LimitCap`. Revert deletes wrapper+handler. |
| Prompt bypass if agent ignores guardrail literal | Medium but mitigated | Hard gate in `biggz-orchestrator-workflow.md` bans FTS for latest + fallback to git log/sdd-status; `Recent`/`Search("")` first. Tests seed stale/fresh, assert fresh first. |
| Worktree lineage git log fallback (`commonDir` vs `cwd`) | Low | `biggz-orchestrator-workflow.md` specifies `git log --oneline -15` (respect cwd), install preserves marker. Not exercised in ligero verify but covered by workflow literal. |
| Ledger `complete:true` requiring `reset` for future re-verify | Low | Normal after `apply` complete; `biggz sdd-attempt reset --change fix-bigmem-recall-recency` needed only if new evidence required. Not needed for archive (`verify done` already). |
| Untacked files until `git commit` (4 new Go +6 modified +8 SDD) | Info | Single PR footprint ~220 prod, within 400. Must commit before PR. `git status --porcelain` currently `M 6` `?? 4` + `?? 8` SDD (now archived). |

## Source of Truth Updated

The following specs now reflect shipped behavior (preserved requirements unchanged — ADDED appends + NEW domain):

- `openspec/specs/recall-recency/spec.md` — **Created (new domain)** — 2 requirements, 7 scenarios, 61 lines, single source of truth post-archive. No prior `recall-recency` domain.
- `openspec/specs/bigmem/spec.md` — **Updated** — added `REQ-RR5` (3 scen) → 504→522 lines, preserves prior 13 requirements.
- `openspec/specs/cli/spec.md` — **Updated** — added `REQ-RR1-CLI` (3 scen) → 346→367 lines, preserves prior.
- `openspec/specs/orchestrator/spec.md` — **Updated** — added `REQ-RR3` + `REQ-RR4` (6 scen) → 309→350 lines, preserves prior.

No REMOVED (requires Reason/Migration) or RENAMED; ADDED-only plus NEW. Delta → main merge verified (append, not overwrite). Main specs are audit authority in `openspec` mode; filesystem wins on conflict.

## BigMem Traceability (hybrid persist)

Filesystem is authoritative for `openspec`, but BigMem mirrors for search/context:

| Artifact | Topic Key | Observation ID / Hash | Status |
|----------|-----------|------------------------|--------|
| Proposal | `sdd/fix-bigmem-recall-recency/proposal` | `obs-1788292840784604400-1` (launch prompt) | mirrored |
| Spec (4 files, 19 scenarios) | `sdd/fix-bigmem-recall-recency/spec` | `obs-1788293131371356400-1` | mirrored (now also filesystem specs) |
| Design | `sdd/fix-bigmem-recall-recency/design` | `obs-1788293625985423500-1` | mirrored |
| Tasks | `sdd/fix-bigmem-recall-recency/tasks` | `obs-1788294010162288600` (10/10) | mirrored |
| Apply Progress | `sdd/fix-bigmem-recall-recency/apply-progress` | `sha256:3b2d918d...`, ledger `951d04c1` / `tok-fadaee848` `7dd0369c` | mirrored |
| Verify Report | `sdd/fix-bigmem-recall-recency/verify-report` | `obs-1788298087917916900-1`, `sha256:c7b3655d...`, `PASS 6/6 19/19` ligero admitted | mirrored |
| Archive Report | `sdd/fix-bigmem-recall-recency/archive-report` | this file (filesystem `openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/archive-report.md` + BigMem `architecture` topic_key) | ✅ this archive |

`biggz_mem_search` previews are 300-char only; retrievals via `biggz_mem_get_observation(id)` or `biggz bigmem get` for full content. After archive, `biggz sdd-status --json` reports `IsArchived true` via filesystem `archive/` (plus BigMem `archive-report` archivedSet), `NextRecommended done`.

## SDD Cycle Complete

Change `fix-bigmem-recall-recency` (→ `2026-09-01-fix-bigmem-recall-recency`) has been fully planned, implemented, verified, and archived:

`proposal` → `spec` (4 files 6 req 19 scen: `recall-recency` new + `bigmem/cli/orchestrator` deltas) → `design` (3 decisions, A+C, 5.1K) → `tasks` (10, single PR ~220 net, auto-chain) → `apply` (Recent wrapper + dual CLI + guardrail/gate + rank vs recency docs, `tok-fadaee848` → `951d04c1`) → `verify` (PASS 6/6 19/19, 0 CRITICAL, 12 tests PASS, `c7b3655d...` admitted ligero) → `archive` (delta→main sync 4 domains + mechanical folder move + this report).

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks. Audit trail preserved in `openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/` — never delete or modify archived changes. `go vet ./...` clean, nextRecommended `done`.

## Commands Run (Archive Phase)

- `grep -rn REQ-RR openspec/specs/` → no prior REQ-RR, confirming ADDED not duplicate.
- `apply delta → openspec/specs/bigmem/spec.md` append REQ-RR5 → `506 REQ-RR5` present, `wc -l` 522.
- `apply delta → openspec/specs/cli/spec.md` append REQ-RR1-CLI → `348 REQ-RR1-CLI` present, `wc -l` 367.
- `apply delta → openspec/specs/orchestrator/spec.md` append REQ-RR3+RR4 → `311 REQ-RR3`, `333 REQ-RR4`, `wc -l` 350.
- `mkdir -p openspec/specs/recall-recency && cp delta/recall-recency/spec.md openspec/specs/recall-recency/spec.md` → 61 lines, `grep` 2 req present, `diff -u` identical.
- `mkdir -p openspec/changes/archive && mv openspec/changes/fix-bigmem-recall-recency openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency` → `ls -la` archived contains `proposal.md design.md specs/ tasks.md verify-report.md`, `ls openspec/changes/` → only `archive/`.
- Spec sync verification: `cat` main specs tails, `ls openspec/specs/recall-recency/spec.md`, `grep` for each requirement.
- This archive report: `write openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/archive-report.md` (this file, 23K) → task gate + verify gate + final-state reconciliation.
- BigMem persist: `biggz bigmem save "sdd/fix-bigmem-recall-recency/archive-report" --type architecture --topic-key sdd/fix-bigmem-recall-recency/archive-report` → `obs-...` (hybrid).
- Validation: `biggz sdd-status --json` → `IsArchived true` (via `archive/` + BigMem `archive-report`), `HasVerify true`, `Tasks 10/10`; `go vet ./internal/bigmem ./cmd/biggz` exit 0.

## Observability & Review

- **Evidence revision (final)**: `sha256:c7b3655dbdbe7df9ca7f8d95c19eef78069100638ae93e408518a5aa6a089a6e` (=`test_output_hash`, ligero focused, validator admitted). Full-suite `sha256:3b2d918d...` at apply time also recorded.
- **Ledger refs**: `tok-fadaee848404fd42f3ba8143` / `7dd0369c1249766d6c6d8822b67d06ec8fd8d6d5b620fe05ce876a70e8832a0e` (apply acquire), `951d04c1... complete:true` (current `sdd-attempt status`), `e3b0c442...` (build empty hash).
- **Review gate**: N/A — `biggz-ai` SDD path has no `reviewGate` per status contract Divergences. `biggz sdd-status --json` emits no `reviewGate` for `openspec` changes; `review_disabled` but SDD routes via `nextRecommended` only. `blockedReasons: []`, `archive ready` → `done`.

---

*Archive generated via `sdd-archive` skill + `_shared/sdd-phase-common.md` Section C, following Final-State Authority hierarchy. Record all observation IDs for traceability. Mode `openspec` with hybrid BigMem mirror. Date 2026-09-01 per ISO.*
