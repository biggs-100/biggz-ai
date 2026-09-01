# Delta for orchestrator

## ADDED Requirements

### Requirement: REQ-PS1 — Language-Aware Synthesis Content
System MUST render content after markers in last human language; markers/ids stay English.
#### Scenario: Spanish → Spanish
- GIVEN last human `en que nos quedamos?`
- WHEN synthesis renders
- THEN content MUST be Spanish, markers English
#### Scenario: English → English
- GIVEN last human `let's continue`
- WHEN synthesis renders
- THEN content MUST be English
#### Scenario: hi/hello → English
- GIVEN last human `hi`/`hello`
- WHEN detection runs
- THEN MUST treat as English
#### Scenario: Mixed → last turn wins
- GIVEN mixed history, last `ok, continua con el spec`
- WHEN rendering
- THEN MUST be Spanish

### Requirement: REQ-PS2 — Scannable Structure (5 sections)
MUST render 5 sections in human language order: 1 Resumen, 2 Decisiones `| Topic | Decision |`+`◆ Phase · Status · Next`, 3 Evidencia, 4 Artefactos, 5 Riesgos/Próximo. Empty Preview/Diff omitted; >50KB paginate.
#### Scenario: Same structure all phases
- GIVEN phase propose/spec/design/tasks/apply/verify/archive
- WHEN synthesis renders
- THEN MUST contain 5 sections in order
#### Scenario: Empty omitted
- GIVEN Preview/Diff empty
- WHEN RenderSynthesis runs
- THEN MUST omit or show `None`
#### Scenario: >50KB paginated
- GIVEN artifact >50KB
- WHEN previewing
- THEN MUST paginate via ReadLoop, Preview 300 width

### Requirement: REQ-PS3 — Technical Whitelist
Paths, `sdd/...`, code `ORDER BY`/`Search`, branches, IDs MUST stay English.
#### Scenario: Path stays English
- GIVEN human Spanish, artifact `internal/sdd/synthesis.go`
- WHEN listing Artifacts/Paths
- THEN MUST stay `internal/sdd/synthesis.go`
#### Scenario: Topic key stays English
- GIVEN human Spanish
- WHEN referencing BigMem
- THEN MUST stay `sdd/polish-synthesis-human-language/proposal`
#### Scenario: Code stays English
- GIVEN snippet `ORDER BY updated_at DESC`
- WHEN rendered Spanish
- THEN MUST stay English

### Requirement: REQ-PS4 — Marker Invariant (b0d2fc1)
Markers `## Sub-agent Result:`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**` MUST stay English; missing MUST block.
#### Scenario: Spanish keeps English markers
- GIVEN content Spanish with markers English
- WHEN `HasSynthesis` checks
- THEN MUST pass
#### Scenario: Missing blocks
- GIVEN missing `**Artifacts/Paths:**`
- WHEN checkpoint ask ≤120s
- THEN MUST `isError:true`/`block:true`
#### Scenario: Session Recall exception
- GIVEN `## Session Recall` present
- WHEN checkpoint without synthesis
- THEN MUST allow
#### Scenario: Thin advise language-agnostic
- GIVEN thin synthesis `count<2`/`len<50` with markers and `BIGGZ_ADVISE=1`
- WHEN gate evaluates
- THEN MUST not block, MAY emit `concern: synthesis is thin`

### Requirement: REQ-PS5 — Detection + Hint Propagation
MUST detect language via keywords/diacritics and inject `Respond executive_summary in {lang}; keep paths English` into `sdd-*` prompts; ambiguous short MUST default English + fallback at render.
#### Scenario: Detect Spanish → hint Spanish
- GIVEN last `ok, continua`
- WHEN launching sdd-spec
- THEN prompt MUST contain `in Spanish; keep paths English`
#### Scenario: Detect English → hint English
- GIVEN last `ok, continue`
- WHEN launching
- THEN prompt MUST contain `in English`
#### Scenario: Short ambiguous defaults English
- GIVEN last `ok`/`dale`/`go`
- WHEN ambiguous
- THEN MUST default English, fallback-translate at render
