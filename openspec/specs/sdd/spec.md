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

### Requirement: ReviewOffer Post-Verify Wiring

System MUST emit `reviewOffer{Available:true, Invocation:"biggz review start --lineage <change>-<shortsha>"}` iff `applyState==all_done && verifyReport==done && passing && RDD enabled`; else MUST be `nil`. Passing=`pass`,0 blockers,8/8. `status.go:523`/`engram_status.go:246,342` MUST compute; `status_v2.go:48-53` MUST expose only `available,invocation`. Invocation MUST use `pathquote.Quote`, MUST NOT embed lineage/binding/receipt.

#### Scenario: Enabled PASS emits offer
- GIVEN `all_done`, `verify done PASS`, `RDD enabled`
- WHEN `biggz sdd-status --json` derives
- THEN `reviewOffer.available==true` and `invocation` quoted

#### Scenario: Disabled or verify failing emits nil
- GIVEN `RDD disabled` OR `verify missing/fail` OR `blockers>0`
- WHEN status derives
- THEN `reviewOffer==nil`

#### Scenario: Invocation quoting
- GIVEN change `my change` shortsha `a1b2c3d`
- WHEN invocation built
- THEN MUST contain `pathquote.Quote` and MUST NOT contain persisted lineage

### Requirement: Hook Lineage-Aware Selection

Hook `pre-push:8-28` MUST select via `ls -t` newest-first filtered by `git merge-base --is-ancestor <commit> HEAD`; MUST NOT use alphabetical first. Ghost `019fbb3a-*` only if ancestor. Fallback newest `ls -t`.

#### Scenario: Ghost ignored when not ancestor
- GIVEN ghost `019fbb3a-*` not ancestor and `my-change-abc123` ancestor of `HEAD`
- WHEN hook selects `lineage`
- THEN `lineage==my-change-abc123`

#### Scenario: Fallback newest
- GIVEN `merge-base` unavailable
- WHEN hook runs
- THEN MUST pick newest `ls -t` lineage

### Requirement: Hook Space-Tolerant Grep

Hook MUST grep with `[[:space:]]*` for `delivery` disabled and `allowed` false.

#### Scenario: JSON with spaces routed
- GIVEN JSON `{"delivery": "disabled"}` and `{"allowed": false}` with spaces
- WHEN hook greps `output`
- THEN delivery grep allows push, allowed-false blocks

### Requirement: Archive Never Auto-Disable

`archive.go:ArchiveChange` MUST NOT call `RDDDisable`/`SetCloneLocalRDDMode`/`RDDEnable` nor write `.git/biggz/rdd-mode`; only `os.Rename`.

#### Scenario: Archive preserves enabled and mtime
- GIVEN `RDD enabled` and mtime T0 before archive
- WHEN `ArchiveChange` moves PASS change
- THEN `rdd status` still `enabled`, mtime==T0, grep finds zero RDD calls

### Requirement: Orchestrator Auto-Run on Block Only

Orchestrator MUST auto-run `reviewOffer.invocation` only when `allowed==false && auto-chain && offer available`; else surface offer only.

#### Scenario: Auto-chain blocked auto-runs
- GIVEN `auto-chain`, `allowed:false`, offer available
- WHEN orchestrator handles
- THEN MUST execute invocation

#### Scenario: Ask-on-risk offers only
- GIVEN `ask-on-risk` interactive, same `allowed:false`
- WHEN orchestrator handles
- THEN MUST NOT auto-run, MUST print offer

### Requirement: REQ-SDD-001 — Mandatory Workflow and Delegation Reads

Orchestrator MUST read `biggz-orchestrator-workflow.md` (workflow, graph, dispatcher, gates, ledger, recall) and `biggz-orchestrator-delegation.md` (ladder, rules, authority, surfaces) before delegating SDD work. Reads MUST be via file read and evidenced in launch prompt. Skipped/unreadable MUST fail-closed.

#### Scenario: Both docs read before delegation

- GIVEN `/sdd-continue` trigger
- WHEN both docs have been read then `sdd-spec` launched
- THEN prompt MUST evidence reads and delegation MUST proceed

#### Scenario: Skipped read blocks

- GIVEN workflow doc skipped
- WHEN routing to any `sdd-*`
- THEN it MUST block with mandatory-read error

### Requirement: REQ-SDD-002 — Work Routing Ladder Fail-Closed

System MUST enforce ladder: 1) Inline Direct (typo, one-file, 1–3 known files), 2) Simple Delegation (`explore` scout, `general` worker/verify), 3) SDD (optional). SDD MUST be selected ONLY on explicit request (`biggz sdd-new`/`sdd-continue` or direct ask) or accepted proposal; size/file-count/risk alone MUST NOT select SDD. MAY suggest SDD when durable artifacts reduce ambiguity.

#### Scenario: Large diff without SDD request does not launch SDD

- GIVEN 12 files, 800 lines, no explicit SDD request
- WHEN ladder evaluated
- THEN it MUST NOT launch `sdd-propose`; select Simple Delegation MAY suggest SDD

#### Scenario: Explicit SDD request selects SDD

- GIVEN user says `use SDD for this feature`
- WHEN ladder evaluated
- THEN it MUST select SDD via preflight/init guards

### Requirement: REQ-SDD-003 — Native Dispatcher Routing

Orchestrator MUST route via native dispatcher `biggz sdd-status --json --instructions` (`biggz sdd-continue <change>`) as single authority for `openspec`/`BigMem`/`hybrid`. MUST route only by `nextRecommended` + dependencies, never free-text. MUST respect `blockedReasons` and ledger attempt authority; `blocked` MUST stop launch.

