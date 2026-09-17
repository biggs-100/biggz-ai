# Archive Report — fix-sdd-sync-empty-spec

```yaml
schema: biggz-ai.archive-report/v1
change: fix-sdd-sync-empty-spec
archived_at: "2026-09-16 (UTC, house convention; matches the sync/verify evidence timestamps of 2026-09-16)"
archived_to: openspec/changes/archive/2026-09-16-fix-sdd-sync-empty-spec/
store: hybrid (filesystem wins for artifacts; the BigMem mirror of this report carries the closure signal)
verification: pass_with_warnings (admitted — 1/1 requirements, 8/8 scenarios, 0 CRITICAL, 0 blockers)
evidence_revision: sha256:06b63f76344950e57e4bba93d70f37a8ac100a6b2120e2aa83e31b2d30225ce5
committed: false (this phase committed nothing; the move and the synced living spec ride the final PR)
delivery: delivered — PR #102 a89f3493 + PR #103 e2adc96c merged to master; master head e2adc96c; issue #101 closed by the merge
```

## Change

`fix-sdd-sync-empty-spec` (issue #101) — close the sync write path's silent false-green: a NEW domain
whose delta file is full-spec shaped (`# X Specification` + `## Purpose` + `### Requirement:` blocks,
no `## ADDED Requirements` section) parses to zero deltas, and the old unconditional write produced a
0-byte living spec while reporting `applied`. The change reworks `syncApplyDeltas` /
`syncResolveDomainWrite` in `internal/sdd/sync_helpers.go` into a fail-closed resolve→write contract:
(1) absent living spec + exactly one full-spec source → byte-identical copy + `applied`; (2) content
without requirement blocks → `blocked` naming file+domain+remedy, no write; (3) no empty write ever —
`applied` only follows a content write. `sync.go` is untouched (its `res != ""` propagation already
carries `blocked`); the contract is pinned by the resolver unit table (7 sub-cases) and integration
tests T1–T5.

Origin: this is defect **F1** recorded during the `fix-false-green-guards` archive (its sync had to
work around exactly this with a manual verbatim copy). This change pays that debt down.

## Archive date & final path

- **Archive date**: 2026-09-16 (UTC; today's ISO date at close).
- **Final path**: `openspec/changes/archive/2026-09-16-fix-sdd-sync-empty-spec/` — same dated
  convention as `2026-09-16-fix-false-green-guards`.
- The source directory no longer exists under `openspec/changes/`; the active directory contains only
  `archive/`.
- No `.biggz-instance` existed in this change (checked pre-move) — nothing to rename-preserve.

## Artifact inventory

| Artifact | Status | Notes |
|----------|--------|-------|
| `_meta.yaml` | present | name/created/phase/description; left untouched (audit trail — it still reads `phase: explore`; `state.yaml` is the state authority) |
| `exploration.md` | present | defect + reproducer study (authored by the orchestrator — see `sdd-explore-readonly-artifact-gap`) |
| `proposal.md` | present | scope/approach/rollback |
| `design.md` | present | D1–D8, data flow, file-changes table (`sync.go` verified-no-change) |
| `specs/sdd/spec.md` | present | sole delta: `Sync Execution Contract` MODIFIED, 1 requirement / 8 scenarios (2 preserved + 6 new) |
| `tasks.md` | present | 27/27 checked, 0 unchecked (persisted artifact — Task Completion Gate PASS; Phases 1–4, every task with an evidence line) |
| `verify-report.md` | present | PASS WITH WARNINGS (admitted); evidence `sha256:06b63f…25ce5`; candidate `d516cf66` |
| `review-subject.json` | present | written by verify for the RDD gate (`{"repository":…,"commit_sha":"HEAD"}`); no RDD review ran |
| `apply-progress.md` | present + archive addendum | Phases 1–4 snapshot (RED/GREEN/delivery STOP). Addendum appended at archive from BigMem `obs-1789583484495097300-1` with the post-verify facts (complexity fix, executed delivery split) so the trail is self-contained |
| `state.yaml` | created at archive | No `state.yaml` ever existed for this change (filesystem + BigMem checked). Created in the house style of `2026-09-16-fix-false-green-guards/state.yaml`: phases done through `archive` (with `sync`), no `pending_question` (never had one), delivery/deviations blocks, four findings appended to `discovered_defects` |
| `archive-report.md` | present | this report |

Not produced: `sync-report.md` (the sync landed through the canonical `sdd.Sync()` path; its facts
live in this report) — same precedent as `2026-09-16-fix-false-green-guards`.

## Sync at archive (sdd — already applied, not re-run)

The delta → canonical application landed before the move through the canonical `sdd.Sync()` path
(`result=applied`, settled as `sync-spec-deltas: passed`); this phase did **not** re-run it and did
**not** touch `openspec/specs/**` (the edit rides the final PR as a tracked working-tree change).

| Domain | Action | Delta (req/scen) | Living spec after (req/scen) |
|--------|--------|------------------|------------------------------|
| sdd | MODIFIED (replace-in-place) | 1/8 | 30/79 (was 30/73 → +6 scenarios) |

**Verification of the applied result (fresh reads at archive)**:

- `git diff --stat openspec/specs/sdd/spec.md` → `1 file changed, 44 insertions(+)` — matches the
  sync's `+44/−0`.
- Anchored counts: 30 `### Requirement:` before (HEAD) vs 30 after (worktree); **name-set diff
  verified empty** (sorted `sed`-extracted name lists, byte-compared).
- `#### Scenario:` 73 before vs 79 after; `Sync Execution Contract` 2 → 8 scenarios (fresh count).
- Untouched requirements and heading hierarchy preserved (single in-place MODIFIED replacement).

**Anchor drift note**: earlier artifacts cited the requirement at `:110/:132` then `:108/:130`; it
now sits at `:130` (verbatim at verify). The stable anchor is the requirement NAME, not the line
number — recorded as `requirement-line-anchor-drift`.

## Delivery — per-PR summary (#102–#103)

Stacked split executed after the implementation measured **644 changed lines** (> the 400-line review
budget; forecast was ~375). The maintainer chose the split over `size:exception`: PR #102 = 397
changed lines, stacked PR #103 = +303, each under budget.

| PR | Branch | Content | Merge | Net |
|----|--------|---------|-------|-----|
| #102 | `fix/sdd-sync-empty-write-guard` | writer guard in `internal/sdd/sync_helpers.go` + resolver unit table in `internal/sdd/sync_helpers_test.go` (includes the `c673889d` complexity decomposition on-branch) | `a89f3493` (2026-09-16T20:52:52Z) | 397 changed lines |
| #103 | `fix/sdd-sync-integration-proof` | `internal/sdd/sync_integration_test.go` with T1–T5 (end-to-end sync write contract) | `e2adc96c` (2026-09-16T21:02:07Z) | +303 lines |

- **Master head at close**: `e2adc96c` (`Merge pull request #103 …`; parent chain `a89f3493`).
- Both PRs carry `type:bug` and a plain `Closes #101`.
- **Issue #101** (`fix(sdd): sync writes 0-byte living specs for full-spec-shaped new domains`,
  labels `type:bug` + `status:approved`): **CLOSED** — verified at archive with
  `gh issue view 101 --repo biggs-100/biggz-ai` (state `CLOSED`). GitHub processed the auto-close
  keyword.

## Complexity finding & fix (mid-flight)

The first delivery attempt failed the repo's **Complexity** check: `syncResolveDomainWrite` measured
**gocyclo 18** (limit 15) and **gocognit 26** (limit 20). It was decomposed into 6 named helpers —
`syncReadMainSpec`, `syncSplitSources`, `syncBlockEmptySources`, `syncResolveFullSpecWrite`,
`syncWriteFullSpecTarget`, `syncResolveDeltaWrite` — with **zero behavior change**: the old top-level
is now a 4-branch decision ladder and the `blocked` message shape is verbatim. Numbers after:
**gocyclo 4 / gocognit 3** (max helper 5/5); Complexity went green. Commit `c673889d` on the PR #102
branch; all 7 resolver sub-cases and T1–T5 pass unmodified.

## Declared deviations (not hidden)

- **D8 — stricter blocked branch**: a full-spec delta against an EXISTING living spec whose bytes
  differ returns `blocked` (unit row `differing_existing_target_blocks`), stricter than scenario 4's
  literal "MUST NOT be replaced". Declared in `design.md` D8 + Open Questions, resolved at task 4.6,
  flagged in both PR bodies. It does not break any scenario (replacement remains impossible; the
  stricter branch just names the conflict). WARNING-level by the verify decision table.
- **`isFullSpecShaped = requirementHeadingRe && !deltaSectionRe`** — a necessary deviation from the
  design's one-line description: delta files also carry `### Requirement:` headings, so without
  excluding the delta section the verbatim-copy rule would have copied raw deltas into the
  canonicals. Documented in the `sync_helpers.go` doc comment and tasks 2.1.
- **Delivery split** (644 > 400): executed as the stacked #102/#103 pair instead of
  `size:exception` — maintainer decision; recorded as a precedent. Each slice stayed ≤400 changed
  lines.

## Final verification (admitted)

- **Verdict**: PASS WITH WARNINGS — 1/1 requirements, 8/8 scenarios (each mapped to a passing
  covering test), 0 CRITICAL, 0 blockers; admitted by
  `biggz sdd-verify-validate … --requirements 1 --scenarios 8` →
  `{"decision":"admitted","requirements":{"declared":1,"counted":1},"scenarios":{"declared":8,"counted":8}}`.
- **Evidence**: `sha256:06b63f76344950e57e4bba93d70f37a8ac100a6b2120e2aa83e31b2d30225ce5` (focused
  `go test ./internal/sdd -count=1` on the exact candidate `d516cf66`); build/vet/gofmt/gitexec
  clean; `sync.go` diff empty (design claim confirmed at verify).
- **CI**: verify left two items explicitly unverified (PR #103's main suite not yet reported — the
  stacked-base gap below; PR #102's `Test (windows-latest)` pending). Final-state facts: **PR #102
  18/18 checks green; PR #103 18/18 green** after retargeting to `master` and re-triggering.
- The verify-report is a snapshot at candidate time (PRs open); final-state facts outrank it — both
  PRs are merged and issue #101 is closed.

## Ledger final shape

- Settled units (native ledger): `sync-empty-write-guard` (**passed**, after a `progress` that
  recorded the delivery split), `sync-resolver-complexity` (**passed**), `verify-report` (**passed**;
  the two then-pending CI items recorded as unverified, both later green), `sync-spec-deltas`
  (**passed**). Current unit at close: `archive-report`.

## Rollback boundary

- Revert PR #103 (`e2adc96c`) to drop the integration pin; revert PR #102 (`a89f3493`) to restore the
  pre-fix write path (the old unconditional `os.WriteFile` behind `syncApplyDeltas`).
- The guard only fires when the living spec is absent; there is no migration and nothing to heal.
  Existing 0-byte living specs are NOT auto-healed (D8 → `blocked`, by design).
- The synced living spec (`openspec/specs/sdd/spec.md`, +44/−0) and this archive move are
  working-tree-only right now; they revert with the final PR that carries them.

## Recorded defects carried forward (canonical detail in `state.yaml`)

New findings raised during this change's run (appended to `state.yaml` `discovered_defects`):

1. `stacked-prs-run-pr-check-only` (**medium-high**) — `.github/workflows/ci.yml` triggers on
   `pull_request: branches: [master, main]`, so a stacked child PR (base = another branch) runs only
   `pr-check.yml` (3 hygiene checks) and NEVER the main CI matrix: it looks validated while never
   running test/complexity/E2E — a false-green surface of exactly the class this repo has been
   closing. Workaround used: merge/retarget to master and push (or close+reopen) to fire the
   `synchronize`/`reopened` event. Candidate for a bounded CI fix.
2. `gh-pr-create-graphql-quirk` (**low**) — `gh pr create` returned `No commits between master and
   <branch>` while `ls-remote` and the compare API agreed the branch was 1 ahead; both PRs were
   created via `gh api repos/.../pulls --method POST`.

   **Correction (2026-09-17, issue #115).** The name and framing of this entry are wrong: it is not
   a GraphQL quirk. This clone carries an `upstream` remote (`Gentleman-Programming/gentle-pi`, since
   renamed to `gentle-shell`) and no `gh` default repository set, so BARE `gh` resolves to THAT
   repository instead of `origin` (`biggs-100/biggz-ai`); `gh pr create` looked for the branch where
   it does not exist. The paragraph's own "related worktree quirk" below was the actual cause all
   along. Mitigated locally with `gh repo set-default biggs-100/biggz-ai`; still pin `--repo` (this
   report's issue #101 verification did, which is why that part worked). The canonical record is
   `gh-default-repo-hijack` in `openspec/changes/archive/2026-09-16-fix-checkpoint-ask-context/`.

   Original text, kept for the record: bare `gh repo view` resolved to
   `Gentleman-Programming/gentle-shell` even though `origin` is `biggs-100/biggz-ai` — always pin
   `--repo`. Candidate for the `branch-pr` skill's known-issues note.
3. `sdd-explore-readonly-artifact-gap` (**low-medium**) — `sdd-explore` agents run read-only
   (read/grep/find/ls + ask_user_question; no bash, no write, no BigMem), so the phase cannot persist
   its own artifact; the orchestrator wrote `exploration.md` and saved the BigMem observation on its
   behalf. Pipeline friction, not data loss.
4. `requirement-line-anchor-drift` (**low**) — the `Sync Execution Contract` line anchors cited in
   earlier artifacts (`:110/:132`, then `:108/:130`, now `:130`) drift as the file changes; the
   requirement NAME is the stable anchor.

Out of scope, unchanged (tracking only): the phantom `biggz sdd-sync <change>` command printed by
`internal/sdd/status.go:1522` (no such subcommand in `cmd/biggz/main.go`); the pre-existing
`# Delta for sdd` first line in `openspec/specs/sdd/spec.md` (defect F2, previous archive); the
`sdd-spec` full-spec contract for new domains.

## Unrankable contradictions

None material. Two counting notes reconciled at archive: (a) the launch-prompt reference "30
requirements unchanged" was re-verified by anchored counts (30 → 30, name-set diff empty) — an
unanchored `grep` reads 33 because of inline `` `### Requirement:` `` mentions in prose, which is
grep noise, not a state conflict; (b) external `gh` reads initially resolved to a different
repository (bare `gh repo view` → `Gentleman-Programming/gentle-shell`); all issue/PR facts in this
report were re-read with `--repo biggs-100/biggz-ai`.

## State reconciliation notes

- `tasks.md` is the persisted authority: 27/27 `[x]`, 0 `[ ]` (fresh count pre-move) — Task
  Completion Gate PASS, no reconciliation needed.
- `verify-report.md` and `apply-progress.md` are intermediate snapshots (history): their "done"
  claims stay true; their open/pending statements (PRs open, CI pending, delivery withheld) expired
  with the merge/CI results and are superseded by the final-state facts above.
- `state.yaml` did not exist for this change (filesystem + BigMem checked); it was **created** at
  archive in the house style, with phases done through `archive` (including `sync`), no
  `pending_question` (the change never had one — kept absent), and the four findings appended to
  `discovered_defects`. `_meta.yaml` is left untouched (audit trail).
- BigMem mirror staleness: the `sdd/fix-sdd-sync-empty-spec/apply-progress` observation was
  last-upserted with the post-verify CI-fix content (see addendum); the filesystem artifacts in this
  archive are the final authority, and the BigMem `archive-report` observation is the documented
  closure signal that flips `sdd-status` classification to archived.

## Residual warnings

- The verify warnings (D8 deviation, budget accounting, CI completeness at verify time) are all
  closed or declared above; none affects the change's guarantees.
- The stacked-PR CI gap means verify's "unverified CI" caveat was correct at snapshot time; the final
  18/18 on both PRs is the resolution, and the gap itself is carried as a defect.
- Anchor drift: cite the requirement name, not the line number (smallest follow-up hygiene item).

## Delivery status

- **Nothing committed by this phase.** The orchestrator creates the final PR; the archive move and
  the synced living spec remain as working-tree changes.
- Working tree at close: ` M openspec/specs/sdd/spec.md` (the synced living spec, +44/−0) and
  `?? openspec/changes/archive/2026-09-16-fix-sdd-sync-empty-spec/` (the archive, untracked). Exact
  `git status --short` recorded in the phase return envelope.

## Post-archive hygiene (branch / worktree)

Non-interactive run — **nothing was deleted** (no `branch -d`, no `-D`, no worktree pruning):

- Observations at close: current branch `master` ✓ (head `e2adc96c`); **0 local branches in `[gone]`
  state**; `git fetch --prune --dry-run` listed nothing; single worktree
  (`C:/Users/USER/Desktop/biggz-ai`, `e2adc96c`). Nothing to candidate, nothing deleted.
- Per non-TTY policy: delete nothing, exit 0.

## Archive verification checklist

- [x] Sync precondition satisfied before move (canonical `sdd.Sync()` path, `applied`; settled `sync-spec-deltas`); not re-run, canonical files untouched by this phase.
- [x] Change folder moved to `openspec/changes/archive/2026-09-16-fix-sdd-sync-empty-spec/` with all artifacts.
- [x] Active directory clean (only `archive/` remains under `openspec/changes/`).
- [x] Tasks complete in the persisted artifact (27/27; 0 unchecked) — no reconciliation needed.
- [x] No CRITICAL verify issues (0 blockers, 0 critical findings; verdict PASS WITH WARNINGS, admitted).
- [x] `apply-progress.md` present + archive addendum from BigMem `obs-1789583484495097300-1`.
- [x] `state.yaml` created/refreshed (phases through archive, no `pending_question`, 4 defects appended).
- [x] No `.biggz-instance` existed; nothing deleted anywhere.
- [x] Nothing committed; the move + synced spec ride the final PR.
- [x] BigMem mirror of this report saved under `sdd/fix-sdd-sync-empty-spec/archive-report` (closure signal).
