// Package resilience provides a ResilienceLens that analyzes code changes
// for resilience concerns such as missing timeouts, retry logic, context
// cancellation checks, and resource cleanup patterns.
package resilience

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/lens/gitdiff"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// ResilienceLens is a LensPlugin that analyzes git diffs to assess
// code resilience based on file path patterns that suggest potential
// resilience gaps such as missing timeouts, retries, and cleanup.
type ResilienceLens struct{}

// ID returns "resilience" — the unique identifier for this lens.
func (l *ResilienceLens) ID() string { return "resilience" }

// Name returns "Resilience Review" — the human-readable name.
func (l *ResilienceLens) Name() string { return "Resilience Review" }

// Version returns "1.0.0" — the current version.
func (l *ResilienceLens) Version() string { return "1.0.0" }

// Policies returns the list of policies associated with this lens.
func (l *ResilienceLens) Policies() []plugin.Policy {
	return []plugin.Policy{
		{Name: "timeout-configuration", Description: "HTTP clients and external callers should have configured timeouts"},
		{Name: "context-propagation", Description: "Goroutines and async operations should respect context cancellation"},
		{Name: "resource-cleanup", Description: "Resources should be properly closed with defer"},
	}
}

// Analyze runs the resilience lens analysis against the given subject.
// It parses the git diff to identify files that may have resilience gaps
// based on path patterns and naming conventions.
func (l *ResilienceLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	files, err := gitdiff.GetDiffStat(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("resilience lens: %w", err)
	}

	findings := analyzeResilience(files)

	return &plugin.LensResult{
		LensID:   l.ID(),
		Findings: findings,
	}, nil
}

// analyzeResilience inspects changed files and returns resilience-related
// findings. For the MVP it uses file-path-based heuristics:
//   - HTTP/transport files flagged for missing timeout configuration
//   - Database/repo files flagged for missing context cancellation
//   - Files with "pool", "worker", "goroutine" in path flagged for missing sync
//   - Resource files (files, streams, bodies) flagged for missing defer cleanup
func analyzeResilience(files []gitdiff.DiffFile) []plugin.Finding {
	var findings []plugin.Finding

	for _, f := range files {
		lowerPath := strings.ToLower(f.Path)
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}

		// Timeout detection: HTTP clients and transport layers
		if strings.Contains(lowerPath, "http") || strings.Contains(lowerPath, "client") {
			findings = append(findings, plugin.Finding{
				ID:       "resilience-timeout",
				Severity: "warning",
				Message:  fmt.Sprintf("File %s may contain HTTP or external calls — verify Timeout is configured", f.Path),
				File:     f.Path,
			})
		}

		// Context cancellation: database and external service files
		if strings.Contains(lowerPath, "db") || strings.Contains(lowerPath, "database") || strings.Contains(lowerPath, "repo") || strings.Contains(lowerPath, "store") || strings.Contains(lowerPath, "queue") {
			findings = append(findings, plugin.Finding{
				ID:       "resilience-context",
				Severity: "warning",
				Message:  fmt.Sprintf("File %s performs external operations — verify context cancellation and timeout propagation", f.Path),
				File:     f.Path,
			})
		}

		// Concurrency: worker pool and goroutine patterns
		if strings.Contains(lowerPath, "pool") || strings.Contains(lowerPath, "worker") || strings.Contains(lowerPath, "goroutine") || strings.Contains(lowerPath, "async") {
			findings = append(findings, plugin.Finding{
				ID:       "resilience-concurrency",
				Severity: "warning",
				Message:  fmt.Sprintf("File %s uses concurrent patterns — verify WaitGroup or errgroup for goroutine lifecycle", f.Path),
				File:     f.Path,
			})
		}

		// Resource cleanup: file and stream operations
		if strings.Contains(lowerPath, "file") || strings.Contains(lowerPath, "stream") || strings.Contains(lowerPath, "reader") || strings.Contains(lowerPath, "writer") || strings.Contains(lowerPath, "body") || strings.Contains(lowerPath, "close") || strings.Contains(lowerPath, "socket") || strings.Contains(lowerPath, "conn") {
			findings = append(findings, plugin.Finding{
				ID:       "resilience-cleanup",
				Severity: "info",
				Message:  fmt.Sprintf("File %s manages resources — verify defer cleanup for closeable resources", f.Path),
				File:     f.Path,
			})
		}
	}

	totalAdditions := 0
	totalDeletions := 0
	for _, f := range files {
		totalAdditions += f.Additions
		totalDeletions += f.Deletions
	}

	findings = append(findings, plugin.Finding{
		ID:       "resilience-overview",
		Severity: "info",
		Message:  fmt.Sprintf("Resilience review: %d files checked, %d additions, %d deletions", len(files), totalAdditions, totalDeletions),
	})

	return findings
}
