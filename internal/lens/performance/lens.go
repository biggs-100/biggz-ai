package performance

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/lens/gitdiff"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

type PerformanceLens struct{}

func (l *PerformanceLens) ID() string      { return "performance" }
func (l *PerformanceLens) Name() string    { return "Performance Review" }
func (l *PerformanceLens) Version() string { return "1.0.0" }

func (l *PerformanceLens) Policies() []plugin.Policy {
	return []plugin.Policy{
		{Name: "allocations-in-hot-path", Description: "Avoid heap allocations in hot code paths"},
		{Name: "no-sync-in-crit-path", Description: "Synchronous I/O in critical paths must have timeout"},
		{Name: "benchmark-evidence", Description: "Performance-sensitive changes should include benchmark results"},
	}
}

func (l *PerformanceLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	files, err := gitdiff.GetDiffStat(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("performance lens: %w", err)
	}
	findings := analyzePerformance(files)
	return &plugin.LensResult{LensID: l.ID(), Findings: findings}, nil
}

func analyzePerformance(files []gitdiff.DiffFile) []plugin.Finding {
	var findings []plugin.Finding
	hotExts := map[string]string{".go": "Go", ".js": "JavaScript", ".ts": "TypeScript", ".rs": "Rust", ".java": "Java"}

	for _, f := range files {
		ext := strings.ToLower(extname(f.Path))
		lang, known := hotExts[ext]
		if !known {
			continue
		}
		if f.Additions > 200 {
			findings = append(findings, plugin.Finding{
				ID:       fmt.Sprintf("R5-%d", len(findings)+1),
				Severity: "warning",
				Message:  fmt.Sprintf("Large %s change (+%d lines) may cause regressions; add benchmark evidence", lang, f.Additions),
				File:     f.Path,
				Line:     1,
			})
		}
		if strings.Contains(f.Path, "handler") || strings.Contains(f.Path, "middleware") {
			if f.Additions+f.Deletions > 50 {
				findings = append(findings, plugin.Finding{
					ID:       fmt.Sprintf("R5-%d", len(findings)+1),
					Severity: "info",
					Message:  fmt.Sprintf("Hot path handler modified (+%d/-%d) — verify latency and allocs", f.Additions, f.Deletions),
					File:     f.Path,
					Line:     1,
				})
			}
		}
	}
	return findings
}

func extname(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}
