# Archive Report — subagent-parity-tools

```yaml
schema: biggz-ai.archive-report/v1
change: subagent-parity-tools
archived_at: "2026-09-19 (orchestrator-specified archive date; local 2026-09-18 evening (−05:00) = 2026-09-19 UTC)"
archived_to: openspec/changes/archive/2026-09-19-subagent-parity-tools/
store: openspec (file-backed; BigMem mirror of this report saved as sdd/subagent-parity-tools/archive-report as a closure signal — the filesystem copy is the authority)
verification: pass_with_warnings (admitted — 3/3 requirements, 11/11 scenarios, 0 CRITICAL, 0 blockers; 1 pre-existing WARNING out of scope)
evidence_revision: sha256:d1f0f13c6161336b4364b7367fe5f933b25dc7eaaa07eb74570c15dcae37f56f
committed: false (this phase committed nothing; the move and the synced living spec are working-tree changes)
delivery: DONE — PR #140 MERGED into master at 6e382e7a3039c7062935a7886cf32bd87e436909 (2026-09-19T00:48:25Z); issue #138 CLOSED
review_gate: unreviewed-at-archive (no RDD review lineage exists for this change; status v2 emits only a mode-only reviewOffer, which per the native contract never governs archive — see Review gate disposition)
```

## Change

