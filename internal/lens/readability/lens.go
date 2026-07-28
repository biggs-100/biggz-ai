// Package readability provides a ReadabilityLens that analyzes code changes
// for readability and maintainability concerns such as file size, naming
// conventions, and complexity indicators.
package readability

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/lens/gitdiff"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

// ReadabilityLens is a LensPlugin that analyzes git diffs to assess
// code readability and maintainability based on file size, naming
// conventions, and structural heuristics.
type ReadabilityLens struct{}

// ID returns "readability" — the unique identifier for this lens.
func (l *ReadabilityLens) ID() string { return "readability" }

// Name returns "Readability Review" — the human-readable name.
func (l *ReadabilityLens) Name() string { return "Readability Review" }

// Version returns "1.0.0" — the current version.
func (l *ReadabilityLens) Version() string { return "1.0.0" }

// Policies returns the list of policies associated with this lens.
func (l *ReadabilityLens) Policies() []plugin.Policy {
	return []plugin.Policy{
		{Name: "file-length", Description: "Files should be kept under 500 lines for maintainability"},
	}
}

// Analyze runs the readability lens analysis against the given subject.
// It parses the git diff to identify files with readability concerns
// based on line count heuristics and naming conventions.
func (l *ReadabilityLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	files, err := gitdiff.GetDiffStat(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("readability lens: %w", err)
	}

	findings := analyzeReadability(files)

	return &plugin.LensResult{
		LensID:   l.ID(),
		Findings: findings,
	}, nil
}

// analyzeReadability inspects changed files and returns readability-related
// findings. It uses file-level heuristics for the MVP:
//   - Files with >500 additions are flagged as too long (warning)
//   - Files with 200-500 additions are flagged as potentially complex (info)
//   - Go files with mixed-case naming are flagged for convention checks
func analyzeReadability(files []gitdiff.DiffFile) []plugin.Finding {
	var findings []plugin.Finding
	totalAdditions := 0
	fileIdx := 0

	for _, f := range files {
		totalAdditions += f.Additions

		// File length heuristics
		if f.Additions > 500 {
			fileIdx++
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("readability-file-too-long-%d", fileIdx),
				Severity: "warning",
				Message:  fmt.Sprintf("File %s has %d additions — consider splitting into smaller files", f.Path, f.Additions),
				File:     f.Path,
			})
		} else if f.Additions > 200 {
			fileIdx++
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("readability-file-long-%d", fileIdx),
				Severity: "info",
				Message:  fmt.Sprintf("File %s has %d additions — may benefit from refactoring", f.Path, f.Additions),
				File:     f.Path,
			})
		}

		// Naming convention: flag Go files with mixed case in base name
		if strings.HasSuffix(f.Path, ".go") && f.Additions > 0 {
			base := filepath.Base(f.Path)
			hasUpper := strings.ToLower(base) != base
			hasUnderscore := strings.Contains(base, "_")
			if hasUpper && hasUnderscore {
				fileIdx++
				findings = append(findings, plugin.Finding{
					ID:       fmt.Sprintf("readability-naming-%d", fileIdx),
					Severity: "info",
					Message:  fmt.Sprintf("File %s uses mixed case and underscores — prefer consistent naming per language convention", f.Path),
					File:     f.Path,
				})
			}
		}
	}

	totalDeletions := 0
	for _, f := range files {
		totalDeletions += f.Deletions
	}

	findings = append(findings, plugin.Finding{
		ID:       "readability-overview",
		Severity: "info",
		Message:  fmt.Sprintf("Readability review: %d files, %d additions, %d deletions", len(files), totalAdditions, totalDeletions),
	})

	return findings
}
