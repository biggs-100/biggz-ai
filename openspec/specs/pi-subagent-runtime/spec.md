# pi-subagent-runtime Specification

## Purpose

Own pi subagent delegation runtime: tool contract `subagent`/`subagent_wait` (+ status/result/cancel/agents), one `pi --mode rpc` child per task, strict JSONL framing, stall watchdog and process-tree kill, background completion delivery, rendering exclusively through pi-tui width primitives (safe by construction), and a width regression harness.

## Requirements

### Requirement: Tool Surface and Registration Gate

The runtime MUST register delegation tools `subagent` (modes `task`, `background`) and `subagent_wait`, plus companions `subagent_status`, `subagent_result`, `subagent_cancel`, `subagent_agents`. Agent discovery MUST read markdown agents from `~/.pi/agent/agents/*.md` honoring frontmatter `tools`/`model`. Legacy j0k3r-era names (`subagent_run`, `subagent_list_*`) MUST NOT register except as temporary transition aliases. Registration MUST occur only when `pi-subagents-j0k3r` is absent; dual registration MUST NOT occur.

#### Scenario: Contract tools registered

- GIVEN the runtime active and `pi-subagents-j0k3r` absent
- WHEN the extension loads
- THEN `subagent`, `subagent_wait`, `subagent_status`, `subagent_result`, `subagent_cancel`, `subagent_agents` MUST be callable

#### Scenario: No dual registration

- GIVEN `pi-subagents-j0k3r` present in Pi packages
- WHEN the runtime extension loads
- THEN it MUST NOT register any tool

#### Scenario: Agents listed with bounded error

- GIVEN markdown agents exist under `~/.pi/agent/agents/`
- WHEN `subagent_agents` runs
- THEN it MUST list them; an unknown agent id MUST return a bounded error naming available agents

### Requirement: Foreground Task Mode and Child Protocol

`subagent` with `mode:"task"` MUST run exactly one `pi --mode rpc` child per task and return its result to the caller. Child stdout MUST be strict JSONL: one JSON object per LF-terminated line; `readline` MUST NOT be used. Parsing MUST be defensive — malformed or partial lines MUST be logged and skipped without crashing the session. Children MUST run with `PI_SUBAGENT_CHILD=1`.

#### Scenario: Foreground run returns result

- GIVEN a discovered agent and a task
- WHEN `subagent` runs with `mode:"task"`
- THEN one `--mode rpc` child MUST spawn with `PI_SUBAGENT_CHILD=1` and the result MUST return to the caller

#### Scenario: Malformed JSONL tolerated

- GIVEN a child emits a non-JSON line
- WHEN the parser reads it
- THEN it MUST log and skip it and the run MUST continue

### Requirement: Child Interactivity Parity

Child `ask_user_question` MUST relay to the parent session and the user's answer MUST return to the child. Steering MUST reach the running child.

#### Scenario: Dialog relay round-trip

- GIVEN a running child asks a question
- WHEN the parent presents it
- THEN the answer MUST return to the child and the run MUST continue

#### Scenario: Steering forwarded mid-run

- GIVEN a running child
- WHEN the user steers the task
- THEN the message MUST be delivered to the child

### Requirement: Stall Watchdog, Kill, and Cancel Semantics

The runtime MUST terminate a child exceeding its configured total in-flight bound or, when no tool execution is announced in flight, its idle (no output) bound, escalating `SIGTERM`→`SIGKILL`. The runtime MUST track `tool_execution_start`/`tool_execution_end` and MUST suspend the idle bound while at least one announced tool is in flight; the total bound MUST keep governing. When the last in-flight tool ends, the idle bound MUST re-arm. If a child line is dropped for exceeding the framing cap, the runtime MUST conservatively re-arm the idle bound. Cancel MUST kill the entire child process tree (Windows included) and mark the task cancelled — never crash the session.

#### Scenario: Idle stall suspended while a tool is in flight

- GIVEN a child that announced an in-flight tool with `tool_execution_start`
- WHEN the child stays silent beyond the idle bound
- THEN the idle watchdog MUST NOT fire and the child MUST stay running

#### Scenario: Idle stall killed

- GIVEN a child silent beyond the idle bound with no tool in flight or after the last `tool_execution_end`
- WHEN the watchdog fires
- THEN the child MUST be terminated and the task MUST report a stall failure

#### Scenario: Cancel kills the tree

- GIVEN a running background task
- WHEN `subagent_cancel` is called
- THEN the whole child tree MUST be dead and the task state MUST be cancelled

### Requirement: Background Mode, Completion Delivery, and Concurrency Cap

`mode:"background"` MUST return task ids immediately and deliver each completion as a session notification/message without polling. `subagent_wait` MUST render a bounded headline: one line `Wait {elapsed}s · {N} runs ({summaries})` plus at most one dim hint (≤2 lines total), never a full run-list dump. Concurrent background tasks MUST honor the runtime cap (default ≤2) and the four-source `BIGGZ_BACKGROUND_SUBAGENTS` policy integration MUST stay unchanged.

#### Scenario: Background ids then completion notice

- GIVEN two independent read-only tasks
- WHEN both launch with `mode:"background"`
- THEN ids MUST return immediately and each completion MUST be delivered without polling

#### Scenario: Wait headline ≤2 lines

- GIVEN runs `sdd-apply running`, `sdd-verify queued`, elapsed 23s
- WHEN `subagent_wait` renders
- THEN output MUST be `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued)` plus optional dim hint, ≤2 lines

#### Scenario: Cap queues excess runs

- GIVEN cap 2 with two background tasks active
- WHEN a third background task launches
- THEN it MUST queue until a slot frees

### Requirement: Safe Rendering by Construction

All runtime rendering MUST use pi-tui cell-aware primitives (`visibleWidth`, `truncateToWidth`, `wrapTextWithAnsi`); the runtime MUST NOT implement independent width math. Background runs MUST render a minimal status widget through the same primitives.

#### Scenario: Emoji card does not overflow

- GIVEN a completion card with two `✅` (2 cells each) at a width where code-point math overflows
- WHEN rendered
- THEN every line MUST measure ≤ terminal width via `visibleWidth` and pi's fail-closed guard MUST NOT trip

#### Scenario: No foreign width math

- GIVEN the runtime sources
- WHEN scanned for width measurement
- THEN it MUST appear only via pi-tui imports

### Requirement: Width Regression Harness

The system MUST ship a `node --test` width harness with fixtures for emoji (`✅`), CJK, ZWJ sequences, and ANSI/OSC; the asymmetric property `measure(line) >= piTui.visibleWidth(line)` MUST hold and wrapped output MUST NOT exceed target width.

#### Scenario: Original crash shape passes

- GIVEN a fixture recreating the `w=190` vs width `188` shape with 2× `✅`
- WHEN the harness runs
- THEN no measured line MUST exceed `visibleWidth`

#### Scenario: Property holds for all fixtures

- GIVEN emoji/CJK/ZWJ/ANSI fixtures
- WHEN `node --test` runs
- THEN `measure >= visibleWidth` MUST hold and wrap output MUST be ≤ width
