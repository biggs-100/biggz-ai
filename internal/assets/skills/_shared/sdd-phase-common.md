---
name: sdd-phase-common
description: Common protocol loaded by every SDD phase skill. Provides skill loading, artifact retrieval, persistence, and return envelope conventions.
---

# SDD Phase — Common Protocol

Boilerplate identical across all SDD phase skills. Sub-agents MUST load this alongside their phase-specific SKILL.md.

Executor boundary: every SDD phase agent is an EXECUTOR, not an orchestrator. Do the phase work yourself. Do NOT launch sub-agents, do NOT call `delegate`/`task`, and do NOT bounce work back unless the phase skill explicitly says to stop and report a blocker.

## A. Skill Loading

1. Check if the orchestrator injected a `## Skills to load before work` block in your launch prompt. If yes, read those exact `SKILL.md` files before task-specific work.
2. If no skills block was provided, check for `SKILL: Load` instructions. If present, load those exact skill files.
3. If neither was provided, search for the skill registry as a fallback:
   a. `biggz_mem_search(query: "skill-registry", project: "{project}")` — if found, `biggz_mem_get_observation(id)` for full content
   b. Fallback: read `.atl/skill-registry.md` from the project root if it exists
   c. From the registry's skills index, match triggers to your task and read the exact listed `SKILL.md` paths.
4. If no registry exists, proceed with your phase skill only.

NOTE: the preferred path is (1) — exact skill paths selected by the orchestrator. Paths (2) and (3) are fallbacks. Searching the registry is SKILL LOADING, not delegation. If `## Skills to load before work` is present, IGNORE redundant `SKILL: Load` instructions.

## B. Artifact Retrieval (BigMem Mode)

**CRITICAL**: `biggz_mem_search` returns 300-char PREVIEWS, not full content. You MUST call `biggz_mem_get_observation(id)` for EVERY artifact. **Skipping this produces wrong output.**

**Run all searches in parallel** — do NOT search sequentially.

```
biggz_mem_search(query: "sdd/{change-name}/{artifact-type}", project: "{project}") → save ID
```

Then **run all retrievals in parallel**:

```
biggz_mem_get_observation(id: {saved_id}) → full content (REQUIRED)
```

Do NOT use search previews as source material.

Read priority: when both BigMem and file-system copies exist, read the BigMem copy first and fall back to the file-system copy only if BigMem returns no results (per `persistence-contract.md`).

## C. Artifact Persistence

Every phase that produces an artifact MUST persist it. Skipping this BREAKS the pipeline — downstream phases will not find your output.

For `verify-report`, first build exact candidate bytes and run `biggz sdd-verify-validate` with authoritative requirement/scenario counts before any OpenSpec or BigMem write. If the validator is unavailable or denies admission, make zero writes and leave the prior report untouched; otherwise persist only the same admitted bytes, including a valid `fail`.

### BigMem mode

```
biggz_mem_save(
  title: "sdd/{change-name}/{artifact-type}",
  topic_key: "sdd/{change-name}/{artifact-type}",
  type: "architecture",
  project: "{project}",
  capture_prompt: false,
  content: "{your full artifact markdown}"
)
```

`topic_key` enables upserts — saving again updates, not duplicates.
`capture_prompt: false` is mandatory for SDD artifacts because they are automated pipeline outputs, not human/proactive memory saves. Set it when the BigMem tool schema supports it; if an older schema rejects or does not expose the field, omit it rather than failing.

### OpenSpec mode

File was already written during the phase's main step. No additional action needed.

### Hybrid mode

Do BOTH: write the file to the filesystem AND call `biggz_mem_save` as above.

### None mode

Return result inline only. Do not write any files or call `biggz_mem_save`.

## D. Return Envelope

> **CRITICAL — Response ordering**: Your FINAL output MUST be text (the return envelope), NOT a tool call. If you need to save to BigMem (`biggz_mem_save`), do it BEFORE your final text response. Do NOT call `biggz_mem_session_summary` — that's for top-level agents only. **Why**: When a sub-agent's last action is a tool call, the parent agent receives only the tool result — your text response (the actual analysis) is lost.

