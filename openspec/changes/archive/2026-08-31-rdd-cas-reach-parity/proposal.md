# Proposal: rdd-cas-reach-parity

## Intent

Bring biggz-ai RDD kill-switch to critical parity with gentle-ai. Today `internal/review/rdd.go` implements a simplified generation store (`gen-%010d.json` + SHA-256 revision) but lacks the concurrency, multi-install, and UX guarantees that make the switch safe in the wild. Three gaps block parity:

- **(A) No CAS guard:** writes do not take `expectedRevision` nor acquire `LOCK` (`flock`) nor surface `ErrRDDModeRevisionMismatch`. Two concurrent `biggz rdd disable --scope=clone` can race and silently overwrite, violating compare-and-set semantics.
- **(B) No Reach / mirror:** gentle publishes each clone-local generation to both the relocated root (`<commonDir>/biggz/rdd-mode`) and the pre-relocation mirror (`<commonDir>/gentle-ai/.../rdd-mode` legacy path) and reports `RDDModeReachMachine` vs `RDDModeReachThisBuild`. Biggz writes only the relocated root, so bug #3284 recurs: a user disables RDD with a new binary but older installed biggz binaries keep enforcing.
- **(C) Generic disabled errors:** `RDDDisabledError{Source, Operation}` prints `biggz rdd enable` for every source/operation. Gentle distinguishes `Source` (`default`/`global`/`clone_local`), prints the exact runnable continuation (`biggz rdd enable --scope=global` vs `global then clone` for clone), and differentiates `start` vs `mutate` (`the review is frozen, not discarded; turn reviews on with … to continue it from where it stopped`).

Goal: port the minimal safe subset of `gentle-ai/internal/reviewtransaction/rdd_mode.go:ResolveRDDMode / SetCloneLocalRDDMode / RDDModeReach / RDDDisabledError + reviewModeEnableForSource / Asked latch (reference only)` and `docs/architecture/organic-rdd.md` into `biggz-ai/internal/review/rdd.go` without touching delivery/review lifecycle.

## Scope

### In Scope

- **CAS with Revision + LOCK:**
  - Persist clone/worktree overrides as `gen-%010d.json` under `<gitDir>/biggz/rdd-mode` with `LOCK` file, schema `biggz-ai.rdd-status/v1` (or `gentle-ai.rdd-mode-override/v1` parity alias), fields `generation / previous_revision / mode / recorded_at / revision` (content-addressed SHA-256 over all fields except `revision`, zero-padded 10-digit generation, `maxGeneration = 999_999_999`).
  - Expose `ErrRDDModeRevisionMismatch` and enforce `expectedRevision` in `SetCloneLocalRDDMode` / `writeRDDMode` / `RDDDisable` CLI (`--expected-revision`). Fail closed with `%w: expected "x" but head is "y"`. Repair path: corrupt head is overwritten in-place at same slot (immutable no-replace publish), not chained.
  - Acquire `flock` on `rdd-mode/LOCK` before reading head → computing next generation → `publishImmutable` (create with `O_CREATE|O_EXCL`); stale detection unchanged. Ordering: relocated root first, mirror second, to avoid re-introducing #2882 hostage.

- **Reach + pre-relocation mirror (multi-install safety):**
  - Introduce `type RDDModeReach string` = `""` (unreported/read projection) / `"machine"` / `"this_build"` mirroring `gentle-ai/internal/reviewtransaction/rdd_mode.go:RDDModeReach`.
  - On clone-scope writes, publish identical bytes at identical slot in both the relocated root and the pre-relocation legacy location. `RDDModeReachMachine` when both succeed, `RDDModeReachThisBuild` when mirror unavailable/unwritable or publish fails. Reads (`ResolveRDDMode`/`RDDStatus`) remain single-root (no mirror probe) → `ReachUnreported`.
  - Surface `RDDModePartialApplyError` (`ErrRDDModePartiallyApplied`) when relocated publish succeeds but mirror publish fails while mirror is reachable — half-applied switch must never be reported as fully applied (inverse of fallback).
  - Add `Reach` to `RDDStatusReport` / `RDDModeStatus` and wire through `RDDEnable`/`RDDDisable`/`SetCloneLocalRDDMode` return values.

