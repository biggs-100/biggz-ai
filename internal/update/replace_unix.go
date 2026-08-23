//go:build !windows

package update

import (
	"errors"
	"fmt"
	"os"
)

// ErrWindowsBinaryLock is kept for API compatibility; it is never returned on Unix.
var ErrWindowsBinaryLock = errors.New("cannot replace running binary on Windows; use ReplaceHint")

// ReplaceBinary performs an atomic rename of src to dst. The rename is atomic
// when src and dst are on the same filesystem, which is the common case for
// temporary downloads in the same directory.
func ReplaceBinary(src, dst string) error {
	return os.Rename(src, dst)
}

// ReplaceHint returns a platform-appropriate message. On Unix the binary was
// replaced atomically.
func ReplaceHint(_ string) string {
	return "Binary replaced successfully."
}

// Ensure fmt is used on Unix build (ReplaceHint ignores modulePath but keeps
// the signature stable).
var _ = fmt.Sprintf
