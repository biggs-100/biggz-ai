# Sync Report: wire-pi-review-relay

**Change**: wire-pi-review-relay
**Date**: 2026-09-12
**Store mode**: file-backed (`artifactStore: openspec` per `biggz sdd-status --json`; orchestrator mirrors this report to BigMem `sdd/wire-pi-review-relay/sync-report`)
**Result**: `applied` — domain `review` synced to `openspec/specs/review/spec.md`; change NOT archived, NOT committed.

## Scope

- **Domains synced**: `review` only.
- **Untouched**: every other `openspec/specs/*` (verified: `git status --porcelain openspec/specs/` → only ` M openspec/specs/review/spec.md`), the delta spec, the code, `tasks.md`, `design.md`, `verify-report.md`.
- **Collision check**: no other active change carries `openspec/changes/*/specs/review/spec.md` → no ordering decision required.

## Verification reference (authorizes this sync)

| Field | Value |
|-------|-------|
| Verdict | `pass_with_warnings` (admitted by `biggz sdd-verify-validate`) |
| Requirements | 3/3 |
| Scenarios | 11/11 |
| verify-report SHA-256 | `sha256:7a5d0faa76d5779155de95817b1ed462be51353ad5a1fb0d70eca7ca16305dce` |
| verify-report `evidence_revision` | `sha256:6f44c5010dc49ebc69ccd6eacce94961653598106591708335dc1949f997fa91` (sweep hash declared inside the report) |

The verify gate passed: `syncVerifyMustPass` treats `pass` and `pass_with_warnings` as passing (`internal/sdd/verify.go`).

## Delta applied

| Kind | Requirement | Scenarios |
|------|-------------|-----------|
| ADDED | `Pi Reviewer Execute Mode` | 3 |
| ADDED | `Bounded Reviewer Execution and Typed Transport Failures` | 4 |
| MODIFIED | `Verbatim Transport and Tool-Less Reviewer` (block replaced in place; heading byte-identical; 30-line body) | 4 (replaces 2) |

Semantics per `internal/sdd/openspec-deltas.go`: ADDED appends the two blocks at the end (delta order preserved); MODIFIED fully replaces the matching requirement block in place; header and all unrelated requirements preserved.

## Destructive approval

- **Trigger**: the MODIFIED block (30 lines) exceeds `largeMutationThreshold` (20 lines) → destructive under `syncIsDestructive` (`internal/sdd/sync_helpers.go`). No `REMOVED` operations.
- **Approval**: explicit maintainer approval (checkpoint 2026-09-12, decision "Proceed — sync + archive"), effective as `allow-destructive` for domain `review` only, for exactly this delta.
- **Scope honored**: no other domain touched, no other delta operation performed.

## Line delta (canonical spec)

| Metric | Before | After |
|--------|--------|-------|
| File | `openspec/specs/review/spec.md` | same |
| Lines | 401 | 469 |
| SHA-256 | `fb3730a112b150a685eebe1367de8baecd350d00173a5d9072c32f1cc62eba81` | `5c2d86a9cafdcfdfd54793cfa5ec1ffef27a514e3a255735a98e24592e39acdf` |

**Exact line delta**: `+69/−1` (`git diff --numstat` → `69	1`), net **+68** lines.

## Requirement/scenario counts

| Metric | Before | Delta | After |
|--------|--------|-------|-------|
| `### Requirement:` | 19 | +2 | 21 |
| `#### Scenario:` | 53 | +9 net (+7 from ADDED, +2 net from MODIFIED: 4 replace 2) | 62 |

Cross-checks:
- All 3 delta requirements present in the canonical spec; each block byte-equal to the delta body (trimmed) — checked with the real parser.
- All 11 delta scenarios present in the canonical spec.
- No duplicated requirement heading; no leftover text from the replaced block (`MUST NOT be caller-authored`, `Reviewer model/provider selection…` absent).
- `ApplyDeltas(main, deltas) == main` → the canonical spec is idempotent under this delta (parser-level sync complete).
- `./biggz.exe sdd-status --json` → `dependencies.sync: "all_done"`, `archive: "ready"`, `nextRecommended: "archive"`.

## How it was applied / verified (exact commands)

The delta was applied with the repository's own parser (`ParseDeltaSpec` + `ApplyDeltas` from `internal/sdd/openspec-deltas.go`), invoked through a read-only Go test overlay (virtual `internal/sdd/zz_sync_check_test.go`; the test file lived in `C:/Users/USER/AppData/Local/Temp/zz-sync-check/`, never in the repo):

```text
go test ./internal/sdd -overlay C:/Users/USER/AppData/Local/Temp/zz-sync-check/overlay.json -run TestZZSyncWirePiReviewRelay -v -count=1
diff -u openspec/specs/review/spec.md C:/Users/USER/AppData/Local/Temp/zz-sync-check/applied.md
cp C:/Users/USER/AppData/Local/Temp/zz-sync-check/applied.md openspec/specs/review/spec.md
git diff --numstat openspec/specs/review/spec.md        # 69  1
grep -c '^### Requirement:' openspec/specs/review/spec.md   # 21
grep -c '^#### Scenario:' openspec/specs/review/spec.md     # 62
sha256sum openspec/specs/review/spec.md                 # 5c2d86a9…
./biggz.exe sdd-status --json                           # dependencies.sync: all_done
```

## Invariants

- `openspec/changes/wire-pi-review-relay/` still exists (no archive move).
- No commit: `git log --oneline -1` → `d0cd87c6` (unchanged by this sync).
- Only the allowed edit surfaces were written: `openspec/specs/review/spec.md` and `openspec/changes/wire-pi-review-relay/sync-report.md`.

## Next

`archive` — `./biggz.exe sdd-status --json` reports `archive: "ready"` and `nextRecommended: "archive"`.
