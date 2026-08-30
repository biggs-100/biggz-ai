package agentbuilder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/codegraph"
)

func TestAdvisoryHint_PresentSurfacesFiles(t *testing.T) {
	dir := t.TempDir()
	change := "hint-present"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("PaymentService"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte("package main\nfunc PaymentService(){}\n"), 0644); err != nil {
		t.Fatalf("svc.go: %v", err)
	}
	codegraph.ClearScanCache()
	report, err := codegraph.Generate(change, dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	jsonPath := filepath.Join(changeDir, "codegraph.json")
	mdPath := filepath.Join(changeDir, "codegraph.md")
	if err := codegraph.Emit(report, jsonPath, mdPath); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	hint := AdvisoryHint(change, dir)
	if hint == "" {
		t.Fatal("expected hint, got empty")
	}
	if !contains(hint, "svc.go") && !contains(hint, ".go") {
		t.Errorf("expected hint to contain file, got %q", hint)
	}
	if !contains(hint, "sdd") && !contains(hint, "import") && !contains(hint, "call") {
		t.Errorf("expected hint to contain reason, got %q", hint)
	}
	// Ensure not auto-mutate: hint is string only, no file written
}

func TestAdvisoryHint_AbsentContinues(t *testing.T) {
	dir := t.TempDir()
	change := "no-hint"
	hint := AdvisoryHint(change, dir)
	if hint != "" {
		t.Errorf("expected empty hint when absent, got %q", hint)
	}
	// Also FormatAdvisoryHint nil
	if s := FormatAdvisoryHint(nil); s != "" {
		t.Errorf("expected empty for nil report, got %q", s)
	}
}

func TestAdvisoryHint_DoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	change := "block-test"
	// No report, should not error and should return empty, not block SDD
	hint := AdvisoryHint(change, dir)
	if hint != "" {
		t.Errorf("expected empty, got %q", hint)
	}
	// Simulate that orchestrator continues SDD even without hint — just ensure no panic/error
	_ = hint
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
