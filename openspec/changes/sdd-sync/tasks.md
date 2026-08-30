# Tasks: sdd-sync — Intermediate File-Backed Delta Sync

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~390 (deltas 120 + sync 150 + status 80 + skills 40) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (~390) — fits 400; optional PR1 deltas+status / PR2 sync+skills if >400 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Delta parser — `openspec-deltas.go` ADDED/MODIFIED/REMOVED | PR 1 | `go test ./internal/sdd -run TestParseDelta -count=1 -v` | N/A — pure parser | `internal/sdd/openspec-deltas.go` |
| 2 | Status routing + sync executor + skills | PR 1 | `go test ./internal/sdd -run TestSync -count=1 -v` | `biggz sdd-status --json` after `verify PASS` with deltas | `internal/sdd/status*.go` + `internal/sdd/sync.go` + `internal/assets/skills/sdd-sync/` |

## Phase 1: Foundation — Delta Parser

- [x] 1.1 Create `internal/sdd/openspec-deltas.go` — `DeltaKind`, `RequirementDelta`, `ParseResult` + `ParseDeltaSpec` scanning `## ADDED/MODIFIED/REMOVED`, `### Requirement:`; detect `## RENAMED`/`isLegacyFlat`
- [x] 1.2 Implement `ApplyDeltas(main string, deltas []RequirementDelta)` — ADDED append, MODIFIED replace block, REMOVED delete
- [x] 1.3 Add helpers `hasSyncDeltas` and `detectCollision` plus `largeMutationThreshold` from `lib/openspec-deltas.ts`

## Phase 2: Status Routing

- [x] 2.1 Modify `internal/sdd/status.go` — `deriveChangeStatus` derives `nextRecommended: sync` (verify PASS + deltas + file store); `blockedReasons` for destructive/collision/RENAMED/legacy/verify/actionContext
- [x] 2.2 Modify `internal/sdd/status_v2.go` — add `sync` to `isValidNextRecommended`
- [x] 2.3 Modify `internal/sdd/engram_status.go` — mirror sync routing with store gate; filesystem wins

## Phase 3: Sync Executor & Assets

- [x] 3.1 Create `internal/sdd/sync.go` — `Sync(change, ws, promptText) (SyncResult,string,error)` store gate, verify check, guardrails, `applyDeltas` to `openspec/specs/{domain}/spec.md`; no commit/archive, respect `allowedEditRoots` and `resolve-via-engram`
- [x] 3.2 Create `internal/assets/skills/sdd-sync/SKILL.md` — skill per oracle `sdd-sync.md`
- [x] 3.3 Create `internal/assets/prompts/sdd/sdd-sync.md` — prompt 1:1 port of gentle-pi oracle

## Phase 4: Testing & Verification

- [x] 4.1 Unit `ParseDeltaSpec` — table fixtures ADDED/MODIFIED/REMOVED, RENAMED→`HasRenamed`, legacy flat→`IsLegacyFlat`
- [x] 4.2 Unit `ApplyDeltas` — ADDED creates `Foo2`, MODIFIED replaces `Foo` block, REMOVED deletes `Foo`
- [x] 4.3 Unit store gate — `engram`/`none` → `not-applicable`, zero writes to `openspec/specs/`
- [x] 4.4 Unit destructive — REMOVED/large MODIFIED without token → `blocked`; with `allow-destructive` → `applied`
- [x] 4.5 Unit collision — two active changes same domain → `blockedReasons` domain+change; ordered → proceeds
- [x] 4.6 Unit RENAMED — `## RENAMED` → `blocked` + ADDED+REMOVED hint; split → `applied`
- [x] 4.7 Integration routing — PASS+deltas→`sync`, synced→`archive`, no deltas/engram→skip; `resolve-via-engram` skips strict; not PASS → blocked
- [x] 4.8 Run `go vet ./...` + `go test ./internal/sdd -count=1 -timeout 180s` — 4 guardrails green, no auto-commit, `changes/{c}/` intact
