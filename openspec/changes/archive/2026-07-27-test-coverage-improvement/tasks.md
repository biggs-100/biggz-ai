# Tasks: Test Coverage Improvement

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~250 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

## Phase 1: Test Helpers

- [x] 1.1 Create `plugintest/lens.go` — extract DummyLens from `cmd/biggz/dummylens.go` into reusable package
- [x] 1.2 Create `plugintest/provider.go` — extract MockProvider from `cmd/biggz/mockprovider.go` into reusable package

## Phase 2: Unit Tests

- [x] 2.1 Write `plugintest/lens_test.go` — LensPlugin happy path + invalid subject + Policies
- [x] 2.2 Write `plugintest/provider_test.go` — ProviderPlugin happy path + unknown capability + Usage fields
- [x] 2.3 Write `policy/evaluator_test.go` — PolicyEvaluator passing + failing scenarios
- [x] 2.4 Write `model/review_test.go` — Schema Versioning matching + mismatch + NewReviewState
- [x] 2.5 Write `orchestrator/orchestrator_test.go` — pipeline failure path → Status Failed

## Phase 3: CLI Tests

- [x] 3.1 Write `cmd/biggz/main_test.go` — invalid JSON input (exit 1, stderr) + valid JSON (exit 0)

## Phase 4: Verify

- [x] 4.1 Run full test suite — 30/30 tests pass (up from 22), `go vet ./...` clean
- [x] 4.2 Update verify-report with new compliance matrix
