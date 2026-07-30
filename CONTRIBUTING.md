# Contributing to biggz-ai

## Getting Started

1. Fork the repository.
2. Clone your fork: `git clone https://github.com/your-username/biggz-ai.git`
3. Install Go 1.25+.
4. Run `go build ./cmd/biggz` to verify the build.
5. Run `go test ./...` to verify all tests pass.

## Development Workflow

This project follows **Spec-Driven Development (SDD)**. Every significant change starts with a proposal:

1. **Propose** — Open an issue describing the change, or use `/sdd-new` inside OpenCode.
2. **Spec** — Write delta specs with GIVEN/WHEN/THEN scenarios in `openspec/specs/`.
3. **Design** — Document architecture decisions, data flow, and file changes.
4. **Tasks** — Break the work into verifiable tasks.
5. **Apply** — Implement, commit in work units.
6. **Verify** — Run tests, validate against specs.
7. **Archive** — Move completed change to `openspec/changes/archive/`.

## Commit Conventions

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`.

Scopes: `cli`, `tui`, `mcp`, `bigmem`, `review`, `sdd`, `install`, `docs`.

Examples:
```
feat(cli): add recoverytrace list command
fix(mcp): handle MCP initialize handshake
docs: add CONTRIBUTING.md
```

**Do not add** `Co-Authored-By` or AI attribution lines.

## Work Unit Commits

Each commit must represent a single, reviewable behavior:

- Keep tests with the code they verify.
- Keep docs with the user-visible change they explain.
- A commit should make sense on its own.
- A reviewer should understand *why* each commit exists.

## Pull Requests

- Open PRs against the `master` branch.
- Keep PRs under 400 changed lines. Split large changes into chained PRs.
- Each PR must include:
  - A clear description of what and why.
  - Test evidence (`go test ./...` output).
  - If applicable, documentation updates.
- Use the PR template: explain the problem, the solution, and verification steps.

## Code Style

- Follow `go fmt` and `go vet` — they must pass.
- Use meaningful names. Avoid abbreviations.
- Handle errors. Do not use `_` to discard errors unless the operation is genuinely best-effort.
- Package comments: every package must have a `// Package ...` doc comment.

## Testing

- Run `go test ./...` before every commit.
- All tests must pass. No `t.Skip()` without a linked issue.
- For new features, add tests alongside the implementation.
- Property-based testing with `pgregory.net/rapid` is encouraged for complex logic.

## Questions?

Open a [GitHub Discussion](https://github.com/biggs-100/biggz-ai/discussions) or an issue.
