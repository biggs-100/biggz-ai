# Archive Report: component-system

**Archived**: 2026-08-25
**Mode**: Standard (strict_tdd: false)
**Artifact Store**: BigMem (hybrid) — `bigmem:sdd/component-system` + `openspec/changes/archive/2026-07-28-component-system/` baseline, synced to `openspec/specs/` source of truth
**Change**: component-system
**Archived to**: `bigmem:sdd/component-system/archive-report` (BigMem) | `openspec/changes/archive/2026-08-25-component-system/` (filesystem hybrid mirror) — baseline `2026-07-28-component-system` retained for historical context
**Previous location**: `bigmem:sdd/component-system` (active) / baseline filesystem `openspec/changes/archive/2026-07-28-component-system/`

## Summary

Completed the Component System + Catalog + Planner + State + Agent Registry change. Delivered 5 domains, 22 requirements, 42 scenarios, 17 tasks across 2 stacked PRs (force-chained stacked-to-main), all verified PASS with 0 blockers and 0 CRITICAL findings on evidence `sha256:f07f94010d4f899217bb8b3892b5b73865e8af0f34223b87844a1642a70f58a6`.

- **PR 1 — Foundation**: `plugin/types.go` CatalogEntry, `plugin/interfaces.go` enriched (SupportsAutoInstall, MCPStrategy, Capabilities → []string), `internal/catalog/catalog.go` (SkillEntry/ComponentEntry, AllAgents/AllComponents/AllSkills with defensive copies), 3 adapters (opencode/claude/qwen) updated, `internal/agents/registry.go` + `discovery.go`, `registry/registry.go` rename (RegisterAdapter/GetAdapter + ListAll)
- **PR 2 — Logic**: `internal/planner/graph.go` (AddNode/AddEdge + Kahn TopologicalSort with cycle detection), `internal/planner/resolver.go` + `types.go` (Resolver.Resolve, Plan/Selection), `internal/state/state.go` (Read/Write/Merge with unknown-field preservation), `internal/components/skills.go|config.go|prompts.go` (Component wrappers)

Final evidence-refresh verification (post-remediation) is green: `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -count=1` exit 0 across 28+ packages. All 3 historical CRITICALs remain fixed.

## Spec Compliance

**Verdict**: PASS (per `sdd/component-system/verify-report` observation `obs-1787698338813111100-1`, evidence_revision `sha256:f07f94010d4f899217bb8b3892b5b73865e8af0f34223b87844a1642a70f58a6`, validated via `biggz sdd-verify-validate`)

| Metric | Value |
|--------|-------|
| Requirements | 22/22 compliant |
| Scenarios | 42/42 compliant |
| Tasks | 17/17 complete |
| Blockers | 0 |
| Critical findings | 0 |
| Build | `go build ./... && go vet ./...` → exit 0 (build_output_hash `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`) |
| Tests | `go test ./... -count=1` → exit 0, all 28+ packages ok (test_output_hash `sha256:f07f94010d4f899217bb8b3892b5b73865e8af0f34223b87844a1642a70f58a6`) |
| Evidence revision | `sha256:f07f94010d4f899217bb8b3892b5b73865e8af0f34223b87844a1642a70f58a6` (settled via native ledger, `biggz sdd-verify-validate` admitted) |

Domains:
- `component-catalog` — 4 req, 8 scen (CatalogEntry, AllAgents/AllComponents/AllSkills) — PASS
- `planner` — 4 req, 7 scen (Graph, Resolver, TopologicalSort, Planner Orchestration) — PASS
- `agent-registry` — 4 req (original) / 7 req (current evolved spec), 7–14 scen — PASS via successor `internal/agents.Registry` (informational note preserved)
- `state-persistence` — 4 req, 9 scen (InstallState schema, Read/Write/Merge) — PASS
- `plugin-system` — 6 req (delta: SupportsAutoInstall, MCPStrategy, Enriched Capabilities, Registry Integration + Modified Interface/Build-Time Registry) — PASS; current main spec at `openspec/specs/plugin-system/spec.md` has evolved to 9 req (preserved, not overwritten)

