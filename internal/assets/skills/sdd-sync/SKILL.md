---
name: sdd-sync
description: "Sync delta specs to main specs without archiving for stacked PRs. Port of gentle-pi sdd-sync. Trigger: orchestrator launches sync after verify PASS for file-backed store."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
  delegate_only: true
---
<!-- section:model-capable -->
## Language Domain Contract

Generated technical artifacts default to English. Do not inherit the user's conversational language or the active persona's regional voice for SDD artifacts unless the user explicitly requests that artifact language or the project convention requires it.

If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish unless the user explicitly asks for a regional variant.

Public/contextual comments follow the target context language by default. Explicit user language or tone overrides win; Spanish comments default to neutral/professional Spanish unless the user or target context clearly calls for regional tone.

## Purpose

You are a sub-agent responsible for SYNC. You synchronize file-backed delta specs from `openspec/changes/{change}/specs/` to `openspec/specs/` without archiving. This keeps main specs current for stacked PRs while the change stays active.

## What You Receive

From the orchestrator:
- Change name
- Artifact store mode (`engram | openspec | hybrid | none`)
- Structured status from `_shared/sdd-status-contract.md`: `schemaName`, `planningHome`, `changeRoot`, `artifactPaths`, `contextFiles`, `applyState`, task progress, dependency states, and `actionContext`
- Prompt text (may contain `allow-destructive`, `resolve-via-engram`, `ordered` markers)
- Delivery strategy and chain strategy

## Execution and Persistence Contract

> Follow **Section B** (retrieval) and **Section C** (persistence) from `_shared/sdd-phase-common.md`.

- **engram**: No filesystem sync — return `not-applicable` with zero writes. The change stays in Engram.
- **openspec**: Read deltas from `openspec/changes/{change}/specs/**/spec.md`, apply ADDED/MODIFIED/REMOVED via `internal/sdd/openspec-deltas.go` to `openspec/specs/{domain}/spec.md`. No commit, no archive move.
- **hybrid**: File-backed sync as in `openspec`; Engram observation remains unchanged (filesystem wins).
- **none**: Return `not-applicable` inline only; no writes.

## Status and Workspace Guard

Before executing, consume structured status:

- If `store` is `engram`/`bigmem`/`none` → `not-applicable`, zero writes.
- If `verify` is not `PASS` → `blocked` (verify must be PASS before sync).
- If `actionContext.mode` is `workspace-planning` → STOP.
- If `allowedEditRoots` is present, ensure target `openspec/specs/{domain}/spec.md` is inside allowed roots; otherwise `blocked`.
- `resolve-via-engram` in prompt or tasks skips strict destructive/collision guards.

## What to Do

### Step 1: Load Skills
Follow **Section A** from `_shared/sdd-phase-common.md`.

### Step 2: Validate Store Gate

```
if declaredArtifactStore(workspaceRoot) is engram/bigmem/none
  → return not-applicable, zero writes to openspec/specs/
```

### Step 3: Validate Verify PASS

Read `verify-report.md` and evaluate via `readVerifyResult`. If not PASS → `blocked` with verify reason.

### Step 4: Parse Deltas

For each `openspec/changes/{change}/specs/**/spec.md`:

- `ParseDeltaSpec` → `Deltas`, `HasRenamed`, `IsLegacyFlat`
- If `HasRenamed` (contains `## RENAMED`) → `blocked` with hint `rewrite as ADDED+REMOVED`; do not provide rename helper.
- Collect per-domain deltas.

### Step 5: Guardrails

- **Legacy flat**: if `openspec/specs/{domain}/spec.md` exists without `### Requirement:` → `blocked` with conversion hint.
- **Destructive**: if any `REMOVED` or `MODIFIED` exceeds `largeMutationThreshold` (20 lines) and prompt lacks `allow-destructive` → `blocked` mentioning `destructive` and `approval`. With `allow-destructive` → proceed. `resolve-via-engram` skips this guard.
- **Collision**: if another active change touches same `openspec/specs/{domain}/spec.md` without order decision → `blocked` listing colliding domain and change. `resolve-via-engram` or prompt containing `ordered`/`allow-collision` skips.
- **RENAMED**: already handled in Step 4.
- **Carve-outs**: `resolve-via-engram` skips strict destructive/collision checks.

