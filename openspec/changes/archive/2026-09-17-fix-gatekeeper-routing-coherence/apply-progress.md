# Apply Progress: fix-gatekeeper-routing-coherence

**Run type:** continuation of the apply phase (verification + deliverable closure). The implementation was landed by the previous run of this same phase; this run verified the landed diff, ran the two focused test commands, and wrote this artifact.

**Mode:** Standard (`openspec/config.yaml` → `strict_tdd: false`). No strict-TDD cycle table applies.

**Evidence revision:** the tracked working-tree diff of the three files below — `internal/sdd/gatekeeper.go`, `internal/sdd/gatekeeper_test.go`, `cmd/biggz/sdd_gatekeeper_cli_test.go` (`366 insertions(+), 58 deletions(-)`). The untracked `openspec/changes/fix-gatekeeper-routing-coherence/` directory is not part of that revision.

**Ledger reference:** attempt token `tok-ba8a322190572c3d5f039326` · work unit `routing-authority-core` · `--max-lines 400` · `--max-attempts 2`. Diffstat is 366 insertions / 58 deletions (424 combined changed lines) — flagged for orchestrator settlement; this run did not touch the ledger.

## Commands run and real output

```console
$ go test ./internal/sdd/ -run 'TestGatekeeper' -count=1
ok  	github.com/biggs-100/biggz-ai/internal/sdd	1.561s
(exit 0; 16 top-level PASS, 0 FAIL under -v)

$ go test ./cmd/biggz/ -run 'TestSddGatekeeperCLI' -count=1
ok  	github.com/biggs-100/biggz-ai/cmd/biggz	0.180s
(exit 0; 6 top-level PASS, 0 FAIL under -v)

$ go test ./internal/sdd/ -run 'TestGatekeeper_RoutingCoherence' -count=1 -v
14 PASS subtests, 0 FAIL

$ go build ./...                       -> exit 0
$ gofmt -l <the 3 changed files>       -> no output (clean), exit 0
$ go vet ./internal/sdd/ ./cmd/biggz/  -> exit 0
```

Full-suite `go test ./... -count=1 -timeout 180s` was NOT run locally: the continuation budget forbids it on Windows (`internal/review` takes ~175–290 s); CI owns the full-matrix run.

## `git diff --stat`

```
 cmd/biggz/sdd_gatekeeper_cli_test.go |  12 +-
 internal/sdd/gatekeeper.go           | 137 ++++++++++++-----
 internal/sdd/gatekeeper_test.go      | 275 ++++++++++++++++++++++++++++++++---
 3 files changed, 366 insertions(+), 58 deletions(-)
```

## Per-task status

