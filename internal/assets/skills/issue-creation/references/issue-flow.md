# The two issue templates, field by field

Both templates live in `.github/ISSUE_TEMPLATE/`. They are the only way to open an issue:
blank issues are disabled. Each one declares a single label and no workflow adds anything else.

## `bug_report.yml` — label `type:bug`

| Field | id | Required | What to write |
| --- | --- | --- | --- |
| What happened? | `what-happened` | yes | The defect and what you expected instead |
| Steps to reproduce | `reproduction` | yes | Exact commands or steps |
| Version | `version` | yes | Output of `biggz version` (or `biggz doctor`) |
| Platform | `platform` | yes | OS version, e.g. Windows 11, macOS 15, Ubuntu 24.04 |
| Relevant output | `logs` | no | Pasted output; the field renders as a shell block |
| Code of Conduct | `terms` | yes | The one required checkbox: follow the project's Code of Conduct |

Body checklist for a bug report: describe the defect, the first command that shows it, the
expected result, and the version that produced it. A reproducible command beats a description.

## `feature_request.yml` — label `type:feature`

| Field | id | Required | What to write |
| --- | --- | --- | --- |
| Problem | `problem` | yes | The pain the feature removes |
| Proposed solution | `solution` | yes | How it should behave, from the user's side |
| Alternatives considered | `alternatives` | no | Other approaches and why they lose |
| Area | `area` | yes | One of: CLI, TUI, MCP Server, BigMem, SDD Workflow, Review System, Install, Documentation, Other |

Body checklist for a feature request: state the problem before the solution, keep the
solution behavioural rather than an implementation sketch, and name the area so triage does
not have to guess.

## Examples

These two are shapes, not reports: the field names, the labels, the scopes and the flow are
this repository's, and every `<placeholder>` is yours to replace. Nothing below asserts how
biggz currently behaves, and nothing is copied from a real issue - an example claiming a
defect the code cannot produce is exactly the error this file exists to prevent.

Bug report:

```bash
gh issue create --template bug_report.yml --label "type:bug" \
  --title "fix(cli): biggz doctor reports <check> as failed after an upgrade" \
  --body "
### What happened?
<what you ran, and what it did instead of what the docs say>

### Steps to reproduce
1. biggz install --agent pi
2. biggz doctor
3. <the check that misbehaves, and how it misbehaves>

### Version
biggz version -> v0.9.x

### Platform
Windows 11

### Relevant output
\`\`\`
<the line biggz doctor printed, e.g. the icon, then the check id, a colon and the message>
\`\`\`
"
```

Feature request:

```bash
gh issue create --template feature_request.yml --label "type:feature" \
  --title "feat(review): <the capability you want>" \
  --body "
### Problem
<the pain, and what you have to do today instead>

### Proposed solution
<how it should behave from the user's side, not how to build it>

### Alternatives considered
<other approaches and why they lose>

### Area
Review System
"
```

## Labels

| Label | Meaning |
| --- | --- |
| `type:bug`, `type:feature`, `type:docs`, `type:refactor`, `type:chore`, `type:breaking-change` | Exactly one per PR; `type:feature` or `type:bug` come from the templates |
| `status:approved` | A maintainer accepted the issue for implementation |
| `size:exception` | A maintainer allows a PR over the 400-line budget, with the rationale documented |

`gh label list` is the source of truth. There is no `priority:*` label in this repository, and
an issue created with `gh issue create` carries exactly what `--label` lists and nothing more.

## What ends the flow

`gh issue edit <N> --add-label status:approved`, or the issue being authored by a maintainer.
After that a PR can open with a `Closes`, `Fixes` or `Resolves #<N>` reference, exactly one
`type:*` label, and the 400-line budget (or `size:exception`). Those three are what
`.github/workflows/pr-check.yml` enforces; nothing reads `status:approved`.
