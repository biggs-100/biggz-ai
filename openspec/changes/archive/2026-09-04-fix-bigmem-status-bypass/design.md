# Design: Fix BigMem Status Bypass

## Technical Approach

Replace the raw-SQL collector in `internal/sdd/engram_status.go` with the Store `*Ctx` API, keeping the filesystem-wins merge untouched. A minimal key-only Store method covers the `sdd/%` sweep that `SearchCtx` cannot express (hard cap 50, no topic-prefix predicate). Caller ctx flows through new `*Ctx` overloads following the Store's own D2 wrapper convention.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Sweep via `SearchCtx("", {Limit:50})`, paginate | No offset param exists; cap 50 truncates status past ~5 changes | Rejected |
| Raise global `SearchCtx` cap | Fragile, affects all callers, still hydrates full `content` | Rejected |
| Key-only sweep + hydrate visible via `GetCtx` | Correct, minimal hydration; needs one new Store method | **Chosen (Q1, Q3)** |
| `ctx` param on existing `Status*` funcs | Breaks 4 CLI callers + tests in `cmd/biggz/cli_sdd.go`, `engram_status_test.go` | Rejected |
| New `*Ctx` overloads, old funcs delegate via `Background()` | Zero caller breakage; mirrors `Search`→`SearchCtx` D2 pattern | **Chosen (Q2)** |
| `ResolveDBPath` + `QueryContext` in sdd | Fixes ctx/errors but keeps `sql` import, violates spec Req1 | Rejected fallback |

Q1 resolved: paginate impossible (no offset), raising the cap is global blast radius — dedicated uncapped key-only method. Q2 resolved: overloads `StatusCtx` / `StatusWithOptionsCtx` / `collectBigMemChangesWithArchiveCtx`; existing names become thin wrappers. Timeout via `bigmem.WithTimeout` (5s default, caller deadline wins). Q3 resolved: hydrate via existing `GetCtx(id)`; key listing needs the new method since Store has no key-only query — extension ships INSIDE this scope (`internal/bigmem/topic_prefix.go`, ~40 lines, served by existing `idx_obs_topic_lookup`).

## Data Flow

```
StatusWithOptionsCtx(ctx) ──→ applyStoreRoutingCtx ──→ collectBigMemChangesWithArchiveCtx
        │                                                          │
        │                                              bigmem.Open(root) [1 Store/ call, Close]
        │                                                          ↓
        │                                              ListByTopicPrefixCtx(sdd/%) — keys only,
        │                                              SQL-side project/scope/deleted_at filter
        │                                                          ↓
        │                                              Go-side: title-pattern parse, archive/seen sets
        │                                                          ↓
        │                                              GetCtx(id) — visible rows only
        │                                                          ↓
        └────────────── mergeFilesystemAndBigMem (unchanged, fs wins) ──→ derive*
IsSessionSummaryBlocked(ctx) — caller ctx replaces both Background sites (status.go:453/723)
```

Absent DB → `(nil,nil,nil)` + explicit `log` warning (allowed fallback). Query error → wrapped `fmt.Errorf("bigmem sdd-status <op>: %w")`, never silent nil.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/bigmem/topic_prefix.go` | Create | `TopicRow{id,topic_key,project,scope}` + `ListByTopicPrefixCtx(ctx,prefix,project,excludePersonal)`; key-only `LIKE ?` + `deleted_at IS NULL` + project/scope predicates, `ORDER BY topic_key`, no cap |
| `internal/sdd/engram_status.go` | Modify | Delete `openBigMemDB`/`queryBigMemRows`/raw scan + `database/sql`, `modernc.org/sqlite` imports; collector opens via `bigmem.Open`, sweeps keys, `GetCtx`-hydrates survivors, wraps errors |
| `internal/sdd/status.go` | Modify | Add `StatusCtx`/`StatusWithOptionsCtx`/routing/derive `*Ctx` variants; `Background` sites pass caller ctx; old funcs delegate |
| `internal/sdd/engram_status_test.go` | Modify (via tasks) | Parity tests: personal excluded, project match/override, visible-only hydration, cancelled-ctx fast fail, corrupt-DB wrapped error |
| `internal/bigmem/topic_prefix_test.go` | Create (via tasks) | Key-only predicate coverage incl. test-override (`project=""` disables filter) |

## Interfaces / Contracts

```go
// internal/bigmem/topic_prefix.go
type TopicRow struct{ ID, TopicKey, Project, Scope string }
func (s *Store) ListByTopicPrefixCtx(ctx context.Context, prefix, project string, excludePersonal bool) ([]TopicRow, error)

// internal/sdd/status.go
func StatusWithOptionsCtx(ctx context.Context, openspecRoot string, opts StatusOptions) (active, archived []ChangeStatus, err error)
// StatusWithOptions = StatusWithOptionsCtx(context.Background(), ...) — unchanged signature.
```

`project=""` disables the project filter (preserves `bigmemStoreRootOverride` test semantics); production passes the inferred project; `scope=personal` excluded in SQL via `COLLATE NOCASE`.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Prefix predicates, personal/case-insensitive exclusion, override bypass | `topic_prefix_test.go` on temp stores |
| Integration | Parity vs full-hydration; 100-row/2-visible hydrates 2; cancel fails fast; corrupt DB wraps | `engram_status_test.go` temp-store cases |
| E2E | `go test ./internal/sdd/... ./internal/bigmem/...`; grep no `sql.Open`/`db.Query`/`Background` at hot spots | CI-equivalent local run |

## Threat Matrix

N/A — read-only status path, no new trust boundary: no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration change. Worst case is a wrapped read error or explicit filesystem fallback.

## Migration / Rollout

No migration — read-only change, no schema or artifact format change. Per-file rollout: `topic_prefix.go` lands first (additive, safe), then `engram_status.go`, then `status.go` ctx threading. Rollback: revert commit; status falls back to filesystem-only on DB absence as before.

## Open Questions

None — all three input questions decided above. Test-override semantics (`SetBigMemStoreRootForTest` disables project filter) preserved by `project=""` convention.
