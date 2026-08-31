# Tasks: rdd-auto-enabled-post-verify

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 260 tracked + ~90 test (~350 total) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain stacked-to-main |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | ReviewOffer wiring | PR 1 | `go test ./internal/sdd -run TestReviewOffer` | `biggz sdd-status --json` | Revert `status.go:523` |
| 2 | Hook lineage-aware | PR 1 | `sh .git/hooks/pre-push` | `merge-base --is-ancestor` | Revert hook |
| 3 | Archive guard | PR 1 | `go test ./internal/sdd -run TestArchiveNeverDisable` | `ArchiveChange` mtime | Revert `archive.go:12-40` |
| 4 | Orchestrator doc | PR 1 | `go test ./internal/review -run TestRDDDefault` | `biggz review gate` | Revert doc |
| 5 | Ghost doc | PR 1 | `grep -R "rm.*019fbb3a" internal` | `rm -rf .../019fbb3a-*` | Revert doc |

## Phase 1: Foundation — RDD default ON, ReviewOffer predicate

- [x] 1.1 `internal/review/rdd.go:280-322` default ON no files→enabled. Dep:none Test:`go test ./internal/review -run TestRDDDefault` REQ RDD Default ON
- [x] 1.2 `internal/sdd/status.go:523` ReviewOffer iff `all_done&&verify&&Passing&&enabled` else nil `pathquote.Quote`. Dep:1.1 Test:`go test ./internal/sdd -run TestReviewOffer` REQ SDD ReviewOffer
- [x] 1.3 `internal/sdd/engram_status.go:246,342` mirror 1.2. Dep:1.2 Test:`go test ./internal/sdd -run TestReviewOffer` REQ SDD ReviewOffer
- [x] 1.4 `internal/sdd/status_v2.go:48-53` allowlist `available,invocation`. Dep:1.2,1.3 Test:`biggz sdd-status --json` REQ SDD ReviewOffer
- [x] 1.5 `my change`→`pathquote.Quote`. Dep:1.2 Test:`go test -run TestReviewOfferQuoting` REQ SDD ReviewOffer

## Phase 2: Hook — lineage-aware + space grep

- [x] 2.1 `.git/hooks/pre-push:8-28` `ls -t`+`merge-base --is-ancestor HEAD` fallback `ls -t`. Dep:1.2 Test:`sh .git/hooks/pre-push` REQ SDD Hook
- [x] 2.2 grep `[[:space:]]*` `delivery disabled`+`allowed false`. Dep:2.1 Test:`grep "[[:space:]]*"` REQ SDD Hook
- [x] 2.3 ghost `019fbb3a-*` not ancestor ignored; fallback newest. Dep:2.1 Test:`go test -run TestHookLineage` REQ SDD Hook
- [x] 2.4 no auto-delete `grep -R "rm.*019fbb3a" internal`==0. Dep:2.1 Test:`grep -R` REQ RDD Ghost

## Phase 3: Archive Guard — never disable + mtime

- [x] 3.1 `internal/sdd/archive.go:12-40` only `os.Rename` `grep RDDDisable`==0. Dep:1.1 Test:`go test -run TestArchiveNeverDisable` REQ SDD Archive
- [x] 3.2 mtime T0 preserved + enabled. Dep:3.1 Test:`go test -run TestArchiveMtime` REQ SDD Archive
- [x] 3.3 `internal/install/install.go:505-560` `ensureRDDEnabled` idempotent warns. Dep:1.1 Test:`go test -run TestEnsureRDDEnabled` REQ RDD Install
- [x] 3.4 `allowed:false` blocks else allows when disabled. Dep:1.1 Test:`go test -run TestGateBlocking` REQ RDD Gate

## Phase 4: Integration — e2e hook + status

- [x] 4.1 e2e `biggz sdd-status --json` PASS→`available:true` else nil. Dep:1.2,1.3,3.1 Test:`biggz sdd-status --json` REQ SDD Orchestrator
- [x] 4.2 hook e2e ghost+real ancestor picks ancestor. Dep:2.1,2.3 Test:`sh pre-push` REQ SDD Hook
- [x] 4.3 `auto-chain&&allowed:false&&offer`→exec else print. Dep:4.1 Test:gate sim REQ SDD Orchestrator
- [x] 4.4 `rm -rf .../019fbb3a-*` after `Temp/biggz-smoke` + `go vet`. Dep:2.4,4.2 Test:`go test ./...` REQ RDD Ghost