- **RDDDisabledError messages with Source + operation-aware continuation:**
  - Change `RDDDisabledError` to carry `Source RDDModeSource` (`default` | `global` | `clone`/`clone_local` | `worktree`) and `Operation RDDOperation` (`start` | `mutate`).
  - `Error()` prints typed source and exact command: `biggz rdd enable --scope=global` for `default`/`global`, `biggz rdd enable --scope=global then biggz rdd enable --scope=clone` (or `disable --scope=clone` clear) for `clone_local` — matching `reviewModeEnableForSource`. No generic `biggz rdd enable`.
  - Differentiate `start` vs `mutate`: `mutate` appends `; the review is frozen, not discarded` + `to continue it from where it stopped` (frozen not discarded invariant).
  - `AuthorizeRDDOperation` (and future `AuthorizeRDDCandidate`) must propagate `Source` from `RDDStatus`'s effective source correctly; `default` resolves as `global` for enable-path wording (opt-in semantics).

- **Status/Resolve alignment:**
  - Keep precedence `worktree > clone > global` but normalize source naming to `default/global/clone_local/worktree` for error mapping; ensure `RDDStatus` revision is the head's `Revision` (CAS token) and `Reach` is populated appropriately.

### Out of Scope

- Full `organic-rdd` lifecycle (atomic STATUS→START→FINALIZE→burn), lens selection, authority, or delivery boundary — unchanged.
- `Asked` one-shot consent latch (`asked.json`, `RDDConsentAsked`/`RecordRDDConsentAsked`) — referenced for completeness but not ported in this slice.
- CLI/TUI wiring beyond `rdd enable/disable/status --expected-revision/--scope` flags and error surfacing.
- `spec.md` / `design.md` / `tasks.md` creation — delegated to `sdd-propose` / `sdd-spec` / `sdd-design` / `sdd-tasks` in following phases (this change only scaffolds `proposal.md`).
- BigMem migration or global-state schema change beyond `rdd-mode.json` readability.

## Capabilities

### New Capabilities
- `rdd-cas`: compare-and-set generation store with `expectedRevision`, `LOCK`, and `ErrRDDModeRevisionMismatch`.
- `rdd-reach`: `RDDModeReach` + pre-relocation mirror publishing with `machine` / `this_build` / `unreported` reporting and `RDDModePartialApplyError` for half-applied switches.

### Modified Capabilities
- `rdd`: `RDDStatus`/`RDDStatusReport` now exposes `Revision` + `Reach`; `RDDDisable`/`writeRDDMode` accept expected revision and honor CAS; `RDDDisabledError` carries source-scoped exact enable command and frozen-not-discarded wording for `mutate`.
- `rdd-cli` (minor): disable/enable surface `--expected-revision` and forward revision errors without fallback.

## Approach

**Option A — minimal faithful port of gentle's `rdd_mode.go` (chosen):**

Port the three invariants verbatim, trimmed to biggz naming:
1. Generation helper: `computeGenerationRevision`, `scanGenerationHead`, `readLatestGeneration`, `publishImmutable(no-replace)` + `LOCK` via `internal/review/lock.go`'s `flock`. Generation width 10, SHA-256 domain `biggz-ai.rdd-mode-override-digest/v1`.
2. `SetCloneLocalRDDMode(ctx, repo, mode, expectedRevision, global)` pattern adapted to `(worktreeGitDir, commonGitDir, mode, expectedRevision)` — reads head under lock, validates `expectedRevision == head.Revision` (or `""` for no record), handles corrupt-head repair in same slot, publishes relocated then mirror, returns `RDDModeStatus{Reach}`.
3. `RDDModeReach` enum + `cloneLocalRDDModeMirror` struct (best-effort open, strict publish).
4. `RDDDisabledError.Error()` → `reviewModeEnableForSource(source)` + `rddOperationSubject` branching for `mutate`.

Rejected: **Option B — ad-hoc CAS without mirror/Reach** — fixes race but reintroduces #3284 multi-install divergence. Rejected: **Option C — full gentle copy including Asked latch + global digest** — larger surface, spills over to consent UX not required for this parity slice.

