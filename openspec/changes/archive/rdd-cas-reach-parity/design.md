# Design: rdd-cas-reach-parity

## Technical Approach

Port the minimal safe subset of `gentle-ai/internal/reviewtransaction/rdd_mode.go` (ResolveRDDMode / SetCloneLocalRDDMode / RDDModeReach / RDDDisabledError) plus `docs/architecture/organic-rdd.md` into `biggz-ai/internal/review/rdd.go` without touching delivery/review lifecycle. The change is a faithful trim: generation helpers, CAS with flock LOCK, Reach + pre-relocation mirror, and source-aware disabled messages. Global state (`~/.biggz/rdd-mode.json`) stays unchanged; only clone/worktree overrides evolve.

Chosen: Option A — minimal faithful port (proposal). Rejected: Option B (ad-hoc CAS without mirror reintroduces #3284 multi-install divergence) and Option C (full port including Asked latch expands UX scope).

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| **Revision digest** | A) raw `sha256(json(noRev))` hex <br> B) `domainHash("biggz-ai.rdd-mode-override-digest/v1", writeLengthPrefixed(...))` | A is current baseline, no domain separation; B matches gentle parity and gives collision-domain isolation with length-prefix framing | **B** — `computeGenerationRevision` returns `domainHash(RDDModeOverrideDigestDomain, writeLengthPrefixed(schema, gen10, prevRev, mode, recordedAt))`; domain constant `biggz-ai.rdd-mode-override-digest/v1`; helpers reused from `artifact.go` (`writeLengthPrefixed`, `domainHash`) |
| **Generation encoding** | A) int64 raw <br> B) zero-padded 10-digit `gen-%010d.json` with `maxGeneration=999_999_999` | A simpler but loses file ordering; B matches spec and preserves lexicographic order + exhaustion guard | **B** — `fmt.Sprintf("gen-%010d.json", gen)`; scan parses `%d`; guard `gen > maxGeneration` fail closed |
| **LOCK contract** | A) new ad-hoc lock <br> B) reuse `internal/review/lock.go:NewNamedFileLock(dir,"LOCK")` with flock + stale(5m) + PID check | A duplicates logic; B reuses tested primitive, covers Unix flock + Windows fallback | **B** — `WithNamedFileLock(genDir,"LOCK", fn)` (or `NewNamedFileLock` + `AcquireWithTimeout`) holds across `read head → validate expectedRevision → compute next → publishImmutable`; lock file path `<gitDir>/biggz/rdd-mode/LOCK` |
| **Immutable publish** | A) `os.WriteFile` truncate <br> B) `O_CREATE\|O_EXCL` no-replace, idempotent same-bytes | A silently overwrites; B preserves CAS and repair semantics (corrupt slot overwritten once, readable slot never replaced) | **B** — `publishImmutable(path, payload []byte) error` opens with `O_CREATE\|O_EXCL\|O_WRONLY`; if exists, `bytes.Equal(existing,payload)` → ok, otherwise `ErrPublishImmutableConflict`; caller never uses tmp+rename for RDD generations |
| **Head discovery vs parsing** | A) `readLatestGeneration` covers scan <br> B) split `scanGenerationHead` (name-only) + `readLatestGeneration` (parse+validate) | A conflates slot naming with parse failure; B lets corrupt head repair reuse same slot without losing max generation | **B** — `scanGenerationHead` returns `bestGen,bestFile` by name without `ReadFile`/`Unmarshal`; `readLatestGeneration` calls scan then reads that single file and validates `mode==disabled` + `revision==computeGenerationRevision` |
| **CAS validation** | A) pre-check via `VerifyCloneRevision` only <br> B) in-LOCK check `expectedRevision == head.Revision` (empty means no record) | A racy without LOCK; B is atomic | **B** — `SetCloneLocalRDDMode(expectedRevision string)` fails with `ErrRDDModeRevisionMismatch: expected "x" but head is "y"` before any write; `VerifyCloneRevision` stays as CLI pre-check but not authoritative |
| **Corrupt repair** | A) chain new slot after corrupt <br> B) overwrite exact corrupt slot in-place | A leaves corrupt head as max, new slot > corrupt hides but pollutes; B restores chain at same generation | **B** — on `*RDDModeUnreadableError` from `readLatestGeneration`, call `scanGenerationHead` again (no parse) to recover `headGen`; set `genNum=headGen`, `prevRev=""` or prior valid revision if recoverable; `publishImmutable` writes that slot |
| **Mirror path + ordering** | A) single root only <br> B) dual publish relocated→mirror | A reintroduces #3284; B covers multi-install old binaries | **B** — `relocated = filepath.Join(commonGitDir,"biggz/rdd-mode")`; `mirror = filepath.Join(commonGitDir,"gentle-ai","rdd-mode")` (pre-relocation legacy location, documented as mirror root; exact legacy segment taken from `internal/install` relocation history and noted in code comment). Order strictly relocated first, mirror second to avoid hostage (#2882). Mirror open is best-effort: if parent directory missing or unwritable, report `ReachThisBuild` without error; only when mirror dir reachable but `publishImmutable` fails → `RDDModePartialApplyError` |
| **Reach reporting** | A) boolean flag <br> B) `RDDModeReach` enum `""`/`machine`/`this_build` mirrors `gentle-ai/internal/reviewtransaction/rdd_mode.go:RDDModeReach` | A loses Read-projection distinction | **B** — `type RDDModeReach string; const ReachUnreported RDDModeReach="" ; ReachMachine="machine"; ReachThisBuild="this_build"`; writes return Machine/ThisBuild; reads (`RDDStatus`/`ResolveRDDMode`) never probe mirror and return `""` |
| **Partial apply** | A) report success even if mirror fails <br> B) half-apply surfaces `ErrRDDModePartiallyApplied` | A masks divergence | **B** — `type RDDModePartialApplyError struct { RelocatedPath, MirrorPath string; Cause error }` with `Is/As` for `ErrRDDModePartiallyApplied`; returned only when `relocated ok && mirror reachable but publish failed`; caller must not treat status as fully machine |
| **RDDDisabledError** | A) `Source string` generic message <br> B) typed `Source RDDModeSource` + `Operation RDDOperation` + `reviewModeEnableForSource` + `rddOperationSubject` | A prints generic `biggz rdd enable` always | **B** — `type RDDModeSource string; const SourceDefault="default"; SourceGlobal="global"; SourceCloneLocal="clone_local"; SourceClone="clone" (alias); SourceWorktree="worktree"`; `reviewModeEnableForSource(source) string` returns `biggz rdd enable --scope=global` for default/global, `biggz rdd enable --scope=global then biggz rdd enable --scope=clone` for clone_local/clone/worktree; `rddOperationSubject(op)` returns `review start` vs `review mutation ... the review is frozen, not discarded; ... to continue it from where it stopped`; `Error()` composes both; preserves `errors.Is(err, ErrRDDDisabled)` sentinel if defined |
| **Reuse of existing primitives** | A) new lock/store impl <br> B) reuse `internal/review/lock.go` + `internal/review/store.go` helpers | A duplicates and drifts | **B** — no new lock impl; `plausibleGitDir`, `SyncReviewDirectory`, `writeLengthPrefixed`/`domainHash` (from `artifact.go`/`store.go`) reused; `delivery lifecycle` (`gate.go`, `contract.go`, `finalize.go`) untouched |
| **Scope normalization** | A) keep `clone` string only <br> B) normalize to `default/global/clone_local/worktree` for error mapping, with `clone` alias preserved | A breaks spec alias | **B** — `RDDStatusReport` source normalized; `RDDDisabledError.Source` typed but string-compatible; `clone` accepted as alias to `clone_local` in `reviewModeEnableForSource` |

