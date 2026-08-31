# Design: sdd-parity-rescope-grant-ledger

## Technical Approach

Close 7 gaps vs gentle-ai 2026-08-31 in one PR (auto-chain stacked-to-main, 255 tracked <400, budget 800). Guard ledger, verbatim rescope, ForInstance, memoised topology, adapted passive, per-token marker. Discard Handoff/Attestation/full scan (150 LOC).

## Architecture Decisions

| Gap | Opt | Decision | Rationale |
|-----|-----|----------|-----------|
| **G1 REQ-G1-01** `attempt.go:12` | del vs guard | Guard `ErrLegacyRetired` with `biggz sdd-attempt acquire\|settle`, no FS | Del breaks import; 15 LOC, `git revert` |
| **G2 REQ-G2-01/02** `sddattempt.go:1992` | wedge vs verbatim | Allow only `Active==0&&ObjID!=""&&!DecReq&&!Complete&&len>0&&last.Outcome!=""&&!drift`; check `new<=old→Widened` before `new<=cum→Exhausted`; carry `Cumulative*` | Wedge widens silently; 55 LOC; drift stub false+TODO `CandidateTree` |
| **G3 REQ-G3-01** `cas_store.go:86` | explicit vs sugar | `Store.ForInstance(i) (Store,error)` validates 1..128 trimmed single-line via `validateChangeInstance`; `grantedRootsFor` filters | Shared validator, equiv test; 25 LOC |
| **G4 REQ-G4-01** topology | off vs verbatim | `resolveExistingPath(EvalSymlinks)→gitRootOf(rev-parse --git-common-dir)→OpenRuntimeStore→SameFile(memo)`, block only `apply/verify/remediate`→`blocked`/`resolve-blockers`/`cross_common_dir_runtime_target` | Off=corruption; `context.Context`; not on `spec/design/tasks`; 85 LOC |
| **G5 REQ-G5-01** `risk.go:165` | full 150 vs adapted | Keep `ClassifyRisk` order; `triviallyInert` adds `isPassiveContentFile` gated by `isPassiveDocumentExtension` (`.md/.rst/.adoc/.txt/.png/.jpg/.gif`); ≤8MiB, NUL/utf8→not passive, `#!`→not passive, `import/export`→not passive, `subprocess/exec`→not passive; >8MiB fail-closed | Full needs blobs; 55 LOC |
| **G7 REQ-G7-01** marker | ignore vs regex | `readOnlyMarkerAfterToken=(?i)^\s*\(read-only\)` per-token suffix after backtick, filter both detectors | Escape without widening; 20 LOC |
| Out | port | Discard `Handoff`(multi-worktree CAS) `Attestation`(review authority) | Saves 150 LOC |

## Data Flow

```
sdd-status --json → StatusWithOptions → readChange
  → readChangeInstanceMarker → StatusWithInstance → GrantedRoots
  → deriveChangeStatus: AllowedRoots=ws+granted → foreignRuntimeTopologyRoots? (memo: resolve→rev-parse→SameFile → blocked if foreign&&phase∈apply/verify/remediate)
Rescope: resolveStore→withStoreLock→replay→predicate→narrow→wedge→carry Cumulative*→commit CAS
ForInstance: validate→grantedRootsFor; ClassifyRisk: sensitive→execConfig→docOnly→volume>400→triviallyInert(isPassiveDoc?isPassiveContentFile)→medium
```

## File Changes

| File | LOC | Gap → What |
|------|-----|------------|
| `internal/sdd/attempt.go` | 15 | G1 `ErrLegacyRetired`, fail-closed |
| `internal/sddattempt/sddattempt.go:1992` | 55 | G2 `ErrRuntimeRescopeExhausted`, guards+narrow-before-wedge+carry |
| `internal/sddattempt/cas_store.go:86/108` | 25 | G3 `ForInstance`, `validateChangeInstance`, `grantedRootsFor` |
| `internal/sdd/edit_authority.go:19/233` | 105 | G4+G7 marker+`foreignRuntimeTopologyRoots`, `resolveExistingPath`, `gitRootOf`, memo |
| `internal/sdd/status.go:340/473` | 30 | G3+G4 wiring + phase gate |
| `internal/review/risk.go:165` | 55 | G5 `isPassiveContentFile`, allowlist, shebang/MDX/exec |
| `internal/sdd/research.go:39` | 0 | G6 compliant |

