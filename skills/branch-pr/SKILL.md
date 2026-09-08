---
name: branch-pr
description: "Create biggz AI pull requests with issue-first checks. Trigger: creating, opening, or preparing PRs for review."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.0"
---

# biggz AI — Branch & PR Skill

## When to Use

Load this skill whenever you need to:
- Create a branch for a new fix or feature
- Open a pull request on [biggs-100/biggz-ai](https://github.com/biggs-100/biggz-ai)
- Prepare changes for review

## Critical Rules

1. **Every PR MUST link an approved issue** — `Closes/Fixes/Resolves #<N>` in the PR body, and that issue MUST have `status:approved`. PRs without this are **automatically rejected** by CI.
2. **Exactly one `type:*` label** — apply exactly ONE type label to the PR. CI will reject PRs with zero or multiple type labels.
3. **400-line review budget** — keep PRs within 400 changed lines (`additions + deletions`) or request/obtain maintainer-applied `size:exception` with rationale documented.
4. **Automated checks must pass** — see the Automated Checks table below.
5. **No `Co-Authored-By` trailers** — never add AI attribution to commits.
6. **No force-push to main/master** — protected branch.

## Workflow

```
1. Confirm the issue has status:approved
   gh issue view <N> --repo biggs-100/biggz-ai

2. Create a branch from main using the naming convention below

3. Implement changes following specs and design

4. Run checks locally (format + unit + E2E)

5. Commit using Conventional Commits format

6. Open a PR referencing the issue
   → Add exactly ONE type:* label
   → Fill in the PR body using the template

7. All automated checks must pass before merge
```

---

## Branch Naming

Branch names **must** match this pattern:

```
^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)\/[a-z0-9._-]+$
```

| Type | Example |
|------|---------|
| `feat/` | `feat/user-login` |
| `fix/` | `fix/duplicate-observation-insert` |
| `docs/` | `docs/api-reference-update` |
| `refactor/` | `refactor/extract-query-sanitizer` |
| `chore/` | `chore/bump-bubbletea-v0.26` |
| `style/` | `style/fix-linter-warnings` |
| `perf/` | `perf/optimize-catalog-loading` |
| `test/` | `test/add-pipeline-coverage` |
| `build/` | `build/update-goreleaser-config` |
| `ci/` | `ci/add-e2e-docker-job` |
| `revert/` | `revert/undo-model-picker-change` |

**Rules:**
- All lowercase
- Use hyphens, dots, or underscores as separators (no spaces, no uppercase)
- Description must be short and descriptive

---

## PR Body Format

Fill every section of `.github/PULL_REQUEST_TEMPLATE.md` (required unless marked optional): Linked Issue (`Closes #<N>`), PR Type (exactly one `type:*` checkbox), Summary, Changes table (`| File / Area | What Changed |`), Test Plan (`go test ./...`, `go run ./internal/gofmtcheck`, `cd e2e && ./docker-test.sh`, manual test), Contributor Checklist (issue link + `status:approved`, 400-line budget or `size:exception`, label, tests green, Conventional Commits, no `Co-Authored-By`).

---

## Automated Checks

These checks run on every PR and **all must pass** before merge:

| Check | What It Verifies | How to Fix |
|-------|-----------------|------------|
| **Check PR Cognitive Load** | PR stays within 400 changed lines (`additions + deletions`) or has `size:exception` | Split the PR, or request/obtain maintainer-applied `size:exception` and document the rationale |
| **Check Issue Reference** | PR body contains `Closes/Fixes/Resolves #N` | Add `Closes #<N>` to the PR body |
| **Check Issue Has `status:approved`** | Linked issue has been approved by a maintainer | Wait for maintainer to add `status:approved` to the issue |
| **Check PR Has `type:*` Label** | Exactly one `type:*` label is applied to the PR | Ask a maintainer to add the correct label; remove extras |
| **Unit Tests** | `go test ./...` passes | Fix failing tests before pushing |
| **Go Format** | `go run ./internal/gofmtcheck` passes | Format malformed Go files before pushing |
| **E2E Tests** | `cd e2e && ./docker-test.sh` passes | Fix failing E2E scenarios before pushing |

---

## Conventional Commits

Commit messages **must** match this pattern:

```
^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([a-z0-9\._-]+\))?!?: .+
```

### Format

```
<type>(<optional-scope>)!: <description>

[optional body]

[optional footer]
```

### Allowed Types

| Type | Purpose | PR Label |
|------|---------|----------|
| `feat` | New feature | `type:feature` |
| `fix` | Bug fix | `type:bug` |
| `docs` | Documentation only | `type:docs` |
| `refactor` | Code change (no behavior change) | `type:refactor` |
| `chore` | Maintenance, dependencies, tooling | `type:chore` |
| `style` | Formatting, linting (no logic change) | `type:chore` |
| `perf` | Performance improvement | `type:feature` |
| `test` | Adding or updating tests | `type:chore` |
| `build` | Build system or external deps | `type:chore` |
| `ci` | CI configuration | `type:chore` |
| `revert` | Reverts a previous commit | matches reverted type |

### Breaking Changes

Add `!` after the type/scope:

```
feat(cli)!: rename --config flag to --config-file

BREAKING CHANGE: the --config flag has been renamed to --config-file.
```

Breaking changes map to `type:breaking-change` label.

### Examples

```
feat(tui): add progress bar to installation steps
fix(agent): correct Claude Code detection on macOS
chore(deps): bump bubbletea to v0.26
ci: split unit and e2e test jobs
feat(cli)!: change default config path
```

---

## Commands

### Setup

```bash
# Confirm issue is approved before starting
gh issue view <N> --repo biggs-100/biggz-ai

# Create branch
git checkout main && git pull
git checkout -b fix/<short-description>
```

### Testing Locally

```bash
go test ./...                    # unit tests (add ./internal/<pkg>/... to scope)
go run ./internal/gofmtcheck     # go format
cd e2e && ./docker-test.sh      # E2E (Docker must be running)
```

### Open a PR

```bash
gh pr create --repo biggs-100/biggz-ai \
  --title "fix(agent): correct Claude Code detection on Linux" \
  --body "Closes #42 — $(cat .github/PULL_REQUEST_TEMPLATE.md)"
# Then fill every template section and add exactly one type:* label.
```

### Check PR Status

```bash
gh pr checks --repo biggs-100/biggz-ai <PR-number>
gh pr view --repo biggs-100/biggz-ai <PR-number>
```

### Add a Label

```bash
gh pr edit <PR-number> --repo biggs-100/biggz-ai --add-label "type:bug"
```
