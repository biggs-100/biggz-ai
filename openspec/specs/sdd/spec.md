# Delta for sdd

## ADDED Requirements

### Requirement: Preflight ArtifactStore Normalization and Canonicalize

The system MUST implement `internal/sdd/preflight.go` `NormalizePreflightArtifactStore(s string) string` (trim + lower; `both/hybrid/engram/bigmem → hybrid`, `openspec → openspec`, `none → ""`, else passthrough lower) and `canonicalizePrefs(p PreflightPrefs) PreflightPrefs` setting `p.ArtifactStore = Normalize(...)` and filling defaults `ExecutionMode → interactive` when empty, `ChainedPrStrategy → stacked-to-main` when empty, `ReviewBudgetLines → 400` when zero.

#### Scenario: Alias folding to hybrid and none to empty

- GIVEN `s` is `both` or `Both` or `hybrid` or `engram` or `bigmem` or `BigMem` versus `none` or `NONE`
- WHEN `NormalizePreflightArtifactStore` is called
- THEN alias variants MUST return `hybrid` and none variants MUST return `""`

#### Scenario: Openspec preserved and unknown passthrough

- GIVEN `s` is `openspec` or `OpenSpec` versus `custom-store`
- WHEN normalized
- THEN it MUST return `openspec` and `custom-store` respectively (lowercased)

#### Scenario: Canonicalize fills defaults and folds alias

- GIVEN `PreflightPrefs{ExecutionMode:"", ArtifactStore:"", ChainedPrStrategy:"", ReviewBudgetLines:0}` versus `{ExecutionMode:"auto", ArtifactStore:"BigMem", ChainedPrStrategy:"feature-branch-chain", ReviewBudgetLines:800}`
- WHEN `canonicalizePrefs` is called
- THEN empty MUST become `{interactive,"",stacked-to-main,400}` and explicit MUST become `{auto,"hybrid",feature-branch-chain,800}`

### Requirement: Preflight Disk Persist and Resolve with Cache

The system MUST implement `internal/sdd/preflight.go` `SddPreflightDiskPath(home ...string) string` (`home[0]` else `GENTLE_PI_CONFIG_HOME` else `UserHomeDir/.pi/gentle-ai` + `sdd-preflight.json`), `WriteSddPreflightToDisk(p PreflightPrefs, home ...string) error` (`canonicalize` + `MkdirAll 0755` + `MarshalIndent` + `WriteFile 0644` `\n`), `ReadSddPreflightToDisk(home ...string) (PreflightPrefs,bool)` (`ReadFile` + `Unmarshal` + `canonicalize`), `Set/Get/Clear/ResolvePreflightPrefs` precedence `cache > disk > defaults {interactive,openspec,stacked-to-main,400}`, `ValidatePreflightQuestionEnvelope(env PreflightQuestionEnvelope) bool` enums `Pace {interactive,auto}`, `Artifacts {openspec,BigMem,both,hybrid}`, `PRs {ask-on-risk,single-pr,auto-chain}`, `Review {400,800,Other}`, and `SessionRecallMarkdown`.

#### Scenario: Disk write canonicalizes and round-trips

- GIVEN `PreflightPrefs{ExecutionMode:"auto", ArtifactStore:"BigMem", ChainedPrStrategy:"", ReviewBudgetLines:0}` and a temp `home`
- WHEN `WriteSddPreflightToDisk` then `ReadSddPreflightToDisk` with same `home`
- THEN read MUST return `{auto,hybrid,stacked-to-main,400}` and `true`, file exists as `0644` pretty JSON

#### Scenario: Resolve precedence cache over disk over defaults

- GIVEN cache has `cwd→{auto}` and disk has `{interactive}` versus cache cleared and disk present versus both absent
- WHEN `ResolvePreflightPrefs(cwd, home)` is called
- THEN it MUST return `auto` when cache present, disk value when only disk present, and `{interactive,openspec,stacked-to-main,400}` when neither

#### Scenario: Validate envelope and recall markdown

- GIVEN envelope `{Pace:"auto", Artifacts:"both", PRs:"auto-chain", Review:"400"}` versus `{Pace:"other", Artifacts:"none"}`, and `SessionRecallMarkdown("biggz-ai",2,3,1)`
- WHEN `ValidatePreflightQuestionEnvelope` and `SessionRecallMarkdown` are called
- THEN valid envelope MUST be `true` and invalid MUST be `false`; markdown MUST contain `## Session Recall`, `Context Loaded: 2 observations, 1 sessions`, `Project: biggz-ai`

### Requirement: Synthesis Gate Markers and 120s Window

