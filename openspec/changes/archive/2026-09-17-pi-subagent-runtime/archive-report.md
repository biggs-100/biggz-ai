# Archive Report — pi-subagent-runtime

```yaml
schema: biggz-ai.archive-report/v1
change: pi-subagent-runtime
archived_at: "2026-09-17 (ISO date at close; local 21:24 (−05:00) = 2026-09-18T02:24Z)"
archived_to: openspec/changes/archive/2026-09-17-pi-subagent-runtime/
store: openspec (file-backed; the BigMem mirror of this report, when saved, is a closure signal only — the filesystem is the authority)
verification: pass_with_warnings (admitted — 19/19 requirements, 48/48 scenarios, 46 COMPLIANT + 2 PARTIAL pre-existing, 0 CRITICAL, 0 blockers)
evidence_revision: sha256:ae7fb693dc17ef06a1f63c81f684a6216944d2362e589dafd9373328108bdf09
committed: false (this phase committed nothing; the move and the synced living specs are working-tree changes)
delivery: pending — 10 stacked PRs planned (S0, 2a1, 2a2, 3a, 3b, 4a, 4b, 5a, 5b, S4), not yet cut; the trail is untracked
review_gate: unreviewed-at-archive (no review transaction exists; review obligation rides with delivery — see Review gate disposition)
```

## Change

`pi-subagent-runtime` — replace the third-party `pi-subagents-j0k3r` dependency with biggz-ai's own pi
subagent runtime (gentle-shell style) and eliminate the pi TUI overflow crash class by rendering
through host width primitives. Delivered scope:

- **Runtime asset** `internal/assets/pi/biggz-subagent-runtime.js` — one RPC child per task, strict
  JSONL reader (LF-only, CR strip, 1 MiB cap, malformed skipped+logged), spawn authorization via
  frontmatter `--tools` with `PI_SUBAGENT_CHILD=1`, stall watchdog (idle 4 min / total 30 min) and
  process-tree kill escalation (abort → SIGTERM group → SIGKILL; Windows `taskkill /PID /T /F`);
  registration gate (one `typeof`-guarded chain, fail-closed, never `pi.getTool`); tool surface
  `subagent` / `subagent_status` / `subagent_result` / `subagent_cancel` / `subagent_agents` /
  `subagent_wait`; background mode with completion delivery, widget rows, FIFO cap (default 2,
  `BIGGZ_BACKGROUND_SUBAGENTS` override); completion card and wait headline rendered via
  `truncateToWidth` from `@earendil-works/pi-tui` (no independent width math).
- **Width regression harness** `internal/assets/pi/biggz-subagent-width.test.mjs` + pi-tui
  resolver/oracle (`test/pi-tui-resolver.mjs`, `test/pi-tui-oracle.mjs`) — the exact w=190>188
  two-✅ crash shape pinned across widths 20–200.
- **Go cutover** — deploy list swaps the runtime in and `biggz-wait-pretty.js` out (13 JS entries
  via swap), stale self-heal removes retired copies, marker ownership
  (`SubagentRuntimeTargetName`/`MarkerPath`/`Capability`) lives in `internal/sdd`, adapter reconcile
  drops j0k3r from settings/`InstallCommand` when the marker is present, doctor checks the marker;
  four dead install helpers + `mergeJSONCWrapper` deleted with zero production callers.
- **Specs** — 6 domains: NEW `pi-subagent-runtime` (7 req / 16 scen), MODIFIED `runtime`,
  `agent-install`, `pi-deploy-list`, `orchestrator`; REMOVED `pi-integration` POLISH-PI-01/02 and
  `orchestrator` POLISH-ORCH-02 (each with Reason + Migration in the delta).

## Archive date & final path

- **Archive date**: 2026-09-17 (ISO date at close; local clock 21:24 (−05:00) = 2026-09-18T02:24Z).
- **Final path**: `openspec/changes/archive/2026-09-17-pi-subagent-runtime/` — moved in one
  `Move-Item` (same-volume rename, the `os.Rename` equivalent; `internal/sdd.ArchiveChange` has no
  CLI wrapper in this repo, so the rename was performed directly).
