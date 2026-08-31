# codegraph-change-intent Specification

## Purpose

Infer change intent as a full dependency + call graph from SDD artifacts and Go source, emitting dual JSON + Markdown consumed advisory by humans and the orchestrator.

## Requirements

### Requirement: SDD Artifact Intent Extraction

The system MUST extract intent signals from `openspec/changes/{change}/` artifacts (`proposal.md` REQUIRED, `spec.md`/`design.md`/`tasks.md` OPTIONAL) by parsing keywords and symbols; symbols MUST be weighted higher than keywords and each signal MUST carry a `sdd` reason tag.

#### Scenario: Proposal-only extraction succeeds

- GIVEN `proposal.md` exists and `spec.md`/`design.md`/`tasks.md` are absent
- WHEN inference runs for `<change>`
- THEN the system MUST derive file hints from proposal keywords/symbols with `sdd` reasons

#### Scenario: Missing proposal blocks inference

- GIVEN `openspec/changes/{change}/proposal.md` is absent
- WHEN inference runs
- THEN the system MUST fail with a non-zero exit and message `proposal required`

#### Scenario: Symbol weight exceeds keyword weight

- GIVEN proposal contains keyword `payment` and symbol `PaymentService`
- WHEN both match the same file
- THEN the `sdd` reason for the symbol match MUST rank higher

### Requirement: Go Dependency and Call Graph Scan

The system MUST scan Go source for dependency and call edges using `go/packages` with stdlib-parse fallback, Go-only scope, cached package loads, and a 30s timeout; it MUST NOT scan non-Go files.

#### Scenario: Import and call edges discovered

- GIVEN a Go repo with `a imports b` and `func A calls B.Do()`
- WHEN the scan runs under `go/packages`
- THEN edges MUST include `a -> b` with reason `import` and `A -> B.Do` with reason `call`

#### Scenario: Scan timeout enforced

- GIVEN a large repo where scan exceeds 30s
- WHEN the timeout fires
- THEN the system MUST abort and return a timeout error without partial JSON

### Requirement: Full Graph with Transitive Closure

The system MUST build a full graph `{nodes, edges}` combining dependency + call edges and expand with transitive closure; output MUST NOT be a flat list. Isolated files with only `sdd` reasons MUST still appear as nodes.

#### Scenario: Transitive closure expands blast radius

- GIVEN intent hits `A`, `A imports B`, `B calls C`
- WHEN the graph is built
- THEN nodes MUST include `A,B,C` and edges MUST include `A->B`, `B->C`, plus derived `A->C` transitive edge

#### Scenario: Flat-list guard

- GIVEN inference completes
- WHEN JSON is emitted
- THEN `graph.nodes` and `graph.edges` MUST be present and non-empty when files are reported

### Requirement: Dual Output JSON and Markdown Emission

The system MUST emit JSON `{files:[{path,reasons}], graph:{nodes,edges}}` with `reasons` in `{sdd,import,call}` and Markdown at `openspec/changes/{change}/codegraph.md`; custom paths via `--json`/`--md` MUST override defaults and parent dirs MUST be created.

#### Scenario: Default dual emission

- GIVEN `biggz codegraph report <change> --cwd .` with no custom flags
- WHEN the command succeeds
- THEN JSON MUST be written to stdout or default path and `codegraph.md` MUST exist under `openspec/changes/{change}/`

#### Scenario: Custom paths override

- GIVEN `--json /tmp/out.json --md /tmp/out.md`
- WHEN report runs
- THEN JSON MUST be at `/tmp/out.json` and Markdown at `/tmp/out.md`

### Requirement: Advisory Consumption by Human and Orchestrator

The report MUST be advisory only, visible to humans (Markdown) and optionally readable by the orchestrator (JSON) for scope hints; the system MUST NOT auto-scope or modify SDD artifacts without human approval and generated `codegraph.md` MUST be git-ignored-safe to delete.

#### Scenario: Human reads Markdown

- GIVEN `codegraph.md` exists after report
- WHEN a human opens it
- THEN it MUST list files with reasons and a rendered graph summary

#### Scenario: Orchestrator optional hint

- GIVEN the orchestrator starts spec/design and report JSON exists
- WHEN it checks `openspec/changes/{change}/codegraph.md` / JSON path
- THEN it SHOULD surface file hints advisory and MUST NOT auto-apply edits if JSON is absent
