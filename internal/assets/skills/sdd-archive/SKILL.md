---
name: sdd-archive
description: "Archive a completed SDD change by syncing delta specs, moving to archive, and producing archive report. Trigger: orchestrator launches archive after implementation and verification."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "2.0"
  delegate_only: true
---
<!-- section:model-capable -->
## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language; comments follow the target context language.

## Purpose

You are a sub-agent responsible for ARCHIVING: merge delta specs into main specs, move the change folder to archive, close the SDD cycle.

## What You Receive

Change name, store mode, structured status, launch-prompt final-state facts, intentional-override text (when provided).

## Final-State Authority

The archive report describes the change AT CLOSE. `apply-progress`/`verify-report` are intermediate snapshots: their "done" stays true, but "pending/blocked/open" claims expire the moment later work lands. Never present a snapshot statement as current state.

Authority rank: (1) native review authority (`reviewGate`, receipt, gate context); (2) persisted tasks artifact; (3) launch-prompt final-state facts; (4) `verify-report`/`apply-progress` (history only).

Rules: higher rank wins (report fix + location). Unrankable contradictions go in the report (both claims, sources, timestamps). Attribute snapshot claims; carry final numbers from the top-ranked source. Causes need evidence or stay undiagnosed. CRITICAL verify issues still block (fix claims need fresh `sdd-verify`).

## Execution and Persistence Contract

Sections B + C from `_shared/sdd-phase-common.md`. `engram`: read proposal/spec/design/tasks/verify-report + `review/*` (record IDs), save `sdd/{change}/archive-report`. `openspec`: merge + moves per `openspec-convention.md`. `hybrid`: both. `none`: summary only.

### Native Review Receipt Gate

Require `reviewGate.result: allow` (or `disabled/unmanaged` when unreviewed). Read the transaction, frozen ledger, receipt, and gate context. Bad review state blocks with no override and no auto-reviewer; receipt must match tree, digest, policy, ledger, delta, evidence, counters, base.

### Task Completion Gate

`sdd-apply` marks tasks; archive validates the persisted artifact first. Unchecked task → STOP (`blocked`), no sync/move. Reconcile stale checkboxes only on explicit order + `apply-progress`/`verify-report` proof (record reason). Persisted checkboxes rule; todos don't count.

### Strict-vs-OpenSpec Archive Policy

Stricter than OpenSpec: incomplete tasks block (unless proven stale); CRITICAL issues always block; missing artifacts need explicit intentional-partial approval in the report.

### Action Context Guard

`workspace-planning` mode → STOP (no cross-repo moves). `allowedEditRoots` present → stay inside them.

## What to Do

### Step 1: Load Skills (Section A, `_shared/sdd-phase-common.md`)

### Step 2: Sync Delta Specs to Main Specs

After the Task Completion Gate passes. `engram`/`none`: skip filesystem sync. `openspec`/`hybrid`: for each delta spec, match requirements by name and apply: ADDED → append; MODIFIED → replace; REMOVED → delete only with `(Reason:)` + `(Migration:)` in the delta; RENAMED → rename (old + new names explicit, scenarios preserved unless modified). Preserve untouched requirements and heading hierarchy. If no main spec exists, the delta IS the spec — copy it to `openspec/specs/{domain}/spec.md`.

### Step 3: Move to Archive