- The source directory no longer exists; the active changes directory now contains only `archive/`
  and the pre-existing `fix-gatekeeper-routing-coherence/` (untouched).
- No `.biggz-instance` existed in this change (checked with `-Force` pre- and post-move) — nothing
  to rename-preserve, nothing deleted anywhere.

## Artifact inventory (post-move, 14 files, 192,968 bytes)

| Artifact | Size | Status | Notes |
|----------|------|--------|-------|
| `_meta.yaml` | 299 | present | left untouched (audit trail; still reads `phase: explore`) |
| `exploration.md` | 28,262 | present | original exploration study |
| `proposal.md` | 4,441 | present | intent/scope/approach/rollback |
| `design.md` | 10,281 | present | D1–D7 + threat matrix + file-change table |
| `specs/` (6 domains) | 19,687 | present | `pi-subagent-runtime` 6,296; `agent-install` 4,238; `pi-deploy-list` 3,275; `runtime` 3,623; `orchestrator` 1,283; `pi-integration` 972 |
| `tasks.md` | 13,174 | present | 38/38 checked, 0 unchecked (Task Completion Gate PASS); delivery re-cut note at line 109 |
| `verify-report.md` | 31,346 | present | PASS WITH WARNINGS (admitted); evidence `sha256:ae7fb693…` |
| `apply-progress.md` | 85,254 | present | per-work-unit snapshots (history; post-verify remediation facts supersede its open items) |
| `state.yaml` | 224 | present | one comment line: checkpoint dual-write cleared (`proceed -> REAL ACTIVATION … then sdd-archive`) |
| `review-subject.json` | — | **NOT produced** | verify #8 wrote none; no RDD review ran (see Review gate disposition) |
| `archive-report.md` | this file | present | added at archive |

## Sync at archive (already applied by sdd-sync — NOT re-run)

The delta → living-spec application landed BEFORE the move through the prior `sdd-sync`
(approved `allow-destructive`; 6 targets, byte-verified against previews). This phase did **not**
re-apply it and did **not** touch `openspec/specs/**`. The skill's sync step was reduced to an
idempotency check; result: **re-applying the deltas today would be a no-op** — so no write was
warranted and none was performed.

**Idempotency check (fresh, read-only, archive-time):**

| Domain | Mode | Delta reqs | Check result |
|--------|------|-----------|--------------|
| pi-subagent-runtime | full-spec copy (new domain) | 7 | PASS — living spec **byte-identical** to delta (sha256 equal, 6,296 bytes) |
| runtime | delta | 2 | PASS — both MODIFIED requirements present exactly once, scenario counts equal |
| agent-install | delta | 3 | PASS — 2 MODIFIED + 1 ADDED present exactly once, scenario counts equal |
| pi-deploy-list | delta | 3 | PASS — 3 MODIFIED present exactly once, scenario counts equal |
| orchestrator | delta | 2 | PASS — 1 ADDED present exactly once; 1 REMOVED absent with `(Reason: …)` in delta |
| pi-integration | delta | 2 | PASS — both REMOVED absent from living; `(Reason: …)` + `(Migration: …)` present in delta |

No duplicates introduced, no missing requirements, no removed requirement resurrected — the merge
state is exactly the post-sync state.

**Living spec hashes at archive (fresh) vs the sync's recorded previews** — all six match
(`%TEMP%\opencode\sync-pi-subagent-runtime\apply-output.txt` / `runner-output.txt`):

| Domain | sha256 at archive | = sync preview |
|--------|-------------------|----------------|
| pi-subagent-runtime | `1d4b92becdbb1318d0059824cc65d0897ea565cdf1feae4599045b8c9229af73` | ✅ |
| runtime | `b07f652c37434dbc0dbffa69fa7137ef9f194f25214b2884cb0bbeede875692d` | ✅ |
| agent-install | `3c052336e851abe9920076b5bd1b35776c01326932c47ff3bb87473cd7018c8f` | ✅ |
| pi-deploy-list | `ed0f6184dab8b0b56a5fe76f0e2ec7591c01709a41d25dd645609fcd83fff3c2` | ✅ |
| orchestrator | `3716d2e3e94afa849d6956c1d41e72cbd5fcc032c8f4e41f23243f8db570fc94` | ✅ |
| pi-integration | `e1bfcbaa39431d9baa6ee59411b5b06e882e5a17a39ef55392884383c48655e2` | ✅ |

