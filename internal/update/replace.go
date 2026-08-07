package update

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

// ErrWindowsBinaryLock is returned by ReplaceBinary on Windows because the
// running .exe cannot be overwritten while in use.
var ErrWindowsBinaryLock = errors.New("cannot replace running binary on Windows; use ReplaceHint")

// ReplaceBinary performs an atomic rename of src to dst on Unix systems.
// On Windows, it returns ErrWindowsBinaryLock.
//
// The rename is atomic when src and dst are on the same filesystem, which
// is the common case for temporary downloads in the same directory.
func ReplaceBinary(src, dst string) error {
	if runtime.GOOS == "windows" {
		return ErrWindowsBinaryLock
	}
	return os.Rename(src, dst)
}

// ReplaceHint returns a platform-appropriate message explaining how to
// manually replace the binary. On Windows, this prints a go install command
// since the running .exe cannot be overwritten.
//
// The modulePath should be the Go module path (e.g., "github.com/biggs-100/biggz-ai").
func ReplaceHint(modulePath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Download complete. To update, run:\n  go install %s@latest", modulePath)
	}
	return "Binary replaced successfully."
}
