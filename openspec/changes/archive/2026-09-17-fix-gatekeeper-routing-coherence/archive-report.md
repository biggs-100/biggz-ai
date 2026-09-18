# Archive Report: fix-gatekeeper-routing-coherence

**Archived**: 2026-09-17
**Archive path**: `openspec/changes/archive/2026-09-17-fix-gatekeeper-routing-coherence/`
**Store**: openspec (file-backed)
**Status**: Complete — verified `pass_with_warnings`, spec synced (exactly once), delivery pending (PR not yet opened)

## Final State

| Fact | Value | Source / rank |
|---|---|---|
| Verify verdict | `pass_with_warnings` | `verify-report.md` (snapshot) + orchestrator final-state handoff (later, authoritative) |
| Evidence revision | `sha256:49e8255540756aeb7dba69ffc6c7db57fe712993dac7df52205259cfa21eddf0` | `verify-report.md` YAML header |
| Requirements | 1/1 PROVEN | verify-report compliance matrix |
| Scenarios | 6/6 PROVEN (S1–S6) | verify-report compliance matrix |
| Blockers | 0 | verify-report YAML header |
| Critical findings | 0 | verify-report YAML header |
| Tasks | 17/17 complete, 0 unchecked | `tasks.md` (Task Completion Gate passed; no stale checkboxes, no reconciliation needed) |
| Static authority `nextPhaseValid` | Removed from code; zero `.go` matches | verify-report §"Static table removed" |
| Sibling checks `artifact_existence` / `no_drift` | Byte-identical, behaviorally pinned | verify-report §"Sibling checks" |
| Delivery | NOT committed — planned as PR(s) from the three gatekeeper files + this sync + this archive trail | orchestrator final-state handoff |

## Spec Sync Record

The delta (`specs/sdd/spec.md`) was ALREADY propagated to the living spec by a prior `sdd-sync`, applied via the native runner without destructive flags (ADDED-only delta). Archive performed an idempotency check and did NOT re-write or re-apply anything:

| Check | Expected | Observed | Result |
|---|---|---|---|
| `openspec/specs/sdd/spec.md` sha256 (worktree) | `d69e8463b19618443d75ba3a2730c77c8dc5444e939e758983fb936de9be73fd` | `d69e8463b19618443d75ba3a2730c77c8dc5444e939e758983fb936de9be73fd` | ✅ match |
| Requirement blocks (`^### Requirement:`) | 31 (30 pre-sync + 1 ADDED) | 31 | ✅ exactly-once |
| New requirement occurrences | 1 ("Gatekeeper Routing Coherence from Dependency Authority") | 1 | ✅ no duplication |
| VS HEAD | +40 insertions, 0 deletions | `40 insertions(+)` | ✅ ADDED-only, non-destructive |
| File state after archive | stays MODIFIED in worktree (delivery payload) | untouched by archive | ✅ preserved |

**Main spec updated**: `openspec/specs/sdd/spec.md` — 1 requirement added, 0 modified, 0 removed, 0 renamed.

## Review Gate Disposition

**unreviewed-at-archive** — RDD is enabled for this repository (`.biggz/config.yaml`: `gate.pre-pr.enabled: true`, `gate.pre-push.enabled: true`), but there is NO review transaction for this change:

- No review artifacts in the change folder (no `review/`, no `review-subject.json` — verified via force listing).
- No review/RDD transaction refs in the repository (`git for-each-ref` shows no `refs/*review*`/receipt refs; only branch names contain "review").
- No receipt is producible pre-commit (the change is not committed yet).

Per orchestrator direction, this follows the same precedent as `pi-subagent-runtime`: the review obligation travels with the delivery PR and will be enforced by the enabled pre-PR/pre-push gates at publication time. Carried-forward reminder from verify-report (SUGGESTION 3): when the RDD gate arms, the orchestrator must write `review-subject.json` with `{"repository":"C:/Users/USER/Desktop/biggz-ai","commit_sha":"HEAD"}` before `biggz review start --subject ...`.

## Ledger State