## Data Flow

```
SetCloneLocalRDDMode(worktreeGitDir, commonGitDir, mode=disabled, expectedRevision)
  │
  ├─ plausibleGitDir(commonGitDir) ? fail closed : continue
  ├─ genDir = <commonGitDir>/biggz/rdd-mode ; mirrorDir = <commonDir>/gentle-ai/rdd-mode
  ├─ WithNamedFileLock(genDir,"LOCK", timeout≤5s)
  │     ├─ scanGenerationHead(genDir) → headGen, headFile (name only)
  │     ├─ readLatestGeneration(genDir) → head *rddGeneration or *RDDModeUnreadableError or nil
  │     ├─ CAS check: if head==nil && expectedRevision!="" → ErrRDDModeRevisionMismatch (head="")
  │     │              if head!=nil && expectedRevision!=head.Revision → ErrRDDModeRevisionMismatch
  │     │              if corrupt && expectedRevision!="" → still mismatch unless caller handled repair token
  │     ├─ compute next: if head!=nil && err==nil → gen=head.Generation+1, prevRev=head.Revision
  │     │              if corrupt → gen=headGen (same slot), prevRev="" (no chain)
  │     │              guard gen>maxGeneration → fail
  │     ├─ build rddGeneration{schema=biggz-ai.rdd-status/v1, generation=gen, previous_revision=prevRev, mode=disabled, recorded_at=now RFC3339Nano}
  │     │        revision=computeGenerationRevision(noRev) with domain biggz-ai.rdd-mode-override-digest/v1
  │     ├─ marshal indent → bytes B ; filename gen-%010d.json
  │     ├─ publishImmutable(relocatedPath,B)  // O_CREATE|O_EXCL, bytes.Equal idempotent
  │     │        └─ sync dir
  │     ├─ mirror probe: Stat(mirrorDir) ? reachable : ThisBuild
  │     ├─ if reachable: publishImmutable(mirrorPath,B)  // same gen, same bytes
  │     │        ├─ ok → ReachMachine
  │     │        └─ fail (reachable but publish error) → return RDDModePartialApplyError wrapping cause, ReachThisBuild, do not report as applied
  │     └─ else → ReachThisBuild
  └─ return RDDModeStatus{Reach, Revision, Generation, RecordedAt}

RDDStatus(worktreeGitDir, commonGitDir)
  ├─ readGlobalMode(), readRDDModeDir(worktreeDir), readRDDModeDir(commonDir) // each calls readLatestGeneration (no LOCK, read-only)
  ├─ precedence worktree > clone > global ; source ∈ {default,global,clone,worktree} normalized to clone_local for disabled path
  ├─ revision = head.Revision of winning source (or "" if default)
  ├─ Reach = "" (ReachUnreported) — never probes mirror
  └─ advisory report + aggregated *RDDModeUnreadableError fails closed

AuthorizeRDDOperation(op, worktreeDir, commonDir)
  ├─ if op==Read → nil
  ├─ else RDDStatus → if EffectiveMode==disabled → return &RDDDisabledError{Source: status.Source (typed), Operation: op}
  └─ default resolves as global for enable-path wording (opt-in)

RDDDisabledError.Error()
  ├─ cmd = reviewModeEnableForSource(Source)  // default/global → single command; clone/clone_local/worktree → chained "global then clone"
  └─ subject = rddOperationSubject(Operation) // start: "review start blocked ... Enable with: <cmd>"
                                               // mutate: "review mutation blocked ... <cmd>; the review is frozen, not discarded; to continue it from where it stopped"
```

