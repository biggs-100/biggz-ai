---
name: sdd-init
description: Initialize SDD context, testing capabilities, registry, and persistence for a project.
trigger: sdd init, iniciar sdd, openspec init; also called automatically by the orchestrator when no openspec/ directory exists.
---

# SDD Init

Initialize SDD workspace for the current project. Detects stack, configures testing capabilities, and creates the openspec/ artifact store.

## Activation Contract

1. Detect project language and testing framework.
2. Initialize OpenSpec artifact store.
3. Cache testing capabilities in config.
4. Set up SDD registry.
5. Must be idempotent — safe to re-run without data loss.

## Hard Rules

- Never overwrite an existing `openspec/config.yaml` without explicit user confirmation.
- `biggz sdd-status` must report a valid state before and after init.
- All paths created under `openspec/` must be relative and portable (no absolute paths).
- If the project has no detectable test framework, still initialize — flag as warning, not error.
- The skill registry must be rebuilt from the embedded filesystem, not from a cache.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| Already initialized | `openspec/config.yaml` exists | Show status, skip. Ask before re-init with `--force`. |
| Unknown stack | No `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml` | Default to generic config, flag for user to update manually. |
| No test framework | `go.mod` exists but no `*_test.go` files found | Set `runner: none` with warning in config. |
| Multiple stacks | Both `go.mod` and `package.json` (monorepo) | Detect primary stack from current directory context, note secondary. |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Check current state** — run `biggz sdd-status` to see if already initialized. If yes and config exists, print status output and exit unless `--force` flag is provided.
3. **Detect language stack** — scan project root for markers: `go.mod` → Go, `package.json` → Node.js/TypeScript, `Cargo.toml` → Rust, `pyproject.toml` → Python. Set `language` field in config. If multiple detected, use the most relevant based on directory context.
4. **Detect testing framework** — for Go: check `*_test.go` files exist and probes for `pgregory.net/rapid` (property-based) or standard `testing` package. For Node: check `vitest`, `jest`, `mocha` in devDependencies. For Rust: check `cargo test` works.
5. **Set testing capabilities** — probe for each capability and set boolean in config:
   - `runner`: test framework detected and working
   - `linter`: golangci-lint, eslint, clippy, etc.
   - `type_checker`: TypeScript compiler, mypy, etc.
   - `formatter`: gofmt, prettier, rustfmt, etc.
   - `coverage`: go test -cover, vitest --coverage, etc.
6. **Create directory structure** — create these paths:
   - `openspec/` — root
   - `openspec/changes/` — change directories
   - `openspec/specs/` — domain specification files
   - `openspec/archive/` — archived completed changes
7. **Write config** — create `openspec/config.yaml` with detected capabilities, phase requirements (all true by default), and project context.
8. **Set up skill registry** — walk `internal/assets/skills/` (embedded FS) and build a registry mapping each skill name to its SKILL.md path. Write to Engram for cross-session availability.
9. **Verify** — run `biggz sdd-status` to confirm clean init state. Capture any warnings about missing capabilities.
10. **Persist** — save Engram observation with: detected stack, config path, test runner, warnings list.

## Output Contract

```yaml
status: success | skipped | blocked
executive_summary: "Initialized SDD for Go project at openspec/config.yaml. Testing: go test."
artifacts:
  - path: openspec/config.yaml
    type: config
    summary: "SDD configuration with testing capabilities"
next_recommended: new
risks:
  - description: "Testing capabilities auto-detected — verify with biggz sdd-status"
    severity: low
skill_resolution: auto
```

## References

- `../_shared/sdd-phase-common.md`
- `../../opencode/commands/sdd-init.md`
- `../../opencode/commands/sdd-status.md`
- `openspec/config.yaml`
- `openspec/changes/`
- `openspec/specs/`