The system MUST implement `internal/sdd/synthesis_gate.go` `synthesisMarkers[4]` (`## Sub-agent Result:`, `**What was done:**`, `**Artifacts/Paths:**`, `**Next Recommended:**`) with `HasSynthesis` (all 4 required), `HasSessionRecall` (`## Session Recall`), `IsChildBypass` (`PI_SUBAGENT_CHILD==1`), `IsCheckpointAsk` (lower contains `proceed|adjust|stop|continue|correct`), globals `currentTurnMarkdown` + `currentTurnTime` via `SetCurrentTurnMarkdown`, `ShouldBlock(question, md, now)` (`false` if child/recall/notCheckpoint/`now-sub>120s`, else `!HasSynthesis`), and `CheckSynthesisPrecondition(question, md) (bool,string)` wrapping `ShouldBlock` with message `synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)`.

#### Scenario: HasSynthesis requires all four markers and recall/child bypass

- GIVEN markdown with all four markers versus missing one, versus markdown with `## Session Recall` or env `PI_SUBAGENT_CHILD=1`
- WHEN `HasSynthesis` / `ShouldBlock` checkpoint within `120s` without synthesis is evaluated
- THEN all-four MUST be `true` and missing-one `false`; recall or child bypass MUST make `ShouldBlock` return `false`

#### Scenario: Checkpoint detection and 120s window expiry

- GIVEN question `proceed?` or `adjust` versus `how are you?` without checkpoint keywords, and `SetCurrentTurnMarkdown` then `now = currentTurnTime+30s` versus `+121s` without synthesis
- WHEN `ShouldBlock` is called
- THEN checkpoint+30s MUST block (`true`), non-checkpoint MUST not block (`false`), and `121s` past MUST allow (`false`)

#### Scenario: CheckSynthesisPrecondition message

- GIVEN `ShouldBlock` would be `true` (checkpoint within `120s` without synthesis, no bypass)
- WHEN `CheckSynthesisPrecondition` is called
- THEN it MUST return `(false, "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)")`; when not blocking, `(true,"")`

### Requirement: Sync Phase Lifecycle

The system MUST introduce phase `sdd-sync` between `verify` and `archive`. `sdd-sync` MUST sync file-backed deltas to `openspec/specs/` without moving the change to archive. Lifecycle order MUST be `proposal → spec → design → tasks → apply → verify → sync → archive`.

#### Scenario: Verify-pass exposes sync before archive

- GIVEN `verifyReport` is `PASS`, tasks are `allDone`, and at least one delta spec exists under `openspec/changes/{change}/specs/`
- WHEN status derives `nextRecommended`
- THEN it MUST be `sync` (not `archive`) until sync clears

#### Scenario: Sync clears enables archive

- GIVEN sync successfully applied deltas to `openspec/specs/`
- WHEN status re-derives
- THEN `nextRecommended` MUST become `archive` and `sync` MUST not reappear unless deltas change

#### Scenario: No deltas or non-file store skips sync

- GIVEN declared store is `engram`/`none` or no delta specs exist
- WHEN status derives
- THEN `nextRecommended` MUST NOT be `sync` and `blockedReasons` MUST not contain sync guards

### Requirement: Sync Execution Contract

The system MUST provide agent `sdd-sync`, skill `internal/assets/skills/sdd-sync/SKILL.md`, prompt `sdd-sync.md`, and implementation `internal/sdd/openspec-deltas.go` + `internal/sdd/sync.go` porting ADDED/MODIFIED/REMOVED from `lib/openspec-deltas.ts` 1:1 without auto-commit, child subagents, or archive move.

#### Scenario: Sync executor without archive move

- GIVEN `openspec/changes/{change}/specs/sdd/spec.md` contains valid deltas
- WHEN `sdd-sync` executes on `openspec` store
- THEN `openspec/specs/sdd/spec.md` MUST reflect deltas and `openspec/changes/{change}/` MUST still exist

#### Scenario: No commit created

- GIVEN sync completed successfully
- WHEN git log is inspected
- THEN no new commit with `sdd-sync` auto-commit MUST exist

### Requirement: REQ-G1-01 — Legacy Ledger Fail-Closed

The system MUST make `internal/sdd/attempt.go` `AttemptBegin/Finish/Reset` return `ErrLegacyRetired` (errors.Is) with message containing `biggz sdd-attempt acquire|settle|status` and MUST NOT create or mutate `.attempt.json`.

#### Scenario: Begin fails closed without mutation

- GIVEN no `.attempt.json` for change
- WHEN `AttemptBegin` is called
- THEN MUST return `ErrLegacyRetired` mentioning `biggz sdd-attempt acquire` and no file created

#### Scenario: Finish/Reset are no-ops

- GIVEN `.attempt.json` exists
- WHEN `AttemptFinish` or `AttemptReset` called
- THEN MUST return `ErrLegacyRetired` and file unchanged

### Requirement: REQ-G2-01 — Rescope Narrowing and Guards

