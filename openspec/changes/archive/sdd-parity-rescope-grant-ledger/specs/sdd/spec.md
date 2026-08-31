# Delta for sdd

## ADDED Requirements

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
