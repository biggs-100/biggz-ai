# Tasks: Risk Lens (R1)

## Review Workload Forecast

Decision needed before apply: No
400-line budget risk: Low
Chain strategy: stacked-to-main

## Tasks

- [x] 1.1 Create `internal/lens/risk/types.go` — RiskLevel, RiskSignal, DiffFile, RiskAssessment
- [x] 1.2 Create `internal/lens/risk/lens.go` — RiskLens (ID: "risk", Name: "Risk Assessment")
- [x] 1.3 Implement git diff parsing (`git diff --stat` output → []DiffFile)
- [x] 1.4 Implement file classification (path patterns → RiskSignals)
- [x] 1.5 Implement risk level assignment (signals + changed lines → RiskLevel)
- [x] 1.6 Create `internal/lens/risk/lens_test.go` — mock git output, test classification, test risk levels
- [x] 1.7 Modify `cmd/biggz/main.go` — register RiskLens, keep DummyLens as fallback
- [x] 1.8 Run test suite + vet