Writes never wait on delivery tree; delivery gates (`gate.go:EvaluateGate`) keep reading `RDDStatus` only.

## File Changes

| File | Action | Description | Est |
|------|--------|-------------|-----|
| `internal/review/rdd.go` | Modify | Add constants/domains/types: `RDDModeOverrideDigestDomain="biggz-ai.rdd-mode-override-digest/v1"`, `rddStatusSchema="biggz-ai.rdd-status/v1"` (existing), `maxGeneration=999999999`, `ErrRDDModeRevisionMismatch`, `ErrRDDModePartiallyApplied`, `RDDModeReach` enum, `RDDModeSource` typing, `RDDModePartialApplyError`, `RDDModeStatus`/`RDDStatusReport{Revision, Reach}`; rewrite helpers: `computeGenerationRevision` (domainHash+writeLengthPrefixed), `scanGenerationHead` (name-only), `readLatestGeneration` (validate mode+revision), `publishImmutable` (O_CREATE\|O_EXCL, bytes.Equal), `SetCloneLocalRDDMode` with `expectedRevision`+LOCK+relocated→mirror+Reach; update `RDDDisable`/`RDDEnable`/`RDDStatus` to thread Revision/Reach; rewrite `RDDDisabledError` with `reviewModeEnableForSource` + `rddOperationSubject`; mirror helper `cloneLocalRDDModeMirror{dir, reachable}` (best-effort open, strict publish) | ~220 |
| `internal/review/lock.go` | Referenced | Reuse `NewNamedFileLock` / `WithNamedFileLock` / `FileLock` flock+stale logic for `rdd-mode/LOCK`; no new lock implementation | 0 |
| `internal/review/store.go` (`artifact.go`) | Referenced | Reuse `writeLengthPrefixed` / `domainHash` / `plausibleGitDir` / `SyncReviewDirectory`; `store.publishImmutable` semantics documented but RDD uses O_EXCL variant directly | 0 |
| `internal/review/consent.go` | Referenced | No change; noted as reference only for `Asked` latch parity alias | 0 |
| `cmd/biggz/cli_rdd.go` | Modify | Wire `biggz rdd disable --scope=clone|worktree|global --expected-revision=<rev>` → forwards `expectedRevision` to `SetCloneLocalRDDMode`; surface `ErrRDDModeRevisionMismatch`/`ErrRDDModePartiallyApplied` without fallback; `status --json` emits `revision`+`reach`; map `clone`→`clone_local` wording only for display, not storage | ~40 |
| `internal/review/rdd_test.go` | Modify | Add golden tests: CAS mismatch fail-closed, LOCK-held generation bump, corrupt-head same-slot repair, immutable publish rejects overwrite, Reach machine/this_build/unreported, PartialApplyError, disabled messages per source/operation, Authorize propagates Source, Read never blocked, VerifyCloneRevision | ~140 |
| `openspec/specs/review/spec.md` | Done | Spec deltas for CAS/Reach/disabled-message (already) | 0 |
| `openspec/specs/cli/spec.md` | Done | Flag contract for `--expected-revision` (already) | 0 |

