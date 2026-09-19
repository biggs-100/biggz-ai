# Archive Report — fix-child-dialog-hang

```yaml
schema: biggz-ai.archive-report/v1
change: fix-child-dialog-hang
archived_at: "2026-09-19 (local evening −05:00 = 2026-09-19 UTC)"
archived_to: openspec/changes/archive/2026-09-19-fix-child-dialog-hang/
store: openspec (file-backed; the filesystem copy is the authority)
verification: not-run-through-SDD — implemented and shipped direct by explicit user decision ("mejor hazlo tú sin sdd"); evidence = node:test 67/67, go test ./internal/assets/... ./internal/install/... green, go vet clean, plus live checks in the user's real pi TUI
evidence_revision: not-applicable (no sdd-attempt ledger entry exists for this change)
committed: false at archive time (the move and the synced living spec land as a docs PR)
delivery: DONE — PR #144 MERGED into master at ba486d03 (2026-09-19T22:58:59Z) and PR #145 MERGED at cd1a3dfe (2026-09-19T22:59:45Z); issues #142 and #143 CLOSED
review_gate: unreviewed-at-archive (no RDD review lineage exists for this change; it was implemented direct, outside the SDD flow)
```

## Change

`fix-child-dialog-hang` started as "an unanswered child question can hang a delegation" and grew, after live
probing, into the whole delegation surface. Delivered scope:

- **Dialog relay bounded** (`internal/assets/pi/biggz-subagent-runtime.js`) — `DEFAULT_DIALOG_TIMEOUT_MS`
  (120s, `BIGGZ_SUBAGENT_DIALOG_MS` override, `0` answers immediately) is passed to the parent dialog as its
  countdown with a runtime backstop timer; the child receives exactly one `{cancelled:true}` at the bound and
  continues; a late human answer never answers twice; the miss is recorded (`dialogRelayed`, `dialogMissed`,
  `pendingDialog`, `dialogMs`) and reported in the tool result.
- **Children never ask the human** (`internal/install/steps/pi_extensions.go`) — `ask_user_question` is gone
  from every child agent tool list; asking is the orchestrator's job and a blocked child returns
  `status: blocked`. A background child's question is answered `{cancelled:true}` at once and never reaches
  the parent UI (`dialogPolicy: "dismiss"`, `dialogsDismissed`); a foreground relay names the asking agent.
  This removed the defect the probes exposed: the relayed dialog is mounted in place of the composer with
  option 0 pre-highlighted and focused, so a stray Enter was swallowed as "select option 0" and the child
  acted on a decision nobody made.
- **Cancellation integrity** — the `subagent` tool honours pi's `AbortSignal` (previously ignored, so a
  running foreground delegation could not be cancelled and killing pi was the only exit), cancels the task
  tree and reports `cancelled by user (Esc)`; the child's stdin gained an `error` listener so a write to an
  already-exited child can no longer raise an uncaught `EPIPE` inside pi on the settle/kill path.
- **Live run visibility** — `lastStep` (the announced tool plus its target, or `thinking`) on the snapshot,
  feeding the widget row and `subagent_status`/`subagent_list_tasks`; tokens/cost accumulated from the
  child's usage; finished rows kept for 60s; `elapsedMs` frozen at settle; and the `/biggz-agents` overlay
  panel (up/down, `s` stop, q/Escape close) with a total render.
- **Tests** `internal/assets/pi/biggz-subagent-runtime.test.mjs` — 67/67 `node:test` (fake child gained
  `FAKE_TOOL_EVENT`/`FAKE_USAGE`), covering the dialog bound, late answer, background dismissal, asker label,
  foreground abort, activity label, spend formatting, finished-row TTL, frozen elapsed and the panel.

## Review gate disposition

No RDD review lineage was created: the user asked for the work to be done directly, so the change carries no
review transaction. The delivery path (two chained PRs, both green on 18/18 checks, each linked to an
approved issue and inside the 400-line budget) is recorded above instead.

## Living spec

`openspec/specs/pi-subagent-runtime/spec.md` was synced by hand: `Child Interactivity Parity` and
`Stall Watchdog, Kill, and Cancel Semantics` were rewritten to the shipped behaviour, and
`Live Run Visibility` was added. The living spec previously claimed a child's `ask_user_question` MUST relay
to the parent, which the shipped policy contradicts.
