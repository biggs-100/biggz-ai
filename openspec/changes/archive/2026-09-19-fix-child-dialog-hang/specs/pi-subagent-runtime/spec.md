# Delta for pi-subagent-runtime

## MODIFIED Requirements

### Requirement: Child Interactivity Parity

Children MUST NOT ship an ask tool: `ask_user_question` MUST NOT appear in the tool list the installer emits for any child agent, because asking the human is the orchestrator's job and a child that needs a decision returns `status: blocked` with its reason. A relayed child question MUST be bounded: when no answer arrives within the dialog bound the runtime MUST answer the child `{cancelled: true}` exactly once, so the child degrades and continues. The dialog bound MUST default to 120s and MUST be configurable (`BIGGZ_SUBAGENT_DIALOG_MS`, `0` answering immediately); the runtime MUST pass the same budget to the parent dialog and MUST own a backstop timer for hosts that ignore it, so exactly one `extension_ui_response` is written per question and a late human answer MUST NOT answer it twice. A background task's question MUST be answered `{cancelled: true}` at once and MUST NOT reach the parent UI (`dialogPolicy: "dismiss"`, recorded as `dialogsDismissed`). A foreground question MAY relay, MUST identify the asking agent in the title, and the runtime MUST record the relay (`dialogRelayed`, `dialogMissed`, `pendingDialog`, `dialogMs`) on the snapshot; the `subagent` tool result MUST state how many questions went unanswered or were dismissed. Steering MUST reach the running child.
(Previously: every child shipped `ask_user_question` and every question relayed to the parent UI regardless of mode, with no bound — so an unanswered question blocked the child until the total watchdog and left the user without any tool result.)

#### Scenario: Children cannot ask the human

- GIVEN the installer emits a child agent definition
- WHEN its tool list is written
- THEN `ask_user_question` MUST NOT be present

#### Scenario: Dialog relay round-trip

- GIVEN a running child asks a question
- WHEN the parent presents it and the user answers
- THEN the answer MUST return to the child and the run MUST continue

#### Scenario: Unanswered question cannot hang the run

- GIVEN a running child that asked a question
- WHEN no answer arrives within the dialog bound
- THEN the child MUST receive `{cancelled: true}` exactly once and the run MUST continue to settlement
- AND the snapshot MUST record the miss and the tool result MUST report it

#### Scenario: Late answer after the bound

- GIVEN a question already answered `cancelled` at the bound
- WHEN the user answers it afterwards
- THEN the child MUST NOT receive a second response for that question

#### Scenario: Background question is dismissed, not relayed

- GIVEN a background task whose child asks a question
- WHEN the runtime sees the request
- THEN the child MUST receive `{cancelled: true}` immediately, the parent UI MUST NOT be asked, and the dismissal MUST be recorded as `dialogsDismissed`

#### Scenario: Foreground relay names the asker

- GIVEN a foreground child asks a question
- WHEN the parent dialog is presented
- THEN the title MUST identify the asking agent

#### Scenario: Dialog bound is configurable

- GIVEN `BIGGZ_SUBAGENT_DIALOG_MS` is set to a non-negative integer
- WHEN a child question is relayed
- THEN that value MUST be the bound used for the parent dialog and the backstop timer

#### Scenario: Steering forwarded mid-run

- GIVEN a running child
- WHEN the user steers the task
- THEN the message MUST be delivered to the child

### Requirement: Stall Watchdog, Kill, and Cancel Semantics

The runtime MUST terminate a child exceeding its configured total in-flight bound or, when no tool execution is announced in flight, its idle (no output) bound, escalating `SIGTERM`→`SIGKILL`. The runtime MUST track `tool_execution_start`/`tool_execution_end` and MUST suspend the idle bound while at least one announced tool is in flight; the total bound MUST keep governing. When the last in-flight tool ends, the idle bound MUST re-arm. If a child line is dropped for exceeding the framing cap, the runtime MUST conservatively re-arm the idle bound. A dialog awaiting the user MUST suspend the idle bound but MUST NOT suspend the dialog bound, which MUST fire and re-arm the idle bound. Cancel MUST kill the entire child process tree (Windows included) and mark the task cancelled — never crash the session. The `subagent` tool MUST honor pi's abort signal (its third `execute` parameter, documented as how Esc cancels extension work): aborting a running foreground delegation MUST cancel the task, kill the child tree and settle with a result that reports the cancellation. The runtime MUST attach an `error` listener to the child's stdin, so writing a command to a child that already exited MUST NOT raise an uncaught exception in the parent.
(Previously: a pending dialog cleared the idle timer with no upper bound of its own, so a run could only be ended by the 30-minute total bound; the tool ignored pi's abort signal, so a running delegation could not be cancelled at all; and a write to a dead child's stdin raised an uncaught `EPIPE` inside pi on the settle/kill path.)

