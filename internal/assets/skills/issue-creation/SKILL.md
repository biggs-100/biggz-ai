---
name: issue-creation
description: "Trigger: creating GitHub issues, bug reports, feature requests, issue triage. File and triage issues for biggs-100/biggz-ai with the real templates, labels and approval rule."
license: Apache-2.0
---

## Activation Contract

Activate when filing a bug report or a feature request in `biggs-100/biggz-ai`, or when
triaging and approving an issue as a maintainer.

Do not activate for questions or open-ended ideas (use Discussions), or when an open issue
already covers the change.

## Hard Rules

- Blank issues are disabled: every issue goes through `bug_report.yml` or
  `feature_request.yml`.
- The template carries the label and nothing else: `type:bug` or `type:feature`. No workflow
  applies a `status:*` label, so add `status:needs-review` by hand when the state should be
  visible.
- An issue is approved before its PR opens: a maintainer adds `status:approved`, or a
  maintainer authored the issue, in which case authorship **is** the approval. The label is a
  filterable marker, never a CI gate: `pr-check.yml` reads only the PR's `Closes #<N>`, its
  single `type:*` label and the 400-line budget.
- Search first, and link the existing issue instead of filing a duplicate.
- These labels exist and no others: `type:*`, `status:approved`, `size:exception`, plus
  GitHub's defaults. There is no `priority:*` label.

## Decision Gates

| Situation | Action |
| --- | --- |
| Reproducible defect in a released build | `bug_report.yml`, including `biggz version` output and the OS |
| New capability or improvement | `feature_request.yml` |
| Question, or an idea not ready to be scoped | GitHub Discussions, not an issue |
| Duplicate found | Link the existing issue and close the new one |
| Maintainer reviewing an issue | Add `status:approved`, or comment with the reason and close |
| Issue a maintainer opened themselves | Already approved by authorship; label it only to keep the list filterable |

## Execution Steps

1. Search: `gh issue list --search "keyword" --state all`.
2. Create through the template, never blank:
   `gh issue create --template bug_report.yml --title "fix(scope): ..."` or
   `--template feature_request.yml --title "feat(scope): ..."`. The field-by-field bodies and
   worked examples are in `references/issue-flow.md`.
3. Mark the state when it helps: `gh label list` shows what exists, and
   `gh issue edit <N> --add-label status:needs-review` marks an issue as waiting.
4. As maintainer, approve it for implementation:
   `gh issue edit <N> --add-label status:approved`. The PR can open afterwards.

## Output Contract

Return: the issue number and URL; the template used and the labels applied; the existing
issue linked instead of filing, if any; and whether the issue waits for a maintainer's
approval or is already approved by authorship.

## References

- `references/issue-flow.md` — both templates field by field, with CLI examples and example
  bodies.
- `.github/workflows/pr-check.yml` — the three checks that actually gate a PR.
- `.github/PULL_REQUEST_TEMPLATE.md` — what the PR that closes the issue must contain.
