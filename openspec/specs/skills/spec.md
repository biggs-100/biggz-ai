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

The system MUST implement `internal/skills/lint.go` `LintSkill(path) (int, []string, error)` and `CountTokens(body) int` (`len(fields)`) that extracts frontmatter between `---` delimiters, validates `description` is single-line quoted containing `Trigger:`/`trigger:` and ≤250 chars, and reports diagnostics: `180–450` pass, `450–HARD_MAX` warn, `>HARD_MAX` fail (`HARD_MAX = 3200`, mirrored in `scripts/check-skill-lint.mjs`); missing/multi-line/unquoted/no-trigger frontmatter MUST be `FAIL`.

#### Scenario: 300 tokens passes without diagnostics

- GIVEN a `SKILL.md` with valid frontmatter trigger and body of ~300 tokens
- WHEN `LintSkill` is called
- THEN it MUST return ~300 tokens with no `FAIL` diagnostics

#### Scenario: 1001 tokens warns (mid-band)

- GIVEN a `SKILL.md` with body of 1001 tokens
- WHEN `LintSkill` is called
- THEN diagnostics MUST contain `WARN:` and MUST NOT contain token `FAIL`

#### Scenario: Over hard max fails

- GIVEN a `SKILL.md` with body over `HARD_MAX` (3200) tokens
- WHEN `LintSkill` is called
- THEN diagnostics MUST contain `FAIL: token count` exceeding hard limit 3200

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

The system MUST provide `scripts/check-skill-lint.mjs` that finds `SKILL.md` under `skills/` and `internal/assets/skills/`, lints each via Go-equivalent semantics (`HARD_MAX = 3200`), and exits `0` on pass or WARN-only, `1` on any `FAIL` (including mirror drift). There is no `exit 2`.

#### Scenario: All pass exits 0

- GIVEN all `SKILL.md` files lint without `FAIL`
- WHEN `node scripts/check-skill-lint.mjs` runs
- THEN it MUST exit `0`

#### Scenario: One fail exits 1

- GIVEN one `SKILL.md` has `FAIL` (e.g., over `HARD_MAX` tokens)
- WHEN the wrapper runs
- THEN it MUST exit `1` and print `FAIL` to stderr

#### Scenario: Only warn exits 0

- GIVEN no `FAIL` but one `WARN` (e.g., 600 tokens)
- WHEN the wrapper runs
- THEN it MUST exit `0`

### Requirement: Skill Mirror Sync

Every `skills/<name>/SKILL.md` with a counterpart at `internal/assets/skills/<name>/SKILL.md` MUST be byte-identical; the wrapper MUST report any drift as `FAIL` (mirror edits land in both trees in the same commit). Assets-only entries have no counterpart and are skipped.

#### Scenario: Drift fails

- GIVEN a one-mirror-only edit
- WHEN the wrapper runs
- THEN it MUST print `FAIL` naming the drifted file and exit `1`

### Requirement: Oversized Skill Trim Plan

The 7 oversized skills trimmed in Slice A (branch-pr 1336, sdd-apply 3018, sdd-verify 1790, sdd-archive 2671, sdd-design 1358, sdd-explore 1175, sdd-onboard 1558 tokens) MUST each be at or under 1000 tokens; raising `HARD_MAX` MUST NOT re-admit over-1000 bodies without a tracked plan.

#### Scenario: Trimmed skills stay under 1000

- GIVEN the Slice A trim landed
- WHEN lint runs
- THEN each listed skill MUST report ≤1000 tokens
