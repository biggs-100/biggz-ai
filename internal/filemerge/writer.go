package filemerge

import (
	"os"
	"path/filepath"
)

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
