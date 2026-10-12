---
name: issue-creation
description: "Create GitHub issues with issue-first checks. Trigger: creating GitHub issues, bug reports, or feature requests."
license: Apache-2.0
---

## When to Use

Use this skill when:
- Creating a GitHub issue (bug report or feature request)
- Helping a contributor file an issue
- Triaging or approving issues as a maintainer

---

## Critical Rules

1. **Blank issues are disabled** — MUST use a template (bug report or feature request)
2. **No `status:*` label is applied automatically** — the issue templates declare only their `type:*` label and no workflow adds a status, so add `status:needs-review` yourself if you want that state visible
3. **An issue must be approved before its PR is opened** — either a maintainer added `status:approved`, or a maintainer opened the issue themselves, in which case authorship **is** the approval and the label is only a filterable marker. No CI job reads the label: what CI actually enforces on the PR is its `Closes #<N>` reference, exactly one `type:*` label, and the 400-line budget (`size:exception` to exceed)
4. **Questions go to [Discussions](https://github.com/biggs-100/biggz-ai/discussions)**, not issues

---

## Workflow

```
1. Search existing issues for duplicates
2. Choose the correct template (Bug Report or Feature Request)
3. Fill in ALL required fields
4. Check pre-flight checkboxes
5. Submit → the template adds its `type:*` label; add `status:needs-review` yourself if you want that state visible
6. Get the issue approved: a maintainer adds `status:approved`, or a maintainer authored it and authorship is the approval
7. Only then open a PR linking this issue
```

---

## Issue Templates

### Bug Report

Template: `.github/ISSUE_TEMPLATE/bug_report.yml`
Auto-labels: `bug`, `status:needs-review`

#### Required Fields

| Field | Description |
|-------|-------------|
| **Pre-flight Checks** | Checkboxes: no duplicate + understands approval workflow |
| **Bug Description** | Clear description of the bug |
| **Steps to Reproduce** | Numbered steps to reproduce |
| **Expected Behavior** | What should have happened |
| **Actual Behavior** | What happened instead (include errors/logs) |
| **Operating System** | Dropdown: macOS, Linux variants, Windows, WSL |
| **Agent / Client** | Dropdown: Claude Code, OpenCode, Gemini CLI, Cursor, Windsurf, Codex, Other |
| **Shell** | Dropdown: bash, zsh, fish, Other |

#### Optional Fields

| Field | Description |
|-------|-------------|
| **Relevant Logs** | Log output (auto-formatted as code block) |
| **Additional Context** | Screenshots, workarounds, extra info |

#### Example — Bug Report via CLI

```bash
gh issue create --template "bug_report.yml" \
  --title "fix(scripts): setup.sh fails on zsh with glob error" \
  --body "
### Pre-flight Checks
- [x] I have searched existing issues and this is not a duplicate
- [x] I understand this issue needs status:approved before a PR can be opened

### Bug Description
Running setup.sh on zsh throws a glob error when no matching files exist.

### Steps to Reproduce
1. Clone the repo
2. Run \`./scripts/setup.sh\` in zsh
3. See error: \`zsh: no matches found: skills/*\`

### Expected Behavior
The script should handle missing glob matches gracefully.

### Actual Behavior
Script crashes with glob error.

### Operating System
macOS

### Agent / Client
Claude Code

### Shell
zsh

### Relevant Logs
\`\`\`
zsh: no matches found: skills/*
\`\`\`
"
```

---

### Feature Request

Template: `.github/ISSUE_TEMPLATE/feature_request.yml`
Auto-labels: `enhancement`, `status:needs-review`

#### Required Fields

| Field | Description |
|-------|-------------|
| **Pre-flight Checks** | Checkboxes: no duplicate + understands approval workflow |
| **Problem Description** | The pain point this feature solves |
| **Proposed Solution** | How it should work from the user's perspective |
| **Affected Area** | Dropdown: Scripts, Skills, Examples, Documentation, CI/Workflows, Other |

#### Optional Fields

| Field | Description |
|-------|-------------|
| **Alternatives Considered** | Other approaches or workarounds |
| **Additional Context** | Mockups, examples, references |

#### Example — Feature Request via CLI

```bash
gh issue create --template "feature_request.yml" \
  --title "feat(scripts): add Codex support to setup.sh" \
  --body "
### Pre-flight Checks
- [x] I have searched existing issues and this is not a duplicate
- [x] I understand this issue needs status:approved before a PR can be opened

### Problem Description
The setup script only configures Claude Code, Gemini CLI, and OpenCode. Codex users have to manually copy skills.

### Proposed Solution
Add a Codex option to setup.sh that links skills to the .codex/ directory.

Example:
\`\`\`bash
./scripts/setup.sh --agent codex
\`\`\`

### Affected Area
Scripts (setup, installation)

### Alternatives Considered
Manually symlinking, but that defeats the purpose of the setup script.
"
```

---

## Label System

### Declared by the Templates

| Template | Labels declared |
|----------|-------------|
| Bug Report | `type:bug` |
| Feature Request | `type:feature` |

Those are the only automatic labels, and only when the issue is created through the
template. Nothing applies a `status:*` label: an issue created with `gh issue create`
carries exactly what `--label` lists and nothing more.

### Applied by Maintainers

| Label | When to apply |
|-------|--------------|
| `status:approved` | Issue accepted for implementation — its PR can be opened. An issue a maintainer created is already approved without it; the label keeps the list filterable |
| `priority:high` | Critical bug or urgent feature |
| `priority:medium` | Important but not blocking |
| `priority:low` | Nice to have |

---

## Maintainer Approval Workflow

```
1. Old or new issue needs a decision
2. Review the issue — is it valid, clear, and in scope?
3. If YES → add status:approved label
4. If NO → comment with reason, close if needed
5. The author can now open a PR linking this issue
6. An issue a maintainer opened themselves needs no approval step: authorship is the
   approval, and no CI job reads the label either way
```

---

## Decision Tree

```
Is it a bug?                    → Use Bug Report template
Is it a new feature/improvement? → Use Feature Request template
Is it a question?               → Use Discussions, NOT issues
Is it a duplicate?              → Link to existing issue, close
```

---

## Commands

```bash
# Search existing issues before creating
gh issue list --search "keyword"

# Create bug report
gh issue create --template "bug_report.yml" --title "fix(scope): description"

# Create feature request
gh issue create --template "feature_request.yml" --title "feat(scope): description"

# Maintainer: approve an issue
gh issue edit <number> --add-label "status:approved"

# Not needed for an issue a maintainer created: authorship is the approval

# Maintainer: add priority
gh issue edit <number> --add-label "priority:high"
```