`biggz sdd-attempt status fix-gatekeeper-routing-coherence` (read, exit 0):

- Active attempt: 0 — no open attempts
- Attempts: 3 (generation 2, lifetime 3), complete: true
- Verify attempt `routing-coherence-verify` settled (`tok-3e291ad262ce66ce7372d96f`) against the evidence revision above
- Next action: `complete`

No ledger blocker for archive.

## Warnings Carried Forward

1. **Review-workload budget exceeded (settled)** — delivered diff is 424 combined lines (366 insertions + 58 deletions) against the 400-line default review budget. `verify-report.md` recorded the settlement as *pending* at verification time; the orchestrator final-state handoff (later, authoritative) records the settlement as **recorded**. Final state: warning acknowledged, orchestrator settlement recorded, spec compliance unaffected.
2. **Full-suite leg delegated to CI** — `go test ./...` was not run locally: the literal `-timeout 180s` command times out on this Windows box (`internal/review` needs 175–290 s; prior phase killed by runner cap). CI owns the full matrix (task 5.2 note; PR #94 ceiling `go test ./internal/review/ -count=1 -timeout 600s` → ok 175.560s). Focused tests, build, vet, gofmt all clean.
3. **Spec-letter nuance (SUGGESTION 1)** — the spec delta asks `checkRouting` to receive "the workspace root"; the implementation receives the openspec root and derives the workspace root (`filepath.Dir`, `gatekeeper.go:449`) — matches design D1 interface; no behavioral impact.
4. **Label nuance (SUGGESTION 2)** — `tasks.md` marks 5.2 `[x]` while apply-progress qualified one leg as deferred to CI; claim is qualified rather than hidden, documented in the task text itself.

## Provenance Notes & Contradictions

- **Snapshot vs final state (ranked)**: `verify-report.md` says the 400-line-budget "orchestrator settlement is pending" — true at verification time. The orchestrator handoff (rank 3, later) states the settlement is recorded. This report carries the final state per the Final-State Authority hierarchy; the snapshot claim is preserved verbatim above as history.
- **Non-ranked discrepancy (recorded, non-blocking)**: the launch context stated the worktree contains untracked `openspec/changes/archive/2026-09-17-pi-subagent-runtime/`. Observed reality: that directory does NOT exist in this worktree (force listing + `Test-Path` false; `git status` never showed it); the 5 `pi-subagent-runtime` branches (local + remote) do exist and were NOT touched by this archive. No effect on this change.
- `_meta.yaml` still reads `phase: propose` — historical snapshot; archived content is an immutable audit trail and was not modified.
- No `.biggz-instance` marker existed in the change folder; nothing to preserve (N/A).

## Archive Contents

| Path | Bytes |
|---|---|
| `_meta.yaml` | 509 |
| `proposal.md` | 3950 |
| `specs/sdd/spec.md` | 3379 |
| `design.md` | 6571 |
| `tasks.md` | 4428 |
| `apply-progress.md` | 7171 |
| `verify-report.md` | 16989 |
| `archive-report.md` | this file |

## Post-Archive Hygiene (Step 3b)

**Skipped** — non-TTY session (agent/CI): no branch/worktree deletions attempted, nothing deleted; hint: `use --dry-run on CI`. The 5 pushed `pi-subagent-runtime` branches/PRs remain untouched.

## Traceability

- Delta spec: `openspec/changes/archive/2026-09-17-fix-gatekeeper-routing-coherence/specs/sdd/spec.md`
- Living spec (source of truth): `openspec/specs/sdd/spec.md`
- Verify evidence revision: `sha256:49e8255540756aeb7dba69ffc6c7db57fe712993dac7df52205259cfa21eddf0`
- Ledger: verify attempt settled, `Active attempt: 0`, `Complete: true`
- Delivery payload (uncommitted, untouched by archive): `internal/sdd/gatekeeper.go`, `internal/sdd/gatekeeper_test.go`, `cmd/biggz/sdd_gatekeeper_cli_test.go`, `openspec/specs/sdd/spec.md`, this archive trail
