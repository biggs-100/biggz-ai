package dependencies

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/lens/gitdiff"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

type DependenciesLens struct{}

func (l *DependenciesLens) ID() string      { return "dependencies" }
func (l *DependenciesLens) Name() string    { return "Dependencies Review" }
func (l *DependenciesLens) Version() string { return "1.0.0" }

func (l *DependenciesLens) Policies() []plugin.Policy {
	return []plugin.Policy{
		{Name: "no-vulnerable-deps", Description: "Dependencies must not have known critical vulnerabilities"},
		{Name: "license-compatibility", Description: "New dependencies must have compatible licenses"},
		{Name: "pin-versions", Description: "Dependencies should be pinned to specific versions"},
	}
}

func (l *DependenciesLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	files, err := gitdiff.GetDiffStat(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("dependencies lens: %w", err)
	}
	findings := analyzeDependencies(files)
	return &plugin.LensResult{LensID: l.ID(), Findings: findings}, nil
}

func analyzeDependencies(files []gitdiff.DiffFile) []plugin.Finding {
	var findings []plugin.Finding
	depFiles := map[string]string{"go.mod": "Go module", "go.sum": "Go checksum",
		"package.json": "npm", "requirements.txt": "pip", "Cargo.toml": "Rust"}

	for _, f := range files {
		base := basename(f.Path)
		depType, known := depFiles[base]
		if !known {
			continue
		}
		if f.Additions > 50 && (base == "go.sum") {
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("R6-%d", len(findings)+1),
				Severity: "warning",
				Message:  fmt.Sprintf("Large lockfile change (+%d lines) — review new dependencies", f.Additions),
				File:     f.Path, Line: 1,
			})
		}
		if f.Additions > 0 {
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("R6-%d", len(findings)+1),
				Severity: "info",
				Message:  fmt.Sprintf("%s dependency changed — verify license compatibility and security", depType),
				File:     f.Path, Line: 1,
			})
		}
	}
	return findings
}

func basename(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
