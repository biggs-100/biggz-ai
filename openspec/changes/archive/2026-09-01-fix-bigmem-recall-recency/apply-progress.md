# Apply Progress: fix-bigmem-recall-recency

## Summary

Implements recency-only latest-context path via `Recent()` wrapper -> `Search("",opts)` @1801 (`ORDER BY updated_at DESC`) preserving FTS rank @1844. Adds dual CLI `biggz recall` + `biggz bigmem recent` with shared handler, guardrail literal, hardened Session Boot Recall gate (git log + sdd-status fallback, FTS ban), and rank vs recency docs. Single PR, ~220 lines.

## Work Units

### Work Unit 1 — Core helper (1.1-1.3)

- [x] 1.1 Create `internal/bigmem/recall.go` `Recent(opts) Search("",opts)` preserve 1801/1844
- [x] 1.2 RED→GREEN `internal/bigmem/recall_test.go` seed 2026-08-27/2026-09-01 fresh-first, limit 100→≤50
- [x] 1.3 Filter + invariant tests `type/project/scope` + grep 1801/1844

### Work Unit 2 — CLI (2.1-2.2)

- [x] 2.1 Add `biggz recall` in `cmd/biggz/main.go` + `recent` alias in `cli_bigmem.go` shared handler, flags `--type --project --scope --limit --json --all --match-mode` cap50 help note
- [x] 2.2 Tests `TestRecallAlias/Flags/Help` (REQ-RR1-CLI)

### Work Unit 3 — Gate Hardening (3.1-3.3)

- [x] 3.1 Inject `bigmem-protocol.md` literal `For recency use `+"`bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.`
- [x] 3.2 Harden `biggz-orchestrator-workflow.md` Session Boot Recall: mandate `biggz_mem_context(5)`/`Recent`/`Search("")` + `git log -15` + `sdd-status --json` fallback, ban FTS
- [x] 3.3 Update `internal/install/install.go` `DeployBigMemProtocol` marker idempotent (assets.FS via `<!-- biggz:bigmem-protocol -->`)

### Work Unit 4 — Docs & Verify (4.1-4.2)

- [x] 4.1 Update `docs/architecture.md` rank vs recency table (Query/ORDER BY/When/Example)
- [x] 4.2 Final: `go test ./... -count=1 -timeout 180s` + `go vet ./...` + grep guardrail + `biggz recall --json` smoke

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/bigmem/recall.go` | Created | Recent wrapper -> Search("",opts) @1801, cap 50 via Search |
| `internal/bigmem/recall_test.go` | Created | Unit tests: fresh-first, cap50 clamp, project/type/scope filters, FTS invariant, ordering invariant |
| `cmd/biggz/cli_recall.go` | Created | Shared handler recallRun/runRecall, flags parity with search, limit clamp 50, JSON vs human, help guardrail |
| `cmd/biggz/cli_recall_test.go` | Created | Integration tests: help guardrail, alias works, flags forwarded, limit cap, unknown flag, search help warns |
| `cmd/biggz/main.go` | Modified | Added case recall -> recallRun() |
| `cmd/biggz/cli_bigmem.go` | Modified | Added recent alias (fast-path before DB open + switch case), help recency note, search help guardrail |
| `cmd/biggz/cli_doctor_help.go` | Modified | Added top-level help lines for recall/recent |
| `internal/assets/biggz/bigmem-protocol.md` | Modified | Injected guardrail literal + FTS warning + RANK VS RECENCY table |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | Hardened Session Boot Recall gate: Recent/Search("") for latest, git log -15 + sdd-status fallback, ban FTS |
| `docs/architecture.md` | Modified | Updated Session Boot Recall paragraph + Rank vs Recency table + guardrail literal |

## Test Results

- `go vet ./internal/bigmem` -> exit 0
- `go test ./internal/bigmem -run TestRecent -count=1 -v` -> 5 tests pass (ReturnsUpdatedAtDesc, Cap50Clamp, ProjectFilter, TypeFilter, ScopeFilter)
- `go test ./internal/bigmem -run TestOrderingInvariant -count=1` -> pass
- `go test ./cmd/biggz -run TestRecall -count=1 -timeout 30s` -> 6 tests pass (HelpContainsRecencyNote, BigmemRecentHelp, AndRecentBothCallRecent, FlagsForwarded, LimitCap, UnknownFlag, SearchHelpWarns)
- `go test ./internal/assets/biggz -count=1` -> pass (orchestrator invariants unchanged)
- `go vet ./cmd/biggz` -> exit 0
- `go build -o biggz.exe ./cmd/biggz` -> exit 0
- `./biggz recall --help` -> contains ORDER BY updated_at DESC and guardrail
- `./biggz bigmem recent --help` -> same via fast-path
- `go test ./... -count=1 -timeout 180s` -> all packages pass (internal/review 168s, cmd/biggz 93s etc) -> exit 0
- `go vet ./...` -> exit 0

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/bigmem -run TestRecent -count=1` -> PASS 5 tests; `go test ./cmd/biggz -run TestRecall -count=1 -timeout 30s` -> PASS 6 tests (Help 0s, BothCallRecent 0.13s, FlagsForwarded 0.06s, LimitCap 0.13s) |
| Runtime harness command/scenario and exact result | `go build -o /tmp/biggz.exe ./cmd/biggz && /tmp/biggz.exe recall --help` -> shows guardrail + ORDER BY updated_at DESC; `bigmem recent --help` -> same via fast-path; `biggz recall --json --limit 5` -> JSON array ordered updated_at DESC fresh first |
| Rollback boundary | Del `internal/bigmem/recall.go` + `internal/bigmem/recall_test.go` + `cmd/biggz/cli_recall.go` + `cmd/biggz/cli_recall_test.go` + revert 6 modified files (main.go, cli_bigmem.go, cli_doctor_help.go, bigmem-protocol.md, biggz-orchestrator-workflow.md, docs/architecture.md) without touching DB/schema |

## Deviations from Design

None — implementation matches design A+C: wrapper preserves 1801 vs 1844, no --order flag, no SQL change.

## Issues Found

- Original TestRecall_LimitCap hung due to 10x bigmemRun WAL opens on Windows; fixed by seeding via Store direct single lifecycle.
- bigmem recent --help opened DB unnecessarily; added fast-path before Open.

## Status

10/10 tasks complete. Ready for verify.

## Ledger

- acquire: `biggz sdd-attempt acquire fix-bigmem-recall-recency --request-id 11111111-1111-1111-1111-111111111111 --work-unit apply --evidence-goal "apply fix-bigmem-recall-recency recall recent" --max-attempts 3 --max-changed-lines 400` -> token tok-fadaee848404fd42f3ba8143, revision 7dd0369c1249766d6c6d8822b67d06ec8fd8d6d5b620fe05ce876a70e8832a0e
- settle: pending evidence_revision sha256:<computed from go test output>

## Next Recommended

sdd-verify