#### Scenario: Idle stall suspended while a tool is in flight

- GIVEN a child that announced an in-flight tool with `tool_execution_start`
- WHEN the child stays silent beyond the idle bound
- THEN the idle watchdog MUST NOT fire and the child MUST stay running

#### Scenario: Idle stall killed

- GIVEN a child silent beyond the idle bound with no tool in flight or after the last `tool_execution_end`
- WHEN the watchdog fires
- THEN the child MUST be terminated and the task MUST report a stall failure

#### Scenario: Pending dialog suspends idle but not the dialog bound

- GIVEN a relayed question that no one answers and an idle bound longer than the dialog bound
- WHEN the dialog bound expires
- THEN the child MUST be answered `cancelled`, the question MUST stop being pending, and the idle bound MUST re-arm

#### Scenario: Cancel kills the tree

- GIVEN a running background task
- WHEN `subagent_cancel` is called
- THEN the whole child tree MUST be dead and the task state MUST be cancelled

#### Scenario: Esc cancels a running delegation

- GIVEN a running foreground delegation
- WHEN pi aborts the turn (the human presses Esc)
- THEN the task MUST be cancelled, the child tree killed, and the tool MUST settle reporting the cancellation

#### Scenario: Dead child stdin cannot throw in the parent

- GIVEN a child that already exited
- WHEN the runtime writes a prompt or abort command to its stdin
- THEN the write failure MUST be contained and MUST NOT raise an uncaught exception in pi

## ADDED Requirements

### Requirement: Live Run Visibility

The runtime MUST derive a bounded activity label from the child's own RPC events — the announced tool plus its target, or `thinking` while it writes — and expose it as `lastStep` on the task snapshot, so the widget row and `subagent_status`/`subagent_list_tasks` answer what a run is doing rather than only that it is alive. The runtime MUST accumulate the child's reported tokens and cost from its assistant messages and expose them (`tokens`, `cost`, `model`) on the snapshot; rows MAY end with that spend. `elapsedMs` MUST be frozen when the task settles. A finished run MUST remain visible in the widget for `WIDGET_FINISHED_TTL_MS` (60s, at most `WIDGET_FINISHED_MAX` = 3) so its completion glyph is seen before it fades. The runtime MUST register a `/biggz-agents` command that opens an overlay listing every run with its activity and spend, where up/down move the selection, `s` cancels the selected run and q/Escape closes; the panel's render MUST be total, so a failure degrades to one line instead of freezing the TUI.

#### Scenario: Live activity reaches every surface

- GIVEN a running child that announced a tool
- WHEN the snapshot is read or a row is rendered
- THEN `lastStep` MUST name that tool and its target, and the status line and widget row MUST show it

#### Scenario: Spend accumulates and time freezes

- GIVEN a child whose assistant messages report usage
- WHEN the task settles
- THEN `tokens`/`cost` MUST reflect the accumulated usage and `elapsedMs` MUST stop growing

#### Scenario: Completions stay visible briefly

- GIVEN a task that just finished
- WHEN the widget refreshes within `WIDGET_FINISHED_TTL_MS`
- THEN its row MUST still be published, and MUST disappear after the TTL

#### Scenario: The panel lists, stops and closes

- GIVEN active runs and the `/biggz-agents` command
- WHEN the panel is open
- THEN it MUST list each run with activity and spend, `s` MUST cancel the selected run, and q/Escape MUST close it

#### Scenario: A broken panel cannot freeze the TUI

- GIVEN the panel's data source throws
- WHEN the panel renders
- THEN it MUST degrade to a single error line
