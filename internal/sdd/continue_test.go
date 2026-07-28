package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextPhase_EmptyChange(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "my-change")
	os.MkdirAll(changeDir, 0755)

	phase, err := NextPhase(root, "my-change")
	if err != nil {
		t.Fatalf("NextPhase() error: %v", err)
	}
	if phase != "proposal" {
		t.Errorf("expected proposal, got %s", phase)
	}
}

func TestNextPhase_AfterProposal(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "my-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P"), 0644)

	phase, err := NextPhase(root, "my-change")
	if err != nil {
		t.Fatalf("NextPhase() error: %v", err)
	}
	// No specs yet → spec
	if phase != "spec" {
		t.Errorf("expected spec, got %s", phase)
	}
}

func TestNextPhase_Complete(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "my-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P"), 0644)
	// Specs exist in openspec/specs/ (not change dir — that's normal for new capabilities)
	specDir := filepath.Join(root, "specs", "core-review")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# S"), 0644)
	os.WriteFile(filepath.Join(changeDir, "design.md"), []byte("# D"), 0644)
	os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [x] Task 1\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "apply-progress.md"), []byte("# A"), 0644)
	os.WriteFile(filepath.Join(changeDir, "verify-report.md"), []byte("# V"), 0644)

	// Specs should be in openspec/specs/ — the change dir might have delta specs
	// For now, just check that without specs in change dir, it still works
	// Actually, the current implementation checks change/specs/ only
	// Let's create specs in the change dir for this test
	specDir2 := filepath.Join(changeDir, "specs", "core-review")
	os.MkdirAll(specDir2, 0755)
	os.WriteFile(filepath.Join(specDir2, "spec.md"), []byte("# S"), 0644)

	phase, err := NextPhase(root, "my-change")
	if err != nil {
		t.Fatalf("NextPhase() error: %v", err)
	}
	if phase != "archive" {
		t.Errorf("expected archive, got %s", phase)
	}
}

func TestNextPhase_PartialTasks(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "my-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P"), 0644)
	specDir := filepath.Join(changeDir, "specs", "core-review")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("# S"), 0644)
	os.WriteFile(filepath.Join(changeDir, "design.md"), []byte("# D"), 0644)
	os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [x] Task 1\n- [ ] Task 2\n"), 0644)

	phase, err := NextPhase(root, "my-change")
	if err != nil {
		t.Fatalf("NextPhase() error: %v", err)
	}
	if !strings.HasPrefix(phase, "apply") {
		t.Errorf("expected apply, got %s", phase)
	}
}

func TestNextPhase_ChangeNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := NextPhase(root, "no-such-change")
	if err == nil {
		t.Fatal("expected error for non-existent change")
	}
}
