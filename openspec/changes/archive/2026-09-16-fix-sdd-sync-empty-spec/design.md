# Design: fix-sdd-sync-empty-spec

## Technical Approach

Make the write path fail closed: `syncResolveDomainWrite` returns either the exact bytes to write for a domain or a `SyncBlocked` result — `Sync()` can then never report `applied` for a contentless write. Rule 1 = raw-byte copy for absent targets; rules 2–3 + empty source = `SyncBlocked`. Traces 1:1 to the 6 new `Sync Execution Contract` scenarios.

## Architecture Decisions

| D | Decision | Options vs choice | Why |
|---|---|---|---|
| D1 | Guard locus | inline in `syncApplyDeltas` / per-domain `syncResolveDomainWrite` / pre-check in `Sync()` → helper | One decision point, unit-testable without the verify gate; `Sync()` unchanged — `res != ""` propagates `SyncBlocked` (`sync.go:60-62`). |
| D2 | Copy mechanics | synthetic ADDED deltas / raw `os.WriteFile` → raw bytes | Byte-identical required; `ApplyDeltas` strips H1/`## Purpose`. Copy fires only on `os.IsNotExist(mainPath)`; existing targets (even 0-byte) never replaced. |
| D3 | Source plumbing | parallel path map / `sources []domainSource` → sources | One read per file; per-file flags feed rules 1–2, D5; bytes feed the copy. |
| D4 | Empty source | skip / create / blocked → blocked | Rule 1 needs requirement blocks, rule 2 non-empty content — neither matches "non-empty source"; silent skip recreates the false-green class; gate routes single-file changes to `not-applicable`. |
| D5 | Multi-file domain | first sorted wins / ambiguous-blocked → classify | `findSpecFiles` recurses (`status.go:1037`); 1 full-spec candidate → copy, ≥2 → blocked naming both, 0 → rule-2 block; deterministic (sorted) order. |
| D6 | Idempotent re-run | re-copy / flip status / byte-equality skip → skip | Target == source bytes → no write, stays `applied`; no duplicate, reorder or degrade. ADDED/MODIFIED re-runs are already trim-equal no-ops. |
| D7 | `blocked` message | free text / fixed shape → fixed | `sync blocked: <diagnosis> (domain D, file path); <remedy>`; path + domain + remedy always present, stable prefix for tests. |
| D8 | Full-spec + existing differing target | silent normalized no-op / blocked → blocked | Stricter than the scenario text (forbids replacement only); flagged below. |

## Data Flow

```
changes/{c}/specs/{d}/spec.md ──▶ syncParseDomainInfos → []domainInfo{sources}
                                          │
                    syncResolveDomainWrite(info, mainPath)
  absent + 1 fullSpec ─▶ copy bytes ─┐
  deltas ─▶ ApplyDeltas ─▶ empty? ─▶ block ├─▶ pathidentity.Contains ─▶ WriteFile
  empty | no-blocks | ambiguous ─▶ block  ┘
                                          ▼
      Sync(): blocked propagates; applied only on content writes
```

## Interfaces / Contracts

```go
type domainSource struct {
    path     string              // changes/{c}/specs/{domain}/spec.md
    bytes    []byte              // copy payload
    empty    bool                // TrimSpace(bytes) == ""
    fullSpec bool                // requirementHeadingRe matches
    deltas   []RequirementDelta
}
type domainInfo struct {
    domain     string
    deltas     []RequirementDelta // aggregation unchanged
    hasRenamed bool
    sources    []domainSource     // NEW — findSpecFiles order
}
// nil bytes + SyncBlocked ⇒ blocked; + "" ⇒ skip (byte-equal re-run).
func syncResolveDomainWrite(info domainInfo, mainPath string) ([]byte, SyncResult, string, error)
func syncWriteBlockedMessage(reason string, info domainInfo, file, remedy string) string
```

Call sites: `syncParseDomainInfos` reworked; `syncGuardRenamed`/`syncCollectGuardrails` unchanged (named reads); `syncApplyDeltas` new loop; `sync.go:41,55` opaque — signatures unchanged, no exported surface.

## File Changes

| File | Action | Description | Magnitude |
|---|---|---|---|
| `internal/sdd/sync_helpers.go` | Modify | structs, `syncParseDomainInfos`, `syncResolveDomainWrite` + message builder, `syncApplyDeltas` loop | ~+100/−15 |
| `internal/sdd/sync_helpers_test.go` | Create | 5 RED + unit table | ~+260 |
| `internal/sdd/sync.go` | Verify (no change) | propagation already correct | 0 |

Untouched: `openspec-deltas.go`, `openspec-deltas_helpers.go`, `sync_guard.go`.

## Testing Strategy

| Layer | What | How |
|---|---|---|
| Unit | `syncResolveDomainWrite` table: empty→blocked; no-blocks→blocked; ≥2 fullSpec→blocked; byte-equal→skip; differing→blocked; REMOVED vs absent main→blocked (rule 3 backstop) | direct calls, `t.TempDir()` |
| Integration | T1 mixed→byte-identical copy + applied; T2 full-spec-only→not-applicable, zero writes; T3 no-blocks→blocked naming file+remedy, zero writes; T4 MODIFIED in place, header + untouched requirements preserved; T5 gate-bypassed `syncApplyDeltas`→blocked, no 0-byte file | in-package `Sync()` + passing `biggz-ai.verify-result/v1` fixture (needed by `syncVerifyMustPass`) |
| E2E | None | no CLI subcommand (phantom `biggz sdd-sync`); `go test ./internal/sdd -count=1` is acceptance |

## Threat Matrix

| Boundary | Applicability | Reason |
|---|---|---|
| Documentation-like paths | N/A | targets fixed (`spec.md`), no exec classification |
| Git repository selection | N/A | no git invocation |
| Commit state | N/A | sync never commits |
| Push state | N/A | no push surface |
| PR commands | N/A | no PR automation |

No routing/shell/subprocess/VCS/exec/process boundary; the new write copies bytes already in the change tree and reuses `pathidentity.Contains`. No matrix RED tests.

## Migration / Rollout

None; no flag. Existing 0-byte living specs are not auto-healed (D8 → `blocked`).

## Rollback

Revert `sync_helpers.go`; copies only fire where no living spec existed.

## Open Questions

- [ ] D8 is stricter than the scenario text (`MUST NOT be replaced`); confirm at tasks/review. Non-blocking.
