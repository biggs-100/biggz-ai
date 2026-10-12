---
name: issue-creation
description: "Trigger: creating GitHub issues, bug reports, feature requests, issue triage. File and triage issues for biggs-100/biggz-ai with the real templates, labels and approval rule."
license: Apache-2.0
---

## Activation Contract

Activate when filing or triaging an issue in `biggs-100/biggz-ai`, or when approving one as
a maintainer.

Do not activate for questions or open-ended ideas (use Discussions), or when an open issue
already covers the change.

## Hard Rules

- Blank issues are disabled: every issue goes through `bug_report.yml` or
  `feature_request.yml`.
- The template carries the label and nothing else: `type:bug` or `type:feature`. No workflow
  applies a `status:*` label, and `status:approved` is the only status label this repository
  defines, so an issue simply waits until a maintainer approves it.
- An issue is approved before its PR opens: a maintainer adds `status:approved`, or a
  maintainer authored the issue, in which case authorship **is** the approval. The label is a
  filterable marker, never a CI gate: `pr-check.yml` reads only a `Closes`, `Fixes` or
  `Resolves #<N>` reference in the PR body, its single `type:*` label and the 400-line budget.
- Search first, and link the existing issue instead of filing a duplicate.
- These labels exist and no others: `type:*`, `status:approved`, `size:exception`.

## Decision Gates

| Situation | Action |
| --- | --- |
| Reproducible defect in a released build | `bug_report.yml`, including `biggz version` output and the OS |
| New capability or improvement | `feature_request.yml` |
| Question, or an idea not ready to be scoped | GitHub Discussions (`https://github.com/biggs-100/biggz-ai/discussions`), not an issue |
| Duplicate found | Link the existing issue and close the new one |
| Maintainer reviewing an issue | Judge it valid, clear and in scope, then add `status:approved` or comment with the reason and close |
| Issue a maintainer opened themselves | Already approved by authorship; label it only to keep the list filterable |

## Execution Steps

1. Search: `gh issue list --search "keyword" --state all`.
2. Create through the template, never blank:
   `gh issue create --template bug_report.yml --title "fix(scope): ..."` or
   `--template feature_request.yml --title "feat(scope): ..."`. The field-by-field bodies and
   worked examples are in `references/issue-flow.md`.
3. `gh label list` is the source of truth for labels; nothing adds a status automatically, so
   an issue needs no marking before a maintainer approves it.
4. As maintainer, approve it for implementation:
   `gh issue edit <N> --add-label status:approved`. The PR can open afterwards.

## Output Contract

Return: the issue number and URL; the template used and the labels applied; the existing
issue linked instead, if any; and whether it waits for approval or is already approved by
authorship.

## References

- `references/issue-flow.md` — both templates field by field, with CLI examples and example
  bodies.
- `.github/workflows/pr-check.yml` — the three checks that actually gate a PR.
- `.github/PULL_REQUEST_TEMPLATE.md` — what the PR that closes the issue must contain.
