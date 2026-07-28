package sdd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAttemptBegin(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	state, err := AttemptBegin(root, "test-change", 400)
	if err != nil {
		t.Fatalf("AttemptBegin() error: %v", err)
	}
	if state.Status != "in_progress" {
		t.Errorf("expected in_progress, got %s", state.Status)
	}
	if state.TotalAttempts != 1 {
		t.Errorf("expected TotalAttempts=1, got %d", state.TotalAttempts)
	}
	if state.ActiveAttempt != 1 {
		t.Errorf("expected ActiveAttempt=1, got %d", state.ActiveAttempt)
	}
}

func TestAttemptBegin_AlreadyInProgress(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	AttemptBegin(root, "test-change", 400)
	_, err := AttemptBegin(root, "test-change", 400)
	if err == nil {
		t.Fatal("expected error for double begin")
	}
}

func TestAttemptFinish_Success(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	AttemptBegin(root, "test-change", 400)
	state, err := AttemptFinish(root, "test-change", true, 50)
	if err != nil {
		t.Fatalf("AttemptFinish() error: %v", err)
	}
	if state.Status != "completed" {
		t.Errorf("expected completed, got %s", state.Status)
	}
	if state.CorrectionLines != 50 {
		t.Errorf("expected CorrectionLines=50, got %d", state.CorrectionLines)
	}
}

func TestAttemptFinish_FailureWithRetries(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	AttemptBegin(root, "test-change", 400)
	state, err := AttemptFinish(root, "test-change", false, 100)
	if err != nil {
		t.Fatalf("AttemptFinish() error: %v", err)
	}
	if state.Status != "idle" {
		t.Errorf("expected idle (retries available), got %s", state.Status)
	}
}

func TestAttemptFinish_NoActiveAttempt(t *testing.T) {
	root := t.TempDir()
	_, err := AttemptFinish(root, "test-change", true, 0)
	if err == nil {
		t.Fatal("expected error for finish without begin")
	}
}

func TestAttemptReset(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	AttemptBegin(root, "test-change", 400)
	state, err := AttemptReset(root, "test-change")
	if err != nil {
		t.Fatalf("AttemptReset() error: %v", err)
	}
	if state.Status != "idle" {
		t.Errorf("expected idle after reset, got %s", state.Status)
	}
	if state.TotalAttempts != 0 {
		t.Errorf("expected TotalAttempts=0 after reset, got %d", state.TotalAttempts)
	}
}

func TestAttemptStatus(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	AttemptBegin(root, "test-change", 400)
	state, err := AttemptStatus(root, "test-change")
	if err != nil {
		t.Fatalf("AttemptStatus() error: %v", err)
	}
	if state.Status != "in_progress" {
		t.Errorf("expected in_progress, got %s", state.Status)
	}
}
