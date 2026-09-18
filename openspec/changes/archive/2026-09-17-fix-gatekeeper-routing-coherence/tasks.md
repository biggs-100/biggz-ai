# Tasks: fix-gatekeeper-routing-coherence

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~300–380 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR; fallback WU1 → WU2+WU3 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| WU1 `routing-authority-core` | D1–D5 rewrite (`internal/sdd/gatekeeper.go`); repairs `TestGatekeeper_ApplyCanLoop`, `TestGatekeeper_VerifyCanRemediate`, `TestGatekeeper_StoreAwareArtifactResolution` | PR 1 | `go test ./internal/sdd/ -run 'TestGatekeeper' -count=1` | N/A (in-process) | Revert `gatekeeper.go` + test hunks |
| WU2 `routing-extended-matrix` | S4–S6 + D2/D3/D4 cases in `gatekeeper_test.go` | PR 1 | `go test ./internal/sdd/ -run 'TestGatekeeper_RoutingCoherence' -count=1` | N/A (unit matrix) | Delete added cases |
| WU3 `cli-preflight-wiring` | `cmd/biggz/sdd_gatekeeper_cli_test.go` exit 1 naming `artifact store is none` | PR 1 | `go test ./cmd/biggz/ -run 'TestSddGatekeeperCLI' -count=1` | No-preflight workspace: `go run ./cmd/biggz sdd-gatekeeper test-change explore --result '<json>'` → exit 1 | Revert CLI test hunk |

## Phase 1 — Plumbing (WU1)

- [x] 1.1 `internal/sdd/gatekeeper.go:136`: call `gr.checkRouting(openspecRoot, changeName, completedPhase, store, result)` (D1); evidence `go build ./...`.
- [x] 1.2 `gatekeeper.go:92-102`: delete `nextPhaseValid`; add `resolveRoutingSuccessor` (D2: openspec → `readChangeWithForcedStore` → `deriveChangeStatusWithForcedStore`; hybrid disk-first else `bigMemRoutingSuccessor`; none/unknown → error).
- [x] 1.3 Add `normalizeRoutingLabel` (trim, strip ` (…)`) + `isRoutingContractPhase` (D5/D3); evidence 4.1–4.3.

## Phase 2 — Rewrite (WU1)

- [x] 2.1 Rewrite `checkRouting` (`gatekeeper.go:404`): normalize → terminal `done`/empty PASS → resolve → compare; mismatch names expected successor (D5); evidence S1/S2.
- [x] 2.2 Fail closed (D4): `Passed=false, Skipped=false`; prefix `cannot resolve the dependency successor for change %q:`; causes: store none, unknown store, record absent (abs path; hybrid + `no BigMem topics`), derivation error.
- [x] 2.3 `research`/`sync` → `Skipped=true` + `phase %q is outside the routing contract; no dependency successor is defined` (D3).
- [x] 2.4 `TestGatekeeper_InvalidRouting`: propose declaring `archive` fails naming `spec` (S3).

## Phase 3 — Test Repairs (WU1)

- [x] 3.1 `TestGatekeeper_ApplyCanLoop`: canonical `specs/sdd/spec.md` fixture; tasks below all-done → resolves `apply`.
- [x] 3.2 `TestGatekeeper_VerifyCanRemediate`: canonical spec fixture; declare `NextRecommended: "remediate"` (`status_helpers2.go:75`).
- [x] 3.3 `TestGatekeeper_StoreAwareArtifactResolution` none-case: `wantPass` true→false; `artifact_existence` still skips.
- [x] 3.4 GREEN: `go test ./internal/sdd/ -run 'TestGatekeeper' -count=1`.

## Phase 4 — Matrix (WU2)

- [x] 4.1 S4 terminal (`done`, `""`) and S6 normalized `apply (3/5 tasks)` → PASS.
- [x] 4.2 S5 underivable ×3: no record → FAIL naming store + abs dir; store `""`; `.biggz-instance` marker → wrapped error; never skipped.
- [x] 4.3 `sync` skip + reason (D3); hybrid BigMem-only (D2); empty record derives `propose` (D4).
- [x] 4.4 GREEN: `go test ./internal/sdd/ -run 'TestGatekeeper_RoutingCoherence' -count=1`.

## Phase 5 — CLI & Suite (WU3)

- [x] 5.1 `TestSddGatekeeperCLI_StoreFromPreflight`: exit 1; stdout names `artifact store is none` in `routing_coherence`; confirm `cli_sdd.go:1341` wiring (unchanged).
- [x] 5.2 `go test ./... -count=1 -timeout 180s`, `gofmt -l .`, `go vet ./...`; `artifact_existence`/`no_drift` untouched. NOTE (orchestrator, 2026-09-17): the literal 180s command times out on this Windows box — `internal/review` needs 175-290s and loses CPU under the parallel full run (panic: test timed out after 3m0s). Re-run with the CI ceiling from PR #94 (`go test ./internal/review/ -count=1 -timeout 600s`) => ok 175.560s; the other 60 packages pass in the same full run. `gofmt -l` clean, `go vet ./...` clean.