No change to `internal/review/gate.go`, `contract.go`, `finalize.go`, `ledger.go`, or any delivery/review lifecycle file.

## Interfaces / Contracts

```go
// domain
const RDDModeOverrideDigestDomain = "biggz-ai.rdd-mode-override-digest/v1"
const rddStatusSchema            = "biggz-ai.rdd-status/v1"
const maxGeneration int64 = 999999999

// errors
var ErrRDDModeRevisionMismatch   = errors.New("rdd mode revision mismatch")
var ErrRDDModePartiallyApplied   = errors.New("rdd mode partially applied")
var ErrRDDModeCorrupt            = errors.New("review mode file is corrupt") // existing

type RDDModePartialApplyError struct {
    RelocatedPath string
    MirrorPath    string
    Cause         error
}
func (e *RDDModePartialApplyError) Error() string
func (e *RDDModePartialApplyError) Unwrap() error // Is ErrRDDModePartiallyApplied

// reach
type RDDModeReach string
const (
    ReachUnreported RDDModeReach = ""
    ReachMachine    RDDModeReach = "machine"
    ReachThisBuild  RDDModeReach = "this_build"
)

// source typing (string-compatible, clone alias)
type RDDModeSource string
const (
    SourceDefault    RDDModeSource = "default"
    SourceGlobal     RDDModeSource = "global"
    SourceClone      RDDModeSource = "clone"       // alias
    SourceCloneLocal RDDModeSource = "clone_local" // canonical
    SourceWorktree   RDDModeSource = "worktree"
)

// status extensions
type RDDModeStatus struct {
    Reach      RDDModeReach `json:"reach"`
    Revision   string       `json:"revision"`
    Generation int64        `json:"generation"`
    RecordedAt string       `json:"recorded_at"`
}
type RDDStatusReport struct {
    Schema        string       `json:"schema"`         // biggz-ai.rdd-status/v1
    EffectiveMode RDDMode      `json:"effective_mode"`
    GlobalMode    RDDMode      `json:"global_mode"`
    CloneMode     RDDMode      `json:"clone_mode"`
    WorktreeMode  RDDMode      `json:"worktree_mode"`
    Source        string       `json:"source"`         // default|global|clone_local|worktree (clone alias tolerated)
    Revision      string       `json:"revision"`       // head Revision CAS token, "" if none/default
    Reach         RDDModeReach `json:"reach"`          // "" on reads
    RecordedAt    *time.Time   `json:"recorded_at"`
    WorktreeCount int          `json:"worktree_count,omitempty"`
}

// generation record (persisted as gen-%010d.json)
type rddGeneration struct {
    Schema           string  `json:"schema"`            // biggz-ai.rdd-status/v1
    Generation       int64   `json:"generation"`
    PreviousRevision string  `json:"previous_revision"`
    Mode             RDDMode `json:"mode"`              // off-only: disabled
    RecordedAt       string  `json:"recorded_at"`       // RFC3339Nano
    Revision         string  `json:"revision"`          // domainHash(biggz-ai.rdd-mode-override-digest/v1, writeLengthPrefixed(...noRev...))
}

// helpers
func computeGenerationRevision(gen rddGeneration) string // domainHash + writeLengthPrefixed, not raw sha256(json)
func scanGenerationHead(dir string) (bestGen int64, bestFile string, err error) // name-only, no read
func readLatestGeneration(dir string) (*rddGeneration, error) // parses head file, validates mode+revision, returns *RDDModeUnreadableError on corrupt/mismatch
func publishImmutable(path string, payload []byte) error // O_CREATE|O_EXCL; bytes.Equal same-bytes idempotent; else conflict

// primary write with CAS+Reach
func SetCloneLocalRDDMode(worktreeGitDir, commonGitDir string, mode RDDMode, expectedRevision string) (*RDDModeStatus, error)
// alternative worktree-aware overload if needed for testing:
// func SetWorktreeRDDMode(worktreeGitDir string, mode RDDMode, expectedRevision string) (*RDDModeStatus, error)
func VerifyCloneRevision(gitDir, expectedRevision string) error

// disabled error helpers (unexported but documented)
func reviewModeEnableForSource(source RDDModeSource) string // source-aware exact command
func rddOperationSubject(op RDDOperation) string             // start vs mutate subject

// RDDDisabledError (modified)
type RDDDisabledError struct {
    Source    RDDModeSource
    Operation RDDOperation // start | mutate (Read never produces this)
}
func (e *RDDDisabledError) Error() string // typed source + exact command + frozen-not-discarded for mutate only

// CLI wiring (cmd/biggz/cli_rdd.go)
// biggz rdd disable --scope clone|worktree|global --expected-revision <rev>
//   forwards expectedRevision to SetCloneLocalRDDMode, prints ErrRDDModeRevisionMismatch verbatim, no fallback
// biggz rdd status --json → includes revision, reach

// helpers reused, not redefined
// internal/review/lock.go: NewNamedFileLock(dir,name) *FileLock; WithNamedFileLock(dir,name,fn) error; FileLock.Acquire/Release with flock
// internal/review/artifact.go: writeLengthPrefixed(fields ...[]byte) []byte; domainHash(domain string, payload []byte) string
// internal/review/store.go: plausibleGitDir(path string) bool; SyncReviewDirectory(path string) error
```

