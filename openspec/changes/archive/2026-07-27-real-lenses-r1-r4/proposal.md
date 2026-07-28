# Proposal: Real Lenses — Risk Lens (R1)

## Intent

Replace DummyLens with a real RiskLens that analyzes git diffs and produces structured findings. First step toward a real code review system.

## Scope

### In Scope
- `internal/lens/risk/lens.go` — implements LensPlugin, parses git diff stat, classifies files, assigns risk level
- `internal/lens/risk/lens_test.go` — unit tests with mock git output
- `internal/lens/risk/types.go` — RiskLevel, RiskReason, RiskAssessment types
- `cmd/biggz/main.go` — register RiskLens, remove DummyLens dependency
- Pipeline produces richer evidence with real findings

### Out of Scope
- Readability/Reliability/Resilience lenses (need AI)
- Git subprocess management (use exec.LookPath + exec.Command)
- Multi-commit analysis

## Capabilities

### New
- `risk-lens`: analyze git diffs, classify files, assign risk level

### Modified
- `core-review`: pipeline now uses real lens instead of DummyLens

## Success Criteria
- [ ] RiskLens classifies files by risk signals (auth, shell, security, executable)
- [ ] RiskLens returns findings with severity based on risk level
- [ ] All tests pass with mock git output
- [ ] `go run ./cmd/biggz` produces evidence with real risk findings
