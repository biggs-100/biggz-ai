# Exploration: Real Lenses R1-R4

## Current State

biggz-ai has a `LensPlugin` interface in `plugin/interfaces.go` and a `DummyLens` in `plugintest/lens.go` that returns a static finding. No real lens implementations exist.

## Reference: gentle-ai's lens system

gentle-ai's lenses (risk.go, ~620 lines) are deeply coupled to the review transaction system:
- **Risk lens (R1)**: Analyzes git diff stats (paths, additions, deletions, modes) to detect auth tokens, shell scripts, executable changes, security paths. Assigns risk level (low/medium/high) and produces structured reasons.
- **Readability lens (R2)**: Reviews naming, complexity, intention, maintainability, code style.
- **Reliability lens (R3)**: Reviews test coverage, edge cases, determinism, contracts, error handling.
- **Resilience lens (R4)**: Reviews fallbacks, retry/backoff, graceful degradation, observability.

Problem: In gentle-ai, lenses are hardcoded constants in the type system with dedicated counter fields and prefix mapping. biggz-ai avoids this via the LensPlugin interface.

## Proposed Design

Each lens is a standalone Go package implementing `plugin.LensPlugin`:

```
internal/lens/
├── risk/lens.go        → RiskLens: analyzes git diff → risk assessment
├── readability/lens.go → ReadabilityLens: reviews code quality
├── reliability/lens.go → ReliabilityLens: reviews test coverage
└── resilience/lens.go  → ResilienceLens: reviews error handling
```

### Risk Lens (first implementation)

Input: ReviewSubject with repository + commit SHA
Algorithm:
1. Get diff stats via `git diff --stat` (exec.LookPath + exec.Command)
2. Classify each file: binary? generated? shell script? config? security-sensitive?
3. Detect signals: auth tokens, service credentials, executable mode changes
4. Assign risk level: low (<100 lines, no signals), medium (100-400 lines), high (signals or >400 lines)
5. Return LensResult with findings per file

For MVP: parse diff stat output, classify by extension/path patterns, assign risk level. No AI needed — pure static analysis.

### Other lenses (deferred to second pass)

Readability, Reliability, Resilience require AI-based analysis (send code to an LLM for review). These should wait until we have a real ProviderPlugin/agent integration working.

## Approach

### In Scope (this change)
- `internal/lens/risk/lens.go` — Risk lens with git diff analysis
- `internal/lens/risk/lens_test.go`
- Integration: register risk lens in cmd/biggz/main.go pipeline
- Update `cmd/biggz/main.go` — replace DummyLens with RiskLens in pipeline

### Out of Scope (deferred)
- Readability/Reliability/Resilience lenses (need AI provider)
- Multi-file diff analysis (single-commit focus for MVP)
- Generated file detection (use path patterns only)

## Recommendations

Start with RiskLens only. It's the most tangible, doesn't need AI, and provides immediate value.