Contract invariants:
- Generation files are exactly `gen-%010d.json`; no other names participate.
- `LOCK` is held for the whole `scan → read → CAS check → compute → publish relocated → publish mirror` window; reads (`RDDStatus`) never take LOCK.
- `expectedRevision==""` means "no expectation" only when no record exists; when head exists it must match or fail with `ErrRDDModeRevisionMismatch: expected "x" but head is "y"`.
- Mirror never blocks relocated success; relocated success + mirror reachable failure → `RDDModePartialApplyError` (still writes relocated), not reported as `ReachMachine`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit | `computeGenerationRevision` uses `biggz-ai.rdd-mode-override-digest/v1` domain + length-prefix; different generation/prevRev/mode/recordedAt produce different revision; same inputs identical | Table test hashing known fixture vs gentle parity vector |
| Unit | `scanGenerationHead` returns max slot by name without reading corrupt file; ignores non-`gen-*.json` names | Temp dir with mixed files including corrupt head |
| Unit | `readLatestGeneration` fails closed with `*RDDModeUnreadableError` when `revision != computeGenerationRevision` or `mode != disabled` | Write mismatched revision file, assert `ErrRDDModeCorrupt` via `errors.Is` and `File` naming |
| Unit | `publishImmutable` O_CREATE\|O_EXCL: same bytes idempotent, different bytes conflict, original bytes preserved | Call twice same slot with B1 then B1 (ok) then B2 (error + read B1) |
| Integration | CAS with LOCK: `SetCloneLocalRDDMode` with `expectedRevision==""` on empty creates `gen-0000000000.json`; concurrent second with stale rev fails `ErrRDDModeRevisionMismatch` and creates no file; `LOCK` file exists during window (use `WithNamedFileLock` mock or assert no race via `go test -race`) | Temp fake git dirs via `newFakeGitDir`; `t.Setenv HOME` isolation |
| Integration | Corrupt head repair: write corrupt `gen-0000000005.json`, then disable → repairs slot 5 in-place (not 6), valid `revision` | Write corrupt JSON, call `SetCloneLocalRDDMode`, assert `exists gen-0000000005.json` valid + no `gen-0000000006.json` |
| Integration | Reach `machine` when mirror writable, `this_build` when mirror missing/unwritable; mirror bytes identical at same slot; relocated first | Two dirs (relocated + mirror), assert both files equal; second case mock unwritable mirror dir |
| Integration | `RDDModePartialApplyError` when mirror reachable but publish fails (e.g., mirror file pre-exists with different bytes via O_EXCL) | Pre-create mirror slot with different payload, relocated succeeds, assert `errors.Is` partial + not reported as `machine` |
| Integration | `RDDStatus` exposes `Revision` token and `Reach==""` without mirror probe | After disable, `RDDStatus` → `Revision==head.Revision`, `Reach==""` even when mirror exists |
| Unit | `RDDDisabledError` per source: `global`/`default` → single `biggz rdd enable --scope=global` without `then`; `clone`/`clone_local`/`worktree` → `global then clone`; no generic `biggz rdd enable` | Golden string contains checks |
| Unit | `RDDDisabledError` per operation: `mutate` → contains `the review is frozen, not discarded` + `to continue it from where it stopped`; `start` → not contains | Golden |
| Integration | `AuthorizeRDDOperation` propagates typed Source from `RDDStatus`; `Read` never blocked | Disable global vs clone vs worktree, assert `Source` field |
| CLI | `biggz rdd disable --scope=clone --expected-revision stale` prints `expected "stale" but head is "head-rev"` and exit 1; `status --json` contains `revision` + `reach` | `cmd/biggz/cli_rdd_test.go` or `biggz rdd` e2e with temp git dir |
| Regression | `go test ./internal/review -run TestRDD -count=1` + `go vet ./internal/review` green; no delivery gate change | CI gates |

