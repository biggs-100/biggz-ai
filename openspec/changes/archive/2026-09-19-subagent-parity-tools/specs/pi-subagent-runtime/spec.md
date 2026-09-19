# Delta for pi-subagent-runtime

## ADDED Requirements

### Requirement: Task Listing Tool (subagent_list_tasks)

The runtime MUST register `subagent_list_tasks`, listing this session's in-memory ring records (limit 50) newest first, settled tasks included, one bounded entry per record. An empty ring MUST return a bounded empty-state result, not an error; the tool MUST read no source outside the session ring.

#### Scenario: Newest first, settled included

- GIVEN a session ring holding settled `t1`, then settled `t2`, then running `t3`
- WHEN `subagent_list_tasks` runs
- THEN output MUST list `t3`, `t2`, `t1` in that order and settled records MUST be included

#### Scenario: Empty ring bounded empty state

- GIVEN no task recorded this session
- WHEN `subagent_list_tasks` runs
- THEN a bounded empty-state message MUST return and MUST NOT error

#### Scenario: Bounded by the ring limit

- GIVEN the ring holds its 50-record limit with older records evicted
- WHEN `subagent_list_tasks` runs
- THEN at most 50 records MUST list and no disk or BigMem source MUST be read

### Requirement: Task Messaging Tool (subagent_send_message)

The runtime MUST register `subagent_send_message` with parameters `task_id` and `message`, forwarding the message to a live child as a steer. When the steer cannot be delivered — unknown or expired `task_id`, or a task not running — a bounded error naming the id MUST return; it MUST NOT throw or crash the session.

#### Scenario: Steer forwarded to running child

- GIVEN a running task `sub-abc`
- WHEN `subagent_send_message` runs with `task_id: "sub-abc"` and `message: "change of plan"`
- THEN a `steer` command carrying the message MUST reach the child and a non-error result MUST return

#### Scenario: Unknown or expired task

- GIVEN no ring record exists for `sub-gone` (never created or evicted)
- WHEN `subagent_send_message` targets `sub-gone`
- THEN a bounded error naming `sub-gone` MUST return and no exception MUST propagate

#### Scenario: Non-running or settled task

- GIVEN a task that cannot receive a steer (queued without a live child, or settled)
- WHEN `subagent_send_message` targets it
- THEN a bounded error naming the task id MUST return and the session MUST NOT crash

## MODIFIED Requirements

### Requirement: Tool Surface and Registration Gate

The runtime MUST register delegation tools `subagent` (modes `task`, `background`) and `subagent_wait`, plus companions `subagent_status`, `subagent_result`, `subagent_cancel`, `subagent_agents`, `subagent_list_tasks`, `subagent_send_message`. Agent discovery MUST read markdown agents from `~/.pi/agent/agents/*.md` honoring frontmatter `tools`/`model`. Legacy j0k3r-era names (`subagent_run`, j0k3r's own `subagent_list_*`) MUST NOT register except as temporary transition aliases, and the runtime's own `subagent_list_tasks` MUST NOT be classified as one of them nor cause registration refusal. Registration MUST occur only when `pi-subagents-j0k3r` is absent; dual registration MUST NOT occur.
(Previously: six tools; the blanket `subagent_list_*` prohibition captured `subagent_list_tasks` and would refuse registration.)

#### Scenario: Contract tools registered

- GIVEN the runtime active and `pi-subagents-j0k3r` absent
- WHEN the extension loads
- THEN `subagent`, `subagent_wait`, `subagent_status`, `subagent_result`, `subagent_cancel`, `subagent_agents`, `subagent_list_tasks`, `subagent_send_message` MUST be callable

#### Scenario: No dual registration

- GIVEN `pi-subagents-j0k3r` present in Pi packages
- WHEN the runtime extension loads
- THEN it MUST NOT register any tool

#### Scenario: Agents listed with bounded error

- GIVEN markdown agents exist under `~/.pi/agent/agents/`
- WHEN `subagent_agents` runs
- THEN it MUST list them; an unknown agent id MUST return a bounded error naming available agents

#### Scenario: Runtime's own list tool is not legacy

- GIVEN the runtime's own tool surface includes `subagent_list_tasks` and `pi-subagents-j0k3r` is absent
- WHEN the legacy j0k3r-era classification and the registration gate evaluate
- THEN `subagent_list_tasks` MUST NOT be classified as a j0k3r-era name and its presence MUST NOT cause registration refusal

#### Scenario: Genuine legacy names still refused

- GIVEN a tool surface presenting genuine j0k3r-era names (`subagent_run`, `subagent_list_running`)
- WHEN the registration gate evaluates
- THEN registration MUST be refused