## Review gate disposition (declared)

**Facts at archive time (all fresh-checked):**

- **Kill switch state**: RDD global mode = `enabled` (`.biggz` not present; the mode file is
  `~/.biggz/rdd-mode.json`, written `enabled` at `2026-09-18T02:15:35.3166103Z`). No
  clone-local or worktree override exists (`.git/biggz/rdd-mode/` and the legacy mirror are
  absent). The skill's `disabled/unmanaged` relaxation therefore does **not** apply.
- **Review transaction state**: **none exists for this change.** `biggz review list --json`
  (fresh inventory) shows no `pi-subagent-runtime` lineage; the newest lineages all date from
  2026-09-11 or earlier and pre-date this change (created 2026-09-17T16:32:45Z). No
  `review-subject.json` was written by verify #8, and the verify-report makes no receipt claim.
- **A receipt is not producible from the current state**: the change's work is uncommitted; the
  RDD candidate resolves to `HEAD` = `8c2b2d19` (merge commit, 2026-09-17T09:40:19−05:00), whose
  tree does not contain this change. `review start` binds a commit subject — nothing this change
  produced can be reviewed until it is committed as part of delivery.
- **Native gate semantics** (read from the source, not assumed): the only native RDD gate is the
  verify preflight (`internal/sdd/verify.go:894` `verifyPreflightAt`): RDD disabled → pass
  regardless of receipt; RDD enabled → require a resolvable lineage + valid receipt. Verify attempt
  #8 settled at `2026-09-18T01:59:59Z` (ledger + record mtime); the global switch was written
  `enabled` at `02:15:35Z` — 15m36s **after** verify settled (the post-verify auto-enable). The
  native archive path (`internal/sdd/archive.go`) performs **no review check** and is pinned
  (guard test `archive_guard_test.go`) to never auto-disable RDD.

**Disposition**: archive proceeds as **unreviewed-at-archive**: no review state was pending,
malformed, scope-changed, invalidated, or escalated — no review exists, and none is producible
before commit. The review/delivery obligation rides with the planned 10-PR delivery, where the
repo's `pre-pr` and `pre-push` gates (both `enabled: true` in `.biggz/config.yaml`) and RDD apply.

**Precedent** (same treatment, recorded not hidden): `2026-09-16-fix-sdd-sync-empty-spec`
("no RDD review ran"), `2026-09-14-fix-attempt-ledger-scope` ("review gate unmanaged"),
`2026-09-13-sdd-fast-lane` (RDD enabled; native gate reported `burned/unmanaged`, did not block).

**Declared tension with the skill letter**: the archive skill's Native Review Receipt Gate asks for
`reviewGate.result: allow` or `disabled/unmanaged`. At this archive neither obtains (kill switch on,
no transaction). This report records the gap explicitly instead of fabricating a disposition; the
native authority does not gate archive, and demanding a receipt pre-commit is not satisfiable.
If the orchestrator wants a different reading, no repository state was mutated that would prevent
it (the move is a plain rename; nothing committed).

## Final verification (admitted)

- **Verdict**: PASS WITH WARNINGS — 19/19 requirements implemented, 48/48 scenarios evaluated fresh
  (46 COMPLIANT, 2 PARTIAL pre-existing WARNING-level gaps), 0 CRITICAL, 0 blockers.
- **Evidence**: `sha256:ae7fb693dc17ef06a1f63c81f684a6216944d2362e589dafd9373328108bdf09` (canonical
  combined suite output; `test_exit_code: 0`, build exit 0, empty build output hash).
