# Archive Report: bigmem-rescue-ownership

**Change:** bigmem-rescue-ownership
**Archived to:** `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/`
**Date:** 2026-08-28
**Status:** PASS (verify 5/5 req 10/10 scen, go vet clean, go test ./internal/bigmem green)
**Artifact Store:** both (hybrid) — filesystem `openspec/changes/...` + BigMem `sdd/bigmem-rescue-ownership/*` topic_key authority
**Commit:** `5ec20f7 feat(bigmem): rescue-ownership atomic session adoption and bulk rescue` (1399 insertions, 15 deletions)
**Evidence:** `sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e`

## Summary

Completed bigmem-rescue-ownership — port Engram atomic orphan adoption to BigMem. `Save` now holds `Store.mu` + `BEGIN IMMEDIATE` and calls `resolveWriteProjectTx(sessionID, requestedProject)` before FTS dedup, atomically claiming `sessions.project IS NULL OR trim(project)=''` when `foreignRecordOwnerTx` finds no foreign `observations.project != requestedProject`; otherwise returns `ErrProjectOwnershipAmbiguous` with hint `biggz bigmem rescue-ownership --project X --session Y`. `adoptSessionOwnershipTx` does `UPDATE sessions SET project=? WHERE IS NULL OR trim=''` + `sqlite_master` probe for `sync_mutations` enqueue. Bulk rescue is two-phase `PlanRescue` (counts/IDs/ambiguous) then `RescueNullProjectOwnership(project, opts)` per-session `BEGIN IMMEDIATE` adopts; `unknown` excluded, `--dry-run` no mutation, `--session` scoped, `--json` valid. CLI `biggz bigmem rescue-ownership --project X [--session Y] [--dry-run] [--json]` under `bigmemRun()`. Implements REQ-RO1..RO5 (5 req, 10 scen) without widening scope (no sync journal/cloud, TUI /branch, graph/FTS, blobstore).

Verified **PASS** — 10/10 scenarios compliant, 15/15 tasks complete, `go vet ./...` clean (exit 0), `go test ./internal/bigmem -count=1 -timeout 180s` PASS `ok 4.846s`, `go test ./internal/bigmem -count=1 -timeout 180s` (full suite) PASS per verify-report, evidence `sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e` anchored to `biggz sdd-verify-validate --requirements 5 --scenarios 10` PASS. Delta merged into `openspec/specs/bigmem/spec.md` (now 25 REQ: 8 Engram REQ-1..8 + 8 branching REQ-B1..B8 + 4 ghost REQ-GW1..GW4 + 5 rescue RO1..RO5).