| Task | Status | Evidence |
|------|--------|----------|
| 1.1 | Done | Diff hunk `@@ -132,8 +124,8 @@` changes call site to `gr.checkRouting(openspecRoot, changeName, completedPhase, store, result)`. `go build ./...` exit 0; focused tests green. |
| 1.2 | Done | Hunk `@@ -88,18 +92,6 @@` deletes `nextPhaseValid`; `resolveRoutingSuccessor` added: openspec → `readChangeWithForcedStore` (`status.go:443`, which internally calls `deriveChangeStatusWithForcedStoreCtx` at `status.go:459`) and reads `cs.NextRecommended`; hybrid disk-first else `bigMemRoutingSuccessor`; store `""` and unknown store → error. |
| 1.3 | Done | `normalizeRoutingLabel` (trim + `strings.Cut(label, " (")`) and `isRoutingContractPhase` (`slices.Contains(validPhases, phase)`; `validPhases` = explore…archive, research/sync excluded) added. Evidence: matrix cases 4.1–4.3 green. |
| 2.1 | Done | `checkRouting` rewritten: terminal `done`/empty PASS before resolution; mismatch reason `next_recommended %q is not the dependency successor of %q: expected %q`. Cases S1 (spec→tasks) and S2 (spec→design rejected naming `tasks`) green. |
| 2.2 | Done | Fail closed: `Passed=false, Skipped=false`; prefix `cannot resolve the dependency successor for change %q:`; causes covered: store none, unknown store, record absent (names absolute change dir; hybrid adds `no BigMem topics under sdd/...`), derivation error (instance-marker case asserts wrapped `read change-instance marker`). Cases 6–10 green. |
| 2.3 | Done | `research`/`sync` are not in `validPhases` → `Skipped=true` with `phase %q is outside the routing contract; no dependency successor is defined`. Case "phase outside the routing contract skips" (sync) green with that exact reason. |
| 2.4 | Done | `TestGatekeeper_InvalidRouting` declares `NextRecommended: "archive"` from `propose`, asserts routing fails (not passed, not skipped) and the reason names `spec`. Green in focused run. |
| 3.1 | Done | `TestGatekeeper_ApplyCanLoop` fixture moved to canonical `specs/sdd/spec.md`; tasks `- [x] Task 1 / - [ ] Task 2` (below all-done) so the authority resolves `apply`. Green. |
| 3.2 | Done | `TestGatekeeper_VerifyCanRemediate` uses canonical `specs/sdd/spec.md`; declares `NextRecommended: "remediate"`. Green. |
| 3.3 | Done | `TestGatekeeper_StoreAwareArtifactResolution` none-case `wantPass: true → false` while `wantArtifactSkip: true` and reason `artifact store is none` are retained (sibling check still skips). Green. |
| 3.4 | Done | `go test ./internal/sdd/ -run 'TestGatekeeper' -count=1` → `ok ... 1.561s` (exit 0). |
| 4.1 | Done | Cases `done stays terminal (S4)`, `empty stays terminal for routing (S4)`, `normalized progress label matches (S6)` (`apply (3/5 tasks)` → PASS). |
| 4.2 | Done | 3 underivable cases green and never skipped: no record naming store + absolute dir; store `""`; `.biggz-instance` marker wraps `read change-instance marker`. |
| 4.3 | Done | `sync` skip + reason; hybrid BigMem-only resolves `spec` with no filesystem record; zero-artifact `state.yaml` record derives `propose`. All green. |
| 4.4 | Done | `TestGatekeeper_RoutingCoherence`: 14/14 subtests PASS, 0 FAIL. |
| 5.1 | Done | `TestSddGatekeeperCLI_StoreFromPreflight` now expects exit 1 and asserts stdout contains `routing_coherence` + `artifact store is none` while `artifact_existence` still reports `"skipped": true`. `cmd/biggz/cli_sdd.go` untouched (git status lists only the 3 files above). Green. |
| 5.2 | Partial — deferred leg | `gofmt -l` clean, `go vet ./internal/sdd/ ./cmd/biggz/` exit 0, `go build ./...` exit 0, focused suites green. The full `go test ./... -count=1 -timeout 180s` leg was not run locally (continuation budget; CI owns the full matrix). |
| 5.2 (sibling) | Done | `artifact_existence` / `no_drift` behavior untouched — see below. |

## Sibling checks untouched (`artifact_existence` / `no_drift`)

The diff hunks touching `internal/sdd/gatekeeper.go` are exactly four: `@@ -9,15 +9,19 @@` (header comment + imports), `@@ -88,18 +92,6 @@` (`nextPhaseValid` deletion), `@@ -132,8 +124,8 @@` (routing call site), and `@@ -400,8 +392,14 @@` (`checkRouting` rewrite + new helpers). No hunk intersects `checkArtifactExistence` or `checkNoDrift` — their bodies are byte-identical. Behavioral proof: `TestGatekeeper_StoreAwareArtifactResolution` none-case still asserts `wantArtifactSkip: true` with reason `artifact store is none`, and the CLI test asserts `artifact_existence` still emits `"skipped": true`. Both pass in the focused runs.

## Notes

- `tasks.md` checkboxes were intentionally left unchecked: this continuation slice's edit surface is limited to `apply-progress.md`; the orchestrator owns task marks.
- No defects found in the landed code. No blocked tasks.
- Rollback boundaries: WU1 → revert `internal/sdd/gatekeeper.go` plus the Phase 1–3 hunks in `gatekeeper_test.go`; WU2 → delete the `TestGatekeeper_RoutingCoherence` block; WU3 → revert the 12-line `cmd/biggz/sdd_gatekeeper_cli_test.go` hunk.