#### Scenario: Dispatcher drives phase

- GIVEN `sdd-status` returns `nextRecommended==spec`
- WHEN routing
- THEN it MUST launch `sdd-spec` only

#### Scenario: Blocked stops apply

- GIVEN `sdd-status` `blockedReasons` non-empty for apply
- WHEN evaluating apply
- THEN it MUST NOT launch `sdd-apply` and MUST surface blockers

### Requirement: REQ-SDD-004 — SDD Phase Authority Mapping

Each SDD phase MUST map to `sdd-*` agent (`sdd-explore`, `sdd-research`, `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-archive`, `sdd-sync`). `general`/`explore` for SDD MUST be rejected by guard (`internal/sdd/synthesis_gate.go` + `internal/orchestrator/*`).

#### Scenario: Design maps to sdd-design

- GIVEN `nextRecommended==design`
- WHEN selecting agent
- THEN it MUST select `sdd-design` not `general`

#### Scenario: SDD explore uses sdd-explore

- GIVEN SDD-tracked change needs exploration
- WHEN launching explore for that change
- THEN it MUST use `sdd-explore`, reject `explore`

### Requirement: REQ-SD-S1 — Orchestrator Gate Before Apply-Close and Final Done (Q1)

The orchestrator MUST call `session_guard` before closing any `apply` batch and before final `done`. If `HasSessionSummary` is false and bash fallback not verified, it MUST block continuation with `needs_decision` and `blockedReasons=["session_summary_missing"]`, and MUST NOT proceed to next phase or `archive`.

#### Scenario: Apply batch close blocked
- GIVEN `sdd-apply` batch finished but `biggz_mem_session_summary` not yet persisted
- WHEN orchestrator evaluates pre-close hook
- THEN `nextRecommended` MUST be `resolve-blockers` with `session_summary_missing` and no phase advance

#### Scenario: Final done blocked recovers after summary
- GIVEN gate blocked `done` for missing summary
- WHEN `session_summary` (MCP or bash) verified
- THEN gate MUST clear and `done`/`archive` MAY proceed

### Requirement: REQ-SD-S2 — Mandatory Bash Fallback Routing (Q2)

When `available_tools` lacks `biggz_mem_*`, orchestrator MUST route close via `biggz bigmem save --type session_summary` bash fallback. The fallback MUST be mandatory and use `PutBlob >100k` + `EndSession` parity (`engram/store.go` 8897L, `SessionActivity` 10m nudge, `DetectProjectFull` 5 cases).

#### Scenario: MCP missing triggers bash path
- GIVEN `available_tools` lacks `biggz_mem_session_summary`
- WHEN closing `apply` batch
- THEN orchestrator MUST exec `biggz bigmem save --type session_summary --project <proj> --json` via bash

#### Scenario: MCP present skips bash
- GIVEN MCP tool available
- WHEN closing
- THEN orchestrator MUST use MCP path and MUST NOT invoke bash fallback

### Requirement: REQ-SD-S3 — Explicit Verification context(5)+search (Q3)

Before allowing `done`, orchestrator MUST run `biggz_mem_context(5)` and `search` (empty-query `recent`/`Search("",…)` ordered `updated_at DESC`, not FTS) plus `sdd-status --json` fallback when BigMem empty. Results MUST contain the new `session_summary`; otherwise MUST be treated as verification failure.

#### Scenario: Verification passes via context
- GIVEN summary saved with `--project biggz-ai`
- WHEN `biggz_mem_context(5)` and `biggz bigmem search --query "" --limit 5` executed
- THEN output MUST list the summary; `done` MUST be allowed

#### Scenario: Empty BigMem fallback
- GIVEN BigMem `context`/`search` empty
- WHEN verification runs
- THEN `git log --oneline -15` and `biggz sdd-status --json --instructions` MUST run and fallback noted, and `done` MUST remain blocked until summary appears

### Requirement: REQ-SD-S4 — Complementary Discipline (Per-Task + Summary) (Q4)

Orchestrator MUST enforce per-task `biggz_mem_save` after every delegated sub-agent (proactive triggers + delivery guarantee) plus `session_summary` on close. Per-task saves MUST use dedup 15m, `capture_prompt` rules, and `compaction` → `EndSession` + `mem_context` recovery. Summary MUST NOT be skipped even if per-task saves exist.

#### Scenario: Delegated sdd-spec completed
- GIVEN `sdd-spec` sub-agent returned
- WHEN synthesis emitted before checkpoint
- THEN orchestrator MUST persist per-task save (or bubble sub-agent save) before next phase

#### Scenario: Summary still required
- GIVEN per-task saves verified in `biggz_mem_context`
- WHEN closing session without `session_summary`
- THEN gate MUST remain blocked per REQ-SD-S1

### Requirement: REQ-SD-S5 — Retry-Once + Degraded Fallback + Delivery Guarantee (Q5)

On save/verify failure, orchestrator MUST retry once. If still failing, it MUST deliver the user-facing answer anyway with brief failure note, write degraded fallback file (`openspec/changes/{change}/session-fallback.md`), and not block reply on memory operation. Next session MUST retry persistence.

#### Scenario: Retry succeeds
- GIVEN first `session_summary` call failed or timed out
- WHEN retried once
- THEN success MUST satisfy REQ-SD-S3 and allow `done`

#### Scenario: Degraded deliver with note
- GIVEN retry still fails
- WHEN closing
- THEN orchestrator MUST deliver complete answer with note `BigMem save failed — fallback persisted, will retry next session` and write fallback file; `review` gate MUST record persisted evidence or note