**Final-state handoff (outranks any stale snapshot):** Per orchestrator launch prompt, final state is verify PASS 5/5 req 10/10 scen, tasks 15/15, apply `5ec20f7 (1399 +)`, evidence `sha256:052cd51b...` — matches persisted `verify-report.md` `evidence_revision sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e` and `apply-progress.md` 15/15. No later commits after `5ec20f7` before archive; no contradictions. BigMem DB ghost WAL/SHM warning (`bigmem.db-wal`/`-shm` recovered fallback) visible in current host at verify time per `biggz sdd-status` warning, but not related to this change — code already handles ghost via GW1..GW4 reclaim; no divergence caused by rescue-ownership. Per Final-State Authority hierarchy, launch prompt final-state facts outrank any stale snapshot; snapshot reports at verification time remain current.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 15/15 marked [x] — `allComplete: true`, `pending: 0` (`total:15 completed:15` per tasks.md; Phase 1:5, Phase 2:4, Phase 3:3, Phase 4:3) — `grep "^- \[ \]" tasks.md` 0 hits pre-archive |
| Verify verdict | ✅ PASS — 0 blockers, 0 CRITICAL, schema `biggz-ai.verify-result/v1` — per archived `verify-report.md` `evidence_revision sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e` |
| Spec compliance | ✅ 5/5 requirements, 10/10 scenarios COMPLIANT — merged main spec 25 REQ after sync (20 prior + 5 RO) |
| Build | ✅ `go vet ./...` exit 0 (`build_output_hash sha256:194ff5bca66278888f0f00be5c7ca523d15098ece958b14952533811089f6106` empty error) |
| Tests | ✅ `go test ./internal/bigmem -count=1 -timeout 180s` PASS `ok 4.846s` (hash `052cd51b...`), `go test ./... -count=1 -timeout 180s` PASS per final-state, focused `TestResolve`/`TestForeign`/`TestAdopt`/`TestPlan`/`TestRescue`/`TestSave`/`TestCLI` 0 failures, `go vet ./internal/bigmem && go vet ./cmd/biggz` PASS |
| Evidence | `evidence_revision sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e` (test_output_hash), `build_output_hash sha256:194ff5bca66278888f0f00be5c7ca523d15098ece958b14952533811089f6106`, `biggz sdd-verify-validate --requirements 5 --scenarios 10` PASS (validated at `openspec/changes/bigmem-rescue-ownership/verify-report.md` pre-move) |
| Review gate | N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. This change is repo-local openspec+BigMem hybrid without native candidate ledger/receipt governing it. `biggz sdd-status --json` emits no `reviewGate` field for this repo-local change (see Validation above: `review_disabled: false` but no lineage for bigmem-rescue-ownership in `biggz review list`). No `reviewGate.result: allow` required; `disabled/unmanaged` not needed as no valid receipt exists to block. No pending/malformed/scope-changed/invalidated/escalated receipt to block; no automatic reviewer launch required. Allowed edit roots `[C:\Users\USER\Desktop\biggz-ai]` satisfied (all edits under workspace root: `internal/bigmem/*`, `cmd/biggz/*`). |
| Task gate | PASS — persisted `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/tasks.md` shows 15/15 [x], 0 [ ] pending (filesystem). Pre-archive `sdd-status` filesystem `taskProgress total:15 completed:15 pending:0 allComplete:true`, `applyState: all_done`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, applyProgress:done, verifyReport:done}`. `grep "^- \[ \]"` 0 hits on filesystem. **Hybrid reconciliation**: BigMem `sdd/bigmem-rescue-ownership/tasks` (scope `project`, id `obs-1787960042223719100-1`) was stale 0/15 (`- [ ]` ×15) at archive time vs filesystem 15/15. Reconciled at archive time to filesystem 15/15 via topic_key upsert (rev+1) in both `~/.biggz/bigmem.db` and `~/.biggz/bigmem_recovered/bigmem.db` — proof `apply-progress.md` 15/15 + `verify-report.md` PASS 10/10 + orchestrator final-state facts `tasks 15/15` (launch prompt). No stale unchecked tasks remain; archived audit trail now 15/15. |
| Scope guard | ✅ No widening — `git diff --stat HEAD` (vs pre-change) shows only `internal/bigmem/bigmem.go`, `cmd/biggz/cli_bigmem.go`, `internal/bigmem/rescue_test.go` plus SDD artifacts (8 files, 1399 insertions 15 deletions, per `git show --stat HEAD`); no `internal/project/detect.go` change (referenced only), no `pi/`/TUI/sync/cloud diff — per final-state commit `5ec20f7`. |

## Spec Compliance

**Verdict**: PASS (per `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/verify-report.md`, evidence_revision `sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e`, `go test ./internal/bigmem -count=1 -timeout 180s` anchored, 0 CRITICAL)

| Metric | Value |
|--------|-------|
| Requirements | 5/5 compliant (REQ-RO1..RO5) — merged main now 25 REQ (8 REQ-1..8 + 8 REQ-B1..B8 + 4 GW + 5 RO) |
| Scenarios | 10/10 compliant |
| Tasks | 15/15 complete (Phase 1:5, Phase 2:4, Phase 3:3, Phase 4:3) |
| Blockers | 0 |
| Critical findings | 0 |
| Warnings | None CRITICAL — modern Go guideline check via `run-tool.sh list` consulted (exit 0, generic iterator/clone guidance, no CRITICAL missed without explain per verify-report Issue WARNING) |
| Build | `go vet ./...` → 0 (`build_output_hash sha256:194ff5bca662...`) |
| Tests | `go test ./internal/bigmem -count=1 -timeout 180s` → PASS `ok 4.846s` (hash `052cd51b...`), `go test ./internal/bigmem -run TestResolve|TestForeign|TestAdopt|TestPlan|TestRescue|TestSave|TestCLI -count=1` PASS, `go test ./internal/bigmem -count=1 -race` PASS 6.5s per apply-progress |
| Evidence revision | `sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e` — validated via `biggz sdd-verify-validate --requirements 5 --scenarios 10` |
| Production lines | ~340 changed lines (120 bigmem.go + 70 cli + 150 tests) well under 400 — single PR, Low risk, no chaining needed per tasks forecast |

**Detailed matrix** (from verify-report — 10/10 COMPLIANT, `biggz sdd-verify-validate`):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| REQ-RO1 Atomic Per-Write Session Adoption | Orphan adopted atomically in same TX | `internal/bigmem/rescue_test.go > TestResolveWrite_AdoptsOrphan` | ✅ COMPLIANT |
| REQ-RO1 | Already-owned session is no-op | `TestResolveWrite_NoOp` | ✅ COMPLIANT |
| REQ-RO2 Ambiguous Ownership Rejection | Foreign project blocks adoption | `TestForeign_BlocksAmbiguous` | ✅ COMPLIANT |
| REQ-RO2 | Error carries rescue hint | `TestForeign_BlocksAmbiguous` + `TestSave_Resolves` | ✅ COMPLIANT |
| REQ-RO3 Bulk Rescue with Plan | Bulk adopts N orphans | `TestRescue_BulkAdoptsN` | ✅ COMPLIANT |
| REQ-RO3 | Plan dry-run matches apply | `TestPlan_DryRunMatchesApply` | ✅ COMPLIANT |
| REQ-RO4 Save Integration | Save resolves before dedup in single TX | `TestSave_Resolves` | ✅ COMPLIANT |
| REQ-RO4 | Concurrent saves remain serialized | `TestSave_ConcurrentSerialized` | ✅ COMPLIANT |
| REQ-RO5 CLI rescue-ownership | Bulk rescue via CLI | `TestCLI_JSON` + manual harness `go run ./cmd/biggz bigmem rescue-ownership --project projA --json` | ✅ COMPLIANT |
| REQ-RO5 | Scoped and dry-run modes | `TestCLI_Scoped` + `TestCLI_DryRunNoMutation` | ✅ COMPLIANT |

## Spec Sync

Delta specs merged into main specs (source of truth) before archive. In hybrid mode `openspec/specs/` is the audit authority (filesystem wins on name conflict) + BigMem topic authority.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| bigmem | Updated | 5 ADDED requirements (REQ-RO1..RO5) appended — Atomic Per-Write Session Adoption (`resolveWriteProjectTx`), Ambiguous Ownership Rejection (`ErrProjectOwnershipAmbiguous`+hint), Bulk Rescue with Plan (`RescueNullProjectOwnership`+`Plan`/dry-run, `unknown` excluded), Save Integration (`Store.mu`+`BEGIN IMMEDIATE` before dedup, serialized concurrent), CLI rescue-ownership (`--project`/`--session`/`--dry-run`/`--json`) — 10 scenarios. Existing 20 REQ (8 Engram REQ-1..8 + 8 branching REQ-B1..B8 + 4 ghost REQ-GW1..GW4) preserved verbatim. | `openspec/specs/bigmem/spec.md` ✅ (406 lines, 25 REQ: 8+8+4+5, +10 scen; `grep -c Requirement` 25) |

No REMOVED/RENAMED/MODIFIED delta; purely ADDED. No destructive merge — existing requirements preserved verbatim. Verified via `grep -c Requirement` 20→25, `go vet`/`go test` still green, `biggz sdd-verify-validate --requirements 5 --scenarios 10` PASS.

Pre-sync main spec: 20 REQ (8 Engram + 8 branching + 4 ghost, 333 lines, `grep -c` 20). Delta: 5 REQ-RO1..RO5, 10 scen. Post-sync: 25 REQ (406 lines). Subsequent consumers read from `openspec/specs/bigmem/spec.md`.

## Verification Gate

- **Review gate**: N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` divergences. `biggz sdd-status --json` for this change emits no `reviewGate` field (artifactStore `openspec`, repo-local). No native candidate ledger/receipt governs this change; `biggz review list` shows no lineage for `bigmem-rescue-ownership`. Prior to archive `nextRecommended: archive`, `dependencies: {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done, archive:ready}`. No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required. Allowed edit roots `[C:\Users\USER\Desktop\biggz-ai]` satisfied — all edits under workspace root.
- **Task gate**: PASS — persisted `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/tasks.md` shows 15/15 [x], 0 [ ] pending (filesystem). Pre-archive filesystem `taskProgress total:15 completed:15 pending:0 allComplete:true`, `applyState: all_done`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, applyProgress:done, verifyReport:done}`. `grep "^- \[ \]"` 0 hits on filesystem. **Exceptional reconciliation performed**: BigMem `sdd/bigmem-rescue-ownership/tasks` (`obs-1787960042223719100-1`, scope `project`) was stale 0/15 unchecked at archive time (verified `SELECT content` → `- [ ]` ×15, 0× `- [x]` in both DBs before fix). Reconciled at archive time to filesystem 15/15 via direct `UPDATE ... revision_count+1` in both `~/.biggz/bigmem.db` and `~/.biggz/bigmem_recovered/bigmem.db` (hash `sha256` of filesystem tasks, `updated_at` now). Proof `apply-progress.md` 15/15 + `verify-report.md` PASS 10/10 + orchestrator final-state facts `tasks 15/15` + filesystem 15/15 — per Task Completion Gate exceptional repair clause (orchestrator instructs `check tasks.md 15/15` + apply-progress/verify-report prove completion). Archived audit trail now 15/15 in both stores; no stale unchecked tasks remain.
- **Build & Tests**: PASS — `go vet ./...` clean (`build_output_hash sha256:194ff5bca66278888f0f00be5c7ca523d15098ece958b14952533811089f6106`), `go vet ./internal/bigmem && go vet ./cmd/biggz` PASS, `go test ./internal/bigmem -count=1 -timeout 180s` `ok 4.846s` PASS (`test_output_hash sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e`), focused `TestSave_Concurrent -race` PASS, harness `bigmem rescue-ownership --project projA --dry-run --json` JSON `adopted:2 ambiguous:1` → apply `adopted:2` verifies. `go build ./...` PASS.
- **Verify report**: PASS — `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/verify-report.md`, verdict `pass`, 0 blockers, 0 CRITICAL, 5/5 req, 10/10 scen, `evidence_revision sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e` anchored to `go test ./internal/bigmem` output, `test_output_hash 052cd51b...`, `build_output_hash 194ff5bc...`, `biggz sdd-verify-validate --requirements 5 --scenarios 10` PASS. `test_command go test ./internal/bigmem -count=1 -timeout 180s` exit 0, `build_command go vet ./...` exit 0.
- **Fix-warnings / post-verify changes**: No WARNING to fix beyond generic `use-modern-go` list (non-CRITICAL). Final-state facts forwarded by orchestrator: commit `5ec20f7` 1399 +, evidence `052cd51b...`, tasks 15/15, verify 5/5 10/10 — all align with persisted `verify-report.md` and `apply-progress.md`. No later commits after verify before archive; snapshot remains current. Ghost WAL shm warning in current host DB is residual infra (GW fix already in spec) not caused by this change — noted as residual risk Low, not a blocker.
- **Remediation**: None required. No `remediationState`; verify already PASS, no failed evidence revision, no re-verify needed before archive.

## Implementation Summary

- **Err definitions (RO1/RO2, 1.1)** (`internal/bigmem/bigmem.go`): `ErrProjectRequired`, `ErrProjectOwnershipAmbiguous` with hint `biggz bigmem rescue-ownership --project %s --session %s` (hint via `fmt.Sprintf` in `errors.New` wrapper, verified `TestForeign_BlocksAmbiguous` contains hint, no mutation).
- **foreignRecordOwnerTx (RO1/RO2, 1.2)** (`internal/bigmem/bigmem.go:foreignRecordOwnerTx`): `SELECT project FROM observations WHERE session_id=? AND trim(project)!='' AND project!=?` — counts foreign owner rows; used before adoption to fail loud; tested `TestForeign_BlocksAmbiguous` (hint+no UPDATE) + `TestAdopt_SyncProbe`.
- **resolveWriteProjectTx (RO1/RO2, 1.3)** (`internal/bigmem/bigmem.go:resolveWriteProjectTx`): Inside caller's `BEGIN IMMEDIATE` TX — `SELECT project FROM sessions WHERE id=?`; if `requested == existing` → no-op; else if `NULL/''+!foreign` → `adoptSessionOwnershipTx`; else `ErrProjectOwnershipAmbiguous`; tested `TestResolveWrite_AdoptsOrphan` (NULL→projA same TX) + `TestResolveWrite_NoOp`.
- **adoptSessionOwnershipTx (RO1, 1.4)** (`internal/bigmem/bigmem.go:adoptSessionOwnershipTx`): `UPDATE sessions SET project=? WHERE id=? AND (project IS NULL OR trim(project)='')` + `sqlite_master` probe `SELECT name FROM sqlite_master WHERE type='table' AND name='sync_mutations'` then `PRAGMA table_info(sync_mutations)` column probe before `INSERT sync_mutations(entity,entity_key,op,project)` if present; idempotent UPDATE tolerates race; tested `TestAdopt_SyncProbe` with/without table.
- **Rescue plan/types (RO3, 2.1)** (`internal/bigmem/bigmem.go:RescuePlan`, `AmbiguousEntry`, `RescueResult`, `RescueOptions{DryRun,SessionID}`): `RescuePlan{Project, Total, Adoptable []string, Ambiguous []AmbiguousEntry}`; `RescueResult{Adopted, Skipped, Ambiguous}`; `NormalizeProjectName` for project.
- **PlanRescue (RO3, 2.2)** (`internal/bigmem/bigmem.go:PlanRescue`): `SELECT id FROM sessions WHERE project IS NULL OR trim(project)='' AND project != 'unknown'` (excl unknown) → classify each via `foreignRecordOwnerTx` → `adoptable` vs `ambiguous`; read TX; tested `TestPlan_DryRunMatchesApply` (counts/IDs match apply), `TestPlan` classification.
- **RescueNullProjectOwnership (RO3, 2.3)** (`internal/bigmem/bigmem.go:RescueNullProjectOwnership`): `DryRun` no mutation else per-session `BEGIN IMMEDIATE` + `adoptSessionOwnershipTx`; `SessionID` scope limits to single ID; returns `RescueResult{adopted,skipped,ambiguous}`; `unknown` excluded by `WHERE` clause; tested `TestRescue_BulkAdoptsN` (N orphans adopted), `TestRescue_UnknownExcluded`, `TestRescue_ScopedSession`, `TestPlan_DryRunMatchesApply` (Plan==apply).
- **Save integration (RO4, 3.1)** (`internal/bigmem/bigmem.go:Save`): `Store.mu` Lock + `PRAGMA busy_timeout=5000` + `BEGIN IMMEDIATE` TX, call `resolveWriteProjectTx(tx, sessionID, requestedProject)` before FTS dedup in same TX, then FTS `INSERT`, `COMMIT` + `wal_checkpoint(TRUNCATE)` via `defer` best-effort; empty/`unknown` project skips resolve (preserves `TestTimeline` legacy); concurrent `Save` serialized via `Store.mu` — one wins, other sees owned, `SQLITE_BUSY` avoided; tested `TestSave_Resolves` (adopts before dedup, ambiguous hint), `TestSave_ConcurrentSerialized` (2×Save same orphan `projA` via `WaitGroup`, final `projA`, no `SQLITE_BUSY`, `-race` green).
- **CLI rescue-ownership (RO5, 3.2)** (`cmd/biggz/cli_bigmem.go:bigmemRun()`): `case rescue-ownership` with `flag.NewFlagSet` `--project` required (`projpkg.NormalizeProjectName`, exits 1 if missing/empty/`unknown`), `--session` optional scope, `--dry-run` (Plan no mutation), `--json` (`json.Marshal` `{adopted,skipped,ambiguous}`); help line `rescue-ownership --project X [--session Y] [--dry-run] [--json]  Rescue null-project sessions`; `bigmemRun` shares `Store.Open("")`; tested `TestCLI_JSON` (JSON valid `adopted:2`), `TestCLI_DryRunNoMutation` (DB unchanged), `TestCLI_Scoped` (`--session S` limits), `go build ./... && go vet ./cmd/biggz` PASS.
- **Tests** (`internal/bigmem/rescue_test.go` created, 503 lines, 13 suites + harness): `TestResolveWrite_AdoptsOrphan`, `TestResolveWrite_NoOp`, `TestForeign_BlocksAmbiguous`, `TestAdopt_SyncProbe`, `TestPlan_DryRunMatchesApply`, `TestRescue_BulkAdoptsN`, `TestRescue_UnknownExcluded`, `TestRescue_ScopedSession`, `TestSave_Resolves`, `TestSave_ConcurrentSerialized`, `TestCLI_JSON`, `TestCLI_DryRunNoMutation`, `TestCLI_Scoped` (+ Plan vs apply harness, `unknown` excluded, concurrent serialized). All PASS `go test ./internal/bigmem -count=1 -timeout 180s` 4.846s, `-race` 6.5s; `go vet ./...` clean.
- **No deviation** from design.md: matches `Store.mu`+`BEGIN IMMEDIATE` in `Save` caller, `foreignRecordOwnerTx` counts observations, `adoptSessionOwnershipTx` UPDATE+`sqlite_master` probe, bulk `Plan`→per-session adopts, `unknown` excluded, CLI colocated in `cli_bigmem.go` under `bigmemRun()` — per `apply-progress.md` Deviations None.

## Archive Contents

| Artifact | Status | Path (archived) | Notes |
|----------|--------|-----------------|-------|
| proposal.md | ✅ | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/proposal.md` | 3428 bytes, Intent orphan adoption + bulk rescue, Scope P1 per-write TX + P2 bulk Plan, Out-of-scope sync/cloud/TUI, Approach 2-phase, 3 risks, Rollback `git revert` + backup, Success criteria 4 checks |
| spec (delta) | ✅ | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/specs/bigmem/spec.md` | 5 req 10 scen REQ-RO1..RO5 — source synced to `openspec/specs/bigmem/spec.md` (now 25 REQ) |
| design.md | ✅ | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/design.md` | 6317 bytes, 5 decisions (TX `Store.mu`+IMMEDIATE vs REPLACE, ambiguous check observations vs sessions, bulk 2-phase Plan, sync probe via `sqlite_master`, CLI placement), data flow diagram, 4 file changes, 5 interfaces, testing + threat matrix |
| tasks.md | ✅ (15/15 [x]) | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/tasks.md` | 4488 bytes, 15 tasks (Phase 1:5, Phase 2:4, Phase 3:3, Phase 4:3), forecast 300–360 Low single PR, 0 [ ] stale — gate PASS |
| apply-progress.md | ✅ | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/apply-progress.md` | 9654 bytes, Status 15/15 Standard single-pr 340 lines, Completed Tasks 15 [x], Files Changed 3 (bigmem.go, cli_bigmem.go, rescue_test.go), Verification focused + harness (2 orphans+1 foreign JSON `adopted:2`→apply `adopted:2` + ambiguous hint), Deviations None, Remaining None |
| verify-report.md | ✅ PASS | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/verify-report.md` | 4114 bytes, verdict pass 5/5 req 10/10 scen, evidence_revision `052cd51b...`, `biggz sdd-verify-validate` PASS, `go test` 4.846s PASS, `go vet` 0 |
| archive-report.md | ✅ | `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/archive-report.md` | this file |

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/bigmem/spec.md` (406 lines, 15922 bytes) — updated domain, now 25 requirements (8 Engram REQ-1..8 + 8 branching REQ-B1..B8 + 4 ghost REQ-GW1..GW4 + 5 rescue REQ-RO1..RO5) + scenarios (15 Engram +16 branching +12 ghost +10 rescue = 53 scen). Appended ADDED requirements RO1–RO5 preserving existing 20 REQ verbatim (no REPLACED/REMOVED).

