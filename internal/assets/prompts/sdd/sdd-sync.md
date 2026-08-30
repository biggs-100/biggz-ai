---
name: sdd-sync
description: Sync delta specs to main specs without archiving for stacked PRs. Port of gentle-pi sdd-sync. Trigger: orchestrator launches sync after verify PASS for file-backed store.
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
  delegate_only: true
---
## Language Domain Contract

Generated technical artifacts default to English. Do not inherit the user's conversational language or the active persona's regional voice for SDD artifacts unless the user explicitly requests that artifact language or the project convention requires it.

If Spanish technical artifacts are explicitly requested, use neutral/professional Spanish unless the user explicitly asks for a regional variant.

Public/contextual comments follow the target context language by default. Explicit user language or tone overrides win; Spanish comments default to neutral/professional Spanish unless the user or target context clearly calls for regional tone.

## Activation Contract

Run when the orchestrator launches sync for an SDD change with `verify` PASS and file-backed deltas. You are the intermediate sync that keeps `openspec/specs/` current without archiving, enabling stacked PRs. Port `sdd-sync.md` + `lib/openspec-deltas.ts` 1:1.

The orchestrator should provide structured status from `_shared/sdd-status-contract.md` and the prompt text (may contain `allow-destructive`, `resolve-via-engram`, `ordered`).

## Hard Rules

- Read all available status `contextFiles` before syncing. The sync reads `proposal`, `specs`, `design`, `tasks`, and `verify-report` to validate readiness.
- Run only when `verify` is `PASS` and `tasks` are `allDone` with file-backed store (`openspec`/`hybrid`) and at least one delta exists; otherwise return `blocked` or `not-applicable` without writes.
- Execute `internal/sdd/openspec-deltas.go` parsing (single parser, heading scan, literal port of `lib/openspec-deltas.ts`): `## ADDED` append, `## MODIFIED` full-replace, `## REMOVED` delete; `## RENAMED` is not a helper.
- `engram`/`none` store → `not-applicable`, zero writes to `openspec/specs/`.
- `## RENAMED` in delta → `blocked` with `ADDED+REMOVED` hint; rewrite as `REMOVED OldName` + `ADDED NewName`.
- Legacy flat `openspec/specs/{domain}/spec.md` (no `### Requirement:`) → `blocked` with conversion hint.
- Destructive `REMOVED` or `MODIFIED` exceeding `largeMutationThreshold` (20 lines) without prompt `allow-destructive` → `blocked` mentioning destructive/approval. With `allow-destructive` → `applied`.
- Same-domain collision (another active `openspec/changes/{other}/specs/{domain}/spec.md`) without order decision → `blocked` listing colliding domain and change. `resolve-via-engram` or prompt `ordered`/`allow-collision` skips strict.
- `resolve-via-engram` skips strict destructive/collision guards.
- Respect `actionContext.mode` and `allowedEditRoots`; do not write outside allowed roots.
- MUST NOT create a git commit and MUST NOT move `openspec/changes/{change}/` to archive; change stays active after sync.
- Build sync result as exact candidate bytes before any write; if validator unavailable, make zero writes.
- Return the Section D envelope from `_shared/sdd-phase-common.md`.

## Decision Gates

| Condition | Action |
|---|---|
| Store is `engram`, `bigmem`, or `none` | `not-applicable`, zero writes |
| No delta specs under `openspec/changes/{change}/specs/` | `not-applicable` |
| `verify` not `PASS` | `blocked` (verify must be PASS) |
| Delta contains `## RENAMED` | `blocked` with `ADDED+REMOVED` hint |
| Main spec is legacy flat | `blocked` with conversion hint |
| `REMOVED` or large `MODIFIED` without `allow-destructive` | `blocked` with destructive/approval hint |
| Collision without order | `blocked` with domain+change |
| `resolve-via-engram` present | Skip strict destructive/collision |
| All guards pass | `applied`: deltas written to `openspec/specs/{domain}/spec.md` |

## Execution Steps

1. Load relevant skills via shared SDD Section A.
2. Retrieve artifacts via shared Section B for the active persistence mode, or read concrete `contextFiles` from structured status.
3. Validate store gate: `engram`/`none` → `not-applicable`.
4. Validate `verify` PASS via `readVerifyResult`; if not PASS → `blocked`.
5. Enumerate delta specs `openspec/changes/{change}/specs/**/spec.md` via `findSpecFiles`; if none → `not-applicable`.
6. For each delta file: `ParseDeltaSpec` → check `HasRenamed` → `blocked`; collect per-domain deltas.
7. For each domain:
   a. Check main spec `openspec/specs/{domain}/spec.md` for legacy flat → `blocked`.
   b. Check destructive (REMOVED or large MODIFIED via `isLargeModification`) without `allow-destructive` → `blocked` (skip if `resolve-via-engram`).
   c. Check collision via `detectCollision` without order → `blocked` (skip if `resolve-via-engram` or prompt `ordered`).
8. For each domain: `ApplyDeltas(main, deltas)` → write `openspec/specs/{domain}/spec.md` (mkdir -p, 0644); respect `allowedEditRoots`.
9. Verify invariants: `openspec/changes/{change}/` still exists, no new git commit, `openspec/specs/` reflects deltas.
10. Return sync summary: `applied`/`not-applicable`/`blocked` with message, no archive move.

## Output Contract

Return `## Sync Report` with change, store, verify state, domains synced, guardrail checks, and result `applied`, `not-applicable`, or `blocked`. Include `blockedReasons` when blocked, per `biggz-ai.sdd-status/v2`.

## Graceful Artifact Handling

- **No deltas or non-file store**: return `not-applicable` with zero writes.
- **Blocked guards**: return `blocked` with reason naming the guard (`RENAMED`, `legacy flat`, `destructive`, `collision`, `verify`) and hint (`ADDED+REMOVED`, `allow-destructive`, `ordered`, `resolve-via-engram`).
- **Success**: `applied` with per-domain actions (e.g., `sdd: 1 added, 1 modified, 0 removed`).

## References

- `internal/sdd/openspec-deltas.go` — ported delta parser and `ApplyDeltas`
- `internal/sdd/sync.go` — executor with guardrails and no-commit invariant
- `internal/sdd/status.go` — derives `nextRecommended: sync` and `blockedReasons`
- `internal/sdd/status_v2.go` — validates `sync` as allowed next action
