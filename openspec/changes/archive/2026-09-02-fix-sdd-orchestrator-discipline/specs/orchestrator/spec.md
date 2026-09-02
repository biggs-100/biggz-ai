# Delta for orchestrator

## Purpose

Harden orchestrator discipline: block fast-forward inline, auto-continue without explicit token, SDD bypass via generic agents, and missing pre-delegation reads. Enforces 4-marker synthesis checkpoint, SD Agent Authority, mandatory workflow/delegation reads, and fail-closed routing.

## Requirements

### Requirement: REQ-ORCH-001 — Blocking Synthesis Checkpoint (120s)

The system MUST enforce `internal/sdd/synthesis_gate.go`. After EVERY sub-agent (SDD or non-SDD) the orchestrator MUST emit `## Sub-agent Result` with 4 markers (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) in current turn BEFORE any checkpoint ask. Gate MUST check `HasSynthesis` + `IsCheckpointAsk` (bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir`) + 120s window. Missing/expired MUST block `isError:true`/`{block:true}`. `## Session Recall` in current turn bypasses only then.

#### Scenario: Synthesis within window allows

- GIVEN 4 markers emitted 30s ago
- WHEN `ask_user_choice` with `proceed` evaluated
- THEN `ShouldBlock` MUST be `false`

#### Scenario: Missing or expired blocks

- GIVEN no synthesis or `now - currentTurnTime = 121s`
- WHEN checkpoint ask evaluated
- THEN `ShouldBlock` MUST be `true` and handler MUST NOT run

#### Scenario: Non-checkpoint never blocks

- GIVEN no synthesis, question `how are you?`
- WHEN `ShouldBlock` evaluated
- THEN it MUST be `false`

### Requirement: REQ-ORCH-002 — SD Agent Authority

SDD phases (`propose/spec/design/tasks/apply/verify/archive`, plus `explore/research` for SDD change) MUST use `sdd-*` agents only. `general`/`explore` for SDD MUST be rejected fail-closed with `SD Agent Authority` error. `general` remains allowed for non-SDD bounded work only.

#### Scenario: SDD via general rejected

- GIVEN orchestrator tries `general` for `spec` artifact
- WHEN guard evaluates
- THEN it MUST error with `SD Agent Authority` and NOT launch

#### Scenario: SDD via sdd-* allowed

- GIVEN orchestrator launches `sdd-apply` for SDD change
- WHEN guard evaluates
- THEN it MUST allow

### Requirement: REQ-ORCH-003 — Mandatory Pre-Delegation Reads

Orchestrator MUST read `biggz-orchestrator-workflow.md` and `biggz-orchestrator-delegation.md` before routing/continuing/delegating any SDD request. Launch prompt MUST evidence reads. Unreadable MUST block routing.

#### Scenario: Reads evidence routing

- GIVEN both docs read this session
- WHEN routing `sdd-spec`
- THEN it MUST proceed and prompt MUST contain workflow/delegation context

#### Scenario: Missing read blocks

- GIVEN delegation doc skipped/unreadable
- WHEN delegating SDD work
- THEN it MUST block with mandatory-read error

### Requirement: REQ-ORCH-004 — No Fast-Forward Inline or Auto-Continue

Orchestrator MUST NOT inline-write SDD spec/design/tasks artifacts that replace delegated `sdd-*` execution, and MUST NOT auto-continue without explicit token (`proceed|adjust|stop` or `continue|correct`, bilingual). `auto` preflight MAY auto-continue only when gate passes; otherwise MUST STOP and await confirmation. File count/size alone MUST never select SDD.

#### Scenario: Fast-forward inline blocked

- GIVEN SDD spec artifact requested without explicit inline scope
- WHEN ladder evaluated
- THEN it MUST delegate to `sdd-spec`, NOT inline-write

#### Scenario: Interactive auto-continue blocked

- GIVEN preflight `interactive`, spec done
- WHEN evaluating launch of `sdd-design` without `proceed`
- THEN it MUST STOP and emit synthesis + checkpoint first
