# Delta for orchestrator

## ADDED Requirements

### Requirement: Explicit Intent Required

The orchestrator MUST require explicit user intent before editing, applying, or continuing SDD work. Investigative phrases (`investigate`, `explore`, `check`, `look into`) and conditional phrases (`if possible`, `maybe`, `consider`, `when ready`) MUST NOT be treated as permission. Pre-proposal gate MUST have confirmed product decisions, valid evidence references, and matching hybrid state; selected research `done` MUST block `propose` until satisfied. The orchestrator MUST offer `sdd-research` after `sdd-explore` and treat selection as mandatory.

#### Scenario: Explicit intent permits apply

- GIVEN user says `apply the fix to internal/sdd/status.go`
- WHEN orchestrator evaluates intent
- THEN it MUST treat as explicit permission and may launch `sdd-apply`

#### Scenario: Investigate does not grant permission

- GIVEN user says `investigate the status bug`
- WHEN orchestrator evaluates intent
- THEN it MUST NOT treat as permission to edit files
- AND it MUST limit to read-only exploration

#### Scenario: Conditional does not grant permission

- GIVEN user says `fix it if possible` or `consider updating the task`
- WHEN orchestrator evaluates intent
- THEN it MUST NOT auto-apply edits and MUST ask for explicit confirmation

#### Scenario: Research blocks propose until done

- GIVEN `sdd-research` was selected and status is `partial`
- WHEN orchestrator attempts `sdd-propose`
- THEN it MUST block and report `blockedReasons` with research incompleteness
- AND MUST NOT invoke proposer

#### Scenario: Unselected research bypasses gate

- GIVEN `sdd-research` was not selected
- WHEN orchestrator evaluates pre-proposal gate
- THEN `propose` MUST be allowed when decisions are `confirmed` and references valid