- **Suites** (fresh): `go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor -count=1`
  5/5 packages ok; `go test ./internal/assets/biggz` ok; node — runtime 30/30, factory 14/14,
  width 11/11. Static gates: `go vet` clean, `gofmt -l` clean.
- **Remediation proof**: the previously-blocked `runtime` "Disabled reporting when policy off"
  scenario is COMPLIANT on both entry points (`internal/sdd/background.go` owner + thin-delegate
  twin in `internal/agents/pi/adapter.go`): `policy: off`, `capability: absent`,
  `disabled/unmanaged`, `type: warning`, positive control stays `info`.
- **Declared scope limitation at verify**: full `go test ./...` not re-run (CI owns the full matrix);
  an unrelated `internal/review` timeout observed during remediation was not re-observed (out of
  scope, zero references to changed packages).

## Activation (real machine, 2026-09-17 — orchestrator-reported, filesystem-corroborated)

Orchestrator-reported final-state facts (rank 3): repo build + `biggz install --agent pi --yes`
exit 0; runtime deployed to `~/.pi/agent/extensions/`; `biggz-wait-pretty.js` self-healed away;
`pi-subagents-j0k3r` dropped from `settings.json` packages and from the printed `InstallCommand`
list; pi loads all extensions with no crash and no new `pi-tui-crash.log` writes.

Fresh filesystem corroboration (this phase, read-only):

| Check | Observation |
|-------|-------------|
| `~/.pi/agent/extensions/biggz-subagent-runtime.js` | present, 39,982 bytes, mtime 2026-09-17 21:15:11 (−05:00) |
| `~/.pi/agent/extensions/biggz-wait-pretty.js` | absent ✅ |
| `~/.pi/agent/settings.json` | 0 occurrences of `pi-subagents-j0k3r`, mtime 21:15:11 ✅ |
| `~/.pi/agent/pi-tui-crash.log` | last write 2026-09-17 11:16:11 (−05:00) — ~10h **before** the 21:15:11 deploy; no writes during/after activation ✅ |
| Global RDD switch | written `enabled` at 21:15:35 (−05:00) = `2026-09-18T02:15:35Z`, 24s after the deploy mtime |

## Delivery

- **Nothing committed by this phase.** The orchestrator creates the 10 stacked PRs
  (S0, 2a1, 2a2, 3a, 3b, 4a, 4b, 5a, 5b, S4 — re-cut recorded in `tasks.md` line 109); the whole
  deliverable trail is currently untracked/modified in the worktree. The archive move and this
  report ride the delivery branch, not a commit made here.
- Work units are dependency-ordered with rollback boundaries tabulated in `tasks.md` (Review
  Workload Forecast); no `size:exception` was needed — the chain absorbs the >400-line slices.

## Ledger final shape (native `sdd-attempt`, read-only)

- Record HEAD revision: `5a782e9e8513e3739d953cedbb7745cc0b2c5839806bfe1a51b8f34ff554e0bd`
  (`complete: true`, `next_action: complete`, generation 7).
- 8 attempts, all work units settled: #1 `S0-pin` passed · #2 `S1a-runner` passed · #3 `S1b-tools`
  passed · #4 `S2-background` passed · #5 `S3-cutover` passed · #6 `S4-harness` passed ·
  #7 `change-verify` **failed** (evidence `sha256:6c9c57cf…`) · #8 `change-verify` **passed**
  (evidence `sha256:ae7fb693…`).
- Note (stale projection, recorded not resolved): the record's top-level `evidence_revision` field
  still carries the failed #7 hash while attempt #8 (the terminal passed verify) carries the final
  `ae7fb693…` hash that matches `verify-report.md`. The attempts list is authoritative for final
  state; the top-level field is an intermediate projection.

## Deferred verification item (NOT executed — recorded honestly)

- **Tool-execution smoke + emoji-heavy real-pi delegated run**: blocked by provider quota
  (`429 GoUsageLimitError`, opencode-go monthly limit, resets in ~6 days). This is the ONLY
  unexecuted item of the change. The activation itself (build, install, extension load) DID run on
  the real machine; what remains unproven is executing an actual subagent task through the runtime
  under real pi. The verify-report had already declared the real-pi activation smoke deferred at
  snapshot time (W4); the activation facts above partially retire W4 — the tool-execution half
  remains open, bounded by quota, not by a defect.

