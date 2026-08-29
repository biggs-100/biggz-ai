# Proposal: Parity Gentle 69 — Ledger Budget

## Intent

Port gentle `e8cc0fcc→782e8dfe` ledger atomicity + dual-budget to biggz `3c8a247f` without regressing 4 FIXED (budget, FixDelta, GitCommonDir/v1/flock, `burned.json`, SDD v2). Close 5 NEW GAPs breaking CAS/budget.

## Proposal question round

Assume strict parity, 3 PRs stacked-to-main. Confirm: keep `2×MaxAttempts` cap? `ChangedLines` per-attempt vs cum? `hybrid` first slice? `Binary files differ` typed?

## Scope

### In Scope
- PR1 `cas_store.go:375` `commit()`: replay `loadRecord(revision)` before `writeLedgerHead` (`:1602`).
- PR2 `sddattempt.go`: `ChangedLines/CumulativeChangedLines`, `runtimeChangedLineBudgetExceeded` (`:2129`), refund capped `2×MaxAttempts` (`:2243/:2217`).
- PR3 `status.go:263`+`engram_status.go`+`sddattempt.go:1973`: `declaredArtifactStore` reads `openspec/config.yaml`, fix `resolveArtifactPaths`; rescope `MaxAttempts>cumAttempts && MaxLines>cumLines` (`:1087/:1425`).
- Taxonomy `review/capture.go` `RuntimeRecordRejectedError` (`:261`) if budget allows.

### Out of Scope
- `docs/`, `prompt-tombstone`, `cloud_upgrade_state`, lenses
- Migration beyond `biggz/sdd-runtime/v1`
- SDD v2 beyond `artifactStore`

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `runtime`: CAS verify-before-commit, dual budget, refund cap, cumulative rescope.
- `sdd-status`: `declaredArtifactStore` + hybrid routing.
- `review`: capture taxonomy (if sliced else deferred).

## Approach

3 PRs `stacked-to-main` `auto-chain`, each `<400` lines. Oracle `e8cc0fcc..782e8dfe`.

- **PR1 ≤20 lines**: `commit()` adds `replay()` before `writeLedgerHead`; test stale `Revision` CAS refuse.
- **PR2**: extend `RuntimeStore/RuntimeAttempt` with `ChangedLines`/`Cumulative`; `Cum+delta>MaxLines`; refunds while `refunded<=MaxAttempts`.
- **PR3**: `declaredArtifactStore()` reads config; `resolveArtifactPaths` branches by store; `Rescope()` enforces `newMax>carried cum`; hybrid filesystem-wins.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sddattempt/cas_store.go` | Modified | replay before HEAD |
| `internal/sddattempt/sddattempt.go` | Modified | budget, refund, `Rescope()` |
| `internal/sdd/status.go` | Modified | `declaredArtifactStore` |
| `internal/sdd/engram_status.go` | Modified | hybrid routing |
| `internal/review/capture.go` | Modified | `RuntimeRecordRejectedError` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| CAS regresses flock | Med | Keep `withStoreLock`; head-verify only |
| Budget mis-accounting | Med | Single owner; test `Cum+delta>MaxLines` |
| Refund `2×` loop | Low | Counter `refunded<=MaxAttempts` |
| Locator breaks `none/engram` | Low | Fallback `openspec` if config missing |

## Rollback Plan

Each PR `git revert` revertible, no migration. PR1 restores direct `writeLedgerHead`. PR2/3 drops `ChangedLines` (`omitempty`); `MaxLines` defaults `400`. Fallback `biggz sdd-attempt reset`.

## Dependencies

- Gentle `runtime_ledger.go:1602/2129/2243/1087/1425` `status.go:379`
- Existing `cas_store.go:320 replay()`/`:455 loadRecord()`/`withStoreLock`
- `modernc.org/sqlite` BigMem; `go test ./... -timeout 180s` + `go vet`

## Success Criteria

- [ ] PR1: stale `Revision` CAS refuses; HEAD never advances on mismatch
- [ ] PR2: `CumLines+delta>MaxLines` blocks `Acquire`; interrupted capped `2×MaxAttempts`
- [ ] PR3: `artifact_store` routes `tasks.md`; hybrid filesystem-wins
- [ ] `Rescope` refuses `newMax<=cum` (not `len`)
- [ ] No regression 4 FIXED: `go test ./internal/sddattempt ./internal/sdd ./internal/review -count=1` green
- [ ] Each PR `<400` lines, stacked-to-main
