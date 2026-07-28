# Proposal: Test Coverage Improvement

## Intent

Cover the 16 untested spec scenarios from the first SDD cycle. The core implementation (evidence chain, FSM, registry, pipeline rollback, CLI end-to-end) is verified. This change adds missing unit tests for Schema Versioning, PolicyEvaluator, LensPlugin, ProviderPlugin, and CLI error paths.

## Scope

### In Scope
- Unit tests for Schema Versioning (matching version, version mismatch)
- Unit tests for PolicyEvaluator (passing policy, failing policy)
- Unit tests for LensPlugin (happy path, invalid subject)
- Unit tests for ProviderPlugin (happy path, unknown capability)
- Unit test for Orchestrator pipeline failure path (Status → Failed)
- Unit tests for CLI error paths (invalid JSON, pipeline failure, error exit)
- Move DummyLens and MockProvider from `cmd/biggz/` to testable packages
- `go test ./...` must still pass, coverage target: +10%

### Out of Scope
- New features or production code changes
- Integration/E2E tests (unit coverage is the gap)
- Production ProviderPlugin implementations (still mock-only)

## Approach

Extract DummyLens and MockProvider into `plugintest/` package so they're reusable in tests. Write table-driven unit tests for each uncovered scenario. Add orchestrator unit test for failure path. Add CLI integration test via `os/exec` for error cases.

## Success Criteria

- [ ] 16/16 previously untested scenarios now have passing coverage
- [ ] `go test ./...` passes
- [ ] No production code changed (test-only + test helper extraction)