Informational warnings from verify (non-blocking): no Planner struct (Resolver provides equivalent), InstallState uses []string per PR 2 spec vs map in design, 2 PARTIAL catalog scenarios lacking cross-reference tests — all documented in verify-report.

## Spec Sync

Delta specs merged into main specs (source of truth) before archive. In hybrid mode the main specs at `openspec/specs/` are the audit authority; filesystem wins on conflict per `internal/sdd/engram_status.go`. The spec artifact in BigMem (`sdd/component-system/spec`, 411 lines, 22 req, 42 scen) was restored from the archive baseline for dispatcher counts prior to evidence refresh.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| component-catalog | Created/Ensured | 4 req, 8 scen — hardcoded slices, defensive copies, lookup helpers | `openspec/specs/component-catalog/spec.md` ✅ (78 lines) |
| planner | Created/Ensured | 4 req, 7 scen — Graph + Resolver + TopologicalSort + Plan | `openspec/specs/planner/spec.md` ✅ (73 lines) |
| agent-registry | Created/Ensured | 4 req delta (7 req current evolved) — Factory, Registry, 3 adapters | `openspec/specs/agent-registry/spec.md` ✅ (130 lines, evolved) |
| state-persistence | Created/Ensured | 4 req, 9 scen — InstallState + Read/Write/Merge | `openspec/specs/state-persistence/spec.md` ✅ (86 lines) |
| plugin-system | Updated (delta merged, preserved) | 6 req delta (SupportsAutoInstall, MCPStrategy, Enriched Capabilities, Registry Integration + AgentAdapter Interface, Build-Time Registry) — appended to existing main spec, no REMOVED/RENAMED, all prior requirements preserved | `openspec/specs/plugin-system/spec.md` ✅ (207 lines, preserved) |

Spec restoration note: BigMem spec `obs-28ac40f0a038febe` (411 lines) was reconstituted from `openspec/changes/archive/2026-07-28-component-system/specs/*` (5 delta files) to restore dispatcher requirement/scenario counts (22/42). Verify report's 22/22 and 42/42 match the restored counts. Main specs already reflected the same domains; no destructive overwrite occurred (existing REMOVED/RENAMED none).

Ledger hashes for sync provenance: evidence_revision `f07f9401...`, test_output_hash `f07f9401...`, build_output_hash `e3b0c442...`.

## Verification Gate

- **Review gate**: N/A — biggz-ai SDD path has no `reviewGate` per `sdd-status-contract.md` Divergences (`No review, resolve-review, or reviewGate values: biggz has no review authority on the SDD path`). `biggz sdd-status --json` emits no `reviewGate` field; `review_disabled: false` (RDD enabled) but SDD changes route via `nextRecommended` only. `nextRecommended: archive`, `blockedReasons: []`, `dependencies.archive: ready` — gate PASS. No pending/malformed/scope-changed receipt to block; no automatic reviewer launch required.
- **Task gate**: PASS — persisted `sdd/component-system/tasks` (`obs-345f2737b9147316`) shows 17/17 `[x]` complete, 0 `[ ]` pending. `taskProgress: {total:17, completed:17, pending:0, allComplete:true}`, `applyState: all_done`.
- **Build & Tests**: PASS as above (post-remediation commit `2b3c56f` fixed stale `pi-web-search` doctor expectations — DuckDuckGo-default — that had poisoned the green suite in `PI_SUBAGENT_CHILD` sandbox; clean shell now green, `PI_SUBAGENT_CHILD` unset).
- **Verify report**: PASS — `obs-1787698338813111100-1` (`sdd/component-system/verify-report`), verdict `pass`, 0 blockers, 0 critical, 22/22 req, 42/42 scen, `go test ./... -count=1` exit 0, `go build ./... && go vet ./...` exit 0. Stale prior verify `obs-a3d3e2b089a0c9d9` (`sdd/component-system/verify`, FAIL with 2 CRITICAL) is superseded — all 3 CRITICALs (BuildReviewPayload, Claude adapter tests, Graph.AddEdge orphan rejection) are HOLD per spot-checks in the evidence-refresh report.
- **Remediation**: No open remediation. Prior remediation commit `2b3c56f` (`test(doctor): align pi-web-search expectations with DuckDuckGo-default behavior`) re-established green suite; evidence `f07f9401...` settled via ledger, validated, and `sdd-attempt status component-system` shows `Next action: begin` with no binding lineage (remediation complete, fresh verification required before archive — now satisfied).

