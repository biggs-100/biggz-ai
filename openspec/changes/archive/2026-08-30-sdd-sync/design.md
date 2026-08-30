# Design: sdd-sync — Intermediate File-Backed Delta Sync

## Technical Approach

Port `sdd-sync.md` + `lib/openspec-deltas.ts` 1:1 into `internal/sdd`. New phase `sdd-sync` syncs `openspec/changes/{change}/specs/**/spec.md` deltas to `openspec/specs/{domain}/spec.md` without archiving, keeping main specs current for stacked PRs. Status derives `nextRecommended: sync` between `verify→archive` when `verify PASS`, file-backed store, and deltas exist. Covers 6 req: Store Gate, Delta Semantics, Destructive, Collision, RENAMED, Carve-outs.

## Architecture Decisions

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Standalone `openspec-deltas.go` vs inline in `sync.go` | Standalone: single parser, mirrors TS lib, reusable by `archive.go`. Inline: duplicates logic. | **Standalone `openspec-deltas.go`** — literal port, oracle-tested. |
| Heading scan vs markdown AST | Scan: O(n) string ops, matches `verify.go` patterns, exact TS port. AST: correct for code blocks, heavy dep. | **Heading scan** — match `## ADDED/MODIFIED/REMOVED` + `### Requirement:`; `## RENAMED`/legacy-flat → blocked. |
| Guards in `deriveChangeStatus` vs executor-only | Derive: visible via `sdd-status --json` before run. Executor-only: late failure. | **Both layers** — status derives `blockedReasons`; executor re-validates before write. |

## Data Flow

```
sdd-status (derive)                         sdd-sync (executor)
─────────────────                           ──────────────────
StatusWithOptions                           Sync(change, ws, prompt)
 declaredArtifactStore(ws)                    store gate: openspec/hybrid?
 readChange → verifyResult                   else → not-applicable (0 writes)
 findSpecFiles(changes/{c}/specs)            parseDeltas(changes/{c}/specs/**/spec.md)
 verify PASS? deltas? → sync else            ├─ RENAMED → blocked + ADDED+REMOVED hint
   archive/verify/propose                    ├─ legacy flat → blocked + hint
 guardrail checks → blockedReasons           ├─ parse ADDED/MODIFIED/REMOVED
  (destructive, collision, RENAMED,          ├─ collision scan → blocked
   legacy flat, actionContext)               ├─ destructive (REMOVED|large MODIFIED)
 ProjectStatusV2 → JSON                      │  w/o approval → blocked
                                             └─ resolve-via-engram skips strict
                                             applyDeltas → openspec/specs/{d}/spec.md
                                             no commit, no archive, respect allowedEditRoots
```

Lifecycle: `proposal → spec → design → tasks → apply → verify → sync → archive`. Sync clears → re-derives `archive`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/sdd/openspec-deltas.go` | Create | Port `lib/openspec-deltas.ts`: parse `## ADDED/MODIFIED/REMOVED`, `ApplyDeltas`, detectors `isLegacyFlat`/`hasRenamed`. |
| `internal/sdd/sync.go` | Create | `Sync(change, ws, prompt) (SyncResult, string, error)` — store gate, verify pre-check, guardrails, writes, invariants. |
| `internal/sdd/status.go` | Modify | Extend `deriveChangeStatus`/`resolveNextRecommended`: `sync` between verify and archive; `blockedReasons` for destructive/collision/RENAMED/legacy-flat/verify/actionContext. |
| `internal/sdd/status_v2.go` | Modify | Add `sync` to `isValidNextRecommended`; project via `ProjectStatusV2`. |
| `internal/assets/skills/sdd-sync/SKILL.md` | Create | Phase skill per `sdd-sync.md` oracle. |
| `internal/assets/prompts/sdd/sdd-sync.md` | Create | Agent prompt — 1:1 port of `sdd-sync.md`. |
| `internal/sdd/engram_status.go` | Modify | Mirror sync routing for BigMem path if needed (else no-op via store gate). |

## Interfaces / Contracts

```go
// openspec-deltas.go — ported model
type DeltaKind string
const (DeltaAdded DeltaKind = "ADDED"; DeltaModified DeltaKind = "MODIFIED"; DeltaRemoved DeltaKind = "REMOVED")
type RequirementDelta struct { Kind DeltaKind; Name, Body string }
type ParseResult struct { Deltas []RequirementDelta; HasRenamed, IsLegacyFlat bool }
func ParseDeltaSpec(delta string) (ParseResult, error)
func ApplyDeltas(main string, deltas []RequirementDelta) (string, error)
// ADDED append, MODIFIED replace block, REMOVED delete

// sync.go
type SyncResult string
const (SyncApplied SyncResult = "applied"; SyncNotApplicable SyncResult = "not-applicable"; SyncBlocked SyncResult = "blocked")
func Sync(change, workspaceRoot, promptText string) (SyncResult, string, error)
// zero writes when not-applicable/blocked; prompt needs explicit token for destructive

func hasSyncDeltas(changeRoot string) bool
func detectCollision(change, ws, domain string) (bool, string)
```

Large MODIFIED threshold ports TS `largeMutationThreshold` (line-count delta); `promptText` must contain approval token (e.g. `allow-destructive`) to clear destructive guard.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `ParseDeltaSpec` ADDED/MODIFIED/REMOVED, RENAMED, legacy-flat | Table-driven fixtures; assert `ParseResult` + `ApplyDeltas` output. |
| Unit | Store gate `engram/none → not-applicable` zero writes | Temp workspace with varied `openspec/config.yaml`; assert `openspec/specs/` unchanged. |
| Unit | Destructive: REMOVED/large MODIFIED w/ and w/o approval | Prompt without/with token → `blocked` vs `applied`. |
| Unit | Collision: two active changes same domain | Two `changes/{a,b}/specs/sdd/spec.md` → `blockedReasons` lists domain+change. |
| Unit | RENAMED → blocked hint; rewrite ADDED+REMOVED succeeds | `## RENAMED` → blocked; split with approval → applied. |
| Integration | Routing `verify PASS → sync → archive` | `seedDeriveChange` matrix: PASS+deltas→sync, synced→archive, no deltas/engram→skip. |
| Integration | No commit / no archive move | After success assert `changes/{c}/` exists and `git log` unchanged. |
| Gate | `go vet` + `go test ./internal/sdd` | CI; 4 guardrails covered by unit+integration. |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Sync uses `os` writes only; no `exec`, no `git commit`, no subagents. `no commit` verified via log unchanged, not via git invocation.

## Migration / Rollout

No migration. Additive: `sync` skipped for `engram`/`none` or no deltas, preserving lifecycle. Rollback: `git revert` in order `status → sync → deltas → skill`; restore specs via `git checkout HEAD -- openspec/specs/{domain}/spec.md`.

## Open Questions

- [ ] Large-MODIFIED threshold exact value from `lib/openspec-deltas.ts` (`diffLines > 20` vs `>50%`).
- [ ] `resolve-via-engram` marker location — align with gentle-pi source.

