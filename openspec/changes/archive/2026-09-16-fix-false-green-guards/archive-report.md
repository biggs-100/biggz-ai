# Archive Report — fix-false-green-guards

```yaml
schema: biggz-ai.archive-report/v1
change: fix-false-green-guards
archived_at: "2026-09-16 (UTC, house convention; matches the sync/verify evidence timestamps of 2026-09-16)"
archived_to: openspec/changes/archive/2026-09-16-fix-false-green-guards/
store: hybrid (filesystem wins for artifacts; the BigMem mirror of this report carries the closure signal)
verification: pass (admitted — 15/15 requirements, 56/56 scenarios, 0 CRITICAL, 0 blockers)
evidence_revision: sha256:1d98acd9a0fcde04c2ed474dac0db9186812ca64ace2bc255ecc3f2efd675f45
committed: false (this phase committed nothing; the move and the synced specs ride the final PR)
delivery: delivered — chain PRs #86–#99 merged to master; final merge 0ffedac1 closed issue #64
```

## Change

`fix-false-green-guards` (issue #64) — make the four `rg`-based CI guards fail closed instead of
reporting success without scanning (measured: 1.35 ms of "scan", 57 live violations in the tree), and
bring the repository into compliance with the invariant those guards enforce: `internal/git` is the
sole owner of `exec.Command("git")`. The change adds a compiled in-repo checker (`tools/gitexec`) with
a scanner positive control and a ratchet baseline, wires the guards into CI with no pipeline masking,
fixes the pre-push hook dead-producer distinction (unknown is never a silent skip), makes the
gatekeeper store-aware, routes the git spawns into `internal/git`, and deletes the unrouted
`internal/review/convergence.go` module.

## Archive date & final path

- **Archive date**: 2026-09-16 (UTC; today's ISO date at close).
- **Final path**: `openspec/changes/archive/2026-09-16-fix-false-green-guards/` — dated naming matches
  `2026-09-14-fix-attempt-ledger-scope`.
- The source directory no longer exists under `openspec/changes/`; the active directory contains only
  `archive/`.
- No `.biggz-instance` existed in this change (checked pre-move) — nothing to rename-preserve. No peer
  archive artifacts were touched.

## Artifact inventory (10 files plus the 8-domain `specs/` tree)

| Artifact | Status | Notes |
|----------|--------|-------|
| `_meta.yaml` | present | name/created/phase/description |
| `exploration.md` | present | the false-green measurement (1.35 ms between echoes) |
| `proposal.md` | present | approved scope |
| `design.md` | present | six-slice chain, threat matrix TM-1..TM-5, file-changes table |
| `specs/` (8 domains) | present | deltas: ci-guard-integrity (2/5), ci-guard-baseline (1/8), complexity-gates (2/8 MOD), prompt-skill-resolver (1/6 MOD), review-authority (4/8 ADDED), sdd (2/8 ADDED), testing-guidance (1/5 MOD), tui-sanitize (2/8 MOD) → 15 requirements / 56 scenarios |
| `tasks.md` | present | 32/32 checked, 0 unchecked (persisted artifact — Task Completion Gate PASS; 7.1–7.3 and 8.1–8.2 carry explicit completion notes) |
| `verify-report.md` | present | PASS (admitted); evidence sha256:1d98acd9a0fcde04c2ed474dac0db9186812ca64ace2bc255ecc3f2efd675f45 |
| `apply-progress.md` | added at archive | rebuilt verbatim from BigMem observation `sdd/fix-false-green-guards/apply-progress` (`obs-1789531615330665400-1`) so the trail is self-contained (the other archives carry `apply-progress.md`) |
| `state.yaml` | refreshed at archive | phases done through archive; stale `pending_question` removed; F1–F3/W1–W4/S1–S2 appended to `discovered_defects` |
| `archive-report.md` | present | this report |

Not produced: `sync-report.md` (the sync landed via the orchestrator with the F1 workaround below; no
file report was authored — the sync facts live in this report) and `review-subject.json` (no RDD
review ran; review gate unmanaged).

## Sync at archive (8 domains — applied before the move, not re-run)

The delta → canonical application landed before the move and was settled (`sync-spec-deltas: passed`);
this phase did **not** re-run it and did **not** touch `openspec/specs/**` (those edits ride the final
PR). Per-domain actions:

| Domain | Action | Delta (req/scen) | Living spec after (req/scen) |
|--------|--------|------------------|------------------------------|
| ci-guard-integrity | NEW domain, full spec | 2/5 | 2/5 |
| ci-guard-baseline | NEW domain, full spec | 1/8 | 1/8 |
| complexity-gates | MODIFIED (replace-in-place) | 2/8 | 5/15 |
| prompt-skill-resolver | MODIFIED (replace-in-place) | 1/6 | 4/19 |
| testing-guidance | MODIFIED (replace-in-place) | 1/5 | 6/12 |
| tui-sanitize | MODIFIED (replace-in-place) | 2/8 | 10/26 |
| review-authority | ADDED | 4/8 | 15/44 |
| sdd | ADDED | 2/8 | 30/73 |

**Totals**: delta 15 requirements / 56 scenarios → living specs **73 requirements / 202 scenarios**
across the 8 files; `218 insertions(+), 14 deletions(-)` across the 6 tracked modified files plus the
2 new `openspec/specs/{ci-guard-integrity,ci-guard-baseline}/spec.md`. Untouched requirements and
heading hierarchy were preserved (MODIFIED replacements in place; ADDED appends).

**F1 workaround (see defects)**: the two NEW domains are full-spec-shaped (no `## ADDED Requirements`
section), and the native `sdd-sync` write path would have emitted **empty** main specs
(`ApplyDeltas("", []) → ""`). They were applied by verbatim copy instead — content preserved, counts
as shown.

**F1 correction (2026-09-17).** This workaround was necessary only on the day of the archive. The
write path was fixed the same day by the change `fix-sdd-sync-empty-spec` (PRs #102/#103), which
split the write decision into named steps and made a single full-spec source for a NEW domain copy
verbatim. The workaround text above is kept as the historical record of what the run did; the defect
entry no longer asks for follow-up work.

## Delivery — per-PR summary (#86–#99)

All merged to `master`; per-merge net stats from the merge commits:

| PR | Branch | Content | Net |
|----|--------|---------|-----|
| #86 | `fix/guard-checker-and-baseline` | compiled `tools/gitexec` checker + fixtures + 71-entry baseline + CI wiring | 11 files, +961 |
| #87 | `fix/guard-ci-wiring` | checker wired into CI fail-closed; guards stop lying | 5 files, +214/−129 |
| #88 | `fix/git-exec-surface` | generic git exec surface behind the single owner | 4 files, +379 |
| #89 | `fix/remove-dead-convergence` | delete the unrouted convergence module (`a7003e38`) | 2 files, −100 |
| #90 | `fix/review-git-routing` | route the review package's git calls to the wrapper | 6 files, +46/−98 |
| #91 | `fix/review-numstat-dedup` | numstat paths through the wrapper; drop triple closure | 4 files, +71/−77 |
| #92 | `fix/review-frozen-inspector` | complete wrapper migration for gate + frozen inspector | 3 files, +33/−57 |
| #94 | `fix/ci-windows-test-budget` | raise the Windows test budget to 600s; drop dead env | 1 file, +7/−4 |
| #95 | `fix/gatekeeper-store-aware` | store-aware gatekeeper artifact resolution | 5 files, +350/−49 |
| #96 | `fix/sdd-git-routing` | route the sdd package's git calls | 4 files, +20/−41 |
| #97 | `fix/cmd-biggz-pr-git-routing` | route `pr.go` write ops; declared streaming deviation | 4 files, +298/−63 |
| #98 | `fix/cmd-biggz-cli-git-routing` | route the remaining CLI git spawns | 6 files, +122/−20 |
| #99 | `fix/final-wave-debt-zero` | final 13 sites (release/install/sddattempt/project/sdd/doctor); debt 0 | 12 files, +256/−75 |

**Final merge**: `0ffedac1 Merge pull request #99 from biggs-100/fix/final-wave-debt-zero` on master;
GitHub closed issue #64 on that merge.

## Debt-zero end state (checker output, final)

- `.github/guard-baseline.txt` holds **exactly 7 lines, all `declared-boundary`**: `internal/assets/pi/biggz-footer.js:422`, `internal/assets/pi/codegraph-tools.ts:80`, `internal/doctor/git.go:69`, `internal/doctor/review.go:82`, `internal/doctor/review.go:93`, `internal/doctor/review.go:251`, `internal/doctor/version.go:86`; **0 `go-git-spawn`**.
- `gitexec -root .` on master → exit 0, exact line:
  `gitexec: OK — self-check 9 sites, 351 .go + 17 host targets, 0 debt, 7 boundary, baseline matched`
  (the same line was re-read in the CI "Forbid Git Exec Outside Wrapper" job).
- The baseline went 71 entries → 7 (64 `go-git-spawn` sites eliminated; the 2 host + 5 doctor boundaries kept live, never hidden); `-update` was used exactly once, after all sites migrated; a stale entry fails, debt-zero-with-boundaries passes (R3 scenarios).

## Final verification (admitted)

- **Verdict**: PASS — 15/15 requirements, 56/56 scenarios, 0 CRITICAL, 0 blockers; admitted by `biggz sdd-verify-validate … --requirements 15 --scenarios 56`.
- **Evidence** (bound to the canonical focused run): `sha256:1d98acd9a0fcde04c2ed474dac0db9186812ca64ace2bc255ecc3f2efd675f45`; 16 packages ok / 0 failed.
- **CI on the candidate** `7196c364`: 18/18 checks pass (run `35115642104`; PR Validation `35115892535`).
- The verify-report is a snapshot at candidate time (PR #99 open, base `2b2daba4`); the final-state
  facts outrank it: #99 is merged as `0ffedac1` and issue #64 is closed.

## Ledger final shape

- Settled units (native ledger, all `passed`): `pr5-cmd-biggz-wave`, `pr6-final-wave-debt-zero`,
  `verify-report`, `sync-spec-deltas`. Current unit at close: `archive-report`.
- Two **archive attempts were interrupted by the harness** (a provider network error at turn 37 and a
  20-minute ceiling) — recorded as `interrupted`/`progress` with their work reused; they are not
  failures of the change.

## Declared deviations (not hidden)

- **Proposal/spec domain split** (authoritative divergence in `state.yaml`): `proposal.md` lists one
  new capability (`ci-guard-integrity`); the spec phase split it into `ci-guard-integrity` +
  `ci-guard-baseline`. Counts unchanged (15/56); only the domain count went 7 → 8. The proposal states
  which capabilities change, not how many files they occupy.
- **Word-budget deviations** (`state.yaml` `budget_deviations`, plus the systemic finding): `tasks.md`
  1203 words vs the 530 budget (**arithmetically unreachable** for a six-slice change with a threat
  matrix — mandated components sum to ~853 words); `design.md` 798/800 (fits); `ci-guard-integrity`
  859 words vs the 650 spec budget → resolved by the domain split instead of deleting mandated
  coverage (both files now fit). The per-artifact budgets not scaling with change size is recorded as
  the change's systemic pipeline finding.
- **8.1 timeout deviation** (declared in `tasks.md` 8.1): the local full suite ran with
  `-timeout 600s`, not the literal 180s, because `internal/review` measures 288.5 s on this Windows
  host against the per-OS runner budget fixed in #94; the literal would red by design on Windows.
  61 packages ok, 0 FAIL; `go vet ./...` clean.
- **Boundary adoptions (explicit declarations, `disable-model-invocation`-style)**: instead of routing
  every last site, the change *adopted and declared* boundaries in-tree — the 7 `declared-boundary`
  baseline entries (host-rule `internal/assets/pi/*.{js,ts}`, 5 doctor exec seams), the checker's
  declared scope exclusions (`internal/git/**`, `*_test.go`, `testdata/`, `e2e/`, `openspec/`), the
  R12–R15 "satisfied by absence" disposition, and the E2E side of 8.2 (allowlisted tree passes, exit 0)
  exercised against a planted violation on a disposable worktree (`go-git-spawn
  internal/release/zz_probe.go 5`, exit 1). Every adopted boundary is a visible declaration — the same
  discipline as the skill frontmatter `disable-model-invocation` flag (an explicit exclusion field,
  never an implicit skip).
- **R11 verified by a live harness only** (no checked-in three-state regression test) — W1 below.
- The verify did not re-run the full suite (standing 8.1 evidence + fresh focused runs + fresh 3-OS CI
  matrix); `internal/review`'s 288.5 s Windows cost is covered by CI.

## Rollback boundary

- Each slice is atomic: a slice deletes its baseline entries **in the same commit** that migrates its
  sites, so reverting any single PR restores sites + entries together (no orphan baseline).
- Revert of the final wave = revert PR #99 as a unit (`db1bd0ff`…`7196c364`); doctor boundary seams
  (`execFn`) are untouched by design.
- Full revert of the chain (#86→#99, newest first) restores the pre-change tree where the four
  `rg`-based guards can still report green without scanning — that is the intentional pre-change state,
  not a regression to preserve.
- The synced living specs and this archive move are working-tree-only right now; they revert with the
  final PR that carries them.

## Recorded defects carried forward (canonical detail in `state.yaml`)

Pre-existing, deliberately NOT fixed in this change:
1. `gatekeeper-routing-static-table` — `nextPhaseValid` hardcoded successor table; verdict depends on
   phrasing, not state.
2. `gofmt-not-toolchain-stable` — Go Format job floats on `stable`; gofmt 1.26.1 flags
   `internal/review/rdd_helpers.go` locally, CI (1.27 form) passes.
3. `pr-skill-references-nonexistent-tool` — `branch-pr` skill instructs `internal/gofmtcheck`, which
   does not exist in this tree.
4. `unassigned-git-spawn-baseline-entries` — **RESOLVED within this change**: the three orphan sites
   (session_guard.go:353, detect.go:357/416) were assigned to the Phase 7 unit by the maintainer and
   migrated in PR #99 so 7.3 (debt=0) was reachable.
5. `git-write-ops-lose-live-output` — `pr.go` buffered git output drops success stderr; accepted with
   the declaration in PR #97 (a streaming runner keeping the inherited env is the restore path).
6. `tui-throttle-test-timing-fragile` — `TestSyncOutput_ThrottleCoalesceBurst` false-RED on loaded
   Windows runners (sleep-based throttle assertions); candidate for its own bounded change.

New findings raised during verify/sync/archive (appended to `state.yaml`):
7. W1 `r11-hook-regression-test-missing` — the R11 three-state branches have no checked-in regression
   test (live harness run only at verify).
8. W2 `state-yaml-stale-bookkeeping` — this change's `state.yaml` was stale; **resolved by this
   archive refresh** (phases now done; stale envelope removed). S2 process recommendation recorded:
   refresh `state.yaml` at the verify gate.
9. W3 `deployed-pre-push-hook-predates-fix` — the deployed `.git/hooks/pre-push` on this workstation
   (mtime 2026-08-31) predates the fix; the template is fixed, runtime pickup needs a
   reinstall/manual copy — maintainer note.
10. W4 `verification-plan-dead-surface` — adjacent dead surface `internal/verification/plan.go` (zero
    importers, unrouted verification subject); candidate for its own bounded change.
11. S1 `workflow-guards-lack-negative-control-job` — consider a scheduled CI job that plants a
    violation on a disposable ref and asserts the guard step fails.
12. F1 `sdd-sync-discards-full-spec-shaped-new-domain` — native sync write path returns `""` for a
    full-spec-shaped file with no `## ADDED Requirements` section; would have created empty main specs.
    Worked around by verbatim copy; sync write-path fix candidate for its own change.

    **Correction (2026-09-17).** Already fixed: the change `fix-sdd-sync-empty-spec` landed the guard
    the same day (PRs #102/#103 — `dae27aab`, `c673889d`), in `internal/sdd/sync_helpers.go` rather
    than in `ApplyDeltas`, because returning `""` for an empty main with no deltas is correct for a
    pure function. No follow-up change is needed; a full-spec new domain is now copied verbatim and a
    delta producing no content fails closed. Verified live on master `879dd15d` with the four pins in
    `internal/sdd/sync_integration_test.go` (T1/T2/T3/T5) passing. Original text, kept for the record:
    native sync write path returns `""` for a full-spec-shaped file with no `## ADDED Requirements`
    section; would have created empty main specs. Worked around by verbatim copy; sync write-path fix
    candidate for its own change.
13. F2 `sdd-spec-header-stale` — `openspec/specs/sdd/spec.md:1` still reads `# Delta for sdd`
    (pre-existing at `0ffedac1`, not introduced by this change).
14. F3 `sdd-continue-renderer-mismaps-phase` — the human-readable `biggz sdd-continue` printed
    `Next phase: apply` while `biggz sdd-status --json` correctly said `nextRecommended: sync`; the
    JSON is authoritative and the text renderer's mapping is wrong.
15. `systemic` — per-artifact word budgets (spec 650 / design 800 / tasks 530) do not scale with
    change size; all three were hit. Candidate for its own SDD change.

## Unrankable contradictions

None. The only rankable divergence: `verify-report.md` (snapshot at candidate `7196c364`) describes
PR #99 as open — final-state facts (launch-prompt handoff, merge commit `0ffedac1`) outrank it; the
report's PASS verdict nonetheless holds (merge did not change the candidate tree content beyond the
merge itself, and CI was green on the exact candidate).

## State reconciliation notes

- `tasks.md` is the persisted authority: 32/32 `[x]`, 0 `[ ]` (fresh count pre-move) — Task Completion
  Gate PASS, no reconciliation needed.
- `verify-report.md` and `apply-progress.md` are intermediate snapshots (history); their "done" claims
  stay true; their open/pending statements (e.g. "PR #99 open") expired with the merge.
- `state.yaml` was refreshed at archive: `phases` now report `done` through `archive` (with `sync`
  added), and the stale `pending_question` envelope (phase 3/4 questions) was removed — nothing is
  pending at close. `spec_counts` (+ divergence), `discovered_defects`, `budget_deviations`, and
  `systemic_finding` were kept; W1–W4/S1–S2/F1–F3 were appended. The refresh also repaired two
  **pre-existing YAML validity defects** found while validating the refresh (a `: ` inside the
  `divergence` plain scalar and one unquoted `where:` value containing `(go-version: stable)`); the
  file now parses under strict YAML (PyYAML) where before it did not — relevant because
  `internal/sdd/pending.go`'s state.yaml fallback ignores unmarshal errors on write but needs a valid
  document on read. Content is unchanged (formatting only).
- BigMem mirror staleness: intermediate artifacts' BigMem copies reflect earlier snapshots; the
  filesystem artifacts in this archive are the final authority, and the BigMem `archive-report`
  observation is the documented closure signal that flips `sdd-status` classification to archived.

## Residual warnings

- W1 (hook regression test), W3 (deployed hook staleness), F1 (sync write path), F3 (renderer) — all
  carried as recorded defects above; none affects this change's guarantees.
- Word-budget overages — declared, systemic finding recorded; no coverage was deleted.
- Review budget: chained PRs kept the merge line small (per-PR slices ≤ ~350 net lines after PR1),
  consistent with the 400-line review budget intent; PR1 (#86, +961) was the declared exception.

## Delivery status

- **Nothing committed by this phase.** The orchestrator creates the final PR; the archive move and the
  synced spec files remain as working-tree changes.
- Working tree at close: 6 tracked files modified under `openspec/specs/` (the synced specs), 3
  untracked entries — `openspec/changes/fix-false-green-guards/` is gone (moved),
  `openspec/changes/archive/2026-09-16-fix-false-green-guards/` and the two new spec domains are
  untracked. Exact `git status --short` recorded in the phase return envelope.

## Post-archive hygiene (branch / worktree)

Non-interactive run — **nothing was deleted** (no `branch -d`, no `-D`, no worktree pruning):

- Branch/worktree observations at close: current branch `master` ✓; **0 local branches in `[gone]`
  state**; single worktree (`C:/Users/USER/Desktop/biggz-ai`, `0ffedac1`); `git fetch --prune
  --dry-run` lists nothing to prune. Nothing to candidate, nothing deleted.
- Per non-TTY policy: delete nothing, exit 0.

## Archive verification checklist

- [x] Sync precondition satisfied before move (8 domains applied; settled `sync-spec-deltas`); not re-run, canonical files untouched by this phase.
- [x] Change folder moved to `openspec/changes/archive/2026-09-16-fix-false-green-guards/` with all artifacts.
- [x] Active directory clean (only `archive/` remains under `openspec/changes/`).
- [x] Tasks complete in the persisted artifact (32/32; 0 unchecked) — no reconciliation was needed.
- [x] No CRITICAL verify issues (0 blockers, 0 critical findings).
- [x] `apply-progress.md` rebuilt from BigMem `obs-1789531615330665400-1`.
- [x] `state.yaml` refreshed (phases + defect append + stale envelope removed).
- [x] No `.biggz-instance` existed; nothing deleted anywhere.
- [x] Nothing committed; the move + synced specs ride the final PR.
- [x] BigMem mirror of this report saved under `sdd/fix-false-green-guards/archive-report` (closure signal).
