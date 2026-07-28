# biggz-ai Validation Guide

Use this prompt with any AI agent (OpenCode, Claude Code, etc.) to validate
that biggz-ai is fully functional. Run these tests in order.

## Prerequisites

- Go 1.22+ installed
- biggz-ai cloned at C:\Users\USER\Desktop\biggz-ai
- biggz binary built (`go build -o biggz.exe .\cmd\biggz\`)
- biggz-mcp binary built (`go build -o biggz-mcp.exe .\cmd\biggz-mcp\`)

## Test 1: Basic CLI

```bash
# Verify all commands respond
biggz --help

# Test each subcommand responds
biggz sdd-status
biggz sdd-verify-validate --help
biggz sdd-attempt --help
biggz sdd-continue --help
biggz engram --help
biggz backup --help
biggz release --help
biggz skill-registry --help
biggz rdd --help
```

Expected: Each command prints usage/help and exits 0.

## Test 2: Install (dry-run)

```bash
# Test install detects the agent
biggz install --agent opencode --dry-run

# Test with custom home dir (isolated, no real files touched)
biggz install --agent opencode --home C:\temp\biggz-test --dry-run
```

Expected: Detects OpenCode binary, reports skills/config/commands.

## Test 3: Install (real, isolated)

```bash
# Install to a temp directory to verify files are written correctly
biggz install --agent opencode --home C:\temp\biggz-test

# Verify skills were written
dir C:\temp\biggz-test\.config\opencode\skills\
# Should show 35 skill directories

# Verify config was merged
type C:\temp\biggz-test\.config\opencode\opencode.jsonc

# Verify commands were written
dir C:\temp\biggz-test\.config\opencode\commands\

# Verify orchestrator prompt was updated
type C:\temp\biggz-test\.config\opencode\opencode.json
# Check that biggz-orchestrator has a prompt field
```

## Test 4: Review Pipeline (end-to-end)

```bash
# Create a test git repo
mkdir C:\temp\biggz-test-repo
cd C:\temp\biggz-test-repo
git init
echo "package main" > main.go
echo "func main() { println(\"hello\") }" >> main.go
git add -A
git commit -m "init"

# Add a change
echo "package main" > auth.go
echo "func auth() { println(\"auth\") }" >> auth.go
git add -A
git commit -m "add auth"

# Run the review pipeline
echo '{"repository":"C:/temp/biggz-test-repo","commit_sha":"HEAD"}' | biggz

# Expected output: valid JSON with:
# - status: "completed"
# - evidence array with lens results + policy verdict
# - merkle_root: non-empty string
# - schema_version: "1.0"
```

## Test 5: Review with DAG Graph

The pipeline uses a DAG graph to run lenses in parallel. Verify by checking
the output JSON includes evidence from all 4 lenses:

```bash
echo '{"repository":"C:/temp/biggz-test-repo","commit_sha":"HEAD"}' | biggz > output.json
type output.json | findstr "lens_result"
# Should show 4+ lens_result entries (risk, readability, reliability, resilience)
type output.json | findstr "policy_verdict"
# Should show 1 policy_verdict
```

## Test 6: RDD Kill Switch

```bash
# Check default status
biggz rdd status
# Expected: enabled

# Disable
biggz rdd disable

# Verify disabled
biggz rdd status
# Expected: disabled

# Clone-local disable (requires git repo)
cd C:\temp\biggz-test-repo
biggz rdd disable --scope clone

# Global + clone should both show disabled
biggz rdd status
# Expected: disabled (clone overrides)

# Re-enable
biggz rdd enable

# Verify enabled
biggz rdd status
# Expected: enabled
```

## Test 7: Engram Memory (via MCP)

```bash
# Start the MCP server in test mode
echo '{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}' | biggz-mcp
# Expected: JSON with 22 tools

# Test mem_save
echo '{"jsonrpc":"2.0","id":"2","method":"tools/call","params":{"name":"mem_save","arguments":{"title":"Test decision","type":"decision","content":"**What**: Testing save"}}}' | biggz-mcp
# Expected: Saved confirmation

# Test mem_search
echo '{"jsonrpc":"2.0","id":"3","method":"tools/call","params":{"name":"mem_search","arguments":{"query":"test"}}}' | biggz-mcp
# Expected: Results with the saved observation

# Test via CLI
biggz engram save "Test" "decision" "Testing engram"
biggz engram search "Test"
biggz engram get <id-from-search>
```

## Test 8: SDD Workflow

```bash
# Create a test project with openspec
mkdir C:\temp\biggz-sdd-test
cd C:\temp\biggz-sdd-test
mkdir openspec\changes\test-change

# Check what phase is next
biggz sdd-continue test-change
# Expected: proposal (no proposal.md exists)

# Test attempt tracking
biggz sdd-attempt begin test-change --budget 400
biggz sdd-attempt status test-change
# Expected: in_progress

biggz sdd-attempt finish test-change --success --lines 50
biggz sdd-attempt status test-change
# Expected: completed, 1/3 attempts
```

## Test 9: Backup/Restore

```bash
# Create a backup of the project
cd C:\Users\USER\Desktop\biggz-ai
biggz backup create .

# List backups
biggz backup list
# Expected: shows the backup with ID

# Restore to temp dir
biggz backup restore <backup-id> C:\temp\biggz-restore-test
# Expected: files restored, verify with dir C:\temp\biggz-restore-test
```

## Test 10: Skill Registry

```bash
cd C:\Users\USER\Desktop\biggz-ai

# Regenerate skill registry
biggz skill-registry refresh

# Verify the registry
type .atl\skill-registry.md
# Expected: lists skills with relative paths, not C:\Users\...
```

## Test 11: Release

```bash
cd C:\temp\biggz-test-repo

# Check git state
biggz release status
# Expected: branch, commit, clean state

# Tag a version
biggz release tag v0.1.0

# Verify tag
biggz release verify v0.1.0
```

## Test 12: Error Handling

```bash
# Invalid JSON input
echo "not json" | biggz
# Expected: exit 1, stderr error message

# Missing change
biggz sdd-continue nonexistent-change
# Expected: error "change not found"

# Empty backup list
biggz backup list
# Expected: "No backups found." or list of backups
```

## Test 13: OpenCode Agent

```bash
# Verify the biggz-orchestrator agent exists
type C:\Users\USER\.config\opencode\opencode.json | findstr "biggz-orchestrator"
# Expected: agent entry found

# Verify the prompt was updated
type C:\Users\USER\.config\opencode\opencode.json | findstr "Engram"
# Expected: "Engram" found in the prompt

# Launch with biggz-orchestrator
opencode --agent biggz-orchestrator
# Inside OpenCode, try:
#   /sdd-status
#   biggz sdd-status
# Both should work
```

## Success Criteria

All 13 tests must pass. Report any test that fails with:
- Test number and name
- Actual output vs expected
- Error message (if any)
- Go version (`go version`)
- biggz version (`git log --oneline -1`)
