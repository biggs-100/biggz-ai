package readability

import (
	"context"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/internal/lens/gitdiff"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

// ---- analyzeReadability ----

func TestAnalyzeReadability_FileTooLong(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "cmd/main.go", Additions: 600, Deletions: 0},
	}
	findings := analyzeReadability(files)

	if !hasFindingWithPrefix(findings, "readability-file-too-long") {
		t.Error("expected finding for file >500 additions")
	}
}

func TestAnalyzeReadability_FileLongWarning(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "internal/handler.go", Additions: 300, Deletions: 0},
	}
	findings := analyzeReadability(files)

	if !hasFindingWithPrefix(findings, "readability-file-long") {
		t.Error("expected finding for file between 200-500 additions")
	}
}

func TestAnalyzeReadability_SmallFileNoWarning(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "main.go", Additions: 10, Deletions: 5},
	}
	findings := analyzeReadability(files)

	if hasFindingWithPrefix(findings, "readability-file-too-long") {
		t.Error("unexpected warning for small file")
	}
	if hasFindingWithPrefix(findings, "readability-file-long") {
		t.Error("unexpected warning for small file")
	}
}

func TestAnalyzeReadability_EmptyInput(t *testing.T) {
	findings := analyzeReadability(nil)

	overview := findFindingByID(findings, "readability-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
	if !strings.Contains(overview.Message, "0 files") {
		t.Errorf("expected 0 files in overview, got: %s", overview.Message)
	}
}

func TestAnalyzeReadability_NoFiles(t *testing.T) {
	findings := analyzeReadability([]gitdiff.DiffFile{})

	overview := findFindingByID(findings, "readability-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
}

func TestAnalyzeReadability_MultipleFilesMixed(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "tiny.go", Additions: 5, Deletions: 0},
		{Path: "medium.go", Additions: 250, Deletions: 10},
		{Path: "huge.go", Additions: 600, Deletions: 200},
	}
	findings := analyzeReadability(files)

	if !hasFindingWithPrefix(findings, "readability-file-too-long") {
		t.Error("expected too-long finding for huge.go")
	}
	if !hasFindingWithPrefix(findings, "readability-file-long") {
		t.Error("expected file-long finding for medium.go")
	}
}

func TestAnalyzeReadability_Exactly200Additions(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "edge.go", Additions: 200, Deletions: 0},
	}
	findings := analyzeReadability(files)

	// 200 is not >200, so no file-long finding
	if hasFindingWithPrefix(findings, "readability-file-long") {
		t.Error("expected no finding for exactly 200 additions")
	}
}

func TestAnalyzeReadability_Exactly500Additions(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "edge.go", Additions: 500, Deletions: 0},
	}
	findings := analyzeReadability(files)

	// 500 is not >500, so no too-long finding
	if hasFindingWithPrefix(findings, "readability-file-too-long") {
		t.Error("expected no finding for exactly 500 additions")
	}
}

func TestAnalyzeReadability_OverviewIncludesTotals(t *testing.T) {
	files := []gitdiff.DiffFile{
		{Path: "a.go", Additions: 30, Deletions: 10},
		{Path: "b.go", Additions: 20, Deletions: 5},
	}
	findings := analyzeReadability(files)

	overview := findFindingByID(findings, "readability-overview")
	if overview == nil {
		t.Fatal("expected overview finding")
	}
	if !strings.Contains(overview.Message, "2 files") {
		t.Errorf("expected '2 files' in overview, got: %s", overview.Message)
	}
	if !strings.Contains(overview.Message, "50 additions") {
		t.Errorf("expected '50 additions' in overview, got: %s", overview.Message)
	}
	if !strings.Contains(overview.Message, "15 deletions") {
		t.Errorf("expected '15 deletions' in overview, got: %s", overview.Message)
	}
}

// ---- ReadabilityLens interface compliance ----

func TestReadabilityLens_ID(t *testing.T) {
	l := &ReadabilityLens{}
	if id := l.ID(); id != "readability" {
		t.Errorf("ID() = %q, want %q", id, "readability")
	}
}

func TestReadabilityLens_Name(t *testing.T) {
	l := &ReadabilityLens{}
	if name := l.Name(); name != "Readability Review" {
		t.Errorf("Name() = %q, want %q", name, "Readability Review")
	}
}

func TestReadabilityLens_Version(t *testing.T) {
	l := &ReadabilityLens{}
	if v := l.Version(); v != "1.0.0" {
		t.Errorf("Version() = %q, want %q", v, "1.0.0")
	}
}

func TestReadabilityLens_Policies(t *testing.T) {
	l := &ReadabilityLens{}
	policies := l.Policies()
	if len(policies) == 0 {
		t.Error("Policies() returned empty slice, expected at least 1 policy")
	}
	if policies[0].Name != "file-length" {
		t.Errorf("first policy Name = %q, want %q", policies[0].Name, "file-length")
	}
}

func TestReadabilityLens_ImplementsLensPlugin(t *testing.T) {
	var _ plugin.LensPlugin = (*ReadabilityLens)(nil)
}

// ---- Analyze error path ----

func TestReadabilityLens_Analyze_InvalidRepo(t *testing.T) {
	l := &ReadabilityLens{}
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