`subagent-parity-tools` — add `subagent_list_tasks` and `subagent_send_message` (steer) to the biggz pi
subagent runtime tool surface, reaching tool-surface parity with gentle-shell (issue #138). Delivered
scope:

- **Runtime asset** `internal/assets/pi/biggz-subagent-runtime.js` — `LEGACY_TOOL_RE` narrowed to the
  exact j0k3r-era names (`/^subagent_run$|^subagent_list_running$/`) so the runtime's own
  `subagent_list_tasks` is no longer captured by the blanket `subagent_list_*` prohibition and cannot
  cause registration refusal; `subagent_list_tasks` reads only the in-memory session ring
  (`registry.all().slice(-registry.limit).reverse()`), newest first, settled included, bounded empty
  state; `subagent_send_message` gates on `running`/`spawning` then forwards `task.steer(message)`,
  with bounded non-throwing errors for unknown/expired/non-running ids; tool surface 6 → 8.
- **Tests** `internal/assets/pi/biggz-subagent-runtime.test.mjs` — legacy classification, own-8-name
  surface registration gate, list ordering/empty/ring-cap, send running/unknown/queued/settled, and
  surface assertions 6 → 8.
- **Specs** — delta sync into living `pi-subagent-runtime`: 2 ADDED requirements (6 scenarios) + 1
  MODIFIED requirement (3 → 5 scenarios) — see Sync at archive.

## Delivery (final state — rank: native review/attempt ledger + corroborated repository evidence)

| Fact | Evidence |
|------|----------|
| PR #140 MERGED | `gh pr view 140`: state MERGED, mergeCommit `6e382e7a3039c7062935a7886cf32bd87e436909`, mergedAt `2026-09-19T00:48:25Z`, title `feat(pi): add subagent_list_tasks and subagent_send_message tools` |
| Merge commit on master | HEAD = `6e382e7a`; `git branch --contains 6e382e7a` lists `master` + `origin/master` |
| Issue #138 CLOSED | `gh issue view 138`: state CLOSED, title `Subagent tool surface parity: add subagent_list_tasks and subagent_send_message (steer)` |
| CI (PR #140) | `gh pr checks 140`: all latest runs pass — 18 distinct check names green (Test/E2E ubuntu-osx-windows, Go Format, Complexity, TestRapid, Release Checksums Smoke, Skill Lint, Provider Contract, Lint No Source-Grep, Forbid Git Exec Outside Wrapper, No fmt.Sprintf in lens prompts, Check Issue Reference, Check PR Has type:* Label, Check PR Cognitive Load) |
| Cognitive-load check | `statusCheckRollup` retains 2 earlier `Check PR Cognitive Load` FAILURE entries superseded by a later SUCCESS — consistent with the maintainer's `size:exception` pass reported in the handoff |
| Handoff count note (recorded, not resolved) | The orchestrator handoff stated "CI 19/19"; GitHub's PR rollup readback exposes 18 distinct check names, all green. The one-count difference is not reproducible from `gh` output; it does not change the pass state (no failing latest check stands) |
| Post-merge master run | Merge-commit check suite at readback: 14/15 completed/success, `Test (windows-latest)` `in_progress` (fresh post-merge run, not a failure) |

## Archive date & final path

- **Archive date**: 2026-09-19 — the orchestrator specified this ISO prefix (UTC date of the merged
  delivery); the local clock at close was 2026-09-18 evening (−05:00).
- **Final path**: `openspec/changes/archive/2026-09-19-subagent-parity-tools/` — moved in one
  `Move-Item` (same-volume rename, the `os.Rename` equivalent; `internal/sdd.ArchiveChange` has no
  CLI wrapper in this repo, so the rename was performed directly).
- The source directory `openspec/changes/subagent-parity-tools/` no longer exists
  (`Test-Path` = False); the active changes directory now contains only `archive/`.
- No `.biggz-instance` existed in the change (checked pre- and post-move with `-Force`) — nothing to
  rename-preserve, nothing deleted anywhere. `.biggz/` at the repo root is the project config
  directory (gallery + config.yaml) and was not part of the change folder.

## Artifact inventory (post-move, 8 files + specs/)

| Artifact | Size | Status | Notes |
|----------|------|--------|-------|
| `_meta.yaml` | 259 | present | left untouched (audit trail; still reads `phase: propose`) |
| `proposal.md` | 3,578 | present | intent/scope/approach/rollback |
| `design.md` | 6,335 | present | decisions, interface contracts, file-change table |
| `specs/pi-subagent-runtime/spec.md` | 4,449 | present | delta spec preserved intact (sha256 `5c456c51cfc991308ef78c73db912db1be0734e5eec2553bc1ac7f4a36c3dce3`) |
| `tasks.md` | 3,308 | present | 16/16 checked, 0 unchecked (Task Completion Gate PASS) |
| `apply-progress.md` | 7,334 | present | implementation snapshot (history) |
| `verify-report.md` | 10,025 | present | admitted PASS WITH WARNINGS; evidence `sha256:d1f0f13c…` |
| `state.yaml` | 1,262 | present | left intact (pending-question envelope + synthesis; audit trail) |
| `archive-report.md` | this file | present | added at archive |

## Sync at archive (delta → living spec — performed by this phase)

Native status had flagged `sync` with a single blocked reason:
`sync blocked: destructive change (REMOVED or large MODIFIED) for domain "pi-subagent-runtime"
without explicit approval; add allow-destructive to prompt`. This phase performed the merge under the
orchestrator's explicit launch instruction (the human `proceed` decision recorded in the change's
`state.yaml` pending-question envelope) — the required explicit approval. The merge was verified
non-destructive: zero requirements removed; only the MODIFIED block's two stale lines were replaced.

**Target**: `openspec/specs/pi-subagent-runtime/spec.md`

| Action | Delta source | Result in living spec |
|--------|--------------|-----------------------|
| MODIFIED `Tool Surface and Registration Gate` | delta's full updated requirement | replaced the old 6-tool block: 8 tools registered, `subagent_list_*` prohibition narrowed to j0k3r's own names, `(Previously: …)` note kept; scenarios 3 → 5 (`Contract tools registered` updated + `Runtime's own list tool is not legacy` + `Genuine legacy names still refused`) |
| ADDED `Task Listing Tool (subagent_list_tasks)` | delta's requirement | appended with 3 scenarios (newest-first settled included; empty ring bounded non-error; ring-limit bound, no disk/BigMem read) |
| ADDED `Task Messaging Tool (subagent_send_message)` | delta's requirement | appended with 3 scenarios (steer forwarded to running child; unknown/expired bounded error; non-running/settled bounded error, no crash) |

**Merge integrity evidence (fresh, read-only):**

| Check | Before | After |
|-------|--------|-------|
| Requirements `### Requirement:` | 7 | 9 (each new name exactly once) |
| Scenarios `#### Scenario:` | 17 | 25 |
| Other 6 requirements | — | preserved byte-identically (diff deletions are exactly the 2 replaced Tool Surface lines) |
| sha256 | `b6cad1c10fa32f8d583980471b6e8e09810f431d4a29b27830df3b972197cf71` | `e5ad850ca6fc4ee06de0c0a789f8aafb2d0ed766aeed622334b760319838f2ed` |
| `git diff --stat` | — | `1 file changed, 59 insertions(+), 2 deletions(-)` |

Delta occurrence counts in the living spec: `subagent_list_tasks` × 11, `subagent_send_message` × 7,
`(Previously:` × 1. No duplicates, no removed requirement resurrected, no untouched requirement
perturbed.

## Final verification (admitted)

- **Verdict**: PASS WITH WARNINGS — 3/3 requirements and 11/11 scenarios compliant with covering
  tests in the fresh run; 0 blockers, 0 CRITICAL findings.
- **Evidence**: `sha256:d1f0f13c6161336b4364b7367fe5f933b25dc7eaaa07eb74570c15dcae37f56f` (fresh
  evidence bundle; canonical test output hash `sha256:09d87219…`), `test_exit_code: 0`,
  `build_exit_code: 0`.
- **Suites** (per admitted verify-report): focused `biggz-subagent-runtime.test.mjs` 50/50 pass;
  full pi suite `node --test --test-force-exit internal/assets/pi/*.test.mjs` 193/193 pass; Go gates
  `go vet ./...` exit 0 and `go test ./internal/install/... ./internal/assets/biggz -count=1` green.
- **Static checks**: `node --check` clean on both changed JS files.

## Ledger final shape (native `sdd-attempt`, read-only)

- Revision `00ddd0cf287321b60e99c6b16d3789feb22ed422b9b358b3b78a44238ac3fc15`, **Complete: true**,
  next action `complete`, 2 attempts, generation 2, 0 active attempts, decision needed false.
- `parity-tools-impl` settled **passed**, evidence
  `sha256:2da5fe5a3ba603b709096396e945099dbcfb027a0175db2125bfa92994312cd4` (handoff final-state
  fact; rank 3, consistent with the terminal `complete` ledger state read back in this phase).
- `parity-tools-verify` settled **passed**, evidence
  `sha256:d1f0f13c6161336b4364b7367fe5f933b25dc7eaaa07eb74570c15dcae37f56f` — byte-equal to the
  admitted verify-report's `evidence_revision`.
- The residual `blocked reason: work_unit_complete` is the ledger's normal terminal projection for a
  completed scope (it explains that continuing would require a successor work unit); it is not an
  open obligation for this change.

## Final-state authority & snapshot reconciliation

- `verify-report.md` and `apply-progress.md` are intermediate snapshots (rank 4). Their "done" claims
  stay true; their open items were re-checked against the final state: no verify warning was fixed in
  a later commit — the single WARNING is carried forward below as a known residual.
- Rank 3 (launch-prompt final-state facts): PR #140 merged at `6e382e7a`, issue #138 closed, CI pass;
  corroborated in this phase by GitHub and git readbacks (see Delivery).
- Rank 1/2 (ledger + persisted tasks artifact): ledger `complete: true` with both work units settled
  `passed`; archived `tasks.md` 16/16 `[x]`, 0 `[ ]`.
- `findings/tasks` reconciliation: the archived `tasks.md` contains no stale unchecked task — no
  exceptional reconciliation was needed.

## Review gate disposition (declared)

**Facts at archive time (fresh-checked):**

- **Kill switch**: RDD global mode = `enabled` (`~/.biggz/rdd-mode.json`, recorded
  `2026-09-18T02:15:35.3166103Z`); no repo-local override (`.git/biggz/rdd-mode` absent). The skill's
  `disabled/unmanaged` relaxation therefore does not apply.
- **Review transaction state**: **none exists for this change.** `biggz review list` (fresh) shows no
  `subagent-parity-tools` lineage, and the git-common review store
  (`.git/biggz/review-transactions/`) contains no entry matching `subagent|parity`. No
  `review-subject.json` exists in the change.
- **Native status shape**: the change's status (`biggz-ai.sdd-status/v2`) reports
  `artifactStore: openspec`, all artifacts `done`, tasks `allComplete: true`,
  `dependencies.archive: ready`, and a `reviewOffer` (`{available, invocation}`) as the only
  review-shaped field. Status v2 deliberately **forbids** `reviewGate`/`reviewTransaction`/receipt
  fields (guard: `internal/sdd/status_v2_test.go:119`), and the native contract states a
  `reviewOffer` "never authorizes, blocks, or governs archive or delivery"
  (`_shared/sdd-status-contract.md`). The archive dependency was `ready`.
- **Delivery is already terminal**: the change shipped via PR #140 (merged) with the repo's
  `pre-pr`/`pre-push` gates enabled (`.biggz/config.yaml`), and CI green. Any future review would be
  a post-delivery action; nothing about this change is pending, malformed, scope-changed,
  invalidated, or escalated in the review store.

**Disposition**: archive proceeds as **unreviewed-at-archive** — no review lineage exists, and none
governs this change under the native contract. Precedent (same treatment, recorded not hidden):
`2026-09-17-pi-subagent-runtime`, `2026-09-16-fix-sdd-sync-empty-spec`.

**Declared tension with the skill letter**: the archive skill's Native Review Receipt Gate asks for
`reviewGate.result: allow` or `disabled/unmanaged`; at this archive, neither literal value is
emitted (kill switch on, no transaction). Recorded explicitly instead of fabricated; no repository
state was mutated that would prevent a different reading (the move is a plain rename; nothing
committed).

## Residual warnings (final state)

1. **EPIPE stdin seam (WARNING, pre-existing, out of delta scope)** — `writeCommand`
   (`internal/assets/pi/biggz-subagent-runtime.js:338-346`) writes to `child.stdin` with no `'error'`
   listener; a write racing a killed/dying child can surface `EPIPE` as an `uncaughtException`. Not
   introduced by this diff; the delta's three `subagent_send_message` scenarios are proven
   non-throwing. Recorded in `apply-progress.md` as a follow-up candidate (and observed again as a
   test-harness artifact during apply). **Known residual, does not falsify any delta scenario.**
2. **Stale line references in planning artifacts (SUGGESTION, info)** — `tasks.md`/`design.md` cite
   pre-change line numbers (`listTasks` at `:951-961` vs actual `:964-972`; surface assertions
   `:632`/`:672` vs actual `:650`/`:727`). Symbols and structure are correct.
3. **`TASK_RING_LIMIT = 50` literal not directly asserted (SUGGESTION, info)** — cap behavior is
   tested at limits 2 and 3; an assertion pinning the default 50 would close the literal gap.

None of these blocks or falsifies the delivered behavior; all are WARNING/SUGGESTION level.

## State reconciliation notes

- `tasks.md` is the persisted authority: 16/16 `[x]`, 0 `[ ]` (fresh count in the archived file) —
  Task Completion Gate PASS, no reconciliation needed.
- `verify-report.md` / `apply-progress.md` are history snapshots; their "done" claims stay true and
  their open items are reconciled above.
- `_meta.yaml` left untouched (audit trail convention; still `phase: propose`). `state.yaml` left
  intact (its pending-question envelope and synthesis record the human `proceed` decision for this
  archive). The delta spec is preserved under `specs/` inside the archive.
- BigMem mirror: this report is also saved as `sdd/subagent-parity-tools/archive-report` (closure
  signal for BigMem-derived status); the filesystem copy remains the authority.

## Post-archive hygiene (branch / worktree) — read-only, nothing deleted

Nothing was deleted (no `branch -d`, no `-D`, no worktree pruning) — this phase's constraints
explicitly forbid anything beyond sync + move + report, and the run is non-interactive:

- Current branch `docs/archive-subagent-parity-tools` at `6e382e7a`; single worktree
  `C:/Users/USER/Desktop/biggz-ai`.
- `git branch -vv` reports 0 local branches in `[gone]` state.
- `git fetch --prune --dry-run` reported one stale remote-tracking ref
  (`origin/ci-probe/child`) that a real prune would remove — nothing was removed by this phase.
- Hint for CI: use `--dry-run`; non-TTY runs delete nothing.

## Archive verification checklist

- [x] Sync precondition satisfied before move (fresh status: all artifacts `done`; sync prepared and
      applied in this phase under explicit orchestrator approval).
- [x] No CRITICAL verify issues (0 blockers, 0 critical; verdict PASS WITH WARNINGS, admitted
      evidence `sha256:d1f0f13c…`).
- [x] Task Completion Gate: 16/16 checked, 0 unchecked in the persisted artifact.
- [x] Delta synced into the living spec BEFORE the move: MODIFIED replaced, 2 ADDED appended, other 6
      requirements preserved; before/after counts and hashes recorded (9 req / 25 scenarios;
      `e5ad850c…`).
- [x] Change folder moved to `openspec/changes/archive/2026-09-19-subagent-parity-tools/` with all
      artifacts (8 files + delta spec); source path confirmed absent.
- [x] Active changes directory clean of this change (only `archive/` remains).
- [x] No `.biggz-instance` existed; nothing deleted anywhere.
- [x] Nothing committed; the move + synced living spec + this report remain working-tree changes.
- [x] Archive report persisted (this file) and mirrored to BigMem as a closure signal.
- [x] Review gate disposition recorded explicitly (unreviewed-at-archive; no lineage; native
      contract does not gate archive on a receipt).

## Working tree at close (caused by this phase only)

`git status --short`:

```
 D openspec/changes/subagent-parity-tools/_meta.yaml
 D openspec/changes/subagent-parity-tools/apply-progress.md
 D openspec/changes/subagent-parity-tools/design.md
 D openspec/changes/subagent-parity-tools/proposal.md
 D openspec/changes/subagent-parity-tools/specs/pi-subagent-runtime/spec.md
 D openspec/changes/subagent-parity-tools/state.yaml
 D openspec/changes/subagent-parity-tools/tasks.md
 D openspec/changes/subagent-parity-tools/verify-report.md
 M openspec/specs/pi-subagent-runtime/spec.md
?? openspec/changes/archive/2026-09-19-subagent-parity-tools/
```

The deletions are the moved paths' old locations (the archive move is untracked yet); the modified
living spec is the delta sync. No other file was touched, and nothing was committed.
