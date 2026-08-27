# Design: hashline-lite

## Technical Approach

Lite `sdd-apply` via `internal/edit/hashline` reusing `filemerge.WriteFileAtomic` (temp+rename). Read hook captures seen ranges + snapshot; parser validates `PUT`/`CUT` `#A1B2`; apply checks `ComputeHash(exactRange)` — match→atomic write, mismatch→`needs_attention`+`freshHash` no overwrite, batch-safe. `NoopLoopGuard` aborts no-ops. `edit.mode=hashline` opt-in, transparent fallback, <400 lines.

## Architecture Decisions

### Decision: Parser

| Option | Tradeoff | Decision |
|---|---|---|
| PEG / goyacc | Heavy dep, >400 lines | Rejected |
| **Hand-rolled regex `^(PUT\|CUT)\s+(<N\|N.=M:)\s+#([0-9a-fA-F]{4})\b`** | Zero dep, <60 lines, strict | **Chosen** |

Missing/non-hex `#ZZZZ`/`#A1B` → error; whole-file fallback forbidden.

### Decision: Hash

| Option | Tradeoff | Decision |
|---|---|---|
| xxhash dep `cespare/xxhash` | Matches oh-my-pi, extra dep | Considered |
| **SHA-256 exact-range + 4-hex prefix alias** | Zero dep, reuses `filemerge.ComputeHash`, `empty→e3b0…` | **Chosen** |

4-hex is SHA-256 prefix (case-insensitive upper); swap to xxhash later without break.

### Decision: Snapshot

| Option | Tradeoff | Decision |
|---|---|---|
| On-disk `.hashline-snapshot/` | Leak/GC | Rejected |
| Global singleton | Cross-batch pollution | Rejected |
| **Per-batch `map[path][]byte` in `snapshot.go`, held by `sdd/apply.go`, cleared after batch** | Bounded ≤N, restore via `WriteFileAtomic` | **Chosen** |

### Decision: NoopLoopGuard

| Option | Tradeoff | Decision |
|---|---|---|
| Inside `WriteFileAtomic` | Tangles atomic concern | Rejected |
| **Before hash check in `apply.go`: `bytes.Equal(new, currentRange)` → abort** | Minimal, idempotent | **Chosen** |

### Decision: Flag & reuse

| Option | Tradeoff | Decision |
|---|---|---|
| Extend `filemerge` | Edits existing | Rejected |
| **New `internal/edit/hashline` importing `filemerge.WriteFileAtomic`; `sdd/apply.go` switch `edit.mode=hashline`** | Isolated, filemerge reused not modified | **Chosen** |

Windows `Access is denied` → contention (no retry); parents NOT auto-created.

## Data Flow

```
read hook → seenRanges{path→[][start,end]} + snapshot[path]=bytes
     │
directive → parser.Parse → ValidateSeen (N∈seen?) → NoopLoopGuard? abort
     │
hash guard: ComputeHash(exactRange) vs #A1B2?
     ├─ match → WriteFileAtomic (temp+rename)
     └─ mismatch → HashMismatchError{needs_attention,freshHash}, no write, batch continues
     └─ parse/seen err → transparent legacy fallback
```

`ComputeHash` exact bytes only; `nil/empty→e3b0…`; range 10-20≠whole.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/edit/hashline/parser.go` | Create | `Parse`, `Directive`, `ValidateSeen`, strict `#A1B2` |
| `internal/edit/hashline/apply.go` | Create | `Apply`, `NoopLoopGuard`, hash via `ComputeHash`, `WriteFileAtomic`, `HashMismatchError` |
| `internal/edit/hashline/snapshot.go` | Create | `Store{map}`, `Capture/Restore/Clear`, bounded ≤N |
| `internal/filemerge/hashline.go` | Reuse | `ComputeHash` exact-range (no edit) |
| `internal/filemerge/writer.go` | Reuse | `WriteFileAtomic` temp+rename (imported) |
| `internal/sdd/apply.go` | Modify | Read hook, `edit.mode` flag, route hashline vs legacy |
| `openspec/specs/hashline-lite/spec.md` | Exists | 8 reqs / 16 scenarios |

New `internal/edit/hashline/*` <400 lines.

## Interfaces / Contracts

```go
// parser.go
type Op string // "PUT" | "CUT"
type Directive struct { Op Op; Start, End int; HashTag string; Raw string }
func Parse(line string) (Directive, error)
func ValidateSeen(d Directive, seen [][2]int) error

// snapshot.go
type Store struct { m map[string][]byte; mu sync.Mutex }
func (s *Store) Capture(path string, content []byte)
func (s *Store) Restore(path string) error
func (s *Store) Clear()

// apply.go
func Hash4(fullSHA string) string // first 4 hex upper
func NoopLoopGuard(current, newContent []byte) bool
type HashMismatchError struct { Code, FreshHash, Path, Expected string }
func Apply(path string, d Directive, seen [][2]int, snap *Store, newContent []byte) (string, error)
```

Mismatch `Code="needs_attention"` + `FreshHash`, file unchanged, batch continues. `Access is denied` as `*os.PathError`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit parser | Valid PUT/CUT, bad `#ZZZZ`/missing, `<N` vs `N.=M` | Table-driven `Parse` |
| Unit seen | Unseen `50.=60` rejected, seen `10.=15` pass | `ValidateSeen` fixture `[1-20]` |
| Unit hash | Range≠whole (100-line 10-20), empty→`e3b0…`, 4-hex | Fixture + `filemerge.ComputeHash` |
| Unit snapshot | Restore, bounded ≤N, Clear | TempDir + Store |
| Unit NoopLoopGuard | equal abort, differ proceed | `bytes.Equal` |
| Integration apply | Match writes, mismatch no overwrite+`freshHash`, CUT removes, batch-safe B after A stale | `t.TempDir` + `errors.As` |
| E2E | `edit.mode` legacy vs hashline, `go vet` + `go test -count=1 -timeout 180s`, token saving ≥60% | Full run + fixture measure |

## Threat Matrix

N/A — no routing/shell/subprocess/VCS/PR/executable/process boundary.

| Boundary | Applicability | Reason |
|----------|---------------|--------|
| Documentation-like paths | N/A | No executable classification; caller-specified edit targets only |
| Git repository selection | N/A | No `git -C`/repo selection; file paths only |
| Commit state | N/A | No git index |
| Push state | N/A | No push |
| PR commands | N/A | No PR automation |

No RED tasks from matrix.

## Migration / Rollout

No migration. Opt-in `edit.mode=hashline` (default legacy). `git revert` deletes `internal/edit/hashline/*` + reverts hook/flag.

## Open Questions

- [ ] `#A1B2` case policy upper vs case-sensitive? → Assume case-insensitive, store upper.
- [ ] Provide `Hash4` helper or inline prefix? → Helper `Hash4`.