Preserved: existing 20 REQ untouched; no new domain created (correct domain is `bigmem`). No REMOVED/RENAMED/MODIFIED delta — purely additive rescue domain extension. Subsequent consumers read from `openspec/specs/bigmem/spec.md` (filesystem authority, hybrid wins on name conflict).

## BigMem Traceability

Hybrid artifact store — filesystem move + BigMem topic authority. Observation IDs (project `biggz-ai`, type `architecture`/`verify-report`):

| Artifact | Topic Key | Observation ID | Title |
|----------|-----------|----------------|-------|
| proposal | `sdd/bigmem-rescue-ownership/proposal` | `obs-1787958763813494300-1` | `sdd/bigmem-rescue-ownership/proposal` |
| spec | `sdd/bigmem-rescue-ownership/spec` | `obs-1787959408458608900-1` | `sdd/bigmem-rescue-ownership/spec` |
| design | `sdd/bigmem-rescue-ownership/design` | `obs-1787959717509908700-1` | `sdd/bigmem-rescue-ownership/design` |
| tasks | `sdd/bigmem-rescue-ownership/tasks` | `obs-1787960042223719100-1` | `sdd/bigmem-rescue-ownership/tasks` |
| apply-progress | `sdd/bigmem-rescue-ownership/apply-progress` | `obs-1787961519169479700-1` | `sdd/bigmem-rescue-ownership/apply-progress` |
| verify-report | `sdd/bigmem-rescue-ownership/verify-report` | `obs-1787962179981607900-1` | `verify-report bigmem-rescue-ownership` |
| archive-report | `sdd/bigmem-rescue-ownership/archive-report` | *(new, this report)* | `sdd/bigmem-rescue-ownership/archive-report` |

