# Archive Report: single-source-session-guard

**Change**: `single-source-session-guard` → `2026-09-07-single-source-session-guard`
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-single-source-session-guard/`
**Previous location**: `openspec/changes/single-source-session-guard/` (active)
**Artifact Store**: `openspec` — `openspec/changes/single-source-session-guard` → `openspec/changes/archive/2026-09-07-single-source-session-guard/` + `openspec/specs/{session-guard-ownership,pi-deploy-list}/spec.md` source of truth
**Testing**: `node --test internal/assets/pi/*.test.mjs` + `node --test internal/assets/pi/biggz-session-stop.test.mjs` (focused contract) + `go test ./internal/install/steps/ -run TestPi` + `go vet` + `go build ./...` + `node --check` + `rg` single-definition / acyclicity probes

## Summary

Completed `single-source-session-guard` — the `session_stop` summary guard (`checkSessionStop` + `SESSION_STOP_TIMEOUT_MS` + `_setSessionStopExecForTest`) lived in `biggz-tool-interception.js` while `biggz-extension-api.js` imported it, leaving two owners and divergence risk (a prior change had already unified a duplicate once). The fix moves the block verbatim into one shared ES module that both extensions resolve to, with zero behavior change.

- **`internal/assets/pi/biggz-session-guard.js` (56 lines, new)** — verbatim move of lines 8–64 from `biggz-tool-interception.js` (including `import { execFileSync }` and APPLY-DECIDE comments, modulo one trailing blank line). Sole owner of all 3 symbols; only import is `node:child_process`, so a cycle is impossible by construction.
- **`internal/assets/pi/biggz-tool-interception.js` (−57/+1)** — moved block deleted; compat re-export from the guard so old import paths keep working. At close the file carries both `import { checkSessionStop }` (local binding for its own line-143 `session_stop` handler) and `export { ... } from` (compat surface) — see WARNING W1, required and sound.
- **`internal/assets/pi/biggz-extension-api.js` (1 line)** — line-16 import repointed to `./biggz-session-guard.js`; comment refreshed. One-way edge into the guard.
- **`internal/assets/pi/biggz-session-stop.test.mjs` (1 line)** — import of the 3 symbols repointed to `./biggz-session-guard.js`; intent untouched.
- **`internal/install/steps/pi_extensions.go` (+1)** — `{"pi/biggz-session-guard.js", "biggz-session-guard.js"},` inserted after line 96 (new line 97, after `extension-api.js`). Covered by existing `//go:embed all:pi`, no embed change needed.
- **SDD artifacts**: proposal (64 lines), specs (2 full specs: session-guard-ownership 63 lines / 3 requirements / 5 scenarios, pi-deploy-list 29 lines / 1 requirement / 2 scenarios), design (61 lines), tasks (40 lines, 8/8), verify-report (105 lines, PASS WITH WARNINGS, 4/4 req, 7/7 scenarios).

Shipped as single candidate commit `737527f8` on branch `feat/single-source-session-guard` (verbatim guard move + compat re-export + repointed imports + deploy-list line; zero behavior change). A docs commit for the `sdd-verify` SKILL.md fix (`193d7edf`) sits on top of the candidate — it is outside this change and was left exactly where it is: not included in, excluded from, or rewritten by this archive. No push/merge; PRs are a later human decision.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 8/8 marked `[x]` — `total:8 completed:8 pending:0 allComplete:true`, `dependencies.tasks: all_done`, `grep "^- \[ \]" 0`, `grep "^- \[x\]" 8` (Phase 1: 1.1, Phase 2: 2.1–2.4, Phase 3: 3.1–3.3) |
| Verify verdict | ✅ `PASS WITH WARNINGS` — `0 blockers`, `0 CRITICAL`, `4/4 requirements`, `7/7 scenarios` compliant (per `verify-report.md` `evidence_revision sha256:ee3c48cde151d875f8b856c72eab42dceb85069ce229bfd8a54e01fbe0c94579`) |
| Build | ✅ `go build ./...` exit 0, empty output (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`) + `node --check` on guard/interception/extension-api all exit 0 + `go vet ./internal/install/steps/` exit 0 |
| Tests | ✅ Full JS suite 49 passed / 0 failed (`node --test internal/assets/pi/*.test.mjs`, exit 0, `test_output_hash sha256:b7dfc8940aee7605668b94ee283ee9661cca475adf45aceaa88488655b22f9da`) incl. focused contract 10/10 (`biggz-session-stop.test.mjs`: Q2 timeout, exit-0 allow, exit-1 block, stale-binary degrade, pending-first x2, timeout degrade, crash degrade, never-throws, both-files parity) + `go test ./internal/install/steps/ -run TestPi -count=1` PASS |
| Coverage | ➖ Not available (JS suite reports no coverage threshold; Standard mode, no threshold configured) |
| Evidence revision | `sha256:ee3c48cde151d875f8b856c72eab42dceb85069ce229bfd8a54e01fbe0c94579` (combined test/deploy/parity output), `test_output_hash sha256:b7dfc894…`, `build_output_hash sha256:e3b0c44298fc…`, ledger `sdd-attempt finish` settled passed with same `evidence_revision` |
| sdd-status pre-archive | ✅ `nextRecommended: archive`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done, sync:all_done, archive:ready}`, `artifacts {proposal:done, specs:done, design:done, tasks:done, verifyReport:done, applyProgress:missing}`, `taskProgress {total:8 completed:8 pending:0 allComplete:true}`, `applyState: all_done`, `artifactStore: openspec`, `HasProposal:true HasSpecs:true HasDesign:true HasTasks:true HasVerify:true IsArchived:false` |
| sdd-status post-archive | ✅ `active` no longer lists `single-source-session-guard` (0 active openspec changes after move); canonical specs present; archived folder verified (see Archive Verification) |
| Review gate | ✅ Lineage `single-source-session-guard` finalized per launch prompt (highest available delivery authority for this change): 4 lenses (risk 0, reliability 0, resilience 1 WARNING, readability 1 WARNING + 2 SUGGESTION), zero BLOCKER/CRITICAL, gate post-apply `allowed:true`, delivery `burned/unmanaged`. `biggz-ai` SDD `openspec` path emits no `reviewGate` in `sdd-status --json` (consistent with archived precedent `2026-09-07-enforce-session-close-summary`); pre-archive `nextRecommended: archive`, `dependencies.archive: ready` — gate PASS. No reviewer launched (gate forbids automatic launch). |
| Task gate | PASS — persisted `tasks.md` 8 `[x]`, 0 `[ ]` pre- and post-archive (`openspec/changes/archive/2026-09-07-single-source-session-guard/tasks.md` verified); no stale-checkbox reconciliation needed, no override |
| Apply state | `all_done` — `sdd-status` reports `applyState: all_done` even though `applyProgress` artifact `missing` (apply did not emit separate `apply-progress.md`; tasks carry completion evidence per dependency `apply: all_done` — same precedent as `enforce-session-close-summary` / `tool-interception`) |
| CRITICAL gate | ✅ `verify-report.md` `critical_findings: 0`, `blockers: 0`, `verdict: pass_with_warnings` — no CRITICAL to block archive; no prompt override needed or accepted |

## Spec Compliance

**Verdict**: `PASS WITH WARNINGS` (per `verify-report.md` `evidence_revision sha256:ee3c48cd…`, `test_exit_code 0`, `build_exit_code 0`)

| Metric | Value |
|--------|-------|
| Requirements | 4/4 compliant |
| Scenarios | 7/7 compliant (0 UNTESTED, 0 FAILING, 0 PARTIAL) |
| Tasks | 8/8 (Phase 1: 1.1, Phase 2: 2.1–2.4, Phase 3: 3.1–3.3) |
| Blockers / Critical | 0 / 0 |
| WARNING at verify time | W1 (required local import alongside re-export — sound, test-proven; see below) — non-blocking |
| SUGGESTION | S1 (pre-existing `gofmt` drift in `pi_extensions.go`, out of scope — do not reformat), S2 (one-line doc note for the required local import in design/tasks 2.1), S3 (add guard to portable-extensions assertion in `pi_extensions_drop_test.go`) |

**Detailed matrix** (from `verify-report.md` Spec Compliance Matrix — 7/7 COMPLIANT):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Single Guard Definition | Search proves single definition | `biggz-session-stop.test.mjs > both files return identical verdicts` + rg definition search → only guard (lines 10/16/24) | ✅ COMPLIANT |
| Single Guard Definition | Old import paths keep working | `biggz-session-stop.test.mjs > both files return identical verdicts` + runtime probe: guard/interception fn identity true | ✅ COMPLIANT |
| Acyclic One-Way Import Graph | Both extensions share one instance | `biggz-session-stop.test.mjs > both files return identical verdicts` + runtime probe: seam identity, shared-seam verdicts equal | ✅ COMPLIANT |
| Acyclic One-Way Import Graph | No cycle at load time | Full suite imports guard + both extensions, 49/49 pass; guard has zero local imports (only `node:child_process`) | ✅ COMPLIANT |
| Zero Behavior Change | Existing contract passes unmodified | `biggz-session-stop.test.mjs` 10/10; test diff = import source line only; moved block diff vs HEAD = one trailing blank line | ✅ COMPLIANT |
| Guard Registered for Deploy | Deploy list contains the guard | `pi_extensions.go` line 97 maps asset `pi/biggz-session-guard.js` to target `biggz-session-guard.js` | ✅ COMPLIANT |
| Guard Registered for Deploy | Build passes and deployed imports resolve | `go build ./...` exit 0 + embed probe (len 2883) + live Apply probe (DEPLOY-OK, guard next to both extensions) | ✅ COMPLIANT |

**Correctness & Coherence** (per verify-report `Correctness (Static Evidence)` + `Coherence (Design)`):

- Guard owns all 3 symbols; interception has zero definitions, only import + re-export. Guard imports only `node:child_process`. Timeout 1000, block/degrade semantics byte-identical. Deploy entry at line 97 after `extension-api.js`; `go:embed all:pi` covers it.
- Design deviation W1 is REQUIRED: pure `export ... from` creates no local binding, but the file's own handler (line 143 `session_stop`) needs one; without the local import the suite drops to 9/10. Runtime probe proves same module instance (fn/seam identity, shared-seam verdicts equal). No spec broken; no action required.

## Spec Sync

Delta specs are full specs (headers `# Session Guard Ownership Specification` / `# Pi Deploy List Specification` with `## Requirements` — no `## ADDED`/`## MODIFIED` sections). Neither canonical domain exists (`ls openspec/specs/session-guard-ownership/`, `ls openspec/specs/pi-deploy-list/` both absent pre-sync). Per archive rule for non-existent main specs, each file was copied verbatim to the canonical tree BEFORE the archive move. No existing spec was modified; no destructive merge; no REMOVED/RENAMED involved.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| session-guard-ownership | **Created** | 3 requirements (Single Guard Definition 2 scenarios, Acyclic One-Way Import Graph 2 scenarios, Zero Behavior Change 1 scenario) + Purpose. Verbatim copy, 63 lines. | `openspec/specs/session-guard-ownership/spec.md` ✅ |
| pi-deploy-list | **Created** | 1 requirement (Guard Registered for Deploy 2 scenarios) + Purpose. Verbatim copy, 29 lines. | `openspec/specs/pi-deploy-list/spec.md` ✅ |

Verification: `diff` change-spec vs canonical-spec identical for both domains post-copy; total 4 requirements / 7 scenarios now source of truth. Unrelated specs (`tool-interception`, `extension-api`, `installer-pipeline`, others) untouched.

## Implementation Traceability

Single candidate `737527f8` on branch `feat/single-source-session-guard` (review tool binds one commit). Suggested Work Units table followed (Unit 1 move + verification, single PR, no chain).

| File | Action | Lines | Description |
|------|--------|-------|-------------|
| `internal/assets/pi/biggz-session-guard.js` | Create | 56 | Guard block moved verbatim incl. APPLY-DECIDE comments; sole owner |
| `internal/assets/pi/biggz-tool-interception.js` | Modify | −57/+1 (+1 local import at close) | Block deleted; compat re-export (+ required local import per W1) |
| `internal/assets/pi/biggz-extension-api.js` | Modify | 1 line + comment | Import source → `./biggz-session-guard.js` |
| `internal/assets/pi/biggz-session-stop.test.mjs` | Modify | 1 line | Import source → `./biggz-session-guard.js` only |
| `internal/install/steps/pi_extensions.go` | Modify | +1 (line 97) | Deploy-list entry after `extension-api.js` |
| `openspec/specs/session-guard-ownership/spec.md` | Sync (new) | 63 | Canonical copy of the ownership full spec |
| `openspec/specs/pi-deploy-list/spec.md` | Sync (new) | 29 | Canonical copy of the deploy-list full spec |

**Tests isolation**: mocked `execFileSync` for JS; `t.Setenv`-style env isolation where applicable; `node --check` static gates; `rg`/`grep` definition and acyclicity probes.

## Final-State Authority & Reconciliation

`verify-report` and `apply-progress` are intermediate snapshots valid at their write time. Per archive contract hierarchy (native review authority > persisted tasks > explicit final-state facts > verify/apply snapshots), the close state below wins over any stale snapshot phrasing.

- **Post-snapshot work**: none that changes the verdict. Sync copies (two new canonical specs) and the archive move are docs/filesystem only, no runtime change; final numbers carried from `verify-report` (`4/4`, `7/7`, `test_output_hash sha256:b7dfc894…`, `build_output_hash sha256:e3b0c44298fc…`, `evidence_revision sha256:ee3c48cd…`); no later test-count change reported.
- **Review lineage (delivery authority)**: launch prompt (rank 3, most recent account) states lineage `single-source-session-guard` finalized — 4 lenses (risk 0, reliability 0, resilience 1 WARNING, readability 1 WARNING + 2 SUGGESTION), zero BLOCKER/CRITICAL, gate post-apply `allowed:true`, delivery `burned/unmanaged`. This supersedes the `verify-report` Ledger note written at verification time ("`rdd_receipt_missing` (empty review chain) ... no review lineage was started ... archive still requires the human review/gatekeeper decision"). Attributed to its time: per `verify-report` at verification time no lineage had been started; at close per the launch prompt the lineage is finalized and the gate allows. No silent resolution — both statements recorded with sources and times. `sdd-status` corroborates archivability (`nextRecommended: archive`, `dependencies.archive: ready`).
- **W1 deviation**: at verification time a sound, test-proven deviation from the design text (local import alongside re-export); still the final state (file carries both lines at close). Non-blocking.
- **S1 gofmt**: pre-existing drift at HEAD (verified via `git stash` + `gofmt -d` at verification time); added line 97 is gofmt-clean. Still the final state; cleanup stays out of scope.
- **applyProgress missing**: `sdd-status` reports `applyProgress: missing` yet `applyState: all_done` / `dependencies.apply: all_done`. Tasks `8/8 [x]` carry completion evidence (same precedent as `enforce-session-close-summary`). Not a blocker, not a contradiction.
- **Docs commit on top**: `193d7edf docs(sdd): fix stale sdd-attempt syntax in sdd-verify skill` (2 files, `internal/assets/skills/sdd-verify/SKILL.md` + `skills/sdd-verify/SKILL.md`) sits on top of candidate `737527f8` at close. Per explicit scope it is outside this change; archive neither includes, excludes, nor rewrites it. Recorded here so a future reader does not attribute it to this change.

No unrankable contradiction between launch-prompt facts and repository evidence. All gates corroborated by `sdd-status` authority plus file evidence. CRITICAL gate holds independently: `critical_findings: 0` — no prompt override was needed or accepted.

## Archive Verification

Pre-archive (from `biggz sdd-status --json --instructions`):

- ✅ `nextRecommended: archive` (archivable)
- ✅ `verifyReport: done` (`artifacts.verifyReport: done`, `dependencies.verify: all_done`, `HasVerify: true`)
- ✅ `taskProgress: {total:8 completed:8 pending:0 allComplete:true}` (`dependencies.tasks: all_done`, 0 `[ ]`)
- ✅ `artifactStore: openspec` preserved
- ✅ `dependencies.sync: all_done`, `archive: ready`; `remediationState: {required:false}` — no remediation required
- ✅ `CRITICAL: 0`, `blockers: 0` — no archive block; no prompt override needed
- ✅ `actionContext.mode: repo-local` (not `workspace-planning`); operations stayed inside `allowedEditRoots` (`C:\Users\USER\Desktop\biggz-ai`)
- ✅ No `reviewGate` in `openspec` `sdd-status` — lineage from launch prompt + `archive:ready` govern (precedent-consistent with `2026-09-07-enforce-session-close-summary`)

Spec sync (BEFORE move):

- ✅ `openspec/specs/session-guard-ownership/spec.md` **Created** (63 lines, verbatim, `diff` identical)
- ✅ `openspec/specs/pi-deploy-list/spec.md` **Created** (29 lines, verbatim, `diff` identical)
- ✅ No existing main spec modified; no destructive merge (no WARN needed)

Archive move:

- ✅ `mv openspec/changes/single-source-session-guard → openspec/changes/archive/2026-09-07-single-source-session-guard` (date prefix `2026-09-07` = today per `date -u +%F`)
- ✅ Main specs still present after move (session-guard-ownership 63 lines, pi-deploy-list 29 lines)
- ✅ Change folder moved to archive (`ls openspec/changes/single-source-session-guard` → absent; archive dir lists `_meta.yaml`, `proposal.md`, `design.md`, `tasks.md`, `verify-report.md`, `specs/{session-guard-ownership,pi-deploy-list}/spec.md`, plus this `archive-report.md`)
- ✅ Archive contains all artifacts (`proposal.md` 64 lines ✅, `specs/session-guard-ownership/spec.md` 63 ✅, `specs/pi-deploy-list/spec.md` 29 ✅, `design.md` 61 ✅, `tasks.md` 40 ✅ 8/8, `verify-report.md` 105 ✅, `_meta.yaml` ✅, plus `archive-report.md` this file)
- ✅ Archived `tasks.md` has no unchecked implementation tasks (8 `[x]`, 0 `[ ]` — no reconciliation needed, no override)
- ✅ Active changes directory no longer has this change
- ✅ Scope exclusions honored: `openspec/changes/archive/sdd-parity-rescope-grant-ledger/.biggz-instance` (untracked tooling stray) untouched — still present and unstaged; docs commit `193d7edf` left exactly in place; nothing staged, committed, pushed, merged, and no PRs created (later human decision)
- ✅ `openspec/changes/archive/` existed already, no create needed

Post-archive:

- ✅ `biggz sdd-status --json` `active` no longer lists `single-source-session-guard` (0 active); canonical specs remain source of truth
- ✅ Nothing beyond the two new canonical specs + the archived folder was modified by archive (worktree shows only the expected move + new specs, unstaged per scope)

## Risks / Open Questions

**Risks at close:**

- **W1 local import (accepted)**: future editors must keep BOTH lines in `biggz-tool-interception.js` (local `import` for the in-file handler + `export ... from` for compat). Removing the import regresses the suite to 9/10. S2 suggests a one-line doc note in design/tasks 2.1 to prevent this.
- **S1 gofmt drift**: `pi_extensions.go` carries pre-existing alignment drift; this change's line 97 is clean. Repo-wide `gofmt -w` is out of scope — do not bundle it into this change's commit.
- **S3 deploy regression guard**: consider adding `biggz-session-guard.js` to the portable-extensions assertion in `pi_extensions_drop_test.go` so `go test` guards deploy regressions.
- **Stale deployed file after revert**: per proposal rollback plan, reverting the move commit leaves a stale `biggz-session-guard.js` in `~/.pi/agent/extensions/` — inert (nothing imports it after revert); remove manually if desired.
- **Ledger provider quirk (context)**: prior changes observed `corrupt_authority: ledger is complete` after `sdd-attempt finish`; no such block affects this archive.

**Open questions at close:** None for this change. Design open questions were resolved in tasks (verbatim move, all-3-symbol re-export, insert after line 96, zero local imports in guard).

## Traceability

- **Proposal**: `openspec/changes/archive/2026-09-07-single-source-session-guard/proposal.md` (64 lines, pure-move scope, rollback plan)
- **Specs (full specs)**: `specs/session-guard-ownership/spec.md` (63 lines, 3 requirements, 5 scenarios) + `specs/pi-deploy-list/spec.md` (29 lines, 1 requirement, 2 scenarios) before move → now archived under `2026-09-07-single-source-session-guard/specs/` + canonical copies at `openspec/specs/{session-guard-ownership,pi-deploy-list}/spec.md`
- **Design**: `openspec/changes/archive/2026-09-07-single-source-session-guard/design.md` (61 lines, verbatim-move approach, 3 architecture decisions, data flow, threat matrix)
- **Tasks**: `openspec/changes/archive/2026-09-07-single-source-session-guard/tasks.md` (40 lines, 3 phases, `8/8 [x]`)
- **Verify**: `openspec/changes/archive/2026-09-07-single-source-session-guard/verify-report.md` (105 lines, `evidence_revision sha256:ee3c48cde151d875f8b856c72eab42dceb85069ce229bfd8a54e01fbe0c94579`, `verdict: pass_with_warnings`, `4/4 req`, `7/7 scenarios`, `0 blockers`, `0 critical`)
- **Apply**: candidate `737527f8` on branch `feat/single-source-session-guard` (5 runtime files: new guard 56, interception −57/+1, extension-api 1-line repoint, test 1-line repoint, deploy list +1; plus 7 SDD files). Zero behavior change.
- **Review**: lineage `single-source-session-guard` finalized per launch prompt (risk 0, reliability 0, resilience 1 WARNING, readability 1 WARNING + 2 SUGGESTION; gate post-apply `allowed:true`, delivery `burned/unmanaged`)
- **sdd-status**: pre-archive `nextRecommended: archive`, `verifyReport: done`, `taskProgress {total:8 completed:8 pending:0 allComplete:true}`; post-archive `active` without the change
- **Commit**: no archive commit per scope (human commits later); frozen candidate `737527f8` preserved in history; docs commit `193d7edf` preserved on top, excluded from this change; unrelated stray `.biggz-instance` excluded; no push/merge/PR

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.

**Change**: `single-source-session-guard`
**Archived to**: `openspec/changes/archive/2026-09-07-single-source-session-guard/` (Engram N/A — `openspec` mode) | `openspec/specs/{session-guard-ownership,pi-deploy-list}/spec.md` source of truth

### Specs Synced
| Domain | Action | Details |
|--------|--------|---------|
| session-guard-ownership | Created | 3 requirements, 5 scenarios (verbatim full-spec copy, 63 lines) |
| pi-deploy-list | Created | 1 requirement, 2 scenarios (verbatim full-spec copy, 29 lines) |

### Archive Contents
- proposal.md ✅ (64 lines)
- specs/session-guard-ownership/spec.md ✅ (63 lines, full spec)
- specs/pi-deploy-list/spec.md ✅ (29 lines, full spec)
- design.md ✅ (61 lines)
- tasks.md ✅ (8/8 tasks complete, 0 pending)
- verify-report.md ✅ (PASS WITH WARNINGS, 4/4 req, 7/7 scenarios, 0 blockers, 0 CRITICAL)
- _meta.yaml ✅
- archive-report.md ✅ (this file)

### Source of Truth Updated
The following specs now reflect the new behavior:
- `openspec/specs/session-guard-ownership/spec.md` — single guard definition, acyclic one-way import graph, zero behavior change
- `openspec/specs/pi-deploy-list/spec.md` — guard registered for deploy

### Next

Ready for the next change. `biggz sdd-status --json` shows no active change (`active: []`), delivery `openspec` preserved, no remediation required. Commit of the archived folder + two new canonical specs, and any PRs from branch `feat/single-source-session-guard`, are explicit later human decisions — nothing was staged, committed, pushed, or merged.
