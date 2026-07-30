# Support

## Documentation

- [README](README.md) — Quick start and overview
- [Architecture](docs/architecture.md) — Package map and data flow
- [Validation Guide](docs/validation-guide.md) — Manual test scenarios
- [Comparison with gentle-ai](docs/comparison-with-gentle.md) — Feature comparison

## CLI Help

```bash
biggz --help          # Top-level help
biggz <cmd> --help    # Per-command help
biggz                 # Launch interactive TUI
```

## Getting Help

- **GitHub Issues**: [github.com/biggs-100/biggz-ai/issues](https://github.com/biggs-100/biggz-ai/issues) — bug reports, feature requests
- **GitHub Discussions**: [github.com/biggs-100/biggz-ai/discussions](https://github.com/biggs-100/biggz-ai/discussions) — questions, ideas

## Troubleshooting

### MCP server fails to connect

Run `biggz doctor` to check system health. Ensure `biggz-mcp.exe` is at `~/.biggz/biggz-mcp.exe`.

### Skills not loading in OpenCode

Run `biggz install` to redeploy skills. Verify `~/.config/opencode/skills/` contains SKILL.md files.

### BigMem database issues

```bash
biggz bigmem doctor    # Check database integrity
biggz bigmem stats     # Show observation/session counts
```

### Report a bug

Open an issue with:
- `biggz doctor --json` output
- Steps to reproduce
- Expected vs actual behavior
