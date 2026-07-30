package filemerge

import (
	"bytes"
	"os"
	"path/filepath"
)

// WriteResult describes what happened during a WriteFileAtomic call.
type WriteResult struct {
	Changed bool // content was different from existing (new content written)
	Created bool // file did not exist before (created new file)
}

// WriteFile writes content to path atomically by writing to a temp file in the
// same directory first, then renaming to the target path. This ensures the
// write either completes fully or leaves the original file unchanged.
// The perm parameter sets the file mode on the final file.
func WriteFile(path string, content []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

// WriteFileAtomic atomically writes content to path. It reads the existing file
// (if any) and compares its bytes to content. If identical, it skips the write
// and returns WriteResult{Changed: false, Created: false}. If the file does not
// exist, it creates it via temp+rename and returns Created: true. If the file
// exists and content differs, it overwrites atomically and returns Changed: true.
// Parent directories MUST exist — callers are responsible for MkdirAll.
func WriteFileAtomic(path string, content []byte, perm os.FileMode) (WriteResult, error) {
	// Read existing content to compare
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return WriteResult{}, nil // unchanged
	}
	created := os.IsNotExist(err)

	// Write to temp file in the same directory, then rename
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return WriteResult{}, err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(content); err != nil {
		return WriteResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return WriteResult{}, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return WriteResult{}, err
	}
	if err := os.Chmod(path, perm); err != nil {
		return WriteResult{}, err
	}

	return WriteResult{Changed: !created, Created: created}, nil
}
