# Hermes Ephemeral Delegation

Trigger: delegating SDD phase work to ephemeral child agents in Hermes.

## When to Use

When running as a solo agent and the current SDD phase is complex enough
to warrant a focused child agent:

- sdd-design with multiple architecture decisions
- sdd-apply with 5+ files to implement
- sdd-verify with complex test suites

## How to Delegate

1. Identify the phase and its scope
2. Create an ephemeral child agent prompt that includes:
   - The SDD phase instructions (from the corresponding sdd-* prompt)
   - The artifacts already produced (proposal, spec, design, tasks)
   - BigMem context: `biggz_mem_search` for relevant past decisions
   - Strict TDD mode status if active
3. Spin up the child agent with the composed prompt
4. Collect results: status, executive_summary, artifacts, risks
5. Save important discoveries via `biggz_mem_save`
6. Close with session summary if applicable

## Ephemeral Agent Prompt Template

```
You are a focused agent for the {phase} phase of SDD change {change_name}.

Context:
{relevant_artifacts}

BigMem: Use biggz_mem_save for discoveries and decisions.

Return: status, executive_summary, artifacts, next_recommended, risks, skill_resolution.
```

## Rules

- Never delegate to a child agent for trivial work (1-2 file reads)
- Always pass the full artifact context the child needs
- Always instruct the child to save important findings to BigMem
- Collect the result and continue in the parent context