## Remediation & Evidence Refresh History

1. Initial verify `obs-a3d3e2b089a0c9d9` — FAIL with 2 CRITICAL (BuildReviewPayload missing, Claude tests missing, Graph.AddEdge orphan not rejected) — since fixed.
2. Fix commit `3c61010f94691550` — added BuildReviewPayload in `internal/planner/types.go:32`, 8 tests in `internal/agents/claude/adapter_test.go`, orphan rejection in `internal/planner/graph.go:28`.
3. Re-verification `obs-3bfab6ba989180cf` — PASS (first green).
4. Stale doctor tests poisoned suite in subagent sandbox (PI_SUBAGENT_CHILD guard).
5. Remediation commit `2b3c56f` — aligned `internal/doctor/pi_web_search_test.go` with DuckDuckGo-default (Tavily→Brave→DuckDuckGo fallback) — full suite green in clean shell.
6. Evidence-refresh verify `obs-1787698338813111100-1` — PASS, evidence_revision `f07f9401...`, build `e3b0c442...`, validated via `biggz sdd-verify-validate`, settled.

Per Final-State Authority hierarchy: the persisted `apply-progress` (`obs-8e0821a5b408ad89`) and intermediate `verify` (`obs-a3d3e2b089a0c9d9`) are superseded by the explicit final-state facts in the orchestrator launch prompt and the terminal `verify-report` (`obs-1787698338813111100-1`). All 3 historical CRITICALs are HOLD; no contradiction remains.

## Archived Artifacts

All SDD artifacts preserved (BigMem authoritative, filesystem hybrid mirror):

| Artifact | BigMem Topic | Observation ID | Status | Filesystem Mirror |
|----------|--------------|----------------|--------|-------------------|
| Proposal | `sdd/component-system/proposal` | `obs-188f6517be90f93d` | ✅ (461 bytes summary; full 4108 bytes in archive baseline) | `proposal.md` ✅ |
| Spec (full, restored) | `sdd/component-system/spec` | `obs-28ac40f0a038febe` | ✅ 411 lines, 22 req, 42 scen | `specs/*` (5 delta files) ✅ |
| Spec-full concat | `sdd/component-system/spec-full` | `obs-01254d246ea1c1c8` | ✅ (1629 bytes) | — |
| Design | `sdd/component-system/design` | `obs-d6dce914563cdb00` | ✅ (720 bytes summary; full 7648 bytes in archive) | `design.md` ✅ |
| Tasks | `sdd/component-system/tasks` | `obs-345f2737b9147316` | ✅ 17/17 [x] | `tasks.md` ✅ (17/17) |
| Apply Progress (PR2) | `sdd/component-system/apply-progress` | `obs-8e0821a5b408ad89` | ✅ | `apply-progress.md` (via PR2) ✅ |
| Verify Report (terminal PASS) | `sdd/component-system/verify-report` | `obs-1787698338813111100-1` | ✅ PASS f07f94... | `verify.md` / `verify-report.md` ✅ |
| Verify (stale FAIL) | `sdd/component-system/verify` | `obs-a3d3e2b089a0c9d9` | superseded FAIL (2 CRITICAL, now resolved) | — |
| Archive Report | `sdd/component-system/archive-report` | `this observation` | ✅ (this file) | `archive-report.md` ✅ |

**Task Completion Gate**: `tasks` observation has 0 unchecked (`- [ ]` count 0), 17 checked (`- [x]` count 17) — gate PASS, no stale checkboxes. `sdd-apply` owns checkbox completion; no archive-time reconciliation needed.

