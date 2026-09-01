# biggz-ai Validation Guide

Use this prompt with any AI agent (OpenCode, Claude Code, etc.) to validate
that biggz-ai is fully functional. Run these tests in order.

## Prerequisites

- Go 1.25+ installed (`go version` — module requires `go 1.25` per `go.mod`)
- biggz-ai cloned at C:\Users\USER\Desktop\biggz-ai
- biggz binary built (`go build -o biggz.exe .\cmd\biggz\`)
- biggz-mcp binary built (`go build -o biggz-mcp.exe .\cmd\biggz-mcp\`)

## Test 1: Basic CLI (smoke subset + full dispatched verbs)

```bash
# Verify all commands respond
biggz --help

# Test each subcommand responds (core smoke subset)
biggz sdd-status
biggz sdd-verify-validate --help
biggz sdd-attempt --help
biggz sdd-continue --help
biggz bigmem --help
biggz backup --help
biggz release --help
biggz skill-registry --help
biggz rdd --help

# Full dispatched verbs from cmd/biggz/main.go (all should respond --help)
# sdd-apply, sdd-new, sdd-profile, sdd-remediate, tdd, plugin, mcp, pr, codegraph, hooks, export, recovery, review, doctor, update, upgrade, sync, help
biggz sdd-apply --help
biggz sdd-new --help
biggz sdd-profile --help
biggz sdd-remediate --help
biggz tdd --help
biggz plugin --help
biggz mcp --help
biggz pr --help
biggz codegraph --help
biggz hooks --help
biggz export --help
biggz recovery --help
biggz review --help
biggz doctor --help
biggz update --help
biggz upgrade --help
biggz sync --help
biggz help --help
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
# Should show ~35 skill directories (count varies — run `dir` and verify >30)

# Verify config was merged
type C:\temp\biggz-test\.config\opencode\opencode.jsonc

# Verify commands were written
dir C:\temp\biggz-test\.config\opencode\commands\

# Verify orchestrator prompt was updated
type C:\temp\biggz-test\.config\opencode\opencode.json
# Check that biggz-orchestrator has a prompt field
```

## Test 4: Review Pipeline (end-to-end) — content-addressed

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
mkdir -p subject && echo '{"repository":"C:/temp/biggz-test-repo","commit_sha":"HEAD"}' > subject.json
git add -A

# Content-addressed review flow (no stdin pipe)
biggz review start --subject subject.json --lineage test-lineage-001
# Expected: genesis appended under .git/biggz/review-transactions/test-lineage-001/v1/events/<sha256>, HEAD set
biggz review capture-result --lineage test-lineage-001 --target <subject-id> --lens risk --order 0 --expected-revision <genesis-sha> --input reviewer-risk.json
biggz review capture-result --lineage test-lineage-001 --target <subject-id> --lens readability --order 1 --expected-revision <prev-sha> --input reviewer-readability.json
biggz review capture-result --lineage test-lineage-001 --target <subject-id> --lens reliability --order 2 --expected-revision <prev-sha> --input reviewer-reliability.json
biggz review capture-result --lineage test-lineage-001 --target <subject-id> --lens resilience --order 3 --expected-revision <prev-sha> --input reviewer-resilience.json
biggz review finalize test-lineage-001
# Expected: receipts/<sha256>.json via publishImmutable (or burned.json when BurnEnabled), complete_review appended
biggz review gate pre-pr test-lineage-001 --json
# Expected output: valid JSON with:
# - verdict: pass/fail, chainValid: true
# - evidenceHash via domainHash("biggz-ai.review-evidence/v1\\x00"+writeLengthPrefixed(...)), MerkleRoot via domainHash("biggz-ai.review-merkle/v1")
# - receipt binding validated, DeliveryBurned when BurnEnabled
```

## Test 5: Review with per-slot capture (content-addressed, no pipe)

The review captures each lens slot independently (replaces legacy stdin | biggz DAG). Verify every slot binds the frozen manifest:

```bash
biggz review capture-result --lineage test-lineage-001 --target <subject-id> --lens risk --order 0 --expected-revision <genesis-sha> --input reviewer-risk.json
biggz review capture-result --lineage test-lineage-001 --target <subject-id> --lens reliability --order 2 --expected-revision <prev> --input reviewer-reliability.json
# Each capture validates subjectHash echo + full-manifest inspection + canonical findings
biggz review finalize test-lineage-001 --json > finalize.json
type finalize.json | findstr "receipt_hash"
# Should show receipt_hash = domainHash("biggz-ai.review-receipt-binding/v1") + FixDeltaHashForSnapshot
biggz review gate pre-pr test-lineage-001 --json > gate.json
type gate.json | findstr "chainValid"
# Should show chainValid:true, receiptValid:true (or DeliveryBurned when BurnEnabled)
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

## Test 7: bigmem Memory (via MCP)

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
biggz bigmem save "Test" "Testing bigmem" --type decision
biggz bigmem search "Test"
biggz bigmem get <id-from-search>

# Verify SQLite store directly (optional)
# dir %USERPROFILE%\.biggz\bigmem\bigmem.db
# Expected: SQLite database file exists
```

## Test 8: Full SDD Project (Real Go Project)

Create a real Go project from scratch, go through the complete SDD cycle,
run the review pipeline, and archive the change.

```bash
# ===== 8.1 Create project from scratch =====
mkdir C:\temp\biggz-full-test
cd C:\temp\biggz-full-test

# Init Go module
go mod init example.com/fulldemo

# Init git
git init
git add -A
git commit -m "chore: initial empty project"

# Init SDD (creates openspec/config.yaml via biggz install or manual)
mkdir openspec\changes\first-feature
mkdir openspec\specs
mkdir openspec\changes\archive

# ===== 8.2 Create a proposal =====
# Write a minimal proposal
@"
# Proposal: First Feature

## Intent
Add a simple HTTP server with health endpoint.

## Scope
### In Scope
- HTTP server on port 8080
- GET /health endpoint

### Out of Scope
- Database, auth, tests

## Approach
Use net/http standard library.

## Success Criteria
- Server starts and responds to /health with 200
"@ | Out-File -FilePath openspec\changes\first-feature\proposal.md -Encoding utf8

# Verify SDD recognizes the proposal
biggz sdd-continue first-feature
# Expected: spec (proposal exists, needs spec)

# ===== 8.3 Write spec =====
@"
# HTTP Server Specification

## Requirements

### Requirement: Health Endpoint
The system MUST expose GET /health returning 200 with JSON body {"status":"ok"}.

#### Scenario: Health check returns ok
- GIVEN the server is running
- WHEN a GET request is sent to /health
- THEN the response MUST be 200
- AND the body MUST be {"status":"ok"}
"@ | Out-File -FilePath openspec\specs\http-server\spec.md -Encoding utf8

# ===== 8.4 Write design =====
@"
# Design: HTTP Server

## Architecture
Single-file main.go using net/http.

## Decisions
- Standard library only (no frameworks)
- Port 8080 hardcoded for MVP

## File Changes
| File | Action | Description |
|------|--------|-------------|
| main.go | Create | HTTP server with /health handler |
"@ | Out-File -FilePath openspec\changes\first-feature\design.md -Encoding utf8

# ===== 8.5 Write tasks =====
@"
# Tasks: First Feature

## Phase 1: Implementation
- [ ] 1.1 Create main.go with HTTP server and /health handler

## Phase 2: Verify
- [ ] 2.1 Start server and test /health endpoint
"@ | Out-File -FilePath openspec\changes\first-feature\tasks.md -Encoding utf8

# ===== 8.6 Implement =====
@"
package main

import (
    "encoding/json"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
"@ | Out-File -FilePath main.go -Encoding utf8

# Build and verify
go build -o server.exe .
if ($LASTEXITCODE -eq 0) { echo "BUILD OK" } else { echo "BUILD FAILED" }

# Mark tasks as done — acquire/settle (acquire/settle replace begin/finish)
# acquire mints a token that settle must present to close that exact attempt
$acquireOut = biggz sdd-attempt acquire --cwd . --change first-feature --request-id req-001 --work-unit impl --evidence-goal "impl http server" --max-attempts 1 --max-lines 400 | ConvertFrom-Json
$token = $acquireOut.token
# Write apply-progress
@"
# Apply Progress

## Completed
- [x] 1.1 main.go created, builds successfully
"@ | Out-File -FilePath openspec\changes\first-feature\apply-progress.md -Encoding utf8

$evidenceSha = (git rev-parse HEAD^{tree}).Trim()
biggz sdd-attempt settle --cwd . --change first-feature --token $token --request-id req-002 --outcome passed --evidence-revision "sha256:$evidenceSha" --diagnosis "http server ok" --harness-disposition completed --cleanup-evidence "none" --process-evidence "go build ok"
# Note: settle requires --token, --request-id, --outcome, --diagnosis, --harness-disposition, --cleanup-evidence, --process-evidence; --evidence-revision is the candidate tree sha256:<64 hex>

# ===== 8.7 Run review pipeline =====
git add -A
git commit -m "feat: add HTTP server with health endpoint"

# Run the full content-addressed review on this commit (replaces echo ... | biggz)
mkdir subject -Force | Out-Null; '{"repository":"C:/temp/biggz-full-test","commit_sha":"HEAD"}' | Out-File -FilePath subject.json -Encoding utf8
biggz review start --subject subject.json --lineage fulltest-001
biggz review capture-result --lineage fulltest-001 --target <subject-id> --lens risk --order 0 --expected-revision <genesis-sha> --input reviewer-risk.json
biggz review capture-result --lineage fulltest-001 --target <subject-id> --lens reliability --order 1 --expected-revision <prev> --input reviewer-reliability.json
biggz review finalize fulltest-001
biggz review gate pre-pr fulltest-001 --json
# Expected: JSON gate output with chainValid:true, receipt binding via domainHash+FixDelta, DeliveryBurned handling

# ===== 8.8 Create verify report =====
@"
yaml
schema: biggz-ai.verify-result/v1
verdict: pass
blockers: 0
critical_findings: 0
requirements: 1/1
scenarios: 1/1
test_command: go build ./...
test_exit_code: 0
build_command: go vet ./...
build_exit_code: 0

## Verification Report

**CRITICAL**: None
"@ | Out-File -FilePath openspec\changes\first-feature\verify-report.md -Encoding utf8

# ===== 8.9 Archive =====
Move-Item openspec\changes\first-feature openspec\changes\archive\2026-07-28-first-feature -Force

# Verify archive
biggz sdd-status
# Expected: No active changes, archived change visible
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

# Verify the registry — location depends on artifact_store:
# - artifact_store: biggz → registry lives at ~/.biggz/skills/ (user skill dir)
# - otherwise (openspec/hybrid) → fallback .atl/skill-registry.md (see internal/skillregistry)
# Check the appropriate location:
type .atl\skill-registry.md
# Also check user dir when using biggz store:
# dir %USERPROFILE%\.biggz\skills\
# Expected: lists skills with relative paths, not C:\Users\... (registry auto-generated by `biggz skill-registry refresh`)
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
# Invalid review subject (TUI guard: bare `biggz` without subcommand goes to TUI, not JSON validator)
# Use invalid-input cases that hit the subcommand validators:
biggz review start --subject bad.json
# Expected: exit 1, stderr "reading subject file" or "parsing subject JSON"
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
type C:\Users\USER\.config\opencode\opencode.json | findstr "bigmem"
# Expected: "bigmem" found in the prompt

# Launch with biggz-orchestrator
opencode --agent biggz-orchestrator
# Inside OpenCode, try:
#   /sdd-status
#   biggz sdd-status
# Both should work
```

## Test 15: Contracts Formalization Layer

The wire-envelope formalization layer is validated by its own walk test plus
the emitted-payload conformance tests:

```bash
go test ./internal/contracts/ -v
```

Expected:
- `TestContractsEverySchemaCompilesWithDeclaredID` — all 23 embedded
  schemas compile with their declared `$id` after AddEmbedded.
- `TestContractsEveryFixtureValidatesAgainstSameNameSchema` — all 23
  positive fixtures validate 1:1 against their same-name schema.
- `TestContractsNegativeConformance` — mutated fixtures (wrong schema const,
  missing required, extra key, bad sha256, wrong enum, collect+execute
  together) are all rejected.
- `TestEnvelopeConformance_*` — real engine output validates: contract
  envelope (collect + stop), consent envelope, captured artifact, persisted
  receipt, every chain event payload + record, refutation round trip,
  verification-retry report, inspect report, SDD edit-authority consent
  envelope, and verify admission (admitted + denied).

Ledger additivity (the layer must never change a ledger byte):

```bash
go test ./internal/review/ -run TestLedgerRegression -v
```

Expected: a frozen pre-layer chain (`testdata/ledger-chain`) loads with
intact content addresses, `IntegrityVerdict` valid, the receipt artifact
re-read and `PersistedReceipt.Validate()` passing, and every frozen event
payload conforming to its contract schema.

## Layer 3 Gates (CI parity)

Before marking any SDD/review change green, run the three synthesis/review gate layers locally:

```bash
go vet ./...
go test ./... -count=1 -timeout 180s
node --test internal/assets/pi/biggz-synthesis-gate.test.mjs
```

Expected: `go vet` clean, `go test` green (including `TestLedgerRegression`, `TestContracts*`, `TestSynthesis*`), and `node --test` green (checkpoint-gated blocking, thin advise, child bypass, same-turn race, preflight allowance). CI enforces the same three commands plus `node --check internal/assets/pi/biggz-synthesis-gate.js`.

## Success Criteria

All 15 tests plus the Layer 3 gates must pass. Report any test that fails with:
- Test number and name
- Actual output vs expected
- Error message (if any)
- Go version (`go version`)
- biggz version (`git log --oneline -1`)
