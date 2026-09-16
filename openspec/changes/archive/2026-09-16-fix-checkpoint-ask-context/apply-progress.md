# Apply Progress — fix-checkpoint-ask-context

> Source of record: BigMem `sdd/fix-checkpoint-ask-context/apply-progress`
> (`obs-1789596779706862800-1`, upserted `2026-09-16T22:37:56Z` after slice 2b; the file never existed
> in the change dir pre-move — this file was created at archive from that observation). Content copied
> verbatim below; an archive addendum with the post-verify final-state facts follows so the trail is
> self-contained. Snapshot statements in the verbatim block (e.g. "#111 NOT merged") expired with the
> merges and are superseded by the addendum.

## BigMem apply-progress (verbatim)

Slice 2b (unit ask-context-2b-docs, tok-dba22c73018ce14c75a3d7fd) — docs truth pass landed; change complete.

Completed cumulative: 1.1-1.4 (branch fix/ask-context-substance-rule), 2.1-2.5 (fix/ask-context-ask-check), 3.1-3.7 + 5.4 slice-2 scope (fix/ask-context-wiring), 4.1-4.4 + whole-change 5.1-5.4 closing re-run (fix/ask-context-docs). All 29 scenarios covered; no pending tasks.
PRs: #108, #109, #110 stacked; #111 https://github.com/biggs-100/biggz-ai/pull/111 base fix/ask-context-wiring, label type:docs, NOT merged.
Slice 2b files: internal/assets/biggz/orchestrator_test.go (+96: TestOrchestratorAskContextItemsInvariant + TestArchitectureEnforcementClaimsMatchLivePath + readArchitectureDoc/askContextSection helpers), biggz-orchestrator-workflow.md (+2), docs/architecture.md (3+/3-). Total 104 changed lines <= 200 budget.
RED->GREEN: RED run 8 verbatim failures (4 pinned phrases missing :430, live-path citation x3 :438, docs deployed-path x3 :454, biggz-synthesis-gate.js "mirrors Go strictly" sentence without deployment-status marker :476) -> GREEN go test ./internal/assets/biggz/... ok 0.631s.
Four phrases pinned inside ## Visible Context Before Every Question (MANDATORY): "evidence found and how it was verified" | "problem, scope, effort, risk, what it unlocks, and deferral cost" | "recommendation with its reason" | "no-go condition - research more instead of asking early". Live path cited: ask-user-choice.ts -> biggz-ask-guard.js -> biggz sdd-ask-check (argv array, no shell, 1000 ms bound, stdin JSON).
Architecture before->after: "is the enforced gate" (no live path named) -> "reached at ask time by the deployed path ask-user-choice.ts -> biggz-ask-guard.js -> biggz sdd-ask-check (a decided block refuses to present; indeterminate degrades visibly with a warning)"; Layer 2 heading now "Go canonical + deployed ask check (blocking) + retired Pi gate source (advisory only)"; synthesis-gate.js sentence opens "(source-only, NOT deployed to ~/.pi/agent/extensions/)".
Closing gates (whole change): go test ./internal/sdd ./cmd/biggz ./internal/install/... ./internal/assets/biggz/... -count=1 -> ok sdd 38.375s / cmd 87.266s / install 14.007s / steps 6.168s / assets 0.772s, 0 FAIL; node --test internal/assets/pi/*.test.mjs -> tests 132, suites 16, pass 132, fail 0, cancelled 0, skipped 0; go vet ./internal/assets/biggz/... clean; gofmt -l clean; gitexec exit 0 "352 .go + 18 host targets, 0 debt, 7 boundary, baseline matched".
Rollback: revert docs commit alone (no code depends on prose); test commit reverts independently.
Deviations: pi suite count reporter delta 133 -> 132 vs part-A record (0 fail / 0 skipped both runs; no pi file touched in 2b). Nothing edited outside allowed surfaces.
Search keywords: checkpoint ask, apply-progress, fix-checkpoint-ask-context, slice 2b, docs pass, sdd-ask-check, ask-user-choice, architecture truth, pinned phrases.

## Archive addendum (2026-09-16, launch-prompt final-state facts; supersedes expired snapshot claims)

- **Delivery complete**: all four PRs merged — #108 `fdddf9ee`, #109 `6cd09745`, #110 `0dd35db4`,
  #111 `dc9c3d61`; master head `dc9c3d61`. The verbatim block's "#111 NOT merged" is a snapshot
  statement that expired with the merge. Every PR ran the full 18-check matrix (the stacked bases
  included, thanks to the earlier `ci.yml` fix); issue #14 is CLOSED (re-verified at archive:
  `gh issue view 14 --repo biggs-100/biggz-ai` → `CLOSED`).
- **Tasks**: 24/24 `[x]` in `tasks.md` with per-task evidence + 4/4 phase-5 closing gates — persisted
  artifact is the Task Completion Gate authority (fresh count at archive: 0 unchecked).
- **Verify**: PASS — 3/3 requirements, 29/29 scenarios, 0 blockers, 0 CRITICAL; admitted by
  `biggz sdd-verify-validate … --requirements 3 --scenarios 29` → "Verify report is valid.".
- **Sync**: landed through the canonical `sdd.Sync()` path (`result=applied`); living specs
  `sdd` 79→85, `orchestrator` 117→119, `pi-integration` 43→47 scenarios; requirement-name sets
  unchanged (30/34/16); diff `+81/−2` across the three tracked files.
- **Ledger**: units `ask-context-1a-predicate`, `ask-context-1b-cli`, `ask-context-2-wiring`,
  `ask-context-2b-docs`, `verify-report`, `sync-spec-deltas` all settled `passed`; `archive-report`
  was the current unit at close.
