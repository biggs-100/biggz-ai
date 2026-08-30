# Skills Specification

## Purpose

Skills covers the style guide and lint validation for `SKILL.md` files, including token buckets, frontmatter validation, and wrapper exit codes ported from gentle-pi hardening ola 1.

## Requirements

### Requirement: Skill Style Guide Presence

The system MUST provide `docs/skill-style-guide.md` with 6 normative sections ported from `gentle-pi/docs/skill-style-guide.md` covering Purpose, When to create, Required structure (`SKILL.md` with 6 ordered sections + `assets/`/`references/`), Frontmatter (`name` kebab-case, `description` single-line quoted trigger ≤250 chars), Writing rules (180–450 ideal, 700 recommended max, 1000 hard max), Decision gates, Output contract, and Registry expectations.

#### Scenario: Guide contains 6 sections

- GIVEN `docs/skill-style-guide.md` exists
- WHEN its contents are inspected
- THEN it MUST contain headings for Required structure, Frontmatter, Writing rules, Decision gates, Output contract, and Registry expectations

#### Scenario: Frontmatter rule quoted trigger

- GIVEN the guide's Frontmatter section
- WHEN reading the `description` rule
- THEN it MUST state `description` is one physical line, quoted, YAML-safe, and trigger-rich with `<=250` chars

### Requirement: LintSkill Token Buckets and Frontmatter Validation

The system MUST implement `internal/skills/lint.go` `LintSkill(path) (int, []string, error)` and `CountTokens(body) int` (`len(fields)`) that extracts frontmatter between `---` delimiters, validates `description` is single-line quoted containing `Trigger:`/`trigger:` and ≤250 chars, and reports diagnostics: `180–450` pass, `450–1000` warn, `>1000` fail; missing/multi-line/unquoted/no-trigger frontmatter MUST be `FAIL`.

#### Scenario: 300 tokens passes without diagnostics

- GIVEN a `SKILL.md` with valid frontmatter trigger and body of ~300 tokens
- WHEN `LintSkill` is called
- THEN it MUST return ~300 tokens with no `FAIL` diagnostics

#### Scenario: 1001 tokens fails hard limit

- GIVEN a `SKILL.md` with body >1000 tokens
- WHEN `LintSkill` is called
- THEN diagnostics MUST contain `FAIL: token count` exceeding hard limit 1000

#### Scenario: 600 tokens warns

- GIVEN a `SKILL.md` with body 600 tokens and valid frontmatter
- WHEN linted
- THEN diagnostics MUST contain `WARN:` for ideal 450 exceedance and MUST NOT contain `FAIL` for tokens

#### Scenario: Missing trigger fails

- GIVEN frontmatter `description: "do something"` without `Trigger:`
- WHEN validated
- THEN diagnostics MUST contain `FAIL: description missing trigger keyword`

#### Scenario: Unquoted description fails

- GIVEN frontmatter `description: Trigger: do thing` without quotes
- WHEN validated
- THEN it MUST be `FAIL: description must be single-line quoted`

### Requirement: Check-Skill-Lint Wrapper Exit Codes

The system MUST provide `scripts/check-skill-lint.mjs` that finds `SKILL.md` under `skills/` and `internal/assets/skills/`, lints each via Go-equivalent semantics, and exits `0` when all pass, `1` when any `FAIL`, `2` when only `WARN` (no `FAIL`).

#### Scenario: All pass exits 0

- GIVEN all `SKILL.md` files lint without `FAIL` or `WARN`
- WHEN `node scripts/check-skill-lint.mjs` runs
- THEN it MUST exit `0`

#### Scenario: One fail exits 1

- GIVEN one `SKILL.md` has `FAIL` (e.g., >1000 tokens)
- WHEN the wrapper runs
- THEN it MUST exit `1` and print `FAIL` to stderr

#### Scenario: Only warn exits 2

- GIVEN no `FAIL` but one `WARN` (e.g., 600 tokens)
- WHEN the wrapper runs
- THEN it MUST exit `2`
