package doctor

import (
	"context"
	"fmt"
)

// Runner executes a set of checks with panic isolation and produces a Report.
type Runner struct {
	Checks []Check
}

// RunAll executes all checks sequentially. Each check is wrapped in
// defer/recover — a panic in one check does NOT abort the remaining checks.
// The context is passed through to each check.
func (r *Runner) RunAll(ctx context.Context) *Report {
	report := &Report{}

	for _, check := range r.Checks {
		result := r.runOne(ctx, check)
		switch result.Severity {
		case SeverityCritical:
			report.Critical = append(report.Critical, result)
		case SeverityWarning:
			report.Warning = append(report.Warning, result)
		case SeverityInfo:
			report.Info = append(report.Info, result)
		}
	}

	return report
}

// runOne executes a single check with panic recovery.
// biggz doctor stays read-only: RunAll never writes, supports --json RO,
// and exposes no --fix (drift checks have nil Remedy). A panic in one
// check is isolated so other checks still run; drift panics surface as warn.
func (r *Runner) runOne(ctx context.Context, check Check) (result *Result) {
	defer func() {
		if p := recover(); p != nil {
			id := check.ID()
			status := StatusFail
			severity := SeverityCritical
			// Drift/local-override checks are warnings, not critical failures.
			if id == GlobalDriftCheckID || id == LocalOverrideCheckID {
				status = StatusWarn
				severity = SeverityWarning
			}
			result = &Result{
				ID:       id,
				Status:   status,
				Message:  "check panicked",
				Severity: severity,
				Error:    fmt.Sprintf("panic: %v", p),
			}
		}
	}()

	return check.Run(ctx)
}