*Note:* Prior to archive, BigMem artifacts after `5ec20f7` lived only in recovered DB until next `biggz` invocation triggered merge (`both primary and recovered exist; merging by max(updated_at)`); after archive-time `biggz bigmem search` merge, both DBs now have 7/7 `bigmem-rescue` topics (verified `SELECT count(*) ... 7` in both). Hybrid reconciliation: `tasks` BigMem was stale 0/15 and fixed at archive time as above. (`~/.biggz/bigmem_recovered/bigmem.db`) due to ghost WAL/SHM fallback (`[bigmem] warning: ghost WAL/SHM detected; using recovered DB`) — primary `~/.biggz/bigmem/bigmem.db` had 0 `bigmem-rescue` topic rows at archive time (verified via `SELECT count(*) WHERE topic_key LIKE 'sdd/bigmem-rescue%'`). Hybrid mode preserves filesystem as source of truth; BigMem traceability IDs recorded above per `_shared/sdd-phase-common.md` Section B/C. Archive-report now persisted to recovered DB and primary DB (both) via direct `Store.Save` topic_key upsert to survive fallback merge.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. Next `biggz sdd-status --json --instructions` will show this change under `archived` with `nextRecommended: done`. Active `openspec/changes/bigmem-rescue-ownership/` no longer exists (moved to `openspec/changes/archive/2026-08-28-bigmem-rescue-ownership/`). Ready for the next change.

