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
