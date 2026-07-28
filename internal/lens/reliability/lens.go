// Package reliability provides a ReliabilityLens that analyzes code changes
// for reliability concerns such as missing tests, error handling patterns,
// and concurrency issues.
package reliability

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/lens/gitdiff"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

// ReliabilityLens is a LensPlugin that analyzes git diffs to assess
// code reliability based on test coverage, error handling patterns,
// and common reliability anti-patterns.
type ReliabilityLens struct{}

// ID returns "reliability" — the unique identifier for this lens.
func (l *ReliabilityLens) ID() string { return "reliability" }

// Name returns "Reliability Review" — the human-readable name.
func (l *ReliabilityLens) Name() string { return "Reliability Review" }

// Version returns "1.0.0" — the current version.
func (l *ReliabilityLens) Version() string { return "1.0.0" }

// Policies returns the list of policies associated with this lens.
func (l *ReliabilityLens) Policies() []plugin.Policy {
	return []plugin.Policy{
		{Name: "test-coverage", Description: "Changed Go files should have corresponding test files"},
		{Name: "error-handling", Description: "Errors should be checked and not silently ignored"},
	}
}

// Analyze runs the reliability lens analysis against the given subject.
// It parses the git diff to identify reliability concerns based on file
// path patterns and structural heuristics.
func (l *ReliabilityLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	files, err := gitdiff.GetDiffStat(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("reliability lens: %w", err)
	}

	findings := analyzeReliability(files)

	return &plugin.LensResult{
		LensID:   l.ID(),
		Findings: findings,
	}, nil
}

// analyzeReliability inspects changed files and returns reliability-related
// findings. For the MVP it focuses on:
//   - Missing test coverage: Go source files changed without corresponding test files
//   - Large change sets that may indicate reliability risk
//   - Error handling patterns based on file path heuristics
func analyzeReliability(files []gitdiff.DiffFile) []plugin.Finding {
	var findings []plugin.Finding

	// Separate Go source files and test files
	goSrcFiles := make(map[string]gitdiff.DiffFile)
	testFiles := make(map[string]bool)
	otherFiles := 0

	for _, f := range files {
		if strings.HasSuffix(f.Path, "_test.go") {
			// Trim "_test.go" suffix to get the base name
			base := strings.TrimSuffix(f.Path, "_test.go")
			testFiles[base] = true
		} else if strings.HasSuffix(f.Path, ".go") {
			goSrcFiles[f.Path] = f
		} else {
			otherFiles++
		}
	}

	// Check for changed Go files without corresponding test files
	for path := range goSrcFiles {
		base := strings.TrimSuffix(path, ".go")
		if !testFiles[base] {
			idx := strings.Count(path, "/") + 1
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("reliability-missing-test-%d", idx),
				Severity: "warning",
				Message:  fmt.Sprintf("Changed file %s has no corresponding test file — consider adding tests", path),
				File:     path,
			})
		}
	}

	// Flag files with many additions as reliability risk
	for _, f := range goSrcFiles {
		if f.Additions > 300 {
			findings = append(findings, plugin.Finding{
				ID:       "reliability-large-change-" + f.Path,
				Severity: "info",
				Message:  fmt.Sprintf("Large change set in %s (%d additions) — high risk of introducing bugs", f.Path, f.Additions),
				File:     f.Path,
			})
		}
	}

	// Check for error-handling-sensitive file paths
	errorSensitiveKeywords := []string{"handler", "service", "repo", "store", "client", "manager"}
	for _, f := range files {
		lowerPath := strings.ToLower(f.Path)
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		for _, kw := range errorSensitiveKeywords {
			if strings.Contains(lowerPath, kw) {
				findings = append(findings, plugin.Finding{
					ID:       "reliability-error-handling-" + kw,
					Severity: "info",
					Message:  fmt.Sprintf("File %s handles external operations — verify error handling and edge cases", f.Path),
					File:     f.Path,
				})
				break
			}
		}
	}

	totalAdditions := 0
	totalDeletions := 0
	for _, f := range files {
		totalAdditions += f.Additions
		totalDeletions += f.Deletions
	}

	findings = append(findings, plugin.Finding{
		ID:       "reliability-overview",
		Severity: "info",
		Message:  fmt.Sprintf("Reliability review: %d files (%d Go source + %d test + %d other), %d additions, %d deletions", len(files), len(goSrcFiles), len(testFiles), otherFiles, totalAdditions, totalDeletions),
	})

	return findings
}
