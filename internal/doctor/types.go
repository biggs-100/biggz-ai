// Package doctor provides a health check framework for biggz-ai installations.
//
// It defines a Check interface, a panic-isolated Runner, severity-categorized
// Report, and atomic Remedies. Each system domain gets its own check file.
package doctor

import "context"

// CheckID uniquely identifies a health check.
type CheckID string

// Status represents the result of a health check.
type Status int

const (
	// StatusPass indicates the check passed.
	StatusPass Status = iota
	// StatusWarn indicates a non-critical issue.
	StatusWarn
	// StatusFail indicates a critical failure.
	StatusFail
)

// String returns a human-readable label for the status.
func (s Status) String() string {
	switch s {
	case StatusPass:
		return "pass"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	default:
		return "unknown"
	}
}

// Severity labels mapped from Status for the Report buckets.
const (
	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityInfo     = "INFO"
)

// Result is the outcome of a single health check.
type Result struct {
	ID       CheckID `json:"id"`
	Status   Status  `json:"status"`
	Message  string  `json:"message"`
	Severity string  `json:"severity"` // "CRITICAL" | "WARNING" | "INFO"
	Error    string  `json:"error,omitempty"`
	Details  any     `json:"details,omitempty"`
}

// Check defines the contract for a single health check.
type Check interface {
	// ID returns the unique identifier for this check.
	ID() CheckID
	// Run executes the check and returns its result. The context controls
	// cancellation — checks SHOULD respect ctx.Done() for long operations.
	Run(ctx context.Context) *Result
	// Remedy returns an optional remediation for issues found by this check.
	// Returns nil when no remediation is available.
	Remedy() *Remedy
}

// Remedy describes an action that can fix an issue found by a check.
type Remedy struct {
	ID          string
	Description string
	Action      func(ctx context.Context) error
}

// Report groups Results by severity bucket for structured output.
type Report struct {
	Critical []*Result `json:"critical"`
	Warning  []*Result `json:"warning"`
	Info     []*Result `json:"info"`
}

// All returns all results across all severity buckets.
func (r *Report) All() []*Result {
	n := len(r.Critical) + len(r.Warning) + len(r.Info)
	out := make([]*Result, 0, n)
	out = append(out, r.Critical...)
	out = append(out, r.Warning...)
	out = append(out, r.Info...)
	return out
}

// CountByStatus returns the number of results with the given status.
func (r *Report) CountByStatus(s Status) int {
	n := 0
	for _, res := range r.All() {
		if res.Status == s {
			n++
		}
	}
	return n
}

// ExitCode returns an appropriate exit code based on the report's severity:
// 2 for CRITICAL, 1 for WARNING only, 0 for all pass.
func (r *Report) ExitCode() int {
	if len(r.Critical) > 0 {
		return 2
	}
	if len(r.Warning) > 0 {
		return 1
	}
	return 0
}
