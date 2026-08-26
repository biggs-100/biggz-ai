package filemerge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// ComputeHash returns the SHA-256 hex digest of the exact byte range supplied.
// It hashes precisely the bytes passed in (no normalization, no whole-file
// fallback). An empty or nil slice yields the SHA-256 of the empty string,
// matching the on-disk hash of a missing/empty file.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// HashMismatchError is returned by ApplyWithHash when the on-disk content
// hash does not match the expected hash. Code is always "needs_attention"
// and FreshHash carries the current on-disk hash so callers can retry with
// up-to-date state without re-reading. The file is never overwritten in this
// case and the batch must not abort (warn-and-stop).
type HashMismatchError struct {
	Code      string // always "needs_attention"
	FreshHash string // hash of current on-disk content
	Path      string // file that mismatched
	Expected  string // hash the caller expected
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("needs_attention: %s expected %s fresh %s", e.Path, e.Expected, e.FreshHash)
}

// ApplyWithHash validates the current on-disk hash against expectedHash
// before atomically overwriting path with newContent.
//
//   - On match (or force==true) it writes atomically via WriteFileAtomic
//     and returns the hash of newContent with nil error.
//   - On mismatch it does NOT overwrite, re-reads the fresh hash, and
//     returns (freshHash, *HashMismatchError{Code:"needs_attention"}).
//   - force bypasses validation when explicitly passed as true.
//
// The variadic force parameter exists to support both the spec's 3-arg call
// shape (no force) and the force-flag requirement without breaking either
// caller. Call with 3 args for normal validation, or 4 args with true to
// force overwrite on mismatch.
func ApplyWithHash(path, expectedHash string, newContent []byte, force ...bool) (string, error) {
	forceFlag := len(force) > 0 && force[0]
	return applyWithHash(path, expectedHash, newContent, forceFlag)
}

// ApplyWithHashForce is an explicit-force alias for callers that prefer a
// non-variadic signature.
func ApplyWithHashForce(path, expectedHash string, newContent []byte, force bool) (string, error) {
	return applyWithHash(path, expectedHash, newContent, force)
}

func applyWithHash(path, expectedHash string, newContent []byte, force bool) (string, error) {
	// Read current on-disk content. Missing file is treated as empty
	// so its hash is ComputeHash(nil) (sha256 of empty string).
	current, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		current = nil
	}
	freshHash := ComputeHash(current)

	if !force && expectedHash != freshHash {
		return freshHash, &HashMismatchError{
			Code:      "needs_attention",
			FreshHash: freshHash,
			Path:      path,
			Expected:  expectedHash,
		}
	}

	// Ensure parent directory exists (WriteFileAtomic requires it). We do
	// not create it implicitly when path is just a filename in cwd, but
	// MkdirAll on the dir is safe when dir == ".".
	// Determine perm to preserve: use existing file mode when available.
	perm := os.FileMode(0644)
	if fi, statErr := os.Stat(path); statErr == nil {
		perm = fi.Mode().Perm()
	}

	// WriteFileAtomic is atomic (temp+rename). It handles Created vs
	// Changed internally and leaves the original intact on error.
	if _, wErr := WriteFileAtomic(path, newContent, perm); wErr != nil {
		// If the error is missing parent, try to create it once and retry
		// to be helpful for new-file cases without masking other errors.
		if os.IsNotExist(wErr) {
			// Attempt to surface a clearer error: let MkdirAll handle it
			// if the caller intended to create intermediate dirs. We don't
			// auto-MkdirAll here to preserve WriteFileAtomic's contract
			// ("Parent directories MUST exist — callers are responsible").
			// So just return the original error for caller to decide.
		}
		return "", wErr
	}

	// On success return hash of the newly written content (consistent with
	// freshHash semantics). Callers that need the new hash can use it.
	return ComputeHash(newContent), nil
}