Reference sources (read-only, not to be duplicated verbatim):
- `gentle-ai/internal/reviewtransaction/rdd_mode.go` — `ResolveRDDMode`, `SetCloneLocalRDDMode`, `RDDModeReach`, `RDDDisabledError`, `reviewModeEnableForSource`, `Asked latch`.
- `gentle-ai/docs/architecture/organic-rdd.md` — opt-in switch semantics.
- `biggz-ai/internal/review/rdd.go` — current baseline (worktree>clone>global, generic messages, generation without LOCK/expectedRevision).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/review/rdd.go` | Modified | Add `RDDModeReach`, `ErrRDDModeRevisionMismatch`, `ErrRDDModePartiallyApplied`, `RDDModeSource` typing, `Revision`/`Reach` in status, `LOCK` + `expectedRevision` in `writeRDDMode`/`SetCloneLocalRDDMode`, mirror publish, `RDDDisabledError` source-aware messages |
| `internal/review/rdd_test.go` | Modified | CAS mismatch, corrupt-head repair, Reach machine/this_build, error message golden tests |
| `internal/review/lock.go` | Referenced | Reuse `flock` for `rdd-mode/LOCK`; no new lock impl |
| `internal/review/store.go` | Referenced | `GitCommonDir` resolution stays shared |
| `cmd/biggz/rdd.go` or `internal/cli/rdd*.go` | Modified | Wire `--expected-revision`, propagate `RDDDisabledError` source/operation, map `clone`→`clone_local` wording |
| `openspec/specs/review/spec.md` | Follow-up (spec phase) | New requirements for CAS/Reach/disabled-message |
| `openspec/specs/cli/spec.md` | Follow-up | Flag contract for `--expected-revision` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Generation exhaustion (>999_999_999) or slot collision with pre-relocation head | Low | `maxGeneration` guard, `generation = max(head, mirror.head)+1`, fail closed with explicit error |
| LOCK ordering deadlock between old/new binaries | Low | Always acquire relocated `LOCK` first, mirror second (documented order); never wait on authority tree (#2882) |
| Corrupt head blocks repair forever | Med | Overwrite corrupt slot in-place, immutable no-replace still prevents overwrite of readable generation; double-read head without parsing for slot number |
| Message change breaks downstream string-match | Med | Keep `errors.Is(err, ErrRDDDisabled)` sentinel; only `Error()` wording changes, not sentinel |
| Mirror directory not writable (permissions) falsely reports `this_build` | Med | `this_build` is correct per spec — old installs fail closed to previous value, not relaxed; document in status |

## Rollback Plan

`git revert <commit>` restores `internal/review/rdd.go` to pre-CAS/Reach state. No BigMem migration. Existing `gen-*.json` remain readable: `readLatestGeneration` tolerates both old (no LOCK) and new (with revision) records because revision is validated and generation scan is name-based. `LOCK` files are orphaned and harmless. RDD global state `~/.biggz/rdd-mode.json` unchanged.

## Dependencies

None. Isolated to `internal/review`. No cross-change dependency. Uses existing `internal/review/lock.go` and `internal/review/store.go` helpers.

## Success Criteria

- [ ] `SetCloneLocalRDDMode` / `RDDDisable --scope=clone --expected-revision=<rev>` returns `ErrRDDModeRevisionMismatch` when `expectedRevision != head.Revision` (both fresh and concurrent races).
- [ ] Generation files are `gen-%010d.json` with `schema`, `generation`, `previous_revision`, `mode="disabled"` (off-only), `recorded_at`, `revision=sha256`; `LOCK` is held during write; corrupt head is repaired at same slot.
- [ ] `RDDStatus` exposes `Revision` (CAS token) and `Reach`; write returns `ReachMachine` when mirror published, `ReachThisBuild` when mirror unreachable, `ReachUnreported` on read.
- [ ] Mirror publish writes identical bytes at identical slot in pre-relocation location; half-publish returns `RDDModePartialApplyError` wrapping cause and is not reported as fully applied.
- [ ] `RDDDisabledError` for `Source=global`/`default` prints `biggz rdd enable --scope=global`; for `Source=clone`/`clone_local` prints `biggz rdd enable --scope=global then biggz rdd enable --scope=clone`; `mutate` appends `the review is frozen, not discarded; … to continue it from where it stopped` while `start` does not.
- [ ] `AuthorizeRDDOperation(op=Read)` never blocks; `Start`/`Mutate` blocked when effective `disabled` with typed `RDDDisabledError` carrying correct `Source`.
- [ ] `go test ./internal/review -run TestRDD -count=1` and `go vet ./internal/review` pass; `biggz sdd-status --json` reports change `rdd-cas-reach-parity` active with `proposal: done`.

## Estimate

~250–350 lines in `internal/review/rdd.go` + ~120 lines tests. Single PR, no chaining (est. `review_budget_lines` <400). If forecast exceeds 400, split into (1) CAS+LOCK and (2) Reach+messages as chained PRs per `delivery_strategy`.
