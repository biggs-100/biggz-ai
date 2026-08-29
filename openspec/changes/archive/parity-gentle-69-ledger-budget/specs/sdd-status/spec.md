# Delta for sdd-status

## ADDED Requirements

### Requirement: Declared Artifact Store and Hybrid Locator

The system MUST resolve the declared artifact store by reading `openspec/config.yaml` key `artifact_store` (normalized: `hybrid` and `engram`/`bigmem` alias to hybrid reading; missing or unreadable file defaults to `openspec`; `none` disables planning I/O). `resolveArtifactPaths`/`declaredArtifactStore` MUST branch per store: `openspec` returns filesystem `openspec/changes/{change}/…` paths; `engram`/`bigmem` returns `bigmem:sdd/{change}/…` paths; `hybrid` merges both stores with filesystem-wins on name collision; `none` returns empty paths. `artifactPaths` and `contextFiles` projected by `ProjectStatusV2` / `StatusWithOptions` MUST reflect the resolved store.

#### Scenario: Reads declared store from config

- GIVEN `openspec/config.yaml` contains `artifact_store: hybrid`
- WHEN `declaredArtifactStore(workspaceRoot)` is called
- THEN it MUST return `hybrid` (normalized) and not default to `openspec`

#### Scenario: Defaults to openspec when config absent

- GIVEN no `openspec/config.yaml` exists
- WHEN status derivation runs
- THEN `ArtifactStore` MUST be `openspec` and `artifactPaths` MUST contain filesystem paths

#### Scenario: Hybrid routing filesystem-wins

- GIVEN BigMem and filesystem both contain change `parity-gentle-69-ledger-budget`
- WHEN `collectBigMemChangesWithArchive` merges
- THEN the resulting `ChangeStatus` MUST be the filesystem entry and the BigMem duplicate MUST be discarded

#### Scenario: artifactPaths per store

- GIVEN store is `engram`
- WHEN `resolveArtifactPaths` projects `proposal`
- THEN it MUST return `bigmem:sdd/{change}/proposal` and not a filesystem path
- AND when store is `openspec` it MUST return `openspec/changes/{change}/proposal.md`

#### Scenario: None store disables planning I/O

- GIVEN `artifact_store: none`
- WHEN status is derived
- THEN `artifactPaths` fields MUST be empty and no filesystem or BigMem read MUST be attempted for planning artifacts
