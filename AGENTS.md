# biggz-ai — Agent Skills Index

When working on this project, load the relevant skill(s) BEFORE writing any code.

1. Find your task in the trigger column
2. Read the SKILL.md at the listed path
3. Follow ALL patterns and rules from the loaded skill

SDD phase skills (`sdd-*`, except `sdd-new`/`sdd-ff`) are `delegate_only`: never
invoke them directly — the orchestrator delegates. Ceremony is quiet by default
(one-line reports; full synthesis plus checkpoint only before irreversible
actions) and memory work is invisible (learnings go into saves, never into
replies). Register new project skills in this file (see `skill-creator`).

Meta-commands `sdd-new`/`sdd-ff` run inline via the orchestrator (see
`internal/assets/prompts/sdd/`); they are not loadable skills.

<!-- biggz-compat: sdd-* delegate_only (orchestrator delegates; new SDD work via sdd-new); quiet ceremony (one-line reports; full synthesis plus checkpoint only before irreversible actions); invisible memory (learnings into saves, never replies); every project skill registered below with trigger and exact path. -->

## Skills

| Skill | Trigger | Path |
|-------|---------|------|
| `agents-md` | New project setup, AGENTS.md index | [`internal/assets/skills/agents-md/SKILL.md`](internal/assets/skills/agents-md/SKILL.md) |
| `branch-pr` | Creating, opening, or preparing PRs | [`internal/assets/skills/branch-pr/SKILL.md`](internal/assets/skills/branch-pr/SKILL.md) |
| `chained-pr` | PRs over 400 lines, stacked PRs, review slices | [`internal/assets/skills/chained-pr/SKILL.md`](internal/assets/skills/chained-pr/SKILL.md) |
| `cognitive-doc-design` | Writing guides, READMEs, RFCs, onboarding, architecture | [`internal/assets/skills/cognitive-doc-design/SKILL.md`](internal/assets/skills/cognitive-doc-design/SKILL.md) |
| `comment-writer` | PR feedback, issue replies, reviews, async updates | [`internal/assets/skills/comment-writer/SKILL.md`](internal/assets/skills/comment-writer/SKILL.md) |
| `go-testing` | Go tests, coverage, golden files | [`internal/assets/skills/go-testing/SKILL.md`](internal/assets/skills/go-testing/SKILL.md) |
| `hermes-ephemeral-delegation` | Broad exploration, multi-step debug via delegation | [`internal/assets/skills/hermes-ephemeral-delegation/SKILL.md`](internal/assets/skills/hermes-ephemeral-delegation/SKILL.md) |
| `issue-creation` | Creating GitHub issues, bug reports, feature requests | [`internal/assets/skills/issue-creation/SKILL.md`](internal/assets/skills/issue-creation/SKILL.md) |
| `issue-root-resolution` | Backlog root audit, resolving issue clusters by root cause | [`internal/assets/skills/issue-root-resolution/SKILL.md`](internal/assets/skills/issue-root-resolution/SKILL.md) |
| `judgment-day` | Blind dual review, adversarial review | [`internal/assets/skills/judgment-day/SKILL.md`](internal/assets/skills/judgment-day/SKILL.md) |
| `rdd-defect-workflow` | RDD receipts, review authority, delivery gates | [`internal/assets/skills/rdd-defect-workflow/SKILL.md`](internal/assets/skills/rdd-defect-workflow/SKILL.md) |
| `sdd-apply` | Orchestrator launches apply (delegate only) | [`internal/assets/skills/sdd-apply/SKILL.md`](internal/assets/skills/sdd-apply/SKILL.md) |
| `sdd-archive` | Orchestrator launches archive (delegate only) | [`internal/assets/skills/sdd-archive/SKILL.md`](internal/assets/skills/sdd-archive/SKILL.md) |
| `sdd-design` | Orchestrator launches design (delegate only) | [`internal/assets/skills/sdd-design/SKILL.md`](internal/assets/skills/sdd-design/SKILL.md) |
| `sdd-explore` | Orchestrator launches exploration (delegate only) | [`internal/assets/skills/sdd-explore/SKILL.md`](internal/assets/skills/sdd-explore/SKILL.md) |
| `sdd-ff` | Meta-command, inline only (see note above) | [`internal/assets/prompts/sdd/sdd-ff.md`](internal/assets/prompts/sdd/sdd-ff.md) |
| `sdd-init` | SDD init, project registry and persistence setup | [`internal/assets/skills/sdd-init/SKILL.md`](internal/assets/skills/sdd-init/SKILL.md) |
| `sdd-new` | Meta-command, inline only (see note above) | [`internal/assets/prompts/sdd/sdd-new.md`](internal/assets/prompts/sdd/sdd-new.md) |
| `sdd-onboard` | Full SDD walkthrough (delegate only) | [`internal/assets/skills/sdd-onboard/SKILL.md`](internal/assets/skills/sdd-onboard/SKILL.md) |
| `sdd-propose` | Orchestrator launches proposal (delegate only) | [`internal/assets/skills/sdd-propose/SKILL.md`](internal/assets/skills/sdd-propose/SKILL.md) |
| `sdd-research` | SDD research lanes (delegate only) | [`internal/assets/skills/sdd-research/SKILL.md`](internal/assets/skills/sdd-research/SKILL.md) |
| `sdd-spec` | Orchestrator launches spec work (delegate only) | [`internal/assets/skills/sdd-spec/SKILL.md`](internal/assets/skills/sdd-spec/SKILL.md) |
| `sdd-sync` | Orchestrator launches sync (delegate only) | [`internal/assets/skills/sdd-sync/SKILL.md`](internal/assets/skills/sdd-sync/SKILL.md) |
| `sdd-tasks` | Orchestrator launches task planning (delegate only) | [`internal/assets/skills/sdd-tasks/SKILL.md`](internal/assets/skills/sdd-tasks/SKILL.md) |
| `sdd-verify` | Orchestrator launches verification (delegate only) | [`internal/assets/skills/sdd-verify/SKILL.md`](internal/assets/skills/sdd-verify/SKILL.md) |
| `skill-creator` | New skills, agent instructions | [`internal/assets/skills/skill-creator/SKILL.md`](internal/assets/skills/skill-creator/SKILL.md) |
| `skill-improver` | Improve, audit, refactor skills | [`internal/assets/skills/skill-improver/SKILL.md`](internal/assets/skills/skill-improver/SKILL.md) |
| `skill-registry` | Update skills, skill registry index | [`internal/assets/skills/skill-registry/SKILL.md`](internal/assets/skills/skill-registry/SKILL.md) |
| `systemic-issue-triage` | Triage, backlog, root cause, blocked users | [`internal/assets/skills/systemic-issue-triage/SKILL.md`](internal/assets/skills/systemic-issue-triage/SKILL.md) |
| `use-modern-go` | Writing or refactoring Go code | [`internal/assets/skills/use-modern-go/SKILL.md`](internal/assets/skills/use-modern-go/SKILL.md) |
| `work-unit-commits` | Splitting work into reviewable commits, chained PRs | [`internal/assets/skills/work-unit-commits/SKILL.md`](internal/assets/skills/work-unit-commits/SKILL.md) |
