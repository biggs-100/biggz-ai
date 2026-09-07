# Delta for sdd

## ADDED Requirements

### Requirement: Archive Step 3b Post-Archive Hygiene

The system MUST run hygiene only after `ArchiveChange` succeeded via `os.Rename`; it MUST call `internal/git` helpers to `fetch --prune`, preview candidates, prompt `Prune/Keep`, and on `Prune` execute `branch -d` and `worktree prune`; `.biggz-instance` MUST remain inside archived folder.

#### Scenario: Preview and consent after archive

- GIVEN `archive` moved `openspec/changes/{change}` to `archive/YYYY-MM-DD-{change}` via `os.Rename`
- WHEN Step 3b starts with TTY
- THEN system MUST show table of candidates and prompt `Prune (delete) / Keep (retain)`

#### Scenario: Prune executes safe deletions

- GIVEN user selected `Prune` and candidates exist
- WHEN hygiene runs
- THEN system MUST delete branches via `branch -d` and prunable clean worktrees via `worktree prune` only

#### Scenario: Keep retains all

- GIVEN user selected `Keep` or non-TTY with no consent
- WHEN hygiene completes
- THEN no branch or worktree MUST be deleted and archive MUST stay intact

#### Scenario: .biggz-instance retained

- GIVEN archived change contained `.biggz-instance`
- WHEN `os.Rename` moves directory
- THEN `.biggz-instance` MUST exist under `archive/YYYY-MM-DD-{change}/.biggz-instance` and MUST NOT be deleted

#### Scenario: Fetch failure continues with warning

- GIVEN `git fetch --prune` fails
- WHEN Step 3b previews
- THEN warning MUST be logged and preview MUST still show local `[gone]` candidates

#### Scenario: Archive stays pure Rename

- GIVEN archive invoked
- WHEN `ArchiveChange` executes
- THEN it MUST only call `os.Rename` and MUST NOT call `branch -d`, `worktree prune`, or `RDDDisable` before move