255 tracked + ~180 test =435 <800 → single PR.

## Interfaces / Contracts

```go
var ErrLegacyRetired = errors.New("legacy ledger retired; use biggz sdd-attempt acquire|settle; status: ...")
var ErrRuntimeRescopeNotAllowed, ErrRuntimeRescopeWidened, ErrRuntimeRescopeExhausted error // errors.Is
// Rescope predicate: Active==0&&ObjID!=""&&!DecReq&&!Complete&&len>0&&last.Outcome!=""&&!drift

type Store struct { Dir,Change,LegacyPath,Scope string; instance string }
func (s Store) ForInstance(string) (Store,error) // 1..128 trimmed single-line
func grantedRootsFor(*RuntimeStore,string) []string
func foreignRuntimeTopologyRoots(text,ws string, allowed []string, memo map[string]*Store) []string
func isPassiveContentFile(string) bool // ≤8MiB cap fail-closed
func isPassiveDocumentExtension(string) bool
var readOnlyMarkerAfterToken = regexp.MustCompile(`(?i)^\s*\(read-only\)`)
```

CAS additive, errors.Is preserved.

## Testing Strategy

| Layer | What | How |
|-------|------|-----|
| Unit | G1 no file | Begin/Finish/Reset→ErrLegacyRetired |
| Unit | G2 | `5/600→5/800`Widened, `→6/500`Exhausted, `→7/800`ok; cum unchanged |
| Unit | G3 | `""`/129/multiline err; equiv; archived reuse empty |
| Unit | G4 | `git init` siblings, foreign blocks `apply` not `spec`, memo 1 call |
| Unit | G5 | shebang/mdx/exec not passive; 9MiB not passive; readme passive |
| Unit | G7 | `(read-only)` exempt else `MissingRoots` |
| Integ | status | foreign → blocked |

## Threat Matrix

| Boundary | Applicable | Response | RED |
|----------|------------|----------|-----|
| Doc paths (`requirements.txt`, MD/MDX, `README.sh`) | Yes | G5 shebang/MDX/exec check | mdx+shebang→RiskMedium |
| Git selection (`git -C`, rel/abs) | Yes | `exec.Command list`, join rel, memo, fail-closed | rel vs abs same; missing git→no block |
| Commit state | N/A | No commit automation (CAS only) | — |
| Push state | N/A | No push transport | — |
| PR commands | N/A | No PR automation | — |

Extended:

| Threat | Mitigation | RED |
|--------|-----------|-----|
| Cmd injection | `exec.Command` not shell, `validateChangeName` | `a/b` err not exec |
| TOCTOU/symlink | `EvalSymlinks` before `SameFile` | symlink canonical |
| 8MiB DoS | `LimitReader 8<<20+1`, >cap fail-closed | 9MiB not passive |
| Git latency | memo per Status | 3 tokens→1 rev-parse |
| Binary | NUL/!utf8→not passive | `\x00` not passive |

## Dependency Graph

`G6→G1+G3→G7→G4→G2→G5` (G4 needs G3+G7; G5 leaf)

## Migration / Rollout

CAS additive, no migration, `git revert` single PR. ForInstance zero-value safe, new Exhausted needs dual errors.Is. 255 <400 passes budget 800.

## Open Questions

- [ ] CandidateTree wiring for drift stub

## Traceability

| Req | Gap | Section |
|-----|-----|---------|
| REQ-G1-01 | G1 | G1, `attempt.go` |
| REQ-G2-01/02 | G2 | G2, `sddattempt.go:1992` |
| REQ-G3-01 | G3 | G3, `cas_store.go` |
| REQ-G4-01 | G4 | G4, `edit_authority.go`+`status.go:473` |
| REQ-G5-01 | G5 | G5, `risk.go:165` |
| REQ-G6 | G6 | `research.go:39` ratified |
| REQ-G7-01 | G7 | G7, `edit_authority.go` marker |
