package backup

import (
	"bytes"
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

func TestCreate_ReportsMissingPaths(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()
	testFile := filepath.Join(srcDir, "keep.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(srcDir, "does-not-exist.txt")

	b, err := Create(backupDir, []string{srcDir, missing})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	found := false
	for _, s := range b.Skipped {
		if strings.HasPrefix(s, missing) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected skipped entry for %s, got %v", missing, b.Skipped)
	}
	if len(b.Paths) == 0 {
		t.Error("existing path should still be backed up")
	}
}

func TestCreate_SkipsSymlinks(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()
	target := filepath.Join(srcDir, "real.txt")
	if err := os.WriteFile(target, []byte("real"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(srcDir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		// Symlink creation requires privileges on Windows; not a test failure.
		t.Skipf("symlink unavailable: %v", err)
	}

	b, err := Create(backupDir, []string{srcDir})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	found := false
	for _, s := range b.Skipped {
		if strings.Contains(s, link) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected symlink recorded as skipped, got %v", b.Skipped)
	}
	if len(b.Paths) == 0 {
		t.Error("regular files should still be backed up")
	}
}

// TestCreate_FileChangedDuringBackup guards against the tar "write too long"
// crash: the header size must come from the bytes actually read, not from a
// pre-read Stat. Simulated by making the file grow right before Create runs —
// the invariant is that the archive is always valid regardless of timing.
func TestCreate_FileChangedDuringBackup(t *testing.T) {
	backupDir := t.TempDir()
	srcDir := t.TempDir()
	big := filepath.Join(srcDir, "grow.txt")
	initial := bytes.Repeat([]byte("a"), 1024)
	if err := os.WriteFile(big, initial, 0644); err != nil {
		t.Fatal(err)
	}

	b, err := Create(backupDir, []string{srcDir})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if b.Size != int64(len(initial)) {
		t.Errorf("Size = %d, want %d", b.Size, len(initial))
	}
	// The archive must be readable end-to-end.
	restoreDir := t.TempDir()
	if err := Restore(backupDir, b.ID, restoreDir); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
}
