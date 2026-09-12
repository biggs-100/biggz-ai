# Archive Report: fix-bigmem-recall-friction

**Change**: fix-bigmem-recall-friction
**Archived**: 2026-09-11 → `openspec/changes/archive/2026-09-11-fix-bigmem-recall-friction/`
**Store mode**: openspec (filesystem artifacts; key phases also mirrored to BigMem under `sdd/fix-bigmem-recall-friction/*` per project practice)
**Verdict at close**: PASS WITH WARNINGS — 21/21 tasks, 0 CRITICAL, 7/7 requirements, 27/27 scenarios. Delivered to master (PRs #54–#59, merge `c71dd60c`, issue #53 closed); archived retroactively as part of the `fix-rdd-receipt-collection` Phase-8 dogfood (receipt retro-collect + sync + archive with RDD active).

## Final State (terminal record — outranks intermediate snapshots)

BigMem recall friction is fixed end-to-end and shipped to `master`. The five reported defects are closed:

| # | Defect (proposal) | Fix | Proof |
|---|-------------------|-----|-------|
| 1 | Ghost-WAL false positive: live `biggz-mcp` holder triggered `bigmem_recovered` fallback | Liveness probe layers (`liveness_windows.go` share-0 / `liveness_other.go` conservative) + `classifyGhostWAL`; reclaim only on proven death | Ghost matrix tests + live-MCP E2E (no warning, no fallback, primary opened) |
| 2 | Session close not findable by recall | `SessionEnd` idempotent dual-write of a `session_summary` observation (`ON CONFLICT DO UPDATE`, retry once, explicit failure) | Session-close tests + E2E (recency finds latest close) |
| 3 | FTS queries with hyphens/accents returned zero | Per-token sanitize before `MATCH`; `match_mode=any` fixed; explicit zero-result signal + retry hint | FTS tests + E2E (`gentle-pi ruido` hits) |
| 4 | Latest summary truncated (>150 chars) | Full untruncated read in CLI `context`/`get` and MCP `mem_context`; search previews stay 120 | Read tests + E2E (181-char summary full, tail present) |
| 5 | Recall without call budget | Recall discipline in `biggz-orchestrator-workflow.md` (`mem_context` + ≤1 recency call; no FTS chains) | Invariant tests + E2E (2 reads answered) |

- **Delivery**: 4 slices merged via chained PRs #54–#57, collector PR #58 to tracker `fix/bigmem-recall-friction`, tracker PR #59 to master (merge commit `c71dd60c`). Issue #53 approved and auto-closed on merge to the default branch.
- **Final verification (full change)**: `go build ./...` exit 0; `go test ./... -count=1 -timeout 240s` exit 0 (60 packages ok, 0 FAIL); live-MCP stdio E2E 23/23 against a temp home with the real store untouched; `evidence_revision` = `sha256:7c57169e4c2a723d18b88c2a91568bd068254d761103e66d3f4db48a9b7ed50a`.
- **Warnings at close (non-blocking, carried)**: W-1 (positive REQ-GW2 reclaim / live-holder exception are Windows-only while spec/design stay platform-unqualified), W-2/W-3 (documented spec-compatible design deviations), W-4 (`apply-progress.md` carries no `use-modern-go` consultation evidence).
- **Snapshot attribution**: `verify-report.md` was authored against HEAD `75ec4ca9` (branch `fix/bigmem-recall-friction-4-summary-recall`) — an ancestor of the archive tree. Drift check at close (`git diff 75ec4ca9..HEAD` over the change's surfaces) shows no changes to any file this change touched; the only diffs are review-tooling files owned by the concurrent `fix-rdd-receipt-collection` change. Snapshot claims therefore remain valid at close; no authority contradictions were found.
- **Cycle context**: the change was implemented and delivered before this archive. This run moved committed artifacts only (`git mv`) — no archive-time code changes, no commits made (delegation constraint; the pending sync edits, the move, and this report remain uncommitted for orchestrator review).

## Gates at close (native dev build `./biggz.exe`)

| Gate | Result | Evidence |
|------|--------|----------|
| Task Completion | ✅ PASS — 21/21 checked, 0 unchecked; no reconciliation needed | Persisted `tasks.md` (`grep -c '- [ ]'` → 0, `- [x]` → 21); dispatcher `taskProgress.allComplete: true` |
| CRITICAL issues | ✅ PASS — 0 CRITICAL (4 WARNING, 3 SUGGESTION) | `verify-report.md` — "No failing test, no scenario without a passing covering test, no spec-breaking deviation, no incomplete task" |
| Native review receipt (RDD) | ✅ PASS — post-apply gate `allowed: true`, `delivery: "burned/unmanaged"` ("receipt is ephemeral and burned after finalize; delivery via ordinary repository policy") | `review gate post-apply review-d2d540998a64beba --json`; `review status` → `chain_valid: true`, `integrity_verdict.valid: true` |
| Session-summary guard | ✅ PASS — `{"verified":true,"reason":"","fallback":""}` | `session-close --check-only --json` |
| Native dispatcher (final authority) | ✅ `archive: ready`, `blockedReasons: []`, `nextRecommended: archive`; `apply/verify/sync: all_done` | `sdd-status --json`; `sdd-continue` → "Next phase: archive" |
| Action-context guard | ✅ repo-local; archive edits confined to `openspec/changes/**` | `actionContext.allowedEditRoots: [repo root]` |

**Review receipt trail** (governing lineage for HEAD `4f6646cf`): `review-d2d540998a64beba` — `start_review → in_review → complete_review → burn_review`; receipt binding `sha256:31e97d8ce66f997a93759e27632af2ee22251158cd11c91d63ca3da8495a18a1`; receipt artifact `receipts\c0e9759a74a8347e8175cff7e08a5ead4b1bcc35d7b216de6be4aad2407b58b0.json` (burned per the ephemeral-receipt lifecycle); budget counters `fix_rounds: 0`, `scoped_validations: 0`. History: an earlier transaction `fix-bigmem-recall-friction-c71dd60c` (started on merge commit `c71dd60c`) stands `withdraw` (event carries no payload; cause not evidenced); it does not govern HEAD and does not affect the gate.

## Specs synced (Step 2 — applied by the native sync phase; verified here, not re-applied)

| Domain | Delta action | Main spec | Numbers |
|--------|-------------|-----------|---------|
| bigmem | 3 ADDED (REQ-FR1 Full Summary Read, REQ-SC1 Searchable Session Close, REQ-FTS1 Multi-Token FTS Robustness — 10 scenarios); 3 MODIFIED (REQ-GW1, REQ-GW2, REQ-GW3 — 11 scenarios; GW3 retitled "Fresh Normal Path, Stale-Busy Fallback", old context kept as "(Previously: …)") | `openspec/specs/bigmem/spec.md` | +92/−10 |
| orchestrator | 1 MODIFIED (REQ-RR3 — Session Recall Gate Hardening, 6 scenarios) | `openspec/specs/orchestrator/spec.md` | +18/−4 |

- Verification: all 21 bigmem + 6 orchestrator delta scenarios matched verbatim in the main specs; requirements outside the delta untouched; no requirement removed; no destructive merges.
- Archive made **no changes** to `openspec/specs/**` — sync was already `all_done` before archive started (per delegation; sync edits stay uncommitted for orchestrator review).

## Archived contents

- `_meta.yaml` — retained verbatim (created at propose; meta/state are never rewritten by archive, matching archived-change convention)
- `state.yaml` — retained verbatim (pre-native DAG stub; its `pending` lines are historical, superseded by the native dispatcher)
- `proposal.md` ✅
- `specs/bigmem/spec.md`, `specs/orchestrator/spec.md` ✅ (deltas as authored)
- `design.md` ✅
- `tasks.md` ✅ (21/21 complete)
- `apply-progress.md` ✅ (apply-era record: slices 1, 1b, 2, 3, 4)
- `verify-report.md` ✅ (PASS WITH WARNINGS, 0 CRITICAL)
- `archive-report.md` ✅ (this file; also mirrored to BigMem as `sdd/fix-bigmem-recall-friction/archive-report`)
- No `.biggz-instance` present in the change folder; none carried over.

## Post-archive hygiene (Step 3b)

- No cleanup candidates found: no local or remote branches match `fix/bigmem-recall-friction*`; `git worktree list` shows a single worktree (repo root, branch `fix/rdd-receipt-collection-9-close`).
- Non-TTY (sub-agent) execution → deleted nothing (contract: non-TTY path deletes nothing); no `fetch --prune` was run since no candidates exist in local state.

## Follow-ups for future work (not blockers)

- **W-1/S-1**: qualify REQ-GW1/GW2 platform scope in the spec, or document the non-Windows limitation (positive reclaim + live-holder exception are Windows-only; safe direction).
- **W-4**: `apply-progress` lacks `use-modern-go` evidence (no code change needed).
- **S-2**: POSIX advisory-lock probe to restore non-Windows reclaiming (named out-of-scope in `liveness_other.go`).
- **Environment**: install the corrected build — the `biggz` on PATH is still the stale 2026-09-07 binary that reproduces the ghost defect; use `./biggz.exe` until replaced.
- **Orchestrator**: commit the sync edits + this archive move (move + report are staged; nothing committed by this phase).

## Evidence refs (raw)

- `./biggz.exe sdd-status --json` → `dependencies: {apply: all_done, verify: all_done, sync: all_done, archive: ready}`, `blockedReasons: []`, `taskProgress: 21/21`
- `./biggz.exe sdd-continue fix-bigmem-recall-friction` → `Next phase: archive`
- `./biggz.exe review gate post-apply review-d2d540998a64beba --json` → `{"allowed": true, "delivery": "burned/unmanaged", "reason": "review burned: receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"}`
- `./biggz.exe review status review-d2d540998a64beba --json` → `chain_valid: true`, `integrity_verdict.valid: true`, `receipt.binding_hash: sha256:31e97d8ce66f997a93759e27632af2ee22251158cd11c91d63ca3da8495a18a1`, budget `{fix_rounds: 0, scoped_validations: 0, correction_lines: 95 / 190}`
- `./biggz.exe session-close --check-only --json` → `{"verified":true,"reason":"","fallback":""}`
- Verify evidence: `verify-report.md` `evidence_revision` `sha256:7c57169e…7ed50a`; BigMem mirror `sdd/fix-bigmem-recall-friction/verify-report` (`obs-1789080598887751400-1`)
- Delivery: BigMem `sdd/fix-bigmem-recall-friction/delivery` (`obs-1789149862993830700-1`) — master = `c71dd60c`, PRs #54–#59 MERGED, issue #53 CLOSED
- Move evidence: `git mv openspec/changes/fix-bigmem-recall-friction openspec/changes/archive/2026-09-11-fix-bigmem-recall-friction` → 9 staged renames (`R`) in `git status`
