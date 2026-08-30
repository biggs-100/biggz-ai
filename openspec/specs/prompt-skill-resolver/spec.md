# prompt-skill-resolver Specification

## Purpose

Externalize review lens prompts to static `.md` via `text/template` and provide secure `skill://` resolver with deterministic 7-provider priority (oh-my-pi parity).

## Requirements

### Requirement: Prompt Templates via go:embed and text/template

The system MUST provide 6 `.md` at `internal/assets/prompts/review/` (R1-R4/external/shared) embedded via `//go:embed all:prompts/review` in `internal/assets/embed.go` through `assets.FS`. Rendering MUST use `text/template` `{{.Var}}` with `missingkey=error`. The system MUST NOT use `html/template`, string literals, or `fmt.Sprintf` for prompts in `internal/review/lens`.

#### Scenario: Templates embedded

- GIVEN `assets.FS` with `//go:embed all:prompts/review`
- WHEN reading `prompts/review`
- THEN 6 `.md` MUST be present and readable

#### Scenario: Missing variable fails

- GIVEN template `{{.Diff}}` with `missingkey=error`
- WHEN executed without `Diff`
- THEN it MUST return error

#### Scenario: No fmt.Sprintf for prompts

- GIVEN any `internal/review/lens/*/*.go`
- WHEN inspected for prompt construction
- THEN zero `fmt.Sprintf` for prompts MUST exist

#### Scenario: Successful render interpolates

- GIVEN valid data for all `{{.Var}}`
- WHEN `Parse`+`Execute` runs
- THEN output MUST contain values and no `{{`

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

### Requirement: Skill URI Resolution with Containment

The system MUST resolve `skill://<name>/<path>` via `path.Clean` + `filepath.EvalSymlinks` + `strings.HasPrefix` against resolved root. It MUST reject `..` after `Clean`, absolute paths, and symlink escapes after `EvalSymlinks`. Violations MUST return error without filesystem access outside root.

#### Scenario: Valid URI resolves inside root

- GIVEN skill `foo` at `/skills/foo` with `docs/a.md`
- WHEN `skill://foo/docs/a.md` resolved
- THEN realpath MUST be under root and bytes returned

#### Scenario: Traversal with .. rejected

- GIVEN `skill://foo/../../etc/passwd`
- WHEN resolved
- THEN it MUST error and not access outside root

#### Scenario: Symlink escape rejected

- GIVEN `skill://foo/link` where `link -> /etc/passwd`
- WHEN `EvalSymlinks`+`HasPrefix` checked
- THEN it MUST error

#### Scenario: Absolute path rejected

- GIVEN `skill://foo//etc/passwd`
- WHEN parsed
- THEN it MUST error before access

### Requirement: CI No-fmtSprintf Guard

CI MUST fail when `rg 'fmt\.Sprintf' internal/review/lens` finds prompt `fmt.Sprintf`. Guard MUST target only `internal/review/lens` and allow `//lint:ignore no-fmtSprintf` for non-prompt uses. Step MUST be required for merge.

#### Scenario: CI fails on fmt.Sprintf in lens

- GIVEN `lens.go` contains prompt `fmt.Sprintf`
- WHEN CI step runs
- THEN job MUST fail reporting file:line

#### Scenario: CI passes when clean

- GIVEN zero prompt `fmt.Sprintf` in `internal/review/lens`
- WHEN CI step runs
- THEN job MUST pass

#### Scenario: Allowlisted exception permitted

- GIVEN `fmt.Sprintf` with `//lint:ignore no-fmtSprintf`
- WHEN CI step runs
- THEN that line MUST not fail
