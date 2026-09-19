# Proposal — fix-child-dialog-hang

## Intent

Make a delegated child's question impossible to turn into a silent, unbounded hang.

## Problem (reproduced)

Every `sdd-*` agent ships `ask_user_question` in its frontmatter. When a child calls it, pi's RPC
child emits `extension_ui_request{method:"select"}` and blocks until the client answers. The runtime
relays that request to the parent UI. When nobody answers — no UI, an unattended session, or a modal
the user never notices — the child blocks forever, the relay deliberately drops the idle watchdog
(a dialog awaiting the user is not idle), and only the 30-minute total bound can end the run. The
user sees a silent hang and no tool result.

Evidence (real pi parent, real `sdd-propose` child, Windows):

- Hang: parent relayed the child's question at t+24s, child alive, `subagent` tool never returned —
  `%TEMP%\biggz-repro\repro-sdd.log`; a session replay shows the same shape in the user's own
  `~/.pi/agent/sessions/.../01a0b739-*.jsonl` (last entry: the `subagent` tool call, no result).
- Not the payload, not the argv, not the write timing (prior sessions rejected all three).
- The relay itself is sound: with the question answered, the child resumes and the tool completes in
  ~4s (`ask2-cancel.log`, `ask2-first.log`). The defect is only the *unanswered* case.
- The child asks non-deterministically (same prompt, one run asked, one run did not), which is why
  the failure looks random and why headless smokes never covered it.

## Approach

1. Bound every relayed question with `dialogMs` (`DEFAULT_DIALOG_TIMEOUT_MS = 120s`,
   `BIGGZ_SUBAGENT_DIALOG_MS` override, `0` answers immediately).
2. Pass the same budget to the parent dialog (`ctx.ui.select/confirm/input/editor` opts) so the host
   renders its own countdown; the runtime's guard timer is the backstop for hosts that ignore it.
3. At the bound the child receives exactly one `{cancelled:true}` — it degrades and continues instead
   of blocking. A late human answer must not answer the same request twice.
4. Keep the idle bound honest: a pending dialog still suspends it, but the dialog bound always fires,
   so no relay can hold a run open.
5. Surface it: announce the question on the parent UI, mark `awaiting answer: <title>` in the status
   line, and append the unanswered-question notice to the `subagent` tool result.

## Rollback

Revert the runtime asset and redeploy (`go install ./cmd/biggz && biggz install --agent pi --yes`).
`BIGGZ_SUBAGENT_DIALOG_MS` can also be raised to restore effectively-unbounded waiting without a
code change.

## Out of scope

- Making the orchestrator the only party that asks a human (child-side prompt contract) — issue #139
  territory (`subagent_parent_message` / `subagent_reply`).
- Dismissing the modal from the parent when the child settles.
