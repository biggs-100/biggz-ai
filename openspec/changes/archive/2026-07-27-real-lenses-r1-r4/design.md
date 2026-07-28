# Design: Risk Lens (R1)

## Approach

Standalone lens package implementing `plugin.LensPlugin`. Analyzes git diff stat via `exec.Command("git", "diff", "--stat")` and classifies files by path patterns and content signals.

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Git command | `git diff --stat` | Simple, universally available, no deps |
| Risk signals | Path-based patterns | No AI needed, deterministic |
| Risk levels | Low / Medium / High | Matches gentle-ai convention |
| Testing | Mock git output files | No real git repo needed in tests |

## Data Flow

```
Subject (repo + commit SHA)
  → git diff --stat HEAD~1..HEAD
  → parse stats (file, additions, deletions)
  → classify each file by path/ext:
      - shell script (.sh, .bash)
      - config/service (.env, .service-token, workflows/)
      - security-sensitive (auth/, crypto/, secret*)
      - executable mode change
  → derive risk signals
  → assign risk level:
      - HIGH: any signal detected
      - MEDIUM: >100 lines or generated files
      - LOW: otherwise
  → return LensResult with findings per file
```

## Types

```go
type RiskLevel string
const (
    RiskLow    RiskLevel = "low"
    RiskMedium RiskLevel = "medium"
    RiskHigh   RiskLevel = "high"
)

type RiskSignal string
const (
    SignalAuth     RiskSignal = "auth"
    SignalShell    RiskSignal = "shell"
    SignalSecurity RiskSignal = "security"
    SignalExecMode RiskSignal = "executable_mode"
)

type DiffFile struct {
    Path      string
    Additions int
    Deletions int
}

type RiskAssessment struct {
    Level       RiskLevel
    ChangedLines int
    Signals     []RiskSignal
    Files       []DiffFile
}
```

## Implementation Plan

1. Parse `git diff --stat` output → []DiffFile
2. Classify each file → detect RiskSignals
3. Aggregate into RiskLevel
4. Build LensResult with findings per file
5. Register in cmd/biggz/main.go pipeline

## Test Strategy

- Mock `git diff --stat` output as string literals
- Test each classification function
- Test full pipeline with fake subject
