# Delta for sdd

## Purpose

Enforce SDD workflow discipline: mandatory pre-delegation reads, Work Routing Ladder fail-closed, and native dispatcher routing. SDD selected only by explicit intent or accepted proposal; size/count/risk alone never selects SDD.

## Requirements

### Requirement: REQ-SDD-001 — Mandatory Workflow and Delegation Reads

Orchestrator MUST read `biggz-orchestrator-workflow.md` (workflow, graph, dispatcher, gates, ledger, recall) and `biggz-orchestrator-delegation.md` (ladder, rules, authority, surfaces) before delegating SDD work. Reads MUST be via file read and evidenced in launch prompt. Skipped/unreadable MUST fail-closed.

#### Scenario: Both docs read before delegation

- GIVEN `/sdd-continue` trigger
- WHEN both docs have been read then `sdd-spec` launched
- THEN prompt MUST evidence reads and delegation MUST proceed

#### Scenario: Skipped read blocks

- GIVEN workflow doc skipped
- WHEN routing to any `sdd-*`
- THEN it MUST block with mandatory-read error

### Requirement: REQ-SDD-002 — Work Routing Ladder Fail-Closed

System MUST enforce ladder: 1) Inline Direct (typo, one-file, 1–3 known files), 2) Simple Delegation (`explore` scout, `general` worker/verify), 3) SDD (optional). SDD MUST be selected ONLY on explicit request (`biggz sdd-new`/`sdd-continue` or direct ask) or accepted proposal; size/file-count/risk alone MUST NOT select SDD. MAY suggest SDD when durable artifacts reduce ambiguity.

#### Scenario: Large diff without SDD request does not launch SDD

- GIVEN 12 files, 800 lines, no explicit SDD request
- WHEN ladder evaluated
- THEN it MUST NOT launch `sdd-propose`; select Simple Delegation MAY suggest SDD

#### Scenario: Explicit SDD request selects SDD

- GIVEN user says `use SDD for this feature`
- WHEN ladder evaluated
- THEN it MUST select SDD via preflight/init guards

### Requirement: REQ-SDD-003 — Native Dispatcher Routing

Orchestrator MUST route via native dispatcher `biggz sdd-status --json --instructions` (`biggz sdd-continue <change>`) as single authority for `openspec`/`BigMem`/`hybrid`. MUST route only by `nextRecommended` + dependencies, never free-text. MUST respect `blockedReasons` and ledger attempt authority; `blocked` MUST stop launch.

#### Scenario: Dispatcher drives phase

- GIVEN `sdd-status` returns `nextRecommended==spec`
- WHEN routing
- THEN it MUST launch `sdd-spec` only

#### Scenario: Blocked stops apply

- GIVEN `sdd-status` `blockedReasons` non-empty for apply
- WHEN evaluating apply
- THEN it MUST NOT launch `sdd-apply` and MUST surface blockers

### Requirement: REQ-SDD-004 — SDD Phase Authority Mapping

Each SDD phase MUST map to `sdd-*` agent (`sdd-explore`, `sdd-research`, `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-archive`, `sdd-sync`). `general`/`explore` for SDD MUST be rejected by guard (`internal/sdd/synthesis_gate.go` + `internal/orchestrator/*`).

#### Scenario: Design maps to sdd-design

- GIVEN `nextRecommended==design`
- WHEN selecting agent
- THEN it MUST select `sdd-design` not `general`

#### Scenario: SDD explore uses sdd-explore

- GIVEN SDD-tracked change needs exploration
- WHEN launching explore for that change
- THEN it MUST use `sdd-explore`, reject `explore`
