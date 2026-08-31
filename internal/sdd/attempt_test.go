package sdd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyGuard(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	// Begin must fail closed without creating file
	_, err := AttemptBegin(root, "test-change", 400)
	if err == nil {
		t.Fatal("AttemptBegin must return ErrLegacyRetired")
	}
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptBegin err = %v, want ErrLegacyRetired", err)
	}
	if !strings.Contains(err.Error(), "biggz sdd-attempt acquire") {
		t.Fatalf("AttemptBegin err %q must mention biggz sdd-attempt acquire", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(changeDir, ".attempt.json")); !os.IsNotExist(statErr) {
		t.Fatal("AttemptBegin must not create .attempt.json")
	}

	// Finish must return ErrLegacyRetired and not mutate file
	// Seed a file first
	seed := []byte(`{"change_name":"test-change","status":"idle"}`)
	os.WriteFile(filepath.Join(changeDir, ".attempt.json"), seed, 0644)
	_, err = AttemptFinish(root, "test-change", true, 10)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptFinish err = %v, want ErrLegacyRetired", err)
	}
	if !strings.Contains(err.Error(), "biggz sdd-attempt") {
		t.Fatalf("AttemptFinish err %q must mention biggz sdd-attempt", err.Error())
	}
	data, _ := os.ReadFile(filepath.Join(changeDir, ".attempt.json"))
	if string(data) != string(seed) {
		t.Fatalf("AttemptFinish mutated file: got %q want %q", string(data), string(seed))
	}

	// Reset must return ErrLegacyRetired and not mutate
	_, err = AttemptReset(root, "test-change")
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptReset err = %v, want ErrLegacyRetired", err)
	}
	data2, _ := os.ReadFile(filepath.Join(changeDir, ".attempt.json"))
	if string(data2) != string(seed) {
		t.Fatalf("AttemptReset mutated file: got %q want %q", string(data2), string(seed))
	}
}

// Legacy TestAttemptBegin now expects ErrLegacyRetired (guard)
func TestAttemptBegin(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	_, err := AttemptBegin(root, "test-change", 400)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptBegin() error: %v, want ErrLegacyRetired", err)
	}
}

func TestAttemptBegin_AlreadyInProgress(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	_, err := AttemptBegin(root, "test-change", 400)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("expected ErrLegacyRetired, got %v", err)
	}
	_, err = AttemptBegin(root, "test-change", 400)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("second AttemptBegin expected ErrLegacyRetired, got %v", err)
	}
}

func TestAttemptFinish_Success(t *testing.T) {
	root := t.TempDir()
	_, err := AttemptFinish(root, "test-change", true, 50)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptFinish() error: %v, want ErrLegacyRetired", err)
	}
}

func TestAttemptFinish_FailureWithRetries(t *testing.T) {
	root := t.TempDir()
	_, err := AttemptFinish(root, "test-change", false, 100)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptFinish() error: %v, want ErrLegacyRetired", err)
	}
}

func TestAttemptFinish_NoActiveAttempt(t *testing.T) {
	root := t.TempDir()
	_, err := AttemptFinish(root, "test-change", true, 0)
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("expected ErrLegacyRetired for finish without begin, got %v", err)
	}
}

func TestAttemptReset(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	_, err := AttemptReset(root, "test-change")
	if !errors.Is(err, ErrLegacyRetired) {
		t.Fatalf("AttemptReset() error: %v, want ErrLegacyRetired", err)
	}
}

func TestAttemptStatus(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	// Status reads file directly; without guard it should still work for file probe,
	// but Begin is now guard. So test that Status on missing file returns error.
	_, err := AttemptStatus(root, "test-change")
	if err == nil {
		t.Fatal("expected error for missing status")
	}
}
