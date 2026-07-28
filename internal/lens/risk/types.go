// Package risk provides a RiskLens that analyzes git diffs to assess
// the risk level of a code review based on file paths, content signals,
// and the scale of changes.
package risk

// RiskLevel represents the overall assessment of a code change's risk.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// RiskSignal represents a category of risk detected in a diff.
type RiskSignal string

const (
	SignalAuth     RiskSignal = "auth"
	SignalShell    RiskSignal = "shell"
	SignalSecurity RiskSignal = "security"
	SignalExecMode RiskSignal = "executable_mode"
)

// DiffFile holds the parsed statistics for a single file in a git diff.
type DiffFile struct {
	Path      string
	Additions int
	Deletions int
}

// RiskAssessment is the complete result of analyzing a diff for risk.
// It contains the aggregated risk level, total changed lines, detected
// signals, and per-file details.
type RiskAssessment struct {
	Level        RiskLevel    `json:"level"`
	ChangedLines int          `json:"changed_lines"`
	Signals      []RiskSignal `json:"signals"`
	Files        []DiffFile   `json:"files"`
}
