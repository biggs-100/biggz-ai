# Delta for skills

## MODIFIED Requirements

### Requirement: LintSkill Token Buckets and Frontmatter Validation

`LintSkill`/`CountTokens` MUST keep frontmatter checks unchanged and report `180–450` pass, `450–HARD_MAX` warn, `>HARD_MAX` fail (`HARD_MAX` > 1000).
(Previously: hard max was fixed at 1000.)

#### Scenario: Over hard max fails

- GIVEN a body over `HARD_MAX`
- WHEN linted
- THEN it MUST report `FAIL: token count`

#### Scenario: Mid-band warns only

- GIVEN a 600-token valid body
- WHEN linted
- THEN it MUST contain `WARN:` and no token `FAIL`

### Requirement: Check-Skill-Lint Wrapper Exit Codes

The wrapper MUST exit `0` on pass or WARN-only, `1` on any `FAIL`.
(Previously: WARN-only exited 2, reddening CI.)

#### Scenario: WARN-only exits 0

- GIVEN no `FAIL`, one `WARN`
- WHEN the wrapper runs
- THEN it MUST exit `0`

#### Scenario: FAIL exits 1

- GIVEN one `FAIL`
- WHEN the wrapper runs
- THEN it MUST exit `1` and print `FAIL` to stderr

## ADDED Requirements

### Requirement: Skill Mirror Sync

Both mirrors MUST stay in sync; drift MUST fail the check.

#### Scenario: Drift fails

- GIVEN a one-mirror-only edit
- WHEN checked
- THEN it MUST report the drifted file

### Requirement: Oversized Skill Trim Plan

The 6 oversized skills MUST carry a trim/split plan that raising `HARD_MAX` MUST NOT close early.

#### Scenario: Raised max keeps plan open

- GIVEN raised `HARD_MAX`, oversized bodies remain
- WHEN lint runs
- THEN it MUST pass with the plan still open
