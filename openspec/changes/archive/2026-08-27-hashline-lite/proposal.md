# Proposal: hashline-lite — Go port of oh-my-pi hashline

## Intent

Cut token cost ~61% vs `str_replace` (68% vs 6% Grok Fast in oh-my-pi) and prevent `file modified since last read` via hashline-lite: `PUT N.=M: / PUT <N / CUT` with `#A1B2` xxhash 4-hex, seen ranges, snapshot, NoopLoopGuard, fallback. Lite line-precise `sdd-apply` without Rust/Bazel.

## Scope

### In Scope
- Parser for `PUT N.=M: / PUT <N / CUT` with `#A1B2` + seen-range validation
- `internal/edit/hashline/{parser,apply,snapshot}.go`: parse, hash-guard, snapshot, NoopLoopGuard, fallback via `WriteFileAtomic`
- `sdd-apply` read hook + flag `edit.mode=hashline`
- Exact-range `needs_attention`+`freshHash` warn-and-stop, batch-safe

### Out of Scope
- Full hashline, whole-file fallback, parent auto-Mkdir
- TUI/web/advisor, themes, Rust/Bazel, desktop sync
- Silent retry

## Capabilities

### New Capabilities
- `hashline-lite`: PUT/CUT DSL (`#A1B2`, seen ranges, snapshot, NoopLoopGuard, fallback) for `sdd-apply`

### Modified Capabilities
- None — new package; `filemerge` reused not modified

## Approach

Read hook captures seen lines + snapshot; parser validates `#A1B2`/ranges; `apply` checks `expected==hash(range)` before `WriteFileAtomic` atomic `temp+rename`; mismatch → `needs_attention`+`freshHash`, no overwrite, batch continues; `NoopLoopGuard` aborts no-ops. <400 lines, fixture-driven.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/edit/hashline/parser.go` | New | PUT/CUT parser, `#A1B2` tag, seen-range check |
| `internal/edit/hashline/apply.go` | New | Hash-guard apply, NoopLoopGuard, fallback |
| `internal/edit/hashline/snapshot.go` | New | Snapshot store |
| `internal/sdd/apply.go` | Modified | Read hook + `edit.mode=hashline` flag |
| `openspec/specs/hashline-lite/spec.md` | New | Spec for lite DSL |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Windows `Access is denied` on concurrent rename | Med | `WriteFileAtomic` + test tolerance |
| Scope creep to full hashline | Med | Lite gate: only PUT/CUT `#A1B2` |
| Snapshot growth | Low | Per-batch bounded snapshot |

## Rollback Plan

`git revert` single commit: delete `internal/edit/hashline/*`, revert `apply.go` hook/flag, remove `hashline-lite` spec + change folder. No migration.

## Dependencies

- `internal/filemerge.WriteFileAtomic`; `xxhash` 4-hex (or SHA-256 normalized to `#A1B2`)
- `biggz sdd-status` dispatcher (`openspec`, filesystem wins)

## Success Criteria

- [ ] PUT `N.=M:` and `PUT <N` with matching `#A1B2` succeeds; stale `h1` → `needs_attention`+`freshHash h2`, no overwrite, batch safe
- [ ] CUT matching hash removes range; mismatch preserves file
- [ ] Snapshot restores; `NoopLoopGuard` aborts loops; unseen `N` rejected
- [ ] `ComputeHash` exact-range ≠ whole-file (100-line fixture 10-20), empty→`e3b0...`, 4-hex `#A1B2`
- [ ] ≥60% token saving vs `str_replace`; `go test ./... -count=1 -timeout 180s` + `go vet` pass

## Proposal question round

Fallback assumptions (answer/skip/correct or second round):
1. Tag: xxhash 4-hex `#A1B2` vs SHA-256?
2. Package: `internal/edit/hashline` vs extend `filemerge/hashline.go`?
3. Flag `edit.mode=hashline` opt-in or global?

Assumes xxhash + new package + opt-in.
