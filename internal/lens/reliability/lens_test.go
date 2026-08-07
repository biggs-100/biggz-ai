package reliability

import (
	"context"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/lens/gitdiff"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// ---- analyzeReliability ----

func TestAnalyzeReliability_MissingTestFile(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/handler/handler.go", Additions: 50, Deletions: 0},
	}
	findings := analyzeReliability(files)

	if !hasFindingWithPrefix(findings, "reliability-missing-test") {
		t.Error("expected missing test finding for handler.go without test file")
	}
}

func TestAnalyzeReliability_WithTestFile(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/handler/handler.go", Additions: 50, Deletions: 0},
		{Path: "internal/handler/handler_test.go", Additions: 30, Deletions: 0},
	}
	findings := analyzeReliability(files)

	if hasFindingWithPrefix(findings, "reliability-missing-test") {
		t.Error("unexpected missing test finding when test file exists")
	}
}

func TestAnalyzeReliability_NoGoFiles(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "README.md", Additions: 10, Deletions: 0},
		{Path: "config.yaml", Additions: 5, Deletions: 0},
	}
	findings := analyzeReliability(files)

	if hasFindingWithPrefix(findings, "reliability-missing-test") {
		t.Error("unexpected missing test finding for non-Go files")
	}
}

func TestAnalyzeReliability_MixedFiles(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/auth/auth.go", Additions: 100, Deletions: 0},
		{Path: "internal/auth/auth_test.go", Additions: 60, Deletions: 0},
		{Path: "internal/api/handler.go", Additions: 80, Deletions: 0},
		{Path: "README.md", Additions: 5, Deletions: 0},
	}
	findings := analyzeReliability(files)

	// handler.go has no test file → missing test finding
	if !hasFindingWithPrefix(findings, "reliability-missing-test") {
		t.Error("expected missing test finding for handler.go")
	}

	// auth.go has auth_test.go → no missing test finding for auth.go
	for _, f := range findings {
		if strings.HasPrefix(f.ID, "reliability-missing-test") && f.File == "internal/auth/auth.go" {
			t.Error("unexpected missing test finding for auth.go which has test file")
		}
	}
}

func TestAnalyzeReliability_EmptyInput(t *testing.T) {
	findings := analyzeReliability(nil)

	overview := findFindingByID(findings, "reliability-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
}

func TestAnalyzeReliability_NoFiles(t *testing.T) {
	findings := analyzeReliability([]gitdiff.DiffFile{})

	overview := findFindingByID(findings, "reliability-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
}

func TestAnalyzeReliability_LargeChangeSet(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/service/big.go", Additions: 400, Deletions: 0},
	}
	findings := analyzeReliability(files)

	if !hasFindingWithPrefix(findings, "reliability-large-change") {
		t.Error("expected large change finding for 400 additions")
	}
}

func TestAnalyzeReliability_SmallChangeNoWarning(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/service/small.go", Additions: 10, Deletions: 5},
	}
	findings := analyzeReliability(files)

	if hasFindingWithPrefix(findings, "reliability-large-change") {
		t.Error("unexpected large change finding for small file")
	}
}

func TestAnalyzeReliability_ErrorHandlingFilePaths(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/handler/orders.go", Additions: 50, Deletions: 0},
		{Path: "internal/service/users.go", Additions: 30, Deletions: 0},
		{Path: "internal/store/db.go", Additions: 20, Deletions: 0},
	}
	findings := analyzeReliability(files)

	// Should flag handler, service, and store for error handling
	sensitiveCount := 0
	for _, kw := range []string{"handler", "service", "store"} {
		if hasFindingWithID(findings, "reliability-error-handling-"+kw) {
			sensitiveCount++
		}
	}
	if sensitiveCount < 2 {
		t.Errorf("expected at least 2 error-handling-sensitive file findings, got %d", sensitiveCount)
	}
}

func TestAnalyzeReliability_TestFileOnly(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/handler/handler_test.go", Additions: 50, Deletions: 0},
	}
	findings := analyzeReliability(files)

	// Only a test file changed — no missing test finding
	if hasFindingWithPrefix(findings, "reliability-missing-test") {
		t.Error("unexpected missing test finding when only test file changed")
	}
}

func TestAnalyzeReliability_OverviewFormat(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "main.go", Additions: 10, Deletions: 5},
		{Path: "main_test.go", Additions: 20, Deletions: 0},
	}
	findings := analyzeReliability(files)

	overview := findFindingByID(findings, "reliability-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
	if !strings.Contains(overview.Message, "1 Go source") {
		t.Errorf("expected '1 Go source' in overview, got: %s", overview.Message)
	}
	if !strings.Contains(overview.Message, "1 test") {
		t.Errorf("expected '1 test' in overview, got: %s", overview.Message)
	}
}

// ---- ReliabilityLens interface compliance ----

func TestReliabilityLens_ID(t *testing.T) {
	l := &ReliabilityLens{}
	if id := l.ID(); id != "reliability" {
		t.Errorf("ID() = %q, want %q", id, "reliability")
	}
}

func TestReliabilityLens_Name(t *testing.T) {
	l := &ReliabilityLens{}
	if name := l.Name(); name != "Reliability Review" {
		t.Errorf("Name() = %q, want %q", name, "Reliability Review")
	}
}

func TestReliabilityLens_Version(t *testing.T) {
	l := &ReliabilityLens{}
	if v := l.Version(); v != "1.0.0" {
		t.Errorf("Version() = %q, want %q", v, "1.0.0")
	}
}

func TestReliabilityLens_Policies(t *testing.T) {
	l := &ReliabilityLens{}
	policies := l.Policies()
	if len(policies) == 0 {
		t.Error("Policies() returned empty slice, expected at least 1 policy")
	}
}

func TestReliabilityLens_ImplementsLensPlugin(t *testing.T) {
	var _ plugin.LensPlugin = (*ReliabilityLens)(nil)
}

// ---- Analyze error path ----

func TestReliabilityLens_Analyze_InvalidRepo(t *testing.T) {
	l := &ReliabilityLens{}
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

func hasFindingWithID(findings []plugin.Finding, id string) bool {
	for _, f := range findings {
		if f.ID == id {
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
