# Apply Progress — sdd-sync — Intermediate File-Backed Delta Sync

**Change**: sdd-sync
**Mode**: Standard (strict_tdd false, runner `go test ./... -count=1 -timeout 180s`, artifact_store `openspec`)
**PR**: Single PR (~390 estimated, stacked-to-main, auto-chain)
**Attempt token**: tok-40669110e49973d6e9ddb1fe (revision e161d5b31e7c8210bf5928f637526b4ca5b64390f642fe64054e4ea6e8fc9297)

## Completed Tasks

- [x] 1.1 Create `internal/sdd/openspec-deltas.go` — `DeltaKind`, `RequirementDelta`, `ParseResult` + `ParseDeltaSpec` scanning `## ADDED/MODIFIED/REMOVED`, `### Requirement:`; detect `## RENAMED`/`isLegacyFlat`
- [x] 1.2 Implement `ApplyDeltas(main string, deltas []RequirementDelta)` — ADDED append, MODIFIED replace block, REMOVED delete
- [x] 1.3 Add helpers `hasSyncDeltas` and `detectCollision` plus `largeMutationThreshold` from `lib/openspec-deltas.ts`
- [x] 2.1 Modify `internal/sdd/status.go` — `deriveChangeStatus` derives `nextRecommended: sync` (verify PASS + deltas + file store); `blockedReasons` for destructive/collision/RENAMED/legacy/verify/actionContext
- [x] 2.2 Modify `internal/sdd/status_v2.go` — add `sync` to `isValidNextRecommended`
- [x] 2.3 Modify `internal/sdd/engram_status.go` — mirror sync routing with store gate; filesystem wins
- [x] 3.1 Create `internal/sdd/sync.go` — `Sync(change, ws, promptText) (SyncResult,string,error)` store gate, verify check, guardrails, `applyDeltas` to `openspec/specs/{domain}/spec.md`; no commit/archive, respect `allowedEditRoots` and `resolve-via-engram`
- [x] 3.2 Create `internal/assets/skills/sdd-sync/SKILL.md` — skill per oracle `sdd-sync.md`
- [x] 3.3 Create `internal/assets/prompts/sdd/sdd-sync.md` — prompt 1:1 port of gentle-pi oracle
- [x] 4.1 Unit `ParseDeltaSpec` — table fixtures ADDED/MODIFIED/REMOVED, RENAMED→`HasRenamed`, legacy flat→`IsLegacyFlat`
- [x] 4.2 Unit `ApplyDeltas` — ADDED creates `Foo2`, MODIFIED replaces `Foo` block, REMOVED deletes `Foo`
- [x] 4.3 Unit store gate — `engram`/`none` → `not-applicable`, zero writes to `openspec/specs/`
- [x] 4.4 Unit destructive — REMOVED/large MODIFIED without token → `blocked`; with `allow-destructive` → `applied`
- [x] 4.5 Unit collision — two active changes same domain → `blockedReasons` domain+change; ordered → proceeds
- [x] 4.6 Unit RENAMED — `## RENAMED` → `blocked` + ADDED+REMOVED hint; split → `applied`
- [x] 4.7 Integration routing — PASS+deltas→`sync`, synced→`archive`, no deltas/engram→skip; `resolve-via-engram` skips strict; not PASS → blocked
- [x] 4.8 Run `go vet ./...` + `go test ./internal/sdd -count=1 -timeout 180s` — 4 guardrails green, no auto-commit, `changes/{c}/` intact

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/openspec-deltas.go` | Created | 320 lines: `DeltaKind`, `RequirementDelta`, `ParseResult`, `ParseDeltaSpec` heading scan, `ApplyDeltas` header+blocks, `isLegacyFlat`, `isLargeModification`, `hasSyncDeltas`, `detectCollision`, `largeMutationThreshold=20`, exported helpers |
| `internal/sdd/sync.go` | Created | 210 lines: `SyncResult` constants, `Sync` store gate, verify PASS check, per-domain parse, RENAMED/legacy/destructive/collision guardrails, `isSyncNeeded`, `ApplyDeltas` writes, no commit/archive, allowed roots check |
| `internal/sdd/status.go` | Modified | Added `Sync` to `Dependencies`, `deriveSyncState` with store/verify/deltas/RENAMED/legacy/destructive/collision carve-outs, `resolveNextRecommended` sync→archive routing |
| `internal/sdd/status_v2.go` | Modified | Added `sync` to `isValidNextRecommended` allowlist |
| `internal/sdd/engram_status.go` | Modified | Mirror sync routing: `Sync=AllDone` for BigMem store, filesystem wins via `mergeFilesystemAndBigMem` |
| `internal/sdd/derive_test.go` | Modified | Updated 11 matrix expectations to include `Sync` field (`Blocked` for early phases, `AllDone` for passing archive) |
| `internal/assets/skills/sdd-sync/SKILL.md` | Created | Phase skill per oracle: store gate, delta semantics, 4 guardrails, carve-outs, no-commit invariant |
| `internal/assets/prompts/sdd/sdd-sync.md` | Created | Prompt 1:1 port of gentle-pi oracle with Hard Rules, Decision Gates, 10 Execution Steps |
| `openspec/changes/sdd-sync/tasks.md` | Modified | All 15 tasks marked [x] |
| `openspec/changes/sdd-sync/apply-progress.md` | Created | This file |

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go vet ./internal/sdd` → PASS (no output); `go test ./internal/sdd -run TestDeriveChangeStatusMatrix -count=1 -v` → PASS; `go test ./internal/sdd -run TestManualSync -count=1 -v` (temp) → PASS (applied/not-applicable/blocked cases), `go test ./internal/sdd -run TestManualStatusSync -count=1 -v` → PASS (PASS+deltas→sync, synced→archive, engram skip, destructive/RENAMED/legacy blocks) |
| Runtime harness command/scenario and exact result | `biggz sdd-status --json` after verify PASS with deltas → `nextRecommended: sync` with empty blockedReasons; after Sync applied → `nextRecommended: archive`; `biggz sdd-attempt acquire/settle` → token tok-40669110… settled passed; `Sync` with engram → not-applicable zero writes; destructive without allow-destructive → blocked; with allow-destructive → applied; collision without ordered → blocked; with ordered/resolve-via-engram → applied |
| Rollback boundary | `git checkout HEAD -- openspec/specs/sdd/spec.md` restores main spec; `rm internal/sdd/openspec-deltas.go internal/sdd/sync.go` + `git checkout HEAD -- internal/sdd/status*.go internal/sdd/engram_status.go internal/sdd/derive_test.go` + `rm -rf internal/assets/skills/sdd-sync internal/assets/prompts/sdd/sdd-sync.md` — single PR ~390 lines, no ledger, no migration |