Every phase MUST return a structured envelope to the orchestrator:

- `status`: `success`, `partial`, `blocked`, or `failed`
- `executive_summary`: 1-3 sentence summary of what was done
- `detailed_report`: (required — MUST include Preview first 300 chars + Diff summary inline even when artifact persisted)
- `artifacts`: list of artifact keys/paths written
- `next_recommended`: the next SDD phase to run, or "none"
- `risks`: risks discovered, or "None"
- `skill_resolution`: how skills were loaded — `paths-injected` (received exact skill paths from orchestrator), `fallback-registry` (self-loaded paths from registry), `fallback-path` (loaded via SKILL: Load path), or `none` (no skills loaded)

Example:

```markdown
**Status**: success
**Summary**: Proposal created for `{change-name}`. Defined scope, approach, and rollback plan.
**Artifacts**: BigMem `sdd/{change-name}/proposal` | `openspec/changes/{change-name}/proposal.md`
**Next**: sdd-spec or sdd-design
**Risks**: None
**Skill Resolution**: paths-injected — 3 skills (react-19, typescript, tailwind-4)
(other values: `fallback-registry`, `fallback-path`, or `none — no registry found`)
```

## E. Review Workload Guard & Allowed Edit Surfaces

Allowed edit surfaces (MANDATORY): exact repository-relative paths or narrow globs, never '.' and never bare repo root — pre-existing untracked targets listed explicitly, directories where new files authorized when task requires new files, nothing beyond delegated task. If surface cannot be derived, do not launch writer, present candidate as approve/decline (not free-text path question).

SDD must protect reviewer cognitive load, not only generate tasks.

- The default PR review budget is **400 changed lines** (`additions + deletions`).
- Count authored text additions plus deletions only for this threshold. Generated goldens are excluded from authored risk count but remain included in complete snapshot identity and receipt validation.
- The orchestrator MUST cache a delivery strategy at session start: `ask-on-risk` (default), `auto-chain`, `single-pr`, or `exception-ok`. Those four are the whole domain.
- Any other `delivery_strategy` value is invalid. A phase MUST NOT map it to the nearest branch, MUST NOT record it in an artifact, and MUST NOT forward it: report the unrecognised value and stop.
- The orchestrator MUST pass `delivery_strategy` to `sdd-tasks` and the resolved decision to `sdd-apply`.
- `sdd-tasks` MUST forecast whether the planned work may exceed that budget.
- The forecast MUST include exact plain-text guard lines: `Decision needed before apply: Yes|No`, `Chained PRs recommended: Yes|No`, and `400-line budget risk: Low|Medium|High`.
- If the forecast is high, `sdd-tasks` MUST recommend chained or stacked PRs using deliverable work units.
- `sdd-apply` MUST NOT start oversized work unless the delivery strategy resolves to chained/stacked PR slices or explicitly accepted `size:exception`.
- Each chained PR slice must have a clear start, clear finish, autonomous scope, verification, and reasonable rollback.
- In a Feature Branch Chain, PR #1 targets the feature/tracker branch and later child PRs target the immediate previous PR branch; if GitHub shows previous slices in a child diff, retarget/rebase until the diff is clean.

This guard exists to reduce reviewer burnout and keep implementation delivery safe. Do not treat it as optional process noise.

## F. Key Learnings Closing

Close your **final report message** (the return envelope) with a `## Key Learnings` section to enable BigMem passive capture.

**Format**: numbered list with 1–5 items. Each item is a standalone factual sentence that is ≥20 characters and ≥4 words.

**Example**:

```markdown
## Key Learnings

1. Async validation in the apply phase caught a race condition in concurrent writes.
2. Golden test regeneration for system prompts requires the `-update` flag before re-run.
3. Bounded review contracts must stay consistent across `sdd-phase-common.md` and `bigmem-protocol.md`.
```

This applies to your final text response to the orchestrator, not intermediate tool outputs or artifact content. BigMem will automatically extract and persist these learnings.