### Step 6: Apply Deltas

```
for each domain:
  main = read openspec/specs/{domain}/spec.md (or empty if missing)
  ApplyDeltas(main, deltas) → newMain
  write openspec/specs/{domain}/spec.md (mkdir -p, 0644)
```

- ADDED appends blocks
- MODIFIED fully replaces matching requirement
- REMOVED deletes it
- Preserve header and untouched requirements
- No `git commit` and no move to `openspec/changes/archive/`

### Step 7: Verify Invariants

- `openspec/changes/{change}/` must still exist (no archive move)
- `git log` must have no auto-commit from sync
- `openspec/specs/{domain}/spec.md` must reflect deltas

### Step 8: Return Summary

Return envelope per **Section D** from `_shared/sdd-phase-common.md` with `SyncResult` (`applied`, `not-applicable`, `blocked`) and message. Sync does not create `apply-progress` or `verify-report`; it updates main specs only.

## Rules

- Store gate: `engram`/`none` → `not-applicable`, zero filesystem writes
- Delta semantics: ADDED append, MODIFIED full-replace, REMOVED delete; legacy flat → `blocked`
- Destructive guard: REMOVED/large MODIFIED without `allow-destructive` → `blocked`
- Collision guard: same-domain active change without order → `blocked` with domain+change
- RENAMED rejection: `## RENAMED` → `blocked` with `ADDED+REMOVED` hint, no helper
- Carve-outs: `resolve-via-engram` skips strict guards; `verify` must be `PASS`; respect `actionContext`; no commit/archive
- Always use `internal/sdd/openspec-deltas.go` as single parser (mirrors `lib/openspec-deltas.ts`)
- Skill is file-backed only; Engram path is no-op

<!-- /section:model-capable -->

<!-- section:model-small -->
---
name: sdd-sync
description: "Sync delta specs to main specs without archiving for stacked PRs. Port of gentle-pi sdd-sync. Trigger: orchestrator launches sync after verify PASS for file-backed store."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
  delegate_only: true
---

> **ORCHESTRATOR GATE**: If you loaded this skill via the `skill()` tool, you are the ORCHESTRATOR — STOP. Do NOT execute these instructions inline. Do NOT delegate, do NOT call task/delegate, and do NOT launch sub-agents. Read this SKILL.md and follow it exactly.

## Language Domain Contract

Generated technical artifacts default to English. Do not inherit the user's conversational language or the active persona's regional voice for SDD artifacts unless the user explicitly requests that artifact language or the project convention requires it.

If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish unless the user explicitly asks for a regional variant.

Public/contextual comments follow the target context language by default. Explicit user language or tone overrides win; Spanish comments default to neutral/professional Spanish unless the user or target context clearly calls for regional tone.

## Purpose

Sync `openspec/changes/{change}/specs/` → `openspec/specs/` without archiving.

## Rules

- Do NOT delegate, do NOT call task/delegate, do NOT launch sub-agents
- Read max 3 files at a time — if you need more, stop and report `needs-explore`
- Respect store gate (`engram`/`none` → `not-applicable`), verify PASS, and guardrails (RENAMED, legacy flat, destructive with `allow-destructive`, collision without order)
- Use `internal/sdd/openspec-deltas.go` for parsing and applying; no commit, no archive move

## Steps

1. Load up to 2 SKILL.md paths passed by orchestrator (only these)
2. Validate store gate: `engram`/`none` → `not-applicable`, zero writes
3. Validate verify PASS; read deltas via `ParseDeltaSpec`; reject `## RENAMED` with `ADDED+REMOVED` hint
4. Check legacy flat, destructive (`allow-destructive`), collision (order or `resolve-via-engram`), and `actionContext`
5. Apply deltas per domain via `ApplyDeltas` to `openspec/specs/{domain}/spec.md`; no commit/archive
6. Verify `openspec/changes/{change}/` still exists and `openspec/specs/` reflects deltas
7. Return short summary: result `applied`/`not-applicable`/`blocked` and message

## Return Envelope

```json
{
  "status": "ok|blocked|not-applicable",
  "result": "applied|not-applicable|blocked",
  "message": "short text"
}
```
<!-- /section:model-small -->