`engram`/`none`: skip (no filesystem ops). `openspec`/`hybrid`: move `openspec/changes/{change-name}/` → `openspec/changes/archive/YYYY-MM-DD-{change-name}/` (today's ISO date).

### Step 3b: Post-Archive Hygiene (branch/worktree cleanup)

Only after `ArchiveChange` (`os.Rename`, no git). `none`/`engram`: skip. Non-TTY (CI): delete nothing, exit 0 (`use --dry-run on CI`). Otherwise: `fetch --prune` (warn-only) → list `[gone]` branches merged to `origin/HEAD`→`origin/main` → list worktrees (porcelain) → candidates = `gone && !protected && !current && (name==change || prefix change+"-" || merged)` (never substring, never `-D` without second confirm) → preview table → `Prune/Keep` consent. Prune uses `branch -d` only; clean+prunable worktrees only (dirty → skip, locked → skip). `.biggz-instance` stays in the archive (rename-preserved, never deleted).

### Step 4: Verify Archive

Confirm specs updated, folder moved with all artifacts, tasks complete (or approved), active dir clean, `.biggz-instance` kept, hygiene done/skipped (`openspec`/`hybrid`); IDs recorded (`engram`); skip (`none`).

### Step 5: Persist Archive Report (MANDATORY)

Section C from `_shared/sdd-phase-common.md`: artifact `archive-report`, topic `sdd/{change}/archive-report`, type `architecture`.

### Step 6: Return Summary

Return `## Change Archived`: change, archived path (or Engram report / inline), per-domain sync actions (N added / M modified / K removed), artifact checklist with task completion, updated source-of-truth specs, cycle-complete note.

## Rules

- Report FINAL state per Final-State Authority; record unrankable contradictions explicitly
- NEVER archive with CRITICAL verify issues, or with stale unchecked tasks
- Explicit partial-archive/reconciliation approval → record reason, mark intentional-with-warnings
- ALWAYS sync specs BEFORE moving; PRESERVE untouched requirements; destructive merges need confirmation
- ISO date (`YYYY-MM-DD`) archive prefix; create `openspec/changes/archive/` if missing
- Archive is an AUDIT TRAIL — never delete/modify it; apply `rules.archive`; return Section D envelope.
<!-- /section:model-capable -->

<!-- section:model-small -->
---
name: sdd-archive
description: "Archive a completed SDD change by syncing delta specs, moving to archive, and producing archive report. Trigger: orchestrator launches archive after implementation and verification."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "2.0"
  delegate_only: true
---

> **ORCHESTRATOR GATE**: If you loaded this skill via the `skill()` tool, you are the ORCHESTRATOR — STOP. Do NOT execute these instructions inline. Do NOT delegate, do NOT call task/delegate, and do NOT launch sub-agents. Read this SKILL.md and follow it exactly.

## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language.

## Purpose

You are an ARCHIVING sub-agent. You merge delta specs into main specs and move the change to archive. You complete the SDD cycle. Do NOT delegate.

## What You Receive

- Change name, store mode, structured status (`artifactPaths`, `reviewGate`, progress, `actionContext`)
- Final-state facts + override text when provided

## Rules

- Do NOT delegate, do NOT call task/delegate, do NOT launch sub-agents
- Read max 3 files at a time — if you need more, stop and report `needs-explore`
- Consume structured status; stop on missing `reviewGate.allow` (unless disabled/unmanaged) or unsafe `actionContext`
- NEVER archive with CRITICAL verify issues or unchecked tasks without explicit proof
- ALWAYS sync delta specs before moving to archive; preserve untouched requirements

## Steps

1. Load ≤2 orchestrator-passed SKILL.md paths (only these)
2. Validate Receipt Gate (`reviewGate.allow` or `disabled/unmanaged`) then Task Completion Gate (stop on unchecked tasks without proof)
3. Retrieve artifacts via Section B; sync delta specs (ADDED/MODIFIED/REMOVED+Reason/Migration/RENAMED)
4. Move folder to `openspec/changes/archive/YYYY-MM-DD-{change}/`; verify (specs updated, folder moved, tasks complete)
5. Persist archive-report via Section C; return archived path, specs synced, completion status.

## Return Envelope

```json
{
  "status": "ok|blocked|error",
  "archived_to": "openspec/changes/archive/YYYY-MM-DD-change/",
  "specs_synced": ["domain"],
  "notes": "short text"
}
```
<!-- /section:model-small -->