## Threat Matrix

| Boundary | Applicable | Design Response | Planned RED Tests |
|----------|------------|-----------------|-------------------|
| Documentation-like paths | N/A — no exec-doc classification | — | — |
| Git directory selection | **Applicable** — `git rev-parse --git-common-dir` vs fake bogus dirs | `plausibleGitDir` guard prevents writing stray `.git/biggz` under source checkout; `resolveGitCommonDir` canonicalizes and rejects NUL/`--` | `TestPlausibleGitDir`, `TestRDDDisable_RejectsNonGitDirectory` (already) |
| Concurrent file publish / CAS | **Applicable** — TOCTOU + O_EXCL | `LOCK` (flock + stale PID+mtime) held across read→write; `publishImmutable` O_EXCL guarantees no silent overwrite; generation 10-digit + max guard | CAS mismatch + corrupt repair + immutable reject tests |
| Mirror hostage | **Applicable** — old bug #2882/3284 | Relocated first, mirror second; mirror best-effort only; `PartialApplyError` never reported as `machine`; reads never probe mirror | Reach + PartialApplyError tests |
| Push/commit/PR | N/A — no git mutation in review paths | — | — |
| Secret disclosure | N/A — file names are generation SHAs, no secrets | — | — |

No new shell/VCS routing added.

## Migration / Rollout

No migration. Existing `gen-*.json` remain readable: `readLatestGeneration` tolerates old records whose `revision` was computed via raw `sha256(json)`? No — such old records will be treated as corrupt and repaired in-place at same slot, which is intentional and safe (old records pre-this-change have domain-less revision and will mismatch `computeGenerationRevision` with domain; repair overwrites corrupt slot once, then chain continues). `LOCK` files are no-ops for old readers (they still read without LOCK) and harmless when orphaned. `~/.biggz/rdd-mode.json` unchanged. Single PR <400 lines; one commit chain with generation helpers → CAS+LOCK → Reach/mirror → disabled messages. `git revert HEAD` restores baseline; `go test ./... -count=1 -timeout 180s` must pass.

## Open Questions

- [ ] Confirm exact legacy mirror path segment (`gentle-ai/rdd-mode` vs `gentle-ai/review-transactions/rdd-mode` or other) against `internal/install` relocation history and gentle reference; current design uses `<commonDir>/gentle-ai/rdd-mode` as placeholder and will adjust to match the actual `cloneLocalRDDModeMirror` helper in gentle.
- [ ] `RDDStatus` alias: should `Source` emit `clone` or canonical `clone_local` in JSON? Decision keeps `clone` for backward-compat display but `reviewModeEnableForSource` normalizes both to chained command.
- [ ] `VerifyCloneRevision` authoritative vs advisory: keep CLI pre-check advisory only; authoritative check stays inside `SetCloneLocalRDDMode` under LOCK — documented to avoid double-check races.
