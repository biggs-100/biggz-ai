# Proposal: fix-sdd-sync-empty-spec

## Intent

`syncApplyDeltas` (`internal/sdd/sync_helpers.go:180`) writes `ApplyDeltas` unconditionally (`:204`). A NEW domain in full-spec shape — mandated by `sdd-spec` — parses to 0 deltas, so `ApplyDeltas("", []) = ""` creates a 0-byte living spec under `applied`: a false-green (F1 of `fix-false-green-guards`). Archive carries the rule (`sdd-archive/SKILL.md:59`); sync doesn't.

## Evidence (OBSERVED)

- Mixed change → `applied` + 0-byte `fullspec-domain` spec (false-green); ADDED → 137 B OK.
- Full-spec-only → `not-applicable`, zero writes; parser returns the domain, `deltas=0`.
- New ADDED → 137 B OK; MODIFIED → 200 B OK.
- Gate-bypass probe → direct call writes 0 bytes, `err=nil`.

## Contract (rules)

1. New domain + main spec absent + delta file contains requirement blocks (full-spec shape) → copy the delta file VERBATIM into `openspec/specs/{domain}/spec.md`.
2. Delta file has content but NO requirement blocks → fail closed: `blocked`, naming the offending file and the remedy.
3. Universal guard: never write empty content over a non-empty source. A sync that produced nothing must never report a successful write.

## Scope

### In Scope
- Writer rules 1–3 in `syncApplyDeltas`; source path plumbed for rule 2.
- `Sync()` honesty: no `applied` for a no-content write.
- 5 RED tests: false-green; skipped; blocked; MODIFIED unchanged; no-empty-write guard.

### Out of Scope
- Zero-delta discovery: co-factor, not cause; gate skips stay; entries reach the writer.
- `sdd-spec` full-spec contract: stays legal.
- Parser beyond rule 1; phantom `biggz sdd-sync`; F2 (`# Delta for` lines).

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `sdd` — `Sync Execution Contract` (`openspec/specs/sdd/spec.md:130`): gains rules 1–3.

## Approach

Rule-1 verbatim branch before `ApplyDeltas`; rules 2–3 return `SyncBlocked`, propagated by `Sync()` — `applied` cannot describe a no-content write. No skill/parser change.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/sync_helpers.go` | Modified | Rules 1–3 |
| `internal/sdd/sync_helpers_test.go` | New | 5 RED tests |
| `internal/sdd/sync.go` | Modified | Honest result |
| `openspec/specs/sdd/spec.md` | Modified (delta) | `Sync Execution Contract` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Copy leaks `# Delta for` headers | Med | Full-spec shape only |
| ADDED/MODIFIED regress | Low | Tests 1–4 pin |
| Zero-byte source undefined | Low | Never empty spec |

## Rollback

Revert the writer commit (`sync_helpers.go`, `sync.go`); copy fires only on absent mains — atomic, no migration.

## Dependencies

None external. Reproducer: `exploration.md` (`go test -overlay`). Delivery: `auto-chain` · 400-line · `stacked-to-main` · `both`.

## Success Criteria

- [ ] Mixed change: full-spec domain verbatim.
- [ ] Full-spec-only: `not-applicable`, zero writes.
- [ ] Content-without-requirements: `blocked` naming the file.
- [ ] MODIFIED unchanged; `go test ./internal/sdd` clean (5 tests).
