package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadChange(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal change with proposal and tasks
	changeDir := filepath.Join(dir, "test-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal"), 0644)
	os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [x] Task 1\n- [ ] Task 2\n- [x] Task 3\n"), 0644)

	cs, err := readChange(changeDir, "test-change", false)
	if err != nil {
		t.Fatalf("readChange() error: %v", err)
	}

	if !cs.HasProposal {
		t.Error("expected HasProposal = true")
	}
	if !cs.HasTasks {
		t.Error("expected HasTasks = true")
	}
	if cs.TasksTotal != 3 {
		t.Errorf("expected TasksTotal = 3, got %d", cs.TasksTotal)
	}
	if cs.TasksDone != 2 {
		t.Errorf("expected TasksDone = 2, got %d", cs.TasksDone)
	}
}

func TestStatus_NoChanges(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "changes", "archive"), 0755)

	active, archived, err := Status(dir)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active, got %d", len(active))
	}
	_ = archived
}

func TestStatus_WithActive(t *testing.T) {
	dir := t.TempDir()
	changesDir := filepath.Join(dir, "changes")
	archiveDir := filepath.Join(changesDir, "archive")
	os.MkdirAll(archiveDir, 0755)

	// Active change
	activeDir := filepath.Join(changesDir, "my-change")
	os.MkdirAll(activeDir, 0755)
	os.WriteFile(filepath.Join(activeDir, "proposal.md"), []byte("# P"), 0644)

	active, _, err := Status(dir)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestFormatStatus(t *testing.T) {
	active := []ChangeStatus{
		{Name: "my-change", HasProposal: true, TasksTotal: 3, TasksDone: 1},
	}
	archived := []ChangeStatus{
		{Name: "2026-07-27-old-change", IsArchived: true, HasProposal: true, HasVerify: true, TasksTotal: 5, TasksDone: 5},
	}

	output := FormatStatus(active, archived, StatusOptions{})
	if !strings.Contains(output, "my-change") {
		t.Error("expected output to contain 'my-change'")
	}
	if !strings.Contains(output, "old-change") {
		t.Error("expected output to contain 'old-change'")
	}
}
