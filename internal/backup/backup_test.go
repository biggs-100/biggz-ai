package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAndList(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(srcDir, "test.txt")
	os.WriteFile(testFile, []byte("hello backup"), 0644)

	// Create backup
	b, err := Create(backupDir, []string{testFile})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if b.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if !strings.HasPrefix(b.ID, "backup-") {
		t.Errorf("expected backup- prefix, got %s", b.ID)
	}

	// List
	backups, err := List(backupDir)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(backups) != 1 {
		t.Errorf("expected 1 backup, got %d", len(backups))
	}
}

func TestCreateAndRestore(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()
	restoreDir := t.TempDir()

	// Create directory structure
	subDir := filepath.Join(srcDir, "sub")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)

	// Backup the srcDir
	b, err := Create(backupDir, []string{srcDir})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Restore to restoreDir
	if err := Restore(backupDir, b.ID, restoreDir); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	// Verify restored files
	data, err := os.ReadFile(filepath.Join(restoreDir, "root.txt"))
	if err != nil {
		t.Fatalf("read restored root.txt: %v", err)
	}
	if string(data) != "root" {
		t.Errorf("root.txt content = %q, want %q", string(data), "root")
	}

	data, err = os.ReadFile(filepath.Join(restoreDir, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("read restored nested.txt: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("nested.txt content = %q, want %q", string(data), "nested")
	}
}

func TestRestore_InvalidID(t *testing.T) {
	backupDir := t.TempDir()
	err := Restore(backupDir, "nonexistent", t.TempDir())
	if err == nil {
		t.Fatal("expected error for nonexistent backup")
	}
}

func TestList_Empty(t *testing.T) {
	backupDir := t.TempDir()
	backups, err := List(backupDir)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}
}