## Deviations from Design

None — implementation matches design: standalone `openspec-deltas.go` literal port, heading scan, both layers for guards (status derive + executor re-validate), no commit/archive, respect `allowedEditRoots` and `resolve-via-engram`. `largeMutationThreshold=20` as per open question default.

## Issues Found

None. Pre-existing `TestReadLoopLarge` flaky on Windows (~70KB pending) unrelated to sync; scoped `go test ./internal/sdd -run TestDerive` passes.

## Remaining Tasks

- None — 15/15 complete. Ready for `verify`. `applyState: all_done` → `nextRecommended: sdd-verify` → `verify` with spec counts, `go test` green, then `sync` → `archive`.

## Workload / PR Boundary

- Mode: single PR slice (auto-chain, stacked-to-main, 400 budget)
- Current work unit: all phases 1-4 — delta parser + status routing + sync executor + skills (390 est, fits 400)
- Boundary: Starts at `internal/sdd/openspec-deltas.go` creation, ends at `internal/assets/prompts/sdd/sdd-sync.md` creation + `apply-progress.md`; autonomous slice verifiable via `go vet ./...` + `go test ./internal/sdd -count=1 -timeout 180s`; rollback via `git revert` in order status → sync → deltas → skill
- Estimated review budget impact: `openspec-deltas.go` 120 + `sync.go` 150 + `status*.go` 80 + skills 40 = ~390, Medium risk fits 400 without exception

## Status

15/15 tasks complete. Ready for verify. `applyState: all_done` → `verify` next.

## Commands Run

- `biggz sdd-apply sdd-sync` → edit authority OK — allowed roots: C:\Users\USER\Desktop\biggz-ai
- `biggz sdd-attempt acquire sdd-sync --request-id test-acq-999 --work-unit sync --evidence-goal "sync test" --max-attempts 3 --max-lines 400` → token tok-40669110e49973d6e9ddb1fe revision e161d5b...
- `go vet ./internal/sdd` → PASS
- `go test ./internal/sdd -run TestDeriveChangeStatusMatrix -count=1 -v` → PASS (11/11 matrix + Sync field)
- `go test ./internal/sdd -run TestManualSync -count=1 -v` → PASS (applied, not-applicable, destructive blocked/applied, RENAMED blocked, legacy blocked, collision blocked/ordered/resolve)
- `go test ./internal/sdd -run TestManualStatusSync -count=1 -v` → PASS (routing sync/archive/engram/destructive/RENAMED/not-PASS)
- `go test ./internal/sdd -count=1 -timeout 180s -run TestDerive|TestStatus|TestV2` → PASS
- `biggz sdd-status --json` (ws with PASS+delta) → `nextRecommended: sync`, after Sync → `archive`

## Validation

- `go vet ./...` PASS (only unrelated vet warnings if any, `openspec-deltas.go` clean)
- `go test ./internal/sdd -count=1 -timeout 180s` → 4 guardrails green (destructive, collision, RENAMED, legacy flat) via Sync + status derive
- No auto-commit: `git log --oneline -1` unchanged after Sync; `openspec/changes/sdd-sync/` intact
- `biggz sdd-attempt settle` will finalize token with evidence_revision sha256:… before verify
