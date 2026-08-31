# Tasks: sdd-parity-rescope-grant-ledger

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 255 tracked + ~180 test = 435 |
| 400-line budget risk | Low |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | G1 guard 15 | PR1 | `go test ./internal/sdd -run TestLegacyGuard -count=1` | `sdd-status --json` | `attempt.go` |
| 2 | G2 55+G3 25 | PR1 | `go test ./internal/sddattempt -run TestRescope -count=1` | `sdd-attempt rescope` | `sddattempt.go`+`cas_store.go` |
| 3 | G4 135+G7 20 | PR1 | `go test ./internal/sdd -run TestTopology -count=1` | `sdd-status --json --instructions` | `edit_authority.go`+`status.go` |
| 4 | G5 55 | PR1 | `go test ./internal/review -run TestPassive -count=1` | `ClassifyRisk` | `risk.go` |
| 5 | Docs 0 | PR1 | `go test ./internal/sdd -run TestHybrid -count=1` | `sdd-status --json` | fixtures |

## Phase 1: Foundation

- [x] 1.1 RED: Begin/Finish/Reset → ErrLegacyRetired (REQ-G1-01, `attempt.go:12`)
- [x] 1.2 Guard ErrLegacyRetired in `attempt.go` 15 (REQ-G1-01)
- [x] 1.3 RED: ForInstance rejects empty/129/multiline (REQ-G3-01, `cas_store.go:86`)
- [x] 1.4 ForInstance 1..128 trimmed + grantedRootsFor (REQ-G3-01, `cas_store.go:86` 25)
- [x] 1.5 Ratify HybridResearchEqual (REQ-G6, `research.go:39` 0)

## Phase 2: Rescope & Marker

- [x] 2.1 RED: block Active/DecReq/Complete/zero → NotAllowed (REQ-G2-01, `sddattempt.go:1992`)
- [x] 2.2 RED: Widened/Exhausted/ok cum preserved (REQ-G2-01/02, `sddattempt.go:1992`)
- [x] 2.3 Predicate + narrow before wedge + carry Cumulative* (REQ-G2-01/02, `sddattempt.go:1992` 55)
- [x] 2.4 RED: `api.md (read-only)` exempt vs `main.go` blocked (REQ-G7-01, `edit_authority.go:19`)
- [x] 2.5 Regex `(?i)^\s*\(read-only\)` filter both detectors (REQ-G7-01, `edit_authority.go` 20)

## Phase 3: Topology & Passive

- [x] 3.1 RED: foreign `../foreign-clone/file.go` blocks apply not spec (REQ-G4-01)
- [x] 3.2 Threat RED: injection a/b err; symlink EvalSymlinks; memo 3→1 (G4)
- [x] 3.3 Implement resolve→gitRootOf→SameFile memo, block apply/verify/remediate (REQ-G4-01, `edit_authority.go` 105+`status.go:473` 30)
- [x] 3.4 RED: 9MiB/NUL/!utf8/shebang/MDX/exec → not passive, plain md Low (REQ-G5-01, `risk.go:165`)
- [x] 3.5 isPassiveContentFile ≤8MiB gated by isPassiveDocumentExtension (REQ-G5-01, `risk.go:165` 55)

## Phase 4: Fixtures & Gates

- [x] 4.1 Fixtures with-shebang.md/comp.mdx/note.md/large.md 9MiB/symlink (REQ-G5-01)
- [x] 4.2 Integration git init siblings + sdd-status (REQ-G4-01)
- [x] 4.3 Gates go vet + go test ./... + sdd-status --json --instructions (all REQs)

## Dependencies

1.1→1.2→1.3→1.4→2.4→2.5→3.1→3.2→3.3; 2.1→2.2→2.3 before 3.3; 3.4→3.5→4.1→4.2→4.3; Phase1→2→3→4

## Test Evidence

| WU | Test | Harness | Rollback |
|----|------|---------|----------|
|1|TestLegacyGuard| sdd-status| attempt.go|
|2|TestRescope+TestForInstance| sdd-attempt| sddattempt.go|
|3|TestTopology+TestReadOnlyMarker| sdd-status --instructions| edit_authority.go|
|4|TestPassive| ClassifyRisk| risk.go|
|5|TestHybrid| sdd-status| fixtures|

## Verification

`go vet ./...` `go test ./... -count=1 -timeout 180s` `biggz sdd-status --json --instructions`

## Traceability
| REQ | Tasks |
|-----|-------|
| G1-01 |1.1,1.2|
| G2-01 |2.1,2.2,2.3|
| G2-02 |2.2,2.3|
| G3-01 |1.3,1.4|
| G4-01 |3.1,3.2,3.3,4.2|
| G6 |1.5|
| G7-01 |2.4,2.5|
| G5-01 |3.4,3.5,4.1|

18 tasks
