---
name: sdd-new
description: "Start a new SDD change from a brief description. Scaffolds the change directory and hands off to explore or propose. Trigger: sdd new, nuevo cambio, new change"
license: MIT
metadata:
  author: biggz-ai
  version: '1.0'
---

# SDD New

Start a new SDD change. This is the entry point for all change workflows. Creates the change directory, initializes metadata, and routes to the appropriate starting phase.

## Activation Contract

1. Verify SDD is initialized (runs `biggz sdd-status`).
2. Derive change name from user description — must be kebab-case.
3. Create change directory under `openspec/changes/`.
4. Write initial metadata file (`_meta.yaml`).
5. Decide whether to enter explore or propose phase based on intent clarity.
6. Delegate to the appropriate phase skill.

## Hard Rules

- Change names MUST be kebab-case (`add-auth`, `fix-n+1-query`). Reject PascalCase, snake_case, or spaced names with a clear error message.
- Each change name must be unique under `openspec/changes/`. If a directory exists, do NOT overwrite — error with `/sdd-continue {name}` suggestion.
- If user intent is unclear (vague description, open-ended goal, multiple possible solutions), route to sdd-explore.
- If user intent is clear and concrete (specific capability, bounded scope), route to sdd-propose directly.
- Always write `_meta.yaml` before delegating — the target phase reads it.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Not initialized | `biggz sdd-status` reports no openspec/config.yaml | Run sdd-init first, then retry |
| Clear intent | Description is specific (e.g. "add JWT auth middleware") | Skip explore, go to propose |
| Unclear intent | Description is vague (e.g. "improve performance") | Enter explore phase |
| Existing change | `openspec/changes/{change-name}/` already exists | Error — suggest different name or `/sdd-continue {name}` |
| Too large | Description spans multiple unrelated capabilities | Suggest splitting into multiple changes |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Verify SDD state** — run `biggz sdd-status`. If status shows "not initialized", run sdd-init first with brief explanation to the user. If init fails, stop and report.
3. **Parse input** — extract change name and description:
   - First positional argument → change name (convert to kebab-case automatically).
   - Remaining text → change description.
   - If no description, prompt the user before proceeding.
4. **Check uniqueness** — verify `openspec/changes/{change-name}/` directory does not exist. If it does, return an error showing the existing path and suggesting `/sdd-continue`.
5. **Create directory** — create `openspec/changes/{change-name}/`.
6. **Write initial metadata** — create `openspec/changes/{change-name}/_meta.yaml`:
   ```yaml
   name: {change-name}
   created: {ISO-8601 timestamp}
   phase: explore | propose
   description: "{user description}"
   status: active
   ```
7. **Route to phase** — based on intent clarity:
   - **Clear** → call sdd-propose skill.
   - **Unclear** → call sdd-explore skill.
   - **Ambiguous** → ask one clarifying question, then route.
8. **Persist** — save change metadata to Engram with the created phase and description.

## Output Contract

```yaml
status: success | blocked
executive_summary: "Created change 'add-auth' at openspec/changes/add-auth/. Starting propose phase."
artifacts:
  - path: openspec/changes/{change-name}/_meta.yaml
    type: change-metadata
    summary: "Change metadata with name, description, and initial phase"
next_recommended: explore | propose
risks:
  - description: "If intent was unclear, explore phase may reveal scope changes"
    severity: low
skill_resolution: fallback-path
```

## Domain Templates

When creating a spec, use one of the following templates from
`openspec/templates/` as a starting point:

| Change type | Template |
|-------------|----------|
| API endpoint | `openspec/templates/api-endpoint.md` |
| CLI command | `openspec/templates/cli-command.md` |
| Bug fix | `openspec/templates/bug-fix.md` |
| Refactor | `openspec/templates/refactor.md` |
| Database migration | `openspec/templates/database-migration.md` |

Each template includes:
- REQ-N numbered requirements
- GIVEN/WHEN/THEN scenarios
- Edge case tables
- Error handling scenarios
- Migration or rollback plans

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-new.md`
- `../sdd-init/SKILL.md`
- `../sdd-explore/SKILL.md`
- `../sdd-propose/SKILL.md`
- `openspec/templates/` (domain spec templates)
- `openspec/changes/{change-name}/_meta.yaml`
