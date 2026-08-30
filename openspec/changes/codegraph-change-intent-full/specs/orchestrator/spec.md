# Delta for orchestrator

## ADDED Requirements

### Requirement: CodeGraph Advisory Scope Hint

The orchestrator MUST optionally read `biggz codegraph report` JSON (`{files:[{path,reasons}], graph:{nodes,edges}}`) when present under `openspec/changes/{change}/` to surface an advisory scope hint before spec/design; it MUST NOT auto-scope, auto-edit, or block SDD when the report is absent or stale, and MUST surface hints visibly in pre-spec output.

#### Scenario: Report present surfaces hint

- GIVEN `openspec/changes/{change}/codegraph.md` and JSON exist before spec phase
- WHEN orchestrator prepares scope
- THEN it SHOULD read JSON and surface `files` with reasons in its summary without modifying tasks

#### Scenario: Report absent continues normally

- GIVEN no JSON/Markdown report exists for `<change>`
- WHEN orchestrator evaluates scope
- THEN it MUST continue SDD without error and MUST NOT block `sdd-spec` or `sdd-design`

#### Scenario: Advisory does not auto-apply

- GIVEN report suggests files `[a.go, b.go]`
- WHEN orchestrator displays the hint
- THEN it MUST require explicit human approval before any edit or task scoping
