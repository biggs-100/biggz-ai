# Apply Progress — rdd-auto-enabled-post-verify

## Work Units
- WU1 ReviewOffer wiring — done (status.go 523 + engram_status.go 246,342, pathquote.Quote, shortSHA)
- WU2 Hook lineage-aware — done (pre-push ls -t + merge-base --is-ancestor, fallback ls -t, space grep)
- WU3 Archive guard — done (archive.go os.Rename only, // never auto-disable RDD, install ensureRDDEnabled idempotent warning)
- WU4 Integration + Ghost doc — done (e2e ReviewOffer, hook ghost, orchestrator auto-run doc, manual rm -rf after Temp check, go vet)

## Files Changed
| File | Action | What Was Done |
|------|--------|---------------|
| internal/sdd/status.go | Modified | Wire ReviewOffer conditional (all_done && verify done && Passing && RDD enabled) via deriveReviewOffer, pathquote.Quote, shortSHA, detectGitDirs |
| internal/sdd/engram_status.go | Modified | Mirror ReviewOffer via deriveReviewOffer |
| internal/sdd/archive.go | Modified | Add // never auto-disable RDD comment, only os.Rename |
| internal/install/install.go | Modified | ensureRDDEnabled warns when overriding explicit disabled, idempotent |
| .git/hooks/pre-push | Modified | Lineage-aware ls -t + merge-base --is-ancestor HEAD loop, fallback newest, keep [[:space:]]* grep |

## Verification
- go vet ./internal/sdd passed
- biggz sdd-status --json emits ReviewOffer when enabled PASS, nil when disabled/fail
- hook selects ancestor lineage not ghost, fallback newest, space grep tolerant
- archive preserves mtime and never calls RDDDisable (grep ==0)
- no auto-delete ghosts (grep rm 019fbb3a ==0)