The system MUST allow `sddattempt.Rescope` only when `ActiveAttempt==0 && ObjectiveID!="" && !DecisionRequired && !Complete && len(Attempts)>0 && last.Outcome!="" && !driftedStub`; else MUST return `ErrRuntimeRescopeNotAllowed`. MUST reject widening (`newMaxAttempts<=oldMaxAttempts || newMaxLines<=oldMaxLines`) with `ErrRuntimeRescopeWidened`, and wedge violation (`newMaxAttempts<=len(Attempts) || newMaxLines<=CumulativeChangedLines`) with `ErrRuntimeRescopeExhausted`.

#### Scenario: Guards block illegal rescope

- GIVEN `ActiveAttempt=1` or `DecisionRequired` or `Complete` or zero attempts
- WHEN `Rescope` called
- THEN MUST return `ErrRuntimeRescopeNotAllowed`

#### Scenario: Narrow/wedge enforcement

- GIVEN `Max 5/600, 5 attempts, cum 500, terminal failed`
- WHEN `5/600→5/800` or `5/600→5/500` THEN MUST return `Widened`; WHEN `5/600→6/500` THEN `Exhausted`; WHEN `5/600→7/800` THEN MUST succeed

### Requirement: REQ-G2-02 — Rescope Preserves History

`Rescope` MUST preserve `Attempts` slice, `CumulativeChangedLines`, and never reset counters; MUST update `MaxAttempts/MaxLines` and clear `DecisionRequired/Complete` to false with `NextAction=begin`.

#### Scenario: History preserved

- GIVEN 3 attempts `cum 350`
- WHEN `Rescope` to `7/800` succeeds
- THEN `len(Attempts)` MUST stay `3` and `cum` MUST stay `350`

### Requirement: REQ-G3-01 — ForInstance Sugar

The system MUST provide `Store.ForInstance(instance) (Store,error)` validating trimmed single-line `1..128` via `validateChangeInstance`, scoping `grantedRootsFor`. `ForInstance(x).Grant` MUST equal `Grant{ChangeInstance:x}`. `StatusWithInstance` MUST project only matching instance deduped. Archived name reuse MUST NOT resurrect prior roots.

#### Scenario: Validation and equivalence

- GIVEN invalid `""`/`"  "`/129 chars/multiline
- WHEN `ForInstance` called THEN MUST error; valid `"tok-1"` MUST succeed

#### Scenario: Isolation

- GIVEN grant `[/a]` with `i1`
- WHEN `StatusWithInstance("i1")` vs `"i2"`/`""` queried
- THEN only `i1` contains `/a`; reuse with new instance MUST be empty

### Requirement: REQ-G4-01 — Topology Guard

The system MUST implement `foreignRuntimeTopologyRoots` via `git rev-parse --git-common-dir` + `os.SameFile` memoized per `Status`. MUST block only `apply/verify/remediate` when backticked checkbox path resolves (`resolveExistingPath→gitRootOf→OpenRuntimeStore→sameRuntimeCommonDirectory`) to different common dir outside `AllowedEditRoots`. Blocked status MUST set `ApplyState=blocked`, `Dependencies.Verify/Archive=blocked`, `NextRecommended="resolve-blockers"`, `BlockedReasons` contains `cross_common_dir_runtime_target`.

#### Scenario: Foreign blocks apply not spec

- GIVEN tasks checkbox `` `../foreign-clone/file.go` `` foreign common dir
- WHEN `biggz sdd-status --json` derives eligible `apply`
- THEN MUST be `blocked` with `cross_common_dir_runtime_target` and `resolve-blockers`; same path in `spec` phase MUST NOT block

### Requirement: REQ-G6 — HybridResearchEqual Ratified

The system MUST keep `research.go:39` `HybridResearchEqual` true only when `revA>0 && revB>0 && revA==revB && len>0 && bytesEqual`; no code change.

#### Scenario: Equality check

- GIVEN `rev 2/2 bytes "ab"/"ab"` vs `1/2` or `"a"/"b"` or empty
- WHEN `HybridResearchEqual` called
- THEN first MUST true, others false; hybrid `EvaluateResearchHybrid` MUST require it

### Requirement: REQ-G7-01 — Read-Only Marker

The system MUST implement `readOnlyMarkerAfterToken` regex `(?i)^\s*\(read-only\)` per-token suffix check. Token with ` (read-only)` MUST be exempt from `detectUnauthorizedEditRoots` and topology guard.

#### Scenario: Per-token exemption

- GIVEN `` `../other/docs/api.md` (read-only) `` and `` `../other/src/main.go` `` outside roots, `other` is git repo
- WHEN status derives
- THEN first MUST NOT appear in `MissingRoots`, second MUST trigger `blocked(edit_authority_missing)`
