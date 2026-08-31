# Tasks: rdd-cas-reach-parity

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 360 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | CAS: gen, LOCK, expectedRevision, repair | PR 1 | `go test ./internal/review -run TestRDD` | `biggz rdd disable --scope=clone --expected-revision` race | `internal/review/rdd.go` + LOCK |
| 2 | Reach + mirror + PartialApplyError | PR 1 | `go test ./internal/review -run TestRDDReach` | `biggz rdd disable` mirror ok vs missing | `internal/review/rdd.go` Reach |
| 3 | RDDDisabledError source-aware | PR 1 | `go test ./internal/review -run TestRDDDisabled` | blocked start/mutate | `internal/review/rdd.go` Error() |
| 4 | CLI wiring status Revision/Reach | PR 1 | `go test ./... -count=1 -timeout 180s` | `biggz rdd status --json` e2e | `cmd/biggz/cli_rdd.go` |

## Phase 1: Foundation

- [x] 1.1 Add types to `internal/review/rdd.go`: `RDDModeReach`, `RDDModeSource`, `RDDModeStatus`, `Revision`/`Reach` in `RDDStatusReport`, `ErrRDDModeRevisionMismatch`, `ErrRDDModePartiallyApplied`, `RDDModePartialApplyError`
- [x] 1.2 Implement `computeGenerationRevision` via `domainHash` + `writeLengthPrefixed`
- [x] 1.3 Implement `scanGenerationHead` (name-only) and `readLatestGeneration` (validate mode+revision)
- [x] 1.4 Implement `publishImmutable` with `O_CREATE|O_EXCL`, `bytes.Equal` idempotent

## Phase 2: CAS + LOCK + Mirror

- [x] 2.1 RED: `TestPlausibleGitDir` and `TestPublishImmutable_RejectsOverwrite` (threat: Git dir, concurrent publish)
- [x] 2.2 Implement `SetCloneLocalRDDMode` with `WithNamedFileLock(LOCK)` scan→read→CAS→compute→publish relocated→mirror, `gen-%010d.json`, `maxGeneration` guard
- [x] 2.3 Enforce CAS: `expectedRevision!=head.Revision` → `expected "x" but head is "y"`, no file, `empty` only if no record
- [x] 2.4 Corrupt repair: `*RDDModeUnreadableError` reuses slot, overwrites same `gen-%010d.json`
- [x] 2.5 Mirror: `relocated=<commonDir>/biggz/rdd-mode` then `mirror=<commonDir>/gentle-ai/rdd-mode`, best-effort Stat, `ReachMachine`/`ReachThisBuild`
- [x] 2.6 `RDDModePartialApplyError` when mirror reachable but publish fails; add `VerifyCloneRevision` advisory

## Phase 3: Status + Disabled Messages

- [x] 3.1 Update `RDDStatus` (no LOCK, no mirror) precedence worktree>clone>global, `Revision` + `Reach=ReachUnreported`
- [x] 3.2 Add `reviewModeEnableForSource` and `rddOperationSubject`; type `RDDDisabledError.Source`
- [x] 3.3 Update `RDDDisabledError.Error()` with source command + frozen `mutate`, `start` omits, keep sentinel
- [x] 3.4 Update `AuthorizeRDDOperation` to propagate `Source` (default→global wording), `Read` never blocked

## Phase 4: CLI Wiring

- [x] 4.1 Add `--expected-revision` to `biggz rdd disable/enable` in `cmd/biggz/cli_rdd.go`, forward to `SetCloneLocalRDDMode`
- [x] 4.2 Wire `rdd status --json` to emit `revision`+`reach`; fail-closed lists each corrupt file

## Phase 5: Testing & Verification

- [x] 5.1 CAS mismatch `expected "stale-rev" but head is "head-rev"` no file, matching creates `gen-0000000004.json`
- [x] 5.2 Corrupt repair at slot 5 in-place, no slot 6, valid revision
- [x] 5.3 Reach: `machine` both identical, `this_build` missing/unwritable, `unreported` on reads, `PartialApplyError` on conflict
- [x] 5.4 Disabled: global single without `then`, clone `global then clone`, `mutate` frozen, `start` not
- [x] 5.5 Integration: `Authorize` correct `Source`, `Read` never blocked; `go test ./internal/review -run TestRDD` + `go vet`
