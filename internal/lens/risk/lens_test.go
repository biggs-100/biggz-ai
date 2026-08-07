package risk

import (
	"context"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// ---- classifyFile ----

func TestClassifyFile_AuthSignal(t *testing.T) {
	tests := []struct {
		path     string
		expected []RiskSignal
	}{
		{"internal/auth/middleware.go", []RiskSignal{SignalAuth}},
		{"config/token.json", []RiskSignal{SignalAuth}},
		{".env", []RiskSignal{SignalAuth}},
		{"secrets/credentials.txt", []RiskSignal{SignalAuth}},
		{"config/password.yaml", []RiskSignal{SignalAuth}},
		{"main.go", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			signals := classifyFile(DiffFile{Path: tt.path})
			if !containsAll(signals, tt.expected) {
				t.Errorf("classifyFile(%q) = %v, want signals containing %v", tt.path, signals, tt.expected)
			}
		})
	}
}

func TestClassifyFile_ShellSignal(t *testing.T) {
	tests := []struct {
		path     string
		expected []RiskSignal
	}{
		{"deploy.sh", []RiskSignal{SignalShell}},
		{"scripts/build.bash", []RiskSignal{SignalShell}},
		{"config/setup.zsh", []RiskSignal{SignalShell}},
		{".github/workflows/ci.yml", []RiskSignal{SignalShell}},
		{"main.go", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			signals := classifyFile(DiffFile{Path: tt.path})
			if !containsAll(signals, tt.expected) {
				t.Errorf("classifyFile(%q) = %v, want signals containing %v", tt.path, signals, tt.expected)
			}
		})
	}
}

func TestClassifyFile_SecuritySignal(t *testing.T) {
	tests := []struct {
		path     string
		expected []RiskSignal
	}{
		{"internal/security/policy.go", []RiskSignal{SignalSecurity}},
		{"pkg/crypto/aes.go", []RiskSignal{SignalSecurity}},
		{"encrypt.go", []RiskSignal{SignalSecurity}},
		{"certs/server.crt", []RiskSignal{SignalSecurity}},
		{"config/api-key.json", []RiskSignal{SignalSecurity}},
		{"main.go", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			signals := classifyFile(DiffFile{Path: tt.path})
			if !containsAll(signals, tt.expected) {
				t.Errorf("classifyFile(%q) = %v, want signals containing %v", tt.path, signals, tt.expected)
			}
		})
	}
}

func TestClassifyFile_MultipleSignals(t *testing.T) {
	signals := classifyFile(DiffFile{Path: "auth/.env"})
	if !containsAll(signals, []RiskSignal{SignalAuth}) {
		t.Errorf("classifyFile(\"auth/.env\") = %v, want SignalAuth", signals)
	}
}

func TestClassifyFile_CaseInsensitive(t *testing.T) {
	signals := classifyFile(DiffFile{Path: "AUTH/Login.go"})
	if !containsAll(signals, []RiskSignal{SignalAuth}) {
		t.Errorf("classifyFile(\"AUTH/Login.go\") = %v, want SignalAuth", signals)
	}
}

// ---- classifyFiles ----

func TestClassifyFiles_NoSignals(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 5, Deletions: 0},
		{Path: "README.md", Additions: 1, Deletions: 1},
	}
	signals := classifyFiles(files, false)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals, got %v", signals)
	}
}

func TestClassifyFiles_WithExecMode(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 5, Deletions: 0},
	}
	signals := classifyFiles(files, true)
	if !containsAll(signals, []RiskSignal{SignalExecMode}) {
		t.Errorf("expected SignalExecMode, got %v", signals)
	}
}

func TestClassifyFiles_Deduplicates(t *testing.T) {
	files := []DiffFile{
		{Path: "auth/login.go", Additions: 5, Deletions: 0},
		{Path: "auth/register.go", Additions: 3, Deletions: 1},
	}
	signals := classifyFiles(files, false)
	if len(signals) != 1 {
		t.Errorf("expected 1 unique signal (auth), got %v", signals)
	}
	if signals[0] != SignalAuth {
		t.Errorf("expected SignalAuth, got %s", signals[0])
	}
}

// ---- assignRiskLevel ----

func TestAssignRiskLevel_HighWithSignals(t *testing.T) {
	files := []DiffFile{
		{Path: "auth/login.go", Additions: 1, Deletions: 0},
	}
	signals := []RiskSignal{SignalAuth}
	level := assignRiskLevel(files, signals)
	if level != RiskHigh {
		t.Errorf("expected RiskHigh, got %s", level)
	}
}

func TestAssignRiskLevel_MediumOver100Lines(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 60, Deletions: 50},
	}
	level := assignRiskLevel(files, nil)
	if level != RiskMedium {
		t.Errorf("expected RiskMedium for 110 lines, got %s", level)
	}
}

func TestAssignRiskLevel_LowUnder100Lines(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 50, Deletions: 30},
	}
	level := assignRiskLevel(files, nil)
	if level != RiskLow {
		t.Errorf("expected RiskLow for 80 lines, got %s", level)
	}
}

func TestAssignRiskLevel_LowExactly100LinesNoSignals(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 77, Deletions: 23},
	}
	level := assignRiskLevel(files, nil)
	if level != RiskLow {
		t.Errorf("expected RiskLow for exactly 100 lines with no signals, got %s", level)
	}
}

