schema: biggz-ai.sdd-preproposal/v1
revision: 3
exploration_outcome: done
exploration_ref: openspec/changes/wire-pi-review-relay/exploration.md
research_request: "upstream relay design: how the TypeScript product hosts review lenses (spawn mechanism, prompt binding, tool restriction, transport, timeouts) and the evidence behind the empty-stdout-on-Windows symptom"
research_classes:
  - local-worktree-snapshot
  - upstream-repo
  - github-issues
admission_outcome: admitted
evidence_outcome: done
openspec_ref: openspec/changes/wire-pi-review-relay/preproposal.md
engram_ref: sdd/wire-pi-review-relay/preproposal
decision: confirmed
proposal_ready: true

# Pre-proposal gate state — wire-pi-review-relay

Revision 3 closes the gate: exploration `done`, selected research lane `done`
with valid evidence references, product decisions `confirmed`. `sdd-propose`
may start from this handoff; the proposer MUST NOT interview or infer consent.

## Confirmed product decisions (maintainer, 2026-09-12)

1. **Surface** — `capture-result --agent pi --execute`: an explicit execute mode
   on the existing verb, mutually exclusive with `--input`, `--preflight` and
   `--materialize`, reusing Preflight → MaterializeReviewerTask → PiAdapter.Review
   → Capture without touching admission or the expected_revision CAS.
2. **Prompt binding** — the CLI embeds the reviewer role: it composes the full
   prompt (role from `internal/assets/prompts/review/*.md` + the materialized
   task) and feeds it to `PiAdapter` on stdin. No new pi host assets are required
   by this change; the host launcher remains a non-goal.
3. **Timeout/cancellation** — a bounded default timeout (10 minutes) with a
   `--timeout` override; cancelation kills the reviewer process and a timeout
   produces a typed failure with zero capture (never a partial capture).

## Evidence references (research lane, outcome `done`)

- `openspec/changes/wire-pi-review-relay/research.md` ↔ BigMem
  `sdd/wire-pi-review-relay/research` — `biggz-ai.sdd-research/v1`, revision 1,
  37 sources, claims C1–C33, contradictions/uncertainty/freshness recorded.
- `openspec/changes/wire-pi-review-relay/ln.md` ↔ BigMem
  `sdd/wire-pi-review-relay/ln` — `biggz-ai.sdd-ln/v1`, revision 1.
- Headline evidence: reviewer execution is a frozen print-mode subprocess with
  the prompt on stdin and raw stdout captured (upstream shape already mirrored by
  our adapter); upstream deliberately composes NO host-side role in the native
  lane, so our embedded-role decision is a documented divergence for design to
  specify; upstream bounds the reviewer with a scale-derived timeout (900s floor
  + 900s/MiB, 2h ceiling) while we chose a fixed 10-minute default; the Windows
  `pi-empty-output` symptom (#943) has a proven host-side typed mapping but an
  explicitly unproven root cause.

## Provenance note (hybrid store)

`research.md` is 49,377 bytes. It was mirrored to BigMem through the MCP save
path (no truncation) rather than `biggz bigmem save`, because the CLI truncates
content over 50 KB and Windows cannot carry a ~49 KB argument on a command line
(~32 KB limit); the CLI also does not read `-` from stdin. Both stores hold the
same revision 1 document; head/mid/tail phrases were verified present in the
BigMem copy.
