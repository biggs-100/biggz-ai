---
name: sdd-onboard
description: Guide users through the full SDD workflow on their real codebase. Step-by-step walkthrough of all 8 phases with explanations.
trigger: orchestrator launches onboarding for the full SDD cycle.
---

# SDD Onboard

Walk a user through their first SDD change end-to-end. Each phase is explained, demonstrated, and executed on the real codebase. The goal is for the user to understand the what, why, and how of every phase.

## Activation Contract

1. Verify SDD is initialized — if not, run sdd-init with explanation.
2. Walk through each of the 8 SDD phases in order.
3. Explain each phase's purpose, input, output, and value.
4. Execute each phase on a real change (user-provided or sample).
5. Ensure the user understands the artifact chain by the end.

## Hard Rules

- Never skip a phase during onboarding — the point is to teach the full cycle. If the user wants to skip, document the purpose briefly and ask explicit confirmation.
- If SDD is not initialized, run sdd-init BEFORE starting the walkthrough.
- If the user has a real change, use it. If not, create a sample change (e.g. "add-health-check").
- The user must confirm understanding before moving to the next phase. Ask "ready to proceed?" after each phase explanation.
- Do NOT rush — onboarding is about understanding, not speed.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Not initialized | `openspec/config.yaml` missing or corrupt | Run sdd-init with explanation of what it does |
| User has real change | User provides a description | Use it for the walkthrough |
| User has no change | User just wants to learn | Create a sample change ("add-health-check") |
| User wants to skip phase | User says "I understand" | Skip only with explicit confirmation, still document the output |
| User gets stuck | User doesn't understand a phase | Explain with an analogy, then re-check understanding |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Check SDD state** — run `biggz sdd-status`. If not initialized, explain: "SDD needs a config file to track your changes. Let me set that up." Then run sdd-init.
3. **Give overview** — explain the SDD lifecycle with a visual:
   ```
   New → Explore → Propose → Spec → Design → Tasks → Apply → Verify → Archive
   ```
   One-sentence per phase:
   - **New**: Scaffold the change directory.
   - **Explore**: Investigate before committing (optional, for unclear ideas).
   - **Propose**: Define intent, scope, and rollback plan.
   - **Spec**: Write testable requirements (WHAT to build).
   - **Design**: Plan architecture, interfaces, data flow (HOW to build).
   - **Tasks**: Break into ordered work units.
   - **Apply**: Write the code.
   - **Verify**: Prove it works against requirements.
   - **Archive**: Preserve the artifact chain.
4. **Phase 1 — New** — explain: "This is where every change starts. We create a directory and metadata." Run `/sdd-new` with the change description. Show the `_meta.yaml` and directory structure.
5. **Phase 2 — Explore** — explain: "Sometimes we need to investigate before committing. We look at the codebase and compare approaches." If needed, run exploration. Show `exploration.md` structure.
6. **Phase 3 — Propose** — explain: "The proposal is our contract — what we're building, why, and how we undo it if it fails." Run sdd-propose or write collaboratively. Emphasize the rollback plan.
7. **Phase 4 — Spec** — explain: "Specs are testable requirements. Each one has GIVEN/WHEN/THEN scenarios." Write 2-3 requirements. Show the REQ-N format.
8. **Phase 5 — Design** — explain: "Design translates requirements into architecture. Interfaces, data flow, file changes." Write design doc. Show the decisions table.
9. **Phase 6 — Tasks** — explain: "Tasks break the design into ordered, verifiable chunks." Write 3-5 tasks. Show dependency ordering and test evidence.
10. **Phase 7 — Apply** — explain: "This is where we write code. Tasks tell us exactly what to build and in what order." Implement TASK-1 together. Run tests.
11. **Phase 8 — Verify** — explain: "Verification proves every requirement is met. Test suite + requirement mapping." Run verification. Show the verify report.
12. **Phase 9 — Archive** — explain: "Archive preserves the work and merges any new specs back." Run archive. Show the archive report.
13. **Artifact chain recap** — show the complete trail:
    ```
    _meta.yaml → exploration.md → proposal.md → spec.md → design.md → tasks.md → apply-progress.md → verify-report.md → archive-report.md
    ```
14. **Persist** — save onboarding completion to Engram with the change that was used.

## Output Contract

```yaml
status: success | blocked
executive_summary: "Completed SDD onboarding. User created and archived 1 change end-to-end."
artifacts:
  - path: openspec/changes/{sample-change}/
    type: onboarding-output
    summary: "Complete artifact chain from the walkthrough"
next_recommended: done
risks:
  - description: "User may need ongoing guidance for real changes — reference individual skill files as needed"
    severity: low
skill_resolution: user_input
```

## References

- `../_shared/sdd-phase-common.md`
- All phase skills: `../sdd-init/`, `../sdd-new/`, `../sdd-explore/`, `../sdd-propose/`, `../sdd-spec/`, `../sdd-design/`, `../sdd-tasks/`, `../sdd-apply/`, `../sdd-verify/`, `../sdd-archive/`
- `../../opencode/commands/sdd-init.md`
- `../../opencode/commands/sdd-new.md`
- `../../opencode/commands/sdd-status.md`
- `../../opencode/commands/sdd-continue.md`
- `../../opencode/commands/sdd-verify.md`
