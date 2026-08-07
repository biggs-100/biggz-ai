# Manual Testing Guide — biggz-ai v1.0.0

## Prerequisites

```powershell
# Build from source
cd C:\Users\USER\Desktop\biggz-ai
go build -o biggz.exe .\cmd\biggz
go build -o biggz-mcp.exe .\cmd\biggz-mcp

# Or use the installed version
biggz version
```

---

## 1. CLI Smoke Test

```powershell
# Help
biggz --help

# Version
biggz version

# Doctor
biggz doctor

# BigMem stats (should show existing data)
biggz bigmem stats
```

**Expected**: All commands print output without errors.

---

## 2. BigMem Full CLI

```powershell
# Save with engram-compatible flags
biggz bigmem save "test validation" "testing the bigmem CLI" --type decision --project biggz-ai

# Search with flags
biggz bigmem search "test" --type decision --limit 5

# Get by ID (copy ID from save output)
biggz bigmem get obs-XXXXXXXXXXXX

# Update
biggz bigmem get obs-XXXXXXXXXXXX
biggz bigmem update obs-XXXXXXXXXXXX --title "updated title"

# Timeline
biggz bigmem timeline obs-XXXXXXXXXXXX --before 3 --after 3

# Conflict scan
biggz bigmem conflicts scan

# Conflict list
biggz bigmem conflicts list

# Stats
biggz bigmem stats

# Doctor
biggz bigmem doctor

# Export
biggz bigmem export test-export.json

# Import
biggz bigmem import test-export.json

# Projects
biggz bigmem projects list

# Compare (use two IDs from list)
biggz bigmem compare obs-XXXXXXXXXXXX obs-YYYYYYYYYYYY
```

**Expected**: All commands succeed with meaningful output.

---

## 3. BigMem Sync

```powershell
# Export to .bigmem/ (auto-detects project from git root)
biggz bigmem sync

# Check status
biggz bigmem sync --status

# Import back
biggz bigmem sync --import
```

**Expected**: `.bigmem/` directory created in project root with ndjson files.

---

## 4. SDD Workflow

```powershell
# Create a new change via interactive wizard
biggz sdd-new test-sdd-flow

# Check status
biggz sdd-status
```

**Expected**: Wizard creates `openspec/changes/test-sdd-flow/` with proposal.md.

---

## 5. Recovery Trace

```powershell
# Generate a recovery ledger from the sample fixture
biggz recovery generate testdata/recovery/test-backlog.json --name "test-recovery"

# List ledgers
biggz recovery list

# Show ledger
biggz recovery show rec-XXXXXXXXXXXX

# Export
biggz recovery export rec-XXXXXXXXXXXX recovery-test.json

# Validate
biggz recovery validate recovery-test.json
```

**Expected**: Ledger created, listed, shown, exported, and validated.

---

## 6. PR Auto-Generation

```powershell
# Dry run
biggz pr create test-pr --dry-run

# Real run (requires git + gh + uncommitted changes)
echo "# test" >> README.md
biggz pr create test-pr --title "feat: test PR auto-generation" --label "type:chore"
```

**Expected**: Dry run shows PR preview. Real run creates branch, commit, push, and PR.

---

## 7. Hooks System

```powershell
# Initialize hooks
biggz hooks init

# Check file was created
Get-Content .biggz/hooks.yaml

# Run a hook event
biggz hooks run on_review_complete
```

**Expected**: Default hooks file created. Hook runs without error.

---

## 8. Export CLI

```powershell
# Export changelog
biggz export changelog

# Export changelog as JSON
biggz export changelog --format json

# Export review
biggz export review test-id

# Export memory
biggz export memory
```

**Expected**: Exports produce formatted output.

---

## 9. MCP Server

```powershell
# Test MCP initialize handshake
'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | & "C:\Users\USER\.biggz\biggz-mcp.exe" --tools=agent --prefix=biggz

# Test tools/list
@('{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}','{"jsonrpc":"2.0","method":"notifications/initialized"}','{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}') | & "C:\Users\USER\.biggz\biggz-mcp.exe" --tools=agent --prefix=biggz
```

**Expected**: First command returns initialize response with server capabilities.
Second command returns list of 22 tools with `biggz_` prefix.

---

## 10. Install (Reinstall)

```powershell
biggz install
```

**Expected**: Skills deployed to `~/.biggz/skills/` and `~/.config/opencode/skills/`.
Config merged into `~/.config/opencode/opencode.json`.
Binary copied to `~/.biggz/biggz.exe`.
PATH updated.

---

## 11. OpenCode Integration Test

```powershell
# Start OpenCode in the biggz-ai project
cd C:\Users\USER\Desktop\biggz-ai
opencode
```

**Inside OpenCode, verify**:
- [ ] Default agent is `biggz-orchestrator`
- [ ] Skills appear in `<available_skills>` (sdd-init, branch-pr, etc.)
- [ ] MCP tools are available (biggz_mem_save, biggz_mem_search, etc.)
- [ ] SDD commands work (/sdd-status, /sdd-new)
- [ ] BigMem accessible via MCP tools

**Test BigMem from OpenCode**:
```
save a test memory to BigMem with title "test from opencode" and content "this is a test"
```

**Expected**: Memory is saved and retrievable via `biggz bigmem search "test"`.

---

## 12. TUI Navigation

```powershell
# Launch TUI
biggz
```

**Test each screen**:
- [ ] **Dashboard** — shows stats + projects + actions
- [ ] **[I]nstall** — runs install
- [ ] **[C]onfigure** — shows config tabs
- [ ] **[S]tatus** — shows health
- [ ] **[M]emory** — browse BigMem, search, detail, timeline
- [ ] **[B]ackup** — create/list
- [ ] **[P]rofiles** — save/load
- [ ] **[U]pdate** — check for updates
- [ ] **Stric[t] TDD** — toggle mode
- [ ] **Re[v]iew** — review lineage
- [ ] **[R]ecovery** — browse ledgers
- [ ] S[e]ssions — browse BigMem sessions
- [ ] **Mode[l] picker** — select provider
- [ ] **[A]gent builder** — walk through wizard
- [ ] C[o]mmunity — browse plugins/skills
- [ ] **[D]ashboard** — return to dashboard
- [ ] **Help overlay** — press `?`
- [ ] **[Q]uit** — exit

---

## 13. Cross-Platform

```powershell
# Test Docker build
docker build -t biggz-test -f Dockerfile .
docker run --rm biggz-test --help
docker run --rm biggz-test bigmem stats
```

**Expected**: Docker image builds and runs successfully.

---

## 14. Regression

```powershell
# Full test suite
go test ./... -count=1 -timeout 300s

# Benchmarks
go test ./internal/lens/ -bench=. -benchtime=1x

# Fuzz tests (short run)
go test ./cmd/biggz-mcp/ -fuzz=FuzzMCPRequest -fuzztime=5s
```

**Expected**: All tests pass, benchmarks complete, fuzz finds no crashes.

---

## Quick Checklist

- [ ] `go build ./cmd/biggz` succeeds
- [ ] `go test ./...` passes
- [ ] `biggz doctor` shows all green
- [ ] `biggz bigmem save/search/get/delete` work
- [ ] `biggz bigmem sync --status` works
- [ ] `biggz bigmem conflicts scan` works
- [ ] `biggz sdd-new` wizard works
- [ ] `biggz recovery generate/list/show` work
- [ ] `biggz install` succeeds
- [ ] MCP `initialize` handshake works
- [ ] MCP `tools/list` returns 22 tools
- [ ] TUI opens with dashboard
- [ ] All TUI screens render
- [ ] `biggz version` prints version
