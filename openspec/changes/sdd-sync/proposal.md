# Proposal: sdd-sync — Intermediate File-Backed Delta Sync

## Intent

`archive` merges deltas only at end. Stacked PRs need intermediate sync to `openspec/specs/` **without archiving** so main specs stay current while change stays active. Port `sdd-sync` from gentle-pi (`openspec-deltas.ts`).

## Scope

### In Scope
- Phase `sdd-sync`: sync `openspec/changes/{change}/specs/` → `openspec/specs/` without archive move
- Agent `sdd-sync` + skill `internal/assets/skills/sdd-sync/SKILL.md` + prompt `sdd-sync.md`
- Port `lib/openspec-deltas.ts` ADDED/MODIFIED/REMOVED to `internal/sdd`
- Status guard `nextRecommended: sync` + `blockedReasons`
- Guardrails: store, destructive, collision, RENAMED
- Tests

### Out of Scope
- `engram`/`none` sync — `not-applicable`, zero writes
- RENAMED, auto-commit, child subagents, TUI/cloud

## Capabilities

### New Capabilities
- `sdd-sync`: intermediate file-backed delta sync without archiving

### Modified Capabilities
- `sdd`: lifecycle adds `sync` between `verify`→`archive`
- `sdd-status`: derives `sync` routing and guardrail blocked reasons

## Approach

Port `sdd-sync.md` 1:1:
1. **Store gate**: `openspec`/`hybrid` only; else `not-applicable`, no writes.
2. **Deltas**: ADDED append, MODIFIED full-replace, REMOVED delete; legacy flat → `blocked`.
3. **Destructive**: REMOVED/large MODIFIED without explicit prompt approval → `blocked`.
4. **Collision**: same `openspec/specs/{domain}/spec.md` touched by active change without order decision → `blocked`.
5. **RENAMED**: `## RENAMED` → `blocked` (rewrite as ADDED+REMOVED, no helper).
6. **Carve-outs**: `resolve-via-engram` skips strict; `verify` must be PASS; respect `actionContext.mode`/`allowedEditRoots`; no commit/archive.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/status*.go` | Modified | `sync` routing + guards |
| `internal/sdd/openspec-deltas.go` | New | Port ADDED/MODIFIED/REMOVED |
| `internal/sdd/sync.go` | New | Executor (no archive move) |
| `internal/assets/skills/sdd-sync/SKILL.md` | New | Phase skill |
| `openspec/specs/sdd/spec.md` | Modified | Lifecycle + sync |
| `openspec/specs/sdd-sync/spec.md` | New | Via sdd-spec |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Delta drift | Medium | Literal port of `openspec-deltas.ts`; oracle tests |
| Destructive without approval | Low | Block unless prompt explicit |
| Same-domain race | Medium | Block + surface colliding domains |
| Legacy flat spec | Low | Detect → `blocked` with hint |

## Rollback Plan

`git revert` reverse: status → executor → deltas → skill. Sync is additive write, no ledger. Restore `openspec/specs/` via `git checkout HEAD -- openspec/specs/{domain}/spec.md` if mutated.

## Dependencies

- `internal/sdd` + `openspec/config.yaml` store resolution
- Oracle `sdd-sync.md` + `lib/openspec-deltas.ts`

## Success Criteria

- [ ] ADDED/MODIFIED/REMOVED synced without archive; `nextRecommended: sync` clears after
- [ ] `engram`/`none` → `not-applicable`, zero writes
- [ ] REMOVED/large MODIFIED without approval → `blocked`; with approval → ok
- [ ] Same-domain without order → `blocked`
- [ ] `## RENAMED` → `blocked` (ADDED+REMOVED hint)
- [ ] `go vet` + `go test ./internal/sdd` green; 4 guardrails tested
