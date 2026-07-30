# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| dev (master) | ✅ Active development — issues accepted |
| < dev | ❌ Not yet released |

## Reporting a Vulnerability

**Do not open a public GitHub issue** for security vulnerabilities.

Instead, report privately via email or a [GitHub Security Advisory](https://github.com/biggs-100/biggz-ai/security/advisories).

### What to include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact
- Suggested fix (optional)

### Response timeline

- **48 hours**: Acknowledgment of receipt
- **7 days**: Initial assessment and remediation plan
- **30 days**: Fix release or mitigation guidance

## Scope

The following are in scope:

- The `biggz` and `biggz-mcp` binaries and their source code
- SDD phase prompts and skills deployed to agent config directories
- BigMem SQLite database integrity and access control
- Review ledger and receipt chain integrity

The following are out of scope:

- Third-party AI coding agents (OpenCode, Claude Code, Qwen)
- LLM provider API keys and model access

## Best Practices

- BigMem stores decisions, bugs, and discoveries — do not store secrets, passwords, or API keys in memory entries
- Review receipts are content-addressed and immutable — treat them as audit evidence
- The RDD kill switch (`biggz rdd disable`) stops all review activity; use it to bypass review gates during incident response