**Archive verification**:
- [x] Main specs updated correctly (5 domains, preserved)
- [x] Change folder archived (BigMem `IsArchived: true`, `NextRecommended: done` after this save; filesystem `openspec/changes/archive/2026-08-25-component-system/` mirror created, baseline retained)
- [x] Archive contains all artifacts (proposal, specs, design, tasks, verify-report, archive-report)
- [x] Archived `tasks.md` has no unchecked implementation tasks
- [x] Active BigMem change will no longer appear in `active` after archive-report persistence (moves to `archived`)

## Source of Truth Updated

The following specs now reflect the new behavior (source of truth):

- `openspec/specs/component-catalog/spec.md` — 4 req, 8 scen
- `openspec/specs/planner/spec.md` — 4 req, 7 scen
- `openspec/specs/agent-registry/spec.md` — 7 req current (4 delta + 3 evolved), 14 scen
- `openspec/specs/state-persistence/spec.md` — 4 req, 9 scen
- `openspec/specs/plugin-system/spec.md` — 9 req current (delta merged, preserved), 23 scen

Preserved requirements not mentioned in delta remain unchanged (e.g., existing Pipeline Stage Execution, AgentAdapter Interface evolution beyond delta).

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived:

`proposal → spec (22/42 restored) → design → tasks (17/17) → apply (PR1+PR2) → verify (PASS f07f94, 22/22 42/42) → archive (2026-08-25)`

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks. `sdd-status` after archive will show `IsArchived: true`, `NextRecommended: done`.

## Audit Trail

- **Structured status at archive**: `biggz sdd-status --json --instructions` for `component-system` — `changeRoot: bigmem:sdd/component-system`, `artifacts: {proposal:done, specs:done, design:done, tasks:done, applyProgress:done, verifyReport:done}`, `taskProgress: {total:17, completed:17, pending:0, allComplete:true}`, `dependencies: {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done, archive:ready}`, `applyState: all_done`, `actionContext: {mode:repo-local, workspaceRoot:C:\\Users\\USER\\Desktop\\biggz-ai, allowedEditRoots:[C:\\Users\\USER\\Desktop\\biggz-ai]}`, `remediationState: {required:false}`, `nextRecommended: archive`, `blockedReasons: []`, `review_disabled: false` (RDD enabled but SDD path has no review authority).
- **Observation IDs for traceability** (Engram/hybrid requirement): `obs-188f6517be90f93d` (proposal), `obs-28ac40f0a038febe` (spec), `obs-01254d246ea1c1c8` (spec-full), `obs-d6dce914563cdb00` (design), `obs-345f2737b9147316` (tasks), `obs-8e0821a5b408ad89` (apply-progress), `obs-1787698338813111100-1` (verify-report terminal PASS), `obs-a3d3e2b089a0c9d9` (verify stale FAIL superseded).
- **Ledger & validation**: evidence_revision `sha256:f07f94010d4f899217bb8b3892b5b73865e8af0f34223b87844a1642a70f58a6`, test_output_hash `sha256:f07f94010d4f899217bb8b3892b5b73865e8af0f34223b87844a1642a70f58a6`, build_output_hash `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `biggz sdd-verify-validate` admitted, remediation commit `2b3c56f` (de9365→8a6259), clean shell `go test ./... -count=1` exit 0 (28+ packages).
- **Filesystem baseline**: `openspec/changes/archive/2026-07-28-component-system/` — 5 delta specs, proposal/design/tasks/verify preserved; new hybrid mirror `openspec/changes/archive/2026-08-25-component-system/` created with archive-report.
- **Previous archive attempt**: `obs-0f4d1f3e1edbc7af` (Archived component-system to openspec archive) superseded by this evidence-refresh archive (f07f94 ledger).

## Notes

- Hybrid mode: BigMem is authoritative for dispatcher counts (22/42); filesystem main specs are the source of truth for consumers; both are now consistent (411 lines restored in BigMem spec, filesystem main specs preserved).
- No `reviewGate` receipt exists for this SDD change — expected per `sdd-status-contract.md` Divergences; archive proceeds on `nextRecommended: archive` with empty `blockedReasons`.
- No intentional partial archive or stale-checkbox reconciliation — all 17 tasks are genuinely complete per persisted tasks artifact and verify-report.