## Residual warnings (from the admitted verify-report; final state)

1. `{{BIGGZ_BACKGROUND_POLICY}}` dead substitution in `biggz-orchestrator-delegation.md:143`
   (pre-existing, untouched by this diff).
2. `install.Result.PiWebSearch` dead field (pre-existing; row 41 PARTIAL).
3. "Offline harmless" hint clause unexercised (pre-existing; row 40 PARTIAL).
4. Real-pi activation smoke deferred → superseded in part by the activation above; tool-execution
   smoke deferred by quota (see previous section).
5. Pi duplicate resolver carried in `adapter.go:750-888` (declared remediation deviation 1;
   functional parity test-covered).
6. Doc-vs-validator contradiction on `fail` persistence (process doc vs `verify.go`
   admissionCheckVerdict) — out of scope, recorded.

None of these falsifies an observable behavior; all are WARNING/declared level.

## State reconciliation notes

- `tasks.md` is the persisted authority: 38/38 `[x]`, 0 `[ ]` (fresh count in the archived file) —
  Task Completion Gate PASS, no reconciliation needed.
- `verify-report.md` / `apply-progress.md` are intermediate snapshots (history): their open items
  (remediation pending, activation unrun) expired with the remediation + activation; their "done"
  claims stay true. Final facts in this report outrank them.
- `_meta.yaml` left untouched (audit trail convention). `state.yaml` left as-is (its one comment
  records the cleared checkpoint). `review-subject.json` intentionally absent (see Review gate
  disposition).
- BigMem mirror: this report is also saved as `sdd/pi-subagent-runtime/archive-report` (closure
  signal for BigMem-derived status); the filesystem copy remains the authority.

## Post-archive hygiene (branch / worktree) — non-TTY

Non-interactive run — **nothing was deleted** (no `branch -d`, no `-D`, no worktree pruning):

- Observations: current branch `master`, single worktree `C:/Users/USER/Desktop/biggz-ai` at
  `8c2b2d19`; **0 local branches in `[gone]` state**; `git fetch --prune --dry-run` reported one
  stale remote-tracking ref (`origin/ci-probe/child`) that a real prune would remove — nothing was
  removed by this phase.
- Per non-TTY policy: delete nothing. Hint recorded: use `--dry-run` on CI.

## Archive verification checklist

- [x] Sync precondition satisfied before move (prior `sdd-sync`, `applied`, byte-verified; fresh
      idempotency check = re-apply no-op); living specs NOT re-written by this phase.
- [x] No CRITICAL verify issues (0 blockers, 0 critical findings; verdict PASS WITH WARNINGS,
      admitted).
- [x] Task Completion Gate: 38/38 checked, 0 unchecked in the persisted artifact.
- [x] Change folder moved to `openspec/changes/archive/2026-09-17-pi-subagent-runtime/` with all
      artifacts (14 files); source path confirmed absent.
- [x] Active changes directory clean of this change (only `archive/` + pre-existing
      `fix-gatekeeper-routing-coherence/`).
- [x] No `.biggz-instance` existed; nothing deleted anywhere.
- [x] Nothing committed; the move + synced living specs + this report remain working-tree changes.
- [x] Archive report persisted (this file); optional BigMem mirror noted above.
- [x] Review gate disposition recorded explicitly (unreviewed-at-archive; obligation rides with
      delivery).

## Working tree at close

The only `git status` delta caused by this phase is the path swap:
`?? openspec/changes/pi-subagent-runtime/` → `?? openspec/changes/archive/2026-09-17-pi-subagent-runtime/`.
All other entries (modified Go/JS/spec files, deleted wait-pretty/subagent-config, untracked runtime
assets, the other active change, `openspec/specs/sdd/spec.md`) are the pre-existing worktree state
and were left untouched. Exact list captured in the phase return envelope.