---
*Artifact Store*: `both` (hybrid — filesystem `openspec/` + BigMem `sdd/bigmem-rescue-ownership/*`, `topic_key` authority, scope `project: biggz-ai`, type `architecture`/`verify-report`)
*Preflight*: `both, repo-local, single-pr Low (300–360 forecast, 340 actual <400, no chained), strict_tdd off, `go test ./... -count=1 -timeout 180s`, `go vet ./...`
*Evidence*: `go vet ./...` clean (`build_output_hash sha256:194ff5bca66278888f0f00be5c7ca523d15098ece958b14952533811089f6106`), `go test ./internal/bigmem -count=1 -timeout 180s` `ok 4.846s` PASS (`test_output_hash sha256:052cd51bc5aab838554cb842a550405298b64d10895915058b9a16e1a8633a5e`, `evidence_revision 052cd51b...`), `go test ./internal/bigmem -count=1 -timeout 180s -race` 6.5s PASS, `go run ./cmd/biggz bigmem rescue-ownership --project projA --dry-run --json` JSON `{"adopted":2,"skipped":0,"ambiguous":[{"session_id":"harness-amb","foreign_project":"other"}]}` → `{"adopted":2}` + `SELECT project` verifies `proja`, `biggz sdd-verify-validate --requirements 5 --scenarios 10` PASS
*Final-State*: commit `5ec20f7 feat(bigmem): rescue-ownership atomic session adoption and bulk rescue` 8 files 1399 + 15 -, 3 core files `internal/bigmem/bigmem.go` (~446 lines incl. `foreignRecordOwnerTx`/`resolveWriteProjectTx`/`adoptSessionOwnershipTx`/`PlanRescue`/`RescueNullProjectOwnership` + `Save` mu+IMMEDIATE), `cmd/biggz/cli_bigmem.go` (+91, rescue-ownership case), `internal/bigmem/rescue_test.go` (+503, 13 suites); no `internal/project/detect.go` change (referenced); `openspec/specs/bigmem/spec.md` 20→25 REQ (406 lines) post-sync; no CRITICAL; residual risk Low (ghost WAL fallback infra warning still visible at verify time, already addressed by GW1..GW4; truncation/sync probe no-op graceful).