func TestAssignRiskLevel_HighOverridesLines(t *testing.T) {
	files := []DiffFile{
		{Path: "auth/login.go", Additions: 200, Deletions: 100},
	}
	signals := []RiskSignal{SignalAuth}
	level := assignRiskLevel(files, signals)
	if level != RiskHigh {
		t.Errorf("expected RiskHigh when signals present even with many lines, got %s", level)
	}
}

// ---- buildFindings ----

func TestBuildFindings_OverviewFields(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 5, Deletions: 3},
	}
	findings := buildFindings(files, nil, RiskLow)

	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding")
	}

	overview := findings[0]
	if overview.ID != "risk-overview" {
		t.Errorf("overview ID = %q, want %q", overview.ID, "risk-overview")
	}
	if overview.Severity != "info" {
		t.Errorf("overview severity for RiskLow = %q, want %q", overview.Severity, "info")
	}
}

func TestBuildFindings_PerFileSignals(t *testing.T) {
	files := []DiffFile{
		{Path: "auth/login.go", Additions: 5, Deletions: 0},
		{Path: "deploy.sh", Additions: 3, Deletions: 1},
	}
	signals := classifyFiles(files, false)
	level := assignRiskLevel(files, signals)
	findings := buildFindings(files, signals, level)

	// Should have: 1 overview + 2 per-file findings
	if len(findings) != 3 {
		t.Errorf("expected 3 findings (1 overview + 2 per-file), got %d", len(findings))
	}

	// Check per-file finding fields
	authFinding := findFindingByID(findings, "risk-auth-1")
	if authFinding == nil {
		t.Fatal("expected finding risk-auth-1")
	}
	if authFinding.Severity != "high" {
		t.Errorf("auth finding severity = %q, want %q", authFinding.Severity, "high")
	}
	if authFinding.File != "auth/login.go" {
		t.Errorf("auth finding File = %q, want %q", authFinding.File, "auth/login.go")
	}

	shellFinding := findFindingByID(findings, "risk-shell-2")
	if shellFinding == nil {
		t.Fatal("expected finding risk-shell-2")
	}
	if shellFinding.Severity != "medium" {
		t.Errorf("shell finding severity = %q, want %q", shellFinding.Severity, "medium")
	}
}

func TestBuildFindings_ExecModeFinding(t *testing.T) {
	files := []DiffFile{
		{Path: "main.go", Additions: 5, Deletions: 0},
	}
	signals := []RiskSignal{SignalExecMode}
	findings := buildFindings(files, signals, RiskHigh)

	execFinding := findFindingByID(findings, "risk-exec-mode")
	if execFinding == nil {
		t.Fatal("expected finding risk-exec-mode")
	}
	if execFinding.Severity != "medium" {
		t.Errorf("exec mode finding severity = %q, want %q", execFinding.Severity, "medium")
	}
}

func TestBuildFindings_HighSeverityOverview(t *testing.T) {
	files := []DiffFile{
		{Path: "auth/token.go", Additions: 5, Deletions: 0},
	}
	signals := []RiskSignal{SignalAuth}
	findings := buildFindings(files, signals, RiskHigh)

	overview := findings[0]
	if overview.Severity != "critical" {
		t.Errorf("RiskHigh overview severity = %q, want %q", overview.Severity, "critical")
	}
	if overview.ID != "risk-overview" {
		t.Errorf("first finding ID = %q, want %q", overview.ID, "risk-overview")
	}
}

// ---- RiskLens interface compliance ----

func TestRiskLens_ID(t *testing.T) {
	lens := &RiskLens{}
	if id := lens.ID(); id != "risk" {
		t.Errorf("ID() = %q, want %q", id, "risk")
	}
}

func TestRiskLens_Name(t *testing.T) {
	lens := &RiskLens{}
	if name := lens.Name(); name != "Risk Assessment" {
		t.Errorf("Name() = %q, want %q", name, "Risk Assessment")
	}
}

func TestRiskLens_Version(t *testing.T) {
	lens := &RiskLens{}
	if v := lens.Version(); v != "1.0.0" {
		t.Errorf("Version() = %q, want %q", v, "1.0.0")
	}
}

func TestRiskLens_Policies(t *testing.T) {
	lens := &RiskLens{}
	policies := lens.Policies()
	if policies == nil {
		t.Errorf("Policies() returned nil, expected empty slice")
	}
	if len(policies) != 0 {
		t.Errorf("Policies() = %v, want empty", policies)
	}
}

func TestRiskLens_ImplementsLensPlugin(t *testing.T) {
	// Compile-time interface check
	var _ plugin.LensPlugin = (*RiskLens)(nil)
}

// ---- Analyze with mock subject ----
// Analyze runs git commands and is not fully testable without a git repo.
// This test verifies the error path when the subject Repository path is invalid.

func TestRiskLens_Analyze_InvalidRepo(t *testing.T) {
	lens := &RiskLens{}
	subject := model.ReviewSubject{
		Repository: "/nonexistent/path",
		CommitSHA:  "abc123",
	}
	_, err := lens.Analyze(context.Background(), subject)
	if err == nil {
		t.Fatal("expected error for invalid repo path, got nil")
	}
}

// ---- helpers ----

func containsAll(signals []RiskSignal, want []RiskSignal) bool {
	for _, w := range want {
		found := false
		for _, s := range signals {
			if s == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func findFindingByID(findings []plugin.Finding, id string) *plugin.Finding {
	for i := range findings {
		if findings[i].ID == id {
			return &findings[i]
		}
	}
	return nil
}
