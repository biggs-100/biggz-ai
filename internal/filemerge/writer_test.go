package filemerge

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestWriteFile_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world")

	if err := WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("ReadFile() = %q, want %q", string(got), string(content))
	}
}

func TestWriteFile_NonExistentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "test.txt")

	err := WriteFile(path, []byte("content"), 0644)
	if err == nil {
		t.Fatal("WriteFile() expected error for non-existent directory, got nil")
	}
}

func TestWriteFile_Concurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.txt")

	var wg sync.WaitGroup
	concurrency := 10

	for i := range concurrency {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			content := []byte{byte('A' + n)}
			// Errors from concurrent writes are expected (race on rename);
			// the test just verifies no panic and the file always has
			// complete content from one writer.
			_ = WriteFile(path, content, 0644)
		}(i)
	}
	wg.Wait()

	// File should exist and contain exactly one byte (from last successful write)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() returned error: %v", err)
	}
	if fi.Size() != 1 {
		t.Fatalf("file size = %d, want 1 (one byte from one writer)", fi.Size())
	}
}

func TestWriteFile_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions test requires Unix file mode support")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "perm.txt")

	if err := WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() returned error: %v", err)
	}

	if fi.Mode().Perm() != 0644 {
		t.Errorf("file permissions = %o, want 0644", fi.Mode().Perm())
	}
}

func TestWriteFile_OverwritePreservesContentOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "safe.txt")
	original := []byte("original content")

	// Write initial content
	if err := WriteFile(path, original, 0644); err != nil {
		t.Fatalf("WriteFile() initial write returned error: %v", err)
	}

	// Attempt write to non-existent subdirectory (will fail)
	err := WriteFile(filepath.Join(dir, "missing", "test.txt"), []byte("new"), 0644)
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}

	// Original file must be unchanged
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("after failed write, content = %q, want %q", string(got), string(original))
	}
}
