package resilience

import (
	"context"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/internal/lens/gitdiff"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

// ---- analyzeResilience ----

func TestAnalyzeResilience_HTTPClientDetected(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/http/client.go", Additions: 100, Deletions: 0},
	}
	findings := analyzeResilience(files)

	if !hasFindingWithPrefix(findings, "resilience-timeout") {
		t.Error("expected timeout finding for HTTP client file")
	}
}

func TestAnalyzeResilience_DatabaseContextDetected(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/db/repo.go", Additions: 80, Deletions: 0},
	}
	findings := analyzeResilience(files)

	if !hasFindingWithPrefix(findings, "resilience-context") {
		t.Error("expected context finding for database file")
	}
}

func TestAnalyzeResilience_ConcurrencyDetected(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/worker/pool.go", Additions: 60, Deletions: 0},
	}
	findings := analyzeResilience(files)

	if !hasFindingWithPrefix(findings, "resilience-concurrency") {
		t.Error("expected concurrency finding for worker pool file")
	}
}

func TestAnalyzeResilience_ResourceCleanupDetected(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/file/reader.go", Additions: 40, Deletions: 0},
	}
	findings := analyzeResilience(files)

	if !hasFindingWithPrefix(findings, "resilience-cleanup") {
		t.Error("expected cleanup finding for file reader")
	}
}

func TestAnalyzeResilience_NoIssues(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "main.go", Additions: 10, Deletions: 5},
		{Path: "README.md", Additions: 3, Deletions: 0},
	}
	findings := analyzeResilience(files)

	// Only overview should be present for non-sensitive files
	nonOverview := 0
	for _, f := range findings {
		if f.ID != "resilience-overview" {
			nonOverview++
		}
	}
	if nonOverview != 0 {
		t.Errorf("expected 0 non-overview findings for neutral files, got %d: %v", nonOverview, findings)
	}
}

func TestAnalyzeResilience_EmptyInput(t *testing.T) {
	findings := analyzeResilience(nil)

	overview := findFindingByID(findings, "resilience-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
}

func TestAnalyzeResilience_NoFiles(t *testing.T) {
	findings := analyzeResilience([]gitdiff.DiffFile{})

	overview := findFindingByID(findings, "resilience-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
}

func TestAnalyzeResilience_MultipleIssuesSameFile(t *testing.T) {
	// A file that matches multiple resilience categories
	files := []gitdiff.DiffFile{
		{Path: "internal/http/client/pool.go", Additions: 200, Deletions: 0},
	}
	findings := analyzeResilience(files)

	timeoutCount := 0
	for _, f := range findings {
		if strings.HasPrefix(f.ID, "resilience-") && f.ID != "resilience-overview" {
			timeoutCount++
		}
	}
	// Should match: timeout (http+client), concurrency (pool)
	if timeoutCount < 2 {
		t.Errorf("expected at least 2 resilience findings for http/client/pool.go, got %d", timeoutCount)
	}
}

func TestAnalyzeResilience_NonGoFileSkipped(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "http/client.py", Additions: 100, Deletions: 0},
	}
	findings := analyzeResilience(files)

	// Non-Go files are skipped for detailed analysis
	nonOverview := 0
	for _, f := range findings {
		if f.ID != "resilience-overview" {
			nonOverview++
		}
	}
	if nonOverview != 0 {
		t.Errorf("expected 0 detailed findings for non-Go files, got %d", nonOverview)
	}
}

func TestAnalyzeResilience_OverviewIncludesTotals(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "main.go", Additions: 30, Deletions: 10},
		{Path: "http/client.go", Additions: 50, Deletions: 20},
	}
	findings := analyzeResilience(files)

	overview := findFindingByID(findings, "resilience-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
	if !strings.Contains(overview.Message, "2 files") {
		t.Errorf("expected '2 files' in overview, got: %s", overview.Message)
	}
	if !strings.Contains(overview.Message, "80 additions") {
		t.Errorf("expected '80 additions' in overview, got: %s", overview.Message)
	}
	if !strings.Contains(overview.Message, "30 deletions") {
		t.Errorf("expected '30 deletions' in overview, got: %s", overview.Message)
	}
}

func TestAnalyzeResilience_StoreMatchesContextAndCleanup(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/store/conn.go", Additions: 60, Deletions: 0},
	}
	findings := analyzeResilience(files)

	// store → context finding, conn → cleanup finding
	if !hasFindingWithPrefix(findings, "resilience-context") {
		t.Error("expected context finding for store file")
	}
	if !hasFindingWithPrefix(findings, "resilience-cleanup") {
		t.Error("expected cleanup finding for conn file")
	}
}

// ---- ResilienceLens interface compliance ----

func TestResilienceLens_ID(t *testing.T) {
	l := &ResilienceLens{}
	if id := l.ID(); id != "resilience" {
		t.Errorf("ID() = %q, want %q", id, "resilience")
	}
}

func TestResilienceLens_Name(t *testing.T) {
	l := &ResilienceLens{}
	if name := l.Name(); name != "Resilience Review" {
		t.Errorf("Name() = %q, want %q", name, "Resilience Review")
	}
}

func TestResilienceLens_Version(t *testing.T) {
	l := &ResilienceLens{}
	if v := l.Version(); v != "1.0.0" {
		t.Errorf("Version() = %q, want %q", v, "1.0.0")
	}
}

func TestResilienceLens_Policies(t *testing.T) {
	l := &ResilienceLens{}
	policies := l.Policies()
	if len(policies) == 0 {
		t.Error("Policies() returned empty slice, expected at least 1 policy")
	}
}

func TestResilienceLens_ImplementsLensPlugin(t *testing.T) {
	var _ plugin.LensPlugin = (*ResilienceLens)(nil)
}

// ---- Analyze error path ----

func TestResilienceLens_Analyze_InvalidRepo(t *testing.T) {
	l := &ResilienceLens{}
	subject := model.ReviewSubject{
		Repository: "/nonexistent/path",
		CommitSHA:  "abc123",
	}
	_, err := l.Analyze(context.Background(), subject)
	if err == nil {
		t.Fatal("expected error for invalid repo path, got nil")
	}
}

// ---- helpers ----

func hasFindingWithPrefix(findings []plugin.Finding, prefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f.ID, prefix) {
			return true
		}
	}
	return false
}

func findFindingByID(findings []plugin.Finding, id string) *plugin.Finding {
	for i := range findings {
		if findings[i].ID == id {
			return &findings[i]
		}
	}
	return nil
}
