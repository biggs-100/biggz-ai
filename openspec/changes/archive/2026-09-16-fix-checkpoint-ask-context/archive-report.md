# Archive Report — fix-checkpoint-ask-context

```yaml
schema: biggz-ai.archive-report/v1
change: fix-checkpoint-ask-context
archived_at: "2026-09-16 (UTC, house convention; matches the sync/verify evidence timestamps of 2026-09-16)"
archived_to: openspec/changes/archive/2026-09-16-fix-checkpoint-ask-context/
store: both (filesystem artifacts are the authority; the BigMem mirror of this report carries the closure signal)
verification: pass (admitted — 3/3 requirements, 29/29 scenarios, 0 CRITICAL, 0 blockers)
evidence_revision: sha256:a63a797521d353a0eeffce8f97aef5d00e1b8462aa6d1cd457eff595eee1505a
committed: false (this phase committed nothing; the move and the synced living specs ride the final PR the orchestrator opens)
delivery: delivered — PR #108 fdddf9ee + PR #109 6cd09745 + PR #110 0dd35db4 + PR #111 dc9c3d61 merged to master in order; master head dc9c3d61; issue #14 CLOSED
```

## Change

`fix-checkpoint-ask-context` (issue #14) — close the gap where a checkpoint question was asked without
decision context (evidence, scope, effort, risk, unlock, deferral, recommendation, no-go condition) and
the human had to demand a report mid-decision. The change's working thesis, proven as shipped: **a rule
nobody invokes is not a rule.** It defines the decision-context contract, makes it enforceable in the
**live runtime** through a `biggz sdd-ask-check` CLI check (synthesis precondition + envelope validation
incl. the substance rule), wires the **deployed** `ask-user-choice.ts` asset to invoke that check before
rendering (argv array, no shell, 1000 ms bound, stdin JSON), and makes the docs tell the truth about
which enforcement path is real.

## Archive date & final path

- **Archive date**: 2026-09-16 (UTC; today's ISO date at close).
- **Final path**: `openspec/changes/archive/2026-09-16-fix-checkpoint-ask-context/` — same dated
  convention as the sibling `2026-09-16-*` archives.
- The source directory no longer exists under `openspec/changes/`; the active directory contains only
  `archive/` (verified post-move).
- No `.biggz-instance` existed in this change (checked pre-move) — nothing to rename-preserve.

## Artifact inventory

| Artifact | Status | Notes |
|----------|--------|-------|
| `_meta.yaml` | present | name/created/phase/description; left untouched (audit trail — it still reads `phase: tasks`; `state.yaml` is the state authority) |
| `proposal.md` | present | scope/approach/rollback + the **superseded first premise** preserved verbatim under `## Superseded premise` |
| `design.md` | present | D1–D7, predicate + fixtures, exit taxonomy, TS seam, file table, slice table, threat matrix, rollback; 1,779 words (over the 800-word budget — declared) |
| `specs/sdd/spec.md` | present | delta: Synthesis Gate block MODIFIED, 9→15 scenarios |
| `specs/orchestrator/spec.md` | present | delta: REQ-ORCH-001 Blocking Synthesis Checkpoint MODIFIED, 5→7 scenarios |
| `specs/pi-integration/spec.md` | present | delta: Question Envelope Validation MODIFIED, 3→7 scenarios |
| `tasks.md` | present | 24/24 `[x]`, 0 unchecked (persisted artifact — Task Completion Gate PASS; Phases 1–5, every task with RED/GREEN evidence + 4/4 phase-5 closing gates) |
| `verify-report.md` | present | PASS (admitted — 3/3 requirements, 29/29 scenarios); evidence `sha256:a63a79…505a`; candidate = PR #111 head `a007191a` |
| `review-subject.json` | present | written by verify for the RDD gate (`{"repository":…,"commit_sha":"HEAD"}`) |
| `apply-progress.md` | present + archive addendum | Created at archive from BigMem `obs-1789596779706862800-1` (the file never existed in the change dir pre-move); addendum carries the post-verify final-state facts |
| `state.yaml` | created at archive | No `state.yaml` ever existed for this change (checked pre-move). Created in the house style of `2026-09-16-fix-sdd-sync-empty-spec/state.yaml`: phases done through `archive` (with `sync`), no `pending_question` (never had one), delivery/deviations blocks, four findings appended to `discovered_defects` |
| `archive-report.md` | present | this report |

Not produced: `sync-report.md` (the sync landed through the canonical `sdd.Sync()` path; its facts live
in this report) — same precedent as the sibling archives.

## Sync at archive (both — already applied, not re-run)

The delta → canonical application landed before the move through the canonical `sdd.Sync()` path
(`result=applied`, settled as `sync-spec-deltas: passed`); this phase did **not** re-run it and did
**not** touch `openspec/specs/**` (those edits ride the final PR as tracked working-tree changes).

| Domain | Action | Delta (req/scen) | Living spec after (req/scen) | Δ scenarios |
|--------|--------|------------------|------------------------------|-------------|
| sdd | MODIFIED (Synthesis Gate block) | 1/15 | 30/85 | 79 → 85 (+6) |
| orchestrator | MODIFIED (REQ-ORCH-001) | 1/7 | 34/119 | 117 → 119 (+2) |
| pi-integration | MODIFIED (Question Envelope Validation) | 1/7 | 16/47 | 43 → 47 (+4) |

**Verification of the applied result (fresh reads at archive)**:

- `git diff --stat openspec/specs/` → `3 files changed, 81 insertions(+), 2 deletions(-)` — matches the
  sync's `+81/−2`.
- Anchored counts: sdd 30 `### Requirement:` before (HEAD) vs 30 after; orchestrator 34/34;
  pi-integration 16/16 — **requirement-name sets unchanged** (verified by the sync run; spot-confirmed
  at archive via anchored counts).
- Scenarios: sdd 79 → 85 (+6, the Synthesis Gate block 9 → 15), orchestrator 117 → 119 (+2),
  pi-integration 43 → 47 (+4) — block byte-parity exact and rebuild-from-HEAD diff empty per the sync
  run; untouched requirements and heading hierarchy preserved (three in-place MODIFIED replacements).

## Delivery — per-PR summary (#108–#111)

Delivered as a **four-PR stack**, each merged in order; every PR ran the full **18-check matrix** green
(the stacked bases included, thanks to the earlier `ci.yml` fix). Each slice stayed ≤ the 400-line
review budget.

| PR | Title | Branch | Content | Merge | Net |
|----|-------|--------|---------|-------|-----|
| #108 | `fix(sdd): reject checkpoint options that carry no decision context` | `fix/ask-context-substance-rule` | slice 1a: the substance predicate + `ErrThinCheckpointOption` in `internal/sdd/question.go` | `fdddf9ee` | +155/−4 |
| #109 | `feat(cli): add biggz sdd-ask-check for the checkpoint ask contract` | `fix/ask-context-ask-check` | slice 1b: `internal/sdd/ask_check.go` + the CLI in `cmd/biggz` | `6cd09745` | +357 |
| #110 | `feat(pi): gate the deployed ask tool on biggz sdd-ask-check` | `fix/ask-context-wiring` | slice 2a: `internal/assets/pi/biggz-ask-guard.js` + its `.mjs` suite, the `ask-user-choice.ts` wiring, deploy list 12→13 JS | `0dd35db4` | +394/−6 |
| #111 | `docs(sdd): make the checkpoint-ask contract and its live path the truth` | `fix/ask-context-docs` | slice 2b: the docs truth pass | `dc9c3d61` | +104 |

- **Master head at close**: `dc9c3d61` (`Merge pull request #111 …`; parent chain #108 → #109 → #110 → #111).
- **Issue #14** (`fix(orchestrator): checkpoint questions asked without decision context`, labels
  `bug` + `status:approved`): **CLOSED** — verified at archive with
  `gh issue view 14 --repo biggs-100/biggz-ai` (state `CLOSED`).

## Superseded premise (the premise story)

The first version of the proposal (same file, 2026-09-16) assumed that adding a substance rule to
`internal/sdd/question.go`'s `ValidateQuestionEnvelope` and mirroring it in
`internal/assets/pi/biggz-synthesis-gate.js` would **enforce** the rule. That premise is **FALSE** and is
superseded — preserved verbatim under `## Superseded premise` in `proposal.md`, not silently replaced.

Evidence that refuted it (re-verified 2026-09-16):

- `ValidateQuestionEnvelope` (`internal/sdd/question.go:68`) has **no production call site** — only
  `question_test.go` calls it.
- `internal/assets/pi/biggz-synthesis-gate.js` is **not deployed**: `internal/install/steps/pi_extensions.go`
  excludes it ("native Go synthesis gate … is the only enforcement path") and the install self-heal
  removes stale copies.
- That "native Go synthesis gate" is itself unwired: `ShouldBlock`, `CheckSynthesisPrecondition`,
  `BuildBlockedEnvelope`, `ShouldBlockApplyAdmission`, `HasSynthesis`, `IsCheckpointAsk`, `HasOptions`
  have call sites only in `_test.go` files; the sole non-test reference is a comment.
- `biggz sdd-gate` is the RDD/review gate — a different surface.
- `docs/architecture.md` claimed the Go gate "is the enforced gate" — false in this build.

The change was re-scoped to close issue #14 **for real**: a live `biggz` CLI check the deployed ask
asset can invoke, the wiring of `ask-user-choice.ts` to that check, the workflow-doc checklist, and
honest enforcement claims in `docs/architecture.md`. The premise as shipped — the deployed asset really
invokes `biggz sdd-ask-check` through `biggz-ask-guard.js` before rendering (argv array, no shell,
1000 ms timeout, JSON on stdin) — was **verified** by static import pin, call-before-UI assertion,
behavioural mock-exec suite, and source inspection (call at `ask-user-choice.ts:79`, UI at :102).

## The predicate and exit-code contract as shipped

**Substance predicate** (checkpoint envelopes only — label-signalled, so non-checkpoint questions are
unaffected): a description is substantive iff `runeLen(TrimSpace(d)) ≥ 24` **and** `≥ 2` distinct
classes of `{scope, effort, risk, unlock, deferral}` (bilingual tokens, case-insensitive). The design's
literal fixture measured 23 runes; the shipped fixture is `"Edit 12 files; low risk!"` with the property
(`runeLen == 24`) asserted, not the characters. The 23-rune sibling remains the negative boundary.

**Exit codes** (`biggz sdd-ask-check`):

| Exit | Meaning |
|------|---------|
| 0 | allow (both preconditions pass; silent stdout) |
| 1 | `blocked(synthesis_required)` |
| 2 | `blocked(envelope_invalid)` |
| 3 | `blocked(checkpoint_option_thin)` (names the offending option and question) |
| 4 | indeterminate (usage/input error; the usage text is deliberately token-free so a token-scanning caller never mistakes it for a block) |

**Refuse vs degrade** in the deployed path: decided blocks (`blocked(synthesis_required)`,
`blocked(envelope_invalid)`, `blocked(checkpoint_option_thin)`) **refuse to present** the question
(`isError:true`, no UI call); indeterminate failures (missing/stale binary, timeout, malformed input,
foreign exit, `BIGGZ_ASK_CHECK=0`) **degrade with a visible notice** (`ctx.ui.notify(..., "warning")`)
so a broken check never makes asking impossible.

## Docs truth pass (slice 2b, PR #111 — RED→GREEN, 8 verbatim failures → green)

- Four phrases pinned inside `## Visible Context Before Every Question (MANDATORY)`
  (`biggz-orchestrator-workflow.md:155`, test `TestOrchestratorAskContextItemsInvariant`):
  "evidence found and how it was verified" | "problem, scope, effort, risk, what it unlocks, and
  deferral cost" | "recommendation with its reason" | "no-go condition — research more instead of
  asking early". Live path cited: `ask-user-choice.ts → biggz-ask-guard.js → biggz sdd-ask-check`.
- `docs/architecture.md` before→after: "is the enforced gate" (no live path named) → "reached at ask
  time by the deployed path … (a decided block refuses to present; indeterminate degrades visibly with
  a warning)"; Layer 2 heading now "Go canonical + deployed ask check (blocking) + retired Pi gate
  source (advisory only)"; the `biggz-synthesis-gate.js` sentence opens "(source-only, NOT deployed to
  `~/.pi/agent/extensions/`)". Pinned by `TestArchitectureEnforcementClaimsMatchLivePath`.
- Rollback: revert the docs commit alone (no code depends on prose); the test commit reverts
  independently.

## Final verification (admitted)

- **Verdict**: PASS — 3/3 requirements, 29/29 scenarios (each mapped to a passing covering test),
  0 CRITICAL, 0 blockers; admitted by
  `biggz sdd-verify-validate … --requirements 3 --scenarios 29` → "Verify report is valid.".
- **Evidence**: `sha256:a63a797521d353a0eeffce8f97aef5d00e1b8462aa6d1cd457eff595eee1505a` (focused
  `go test ./internal/sdd ./cmd/biggz ./internal/install/... ./internal/assets/biggz/... -count=1`,
  exit 0; build `go build ./...` exit 0); node pi suite 132 passed / 0 failed / 0 skipped.
- **Tasks**: 24/24 complete + 4/4 phase-5 closing gates with recorded output (5.1 focused suites,
  5.2 vet/fmt, 5.3 gitexec — `0 debt, 7 boundary, baseline matched`, 5.4 node pi suite).
- **CI**: all four PR heads 18/18 checks green; `#111`'s `Test (windows-latest)` leg was pending at
  report time (recorded as unverified, never green) and completed green on the final re-read.
- **Verify warnings**: none. **Suggestions**: 2 — the textual call-order assertion (recorded as
  `source-scan-order-assertion-textual`) and the missing labeled A1–A6 list (traceability nit).
- The verify-report is a snapshot at candidate time; final-state facts outrank it — all four PRs are
  merged (master head `dc9c3d61`) and issue #14 is closed.

## Ledger final shape

- Settled units (native ledger): `ask-context-1a-predicate`, `ask-context-1b-cli`,
  `ask-context-2-wiring`, `ask-context-2b-docs`, `verify-report`, `sync-spec-deltas` — all **passed**;
  `archive-report` was the current unit at close (token `tok-6c3c2cbb8869f4553d83d496`; this phase
  does not settle — the orchestrator settles).

## Rollback boundary

- Revert slice commits: `ask-user-choice.ts` presents directly again, the CLI dispatch is removed, the
  rule reverts to emptiness-only, docs revert. No schema/persistence touched.
- Field escape hatch without revert: `BIGGZ_ASK_CHECK=0` disables the invocation (degrade-open seam) —
  the rule then reverts to advisory.
- The synced living specs (`openspec/specs/{sdd,orchestrator,pi-integration}/spec.md`, `+81/−2`) and
  this archive move are working-tree-only right now; they revert with the final PR that carries them.

## Recorded defects carried forward (canonical detail in `state.yaml`)

New findings raised during this change's run (appended to `state.yaml` `discovered_defects`):

1. `ask-check-plain-question-allows` (**medium**) — a payload whose `question` field is **not JSON**
   makes `biggz sdd-ask-check` exit **0 (allow)** instead of 4 (indeterminate); observed on master:
   `{"question":"Test?","markdown":"<synthesis>"}` exits 0, so a caller that forgets to encode the ask
   params inside the `question` string silently bypasses the envelope limits, the ownership rule and
   the substance rule (the synthesis precondition still applies — not a total bypass). The correct
   shape behaves as designed. Candidate fix: exit 4 for a non-JSON `question` + a documented example
   of the payload shape.
2. `gh-default-repo-hijack` (**medium**, and a **CORRECTION** of an earlier record) — this clone has an
   `upstream` remote pointing at `Gentleman-Programming/gentle-pi` (renamed `gentle-shell`) and no `gh`
   default repository, so bare `gh` commands resolved to THAT repository. This is the real root cause
   behind the defect recorded as `gh-pr-create-graphql-quirk` in
   `openspec/changes/archive/2026-09-16-fix-sdd-sync-empty-spec/state.yaml` (the "No commits between
   master and `<branch>`" error was `gh` looking at a repo where the branch did not exist). Mitigated
   locally with `gh repo set-default biggs-100/biggz-ai`. The earlier archive was **not** edited —
   this entry supersedes its root-cause hypothesis; the other archive should be annotated in a future
   maintenance pass.
3. `tmp-path-divergence-write-tool-vs-bash` (**low**) — the host's write tool resolves `/tmp` to
   `C:\tmp` while Git Bash uses `C:\Users\USER\AppData\Local\Temp`, so a file written with one path is
   invisible to a command run with the other. Two sub-agents hit it this session; in the verify run it
   silently validated a STALE report until the divergence was found. Mitigation: keep scratch files
   inside the repo tree or pass explicit Windows paths.
4. `source-scan-order-assertion-textual` (**low**, SUGGESTION from verify) — the assertion proving the
   deployed asset invokes the check before rendering is a textual source scan
   (`indexOf('checkCheckpointAsk(') < indexOf('ctx.ui.custom')`), which a comment could satisfy; a
   parse-based assertion would harden it. Corroborated meanwhile by the import regex, the behavioural
   guard suite and source inspection.

Out of scope, unchanged (tracking only): `internal/assets/pi/biggz-synthesis-gate.js` stays on disk but
NOT deployed (retired pending its own decision); `biggz sdd-gate` remains the RDD/review gate; the
pending-question persistence format is untouched.

## Unrankable contradictions

None material. Counting note reconciled at archive: the launch-prompt "block 9→15" for sdd matches the
anchored per-domain counts (sdd scenarios 79 → 85 = +6 for that block; orchestrator 117 → 119 = +2 for
5 → 7; pi-integration 43 → 47 = +4 for 3 → 7); the three deltas carry 29 scenarios in total (15 + 7 + 7),
matching the admitted verify counts. An unanchored `grep` can overcount `### Requirement:` occurrences
from inline prose mentions — anchored counts were used for every number in this report.

## State reconciliation notes

- `tasks.md` is the persisted authority: 24/24 `[x]`, 0 `[ ]` (fresh count pre-move) — Task
  Completion Gate PASS, no reconciliation needed.
- `verify-report.md` and `apply-progress.md` are intermediate snapshots (history): their "done" claims
  stay true; their open/pending statements (CI leg pending, PRs open, "#111 NOT merged") expired with
  the merge/CI results and are superseded by the final-state facts above.
- `state.yaml` did not exist for this change (checked pre-move); it was **created** at archive in the
  house style, with phases done through `archive` (including `sync`), no `pending_question` (the
  change never had one — kept absent), and the four findings appended to `discovered_defects`.
  `_meta.yaml` is left untouched (audit trail).
- BigMem mirror: the `sdd/fix-checkpoint-ask-context/apply-progress` observation
  (`obs-1789596779706862800-1`) was the source of the archived `apply-progress.md`; the filesystem
  artifacts in this archive are the final authority, and the BigMem `archive-report` observation
  (saved at close) is the documented closure signal that flips `sdd-status` classification to archived.

## Residual warnings

- The verify suggestions (textual call-order assertion; missing labeled A1–A6 list) are recorded as
  findings above; neither affects the change's guarantees.
- `ask-check-plain-question-allows` is a real hole in the CLI's input-shape validation worth a bounded
  follow-up; it does not affect the deployed path (`biggz-ask-guard.js` always sends the correct shape).

## Delivery status

- **Nothing committed by this phase.** The orchestrator creates the final PR; the archive move and the
  three synced living specs remain as working-tree changes.
- Working tree at close: ` M openspec/specs/orchestrator/spec.md`, ` M openspec/specs/pi-integration/spec.md`,
  ` M openspec/specs/sdd/spec.md` (the synced living specs, +81/−2 combined) and
  `?? openspec/changes/archive/2026-09-16-fix-checkpoint-ask-context/` (the archive, untracked).
  Exact `git status --short` recorded in the phase return envelope.

## Post-archive hygiene (branch / worktree)

Non-interactive run — **nothing was deleted** (no `branch -d`, no `-D`, no worktree pruning):

- Observations at close: current branch `master` ✓ (head `dc9c3d61`); **0 local branches in `[gone]`
  state**; `git fetch --prune --dry-run` listed one stale remote-tracking ref that a real prune would
  remove (`origin/ci-probe/child`) — a dry-run observation only, nothing pruned; single worktree
  (`C:/Users/USER/Desktop/biggz-ai`, `dc9c3d61`). No local branch was candidated for deletion.
- Per non-TTY policy: delete nothing, exit 0.

## Archive verification checklist

- [x] Sync precondition satisfied before move (canonical `sdd.Sync()` path, `applied`; settled `sync-spec-deltas`); not re-run, canonical files untouched by this phase.
- [x] Change folder moved to `openspec/changes/archive/2026-09-16-fix-checkpoint-ask-context/` with all artifacts.
- [x] Active directory clean (only `archive/` remains under `openspec/changes/`).
- [x] Tasks complete in the persisted artifact (24/24; 0 unchecked) — no reconciliation needed.
- [x] No CRITICAL verify issues (0 blockers, 0 critical findings; verdict PASS, admitted).
- [x] `apply-progress.md` created from BigMem `obs-1789596779706862800-1` + archive addendum.
- [x] `state.yaml` created (phases through archive incl. `sync`, no `pending_question`, 4 defects appended).
- [x] No `.biggz-instance` existed; nothing deleted anywhere.
- [x] Nothing committed; the move + synced specs ride the final PR.
- [x] BigMem mirror of this report saved under `sdd/fix-checkpoint-ask-context/archive-report` (closure signal).
