---
name: agents-md
description: "Trigger: new project setup, AGENTS.md, agent skills index. Scaffold a workflow-compatible AGENTS.md so agents load skills on demand without breaking biggz-ai routing."
license: Apache-2.0
metadata:
  author: biggz-ai
  version: "1.0"
---

## Activation Contract

Create or repair a project's `AGENTS.md` when:
- A project has no agent entry point and agents guess instead of loading skills
- An existing `AGENTS.md` routes around the workflow (invokes `sdd-*` directly, duplicates ceremony, narrates memory)
- New project skills exist but are not registered in any index

Do not create one when the project already has a working index that the
registry resolves — repair the broken rows instead.

## Hard Rules

- Copy `assets/AGENTS.template.md` as the starting point; never invent a new format.
- Keep the `biggz-compat` block verbatim: `delegate_only` phases, quiet
  ceremony, invisible memory, and the register-every-skill rule.
- One row per skill: name, trigger, exact relative `SKILL.md` path. Paths must
  resolve from the repo root or the entry is worse than useless.
- Reference workflow docs; never duplicate their content into the index.
- Index skills, not phases: `sdd-*` rows stay marked delegate only.

## Decision Gates

| Need | Action |
|------|--------|
| No `AGENTS.md` exists | Scaffold from template, fill rows from skill scan |
| Index exists but stale | Update rows, keep compat block, verify paths |
| Project-specific conventions | Add one short paragraph above the table, not new sections |
| Unsure a skill qualifies | If it has a trigger and a path, it gets a row |

## Execution Steps

1. Scan the project's skills (`skills/`, `internal/assets/skills/`, or equivalent) for name, trigger, and path.
2. Copy the template to the repo root as `AGENTS.md` (or repair in place).
3. Fill one table row per skill; verify every path resolves.
4. Confirm the compat block is present and verbatim.
5. Report: file path, row count, and any unresolvable paths.

## Output Contract

- `AGENTS.md` path and row count.
- List of skills registered vs skipped (with reason).
- Any paths that did not resolve.

## References

- `assets/AGENTS.template.md` — starting template with compat block.
