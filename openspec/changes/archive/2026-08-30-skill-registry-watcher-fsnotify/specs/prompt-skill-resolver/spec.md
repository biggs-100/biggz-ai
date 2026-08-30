# Delta for prompt-skill-resolver

## MODIFIED Requirements

### Requirement: Skill Registry Scanning — Priority, Non-Recursive, Filtering

The system MUST implement `ScanSkillsFromDir` via `os.ReadDir` non-recursive (top-level only) in `internal/skillregistry/resolver.go`. Priority MUST be explicit 7-provider ordered array (first match wins, oh-my-pi order). `disabledExtensions` `skill:<name>` MUST exclude that skill. `ignored`/`include` globs MUST filter before registration. Watcher MUST reuse `ProviderPriority` and `Fingerprint` as trigger without changing scan semantics.
(Previously: scanning only; now watcher reuses ProviderPriority+Fingerprint as trigger)

#### Scenario: Priority deterministic

- GIVEN same name in providers 2 and 5
- WHEN registry resolves
- THEN provider 2 MUST win

#### Scenario: Non-recursive ignores nested

- GIVEN `skills/a/SKILL.md` and `skills/a/nested/SKILL.md`
- WHEN `ScanSkillsFromDir("skills/a")` runs
- THEN only `a` MUST be returned

#### Scenario: disabledExtensions excludes

- GIVEN `disabledExtensions: ["skill:foo"]` and `foo` present
- WHEN scan completes
- THEN `foo` MUST NOT appear

#### Scenario: Glob filtering applied

- GIVEN `ignored: ["*_test*"]` with `bar` and `bar_test`
- WHEN scan completes
- THEN `bar_test` MUST be excluded, `bar` remains

#### Scenario: Watcher reuses priority and fingerprint without scan change

- GIVEN watcher `Start` resolves dirs and debounce fires
- WHEN it evaluates `Fingerprint` and `ProviderPriority`
- THEN it MUST use same 7-provider order and fingerprint logic as `ScanSkillsFromDir`/`Fingerprint` and MUST NOT alter scanning
