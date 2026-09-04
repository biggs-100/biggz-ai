package bigmem

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BlobStore externalizes payloads >100KB or data:image/ to ~/.biggz/blobs/<sha256>.

const BlobPrefix = "blob:sha256:"

var blobAddrRe = regexp.MustCompile(`^blob:sha256:[0-9a-f]{64}$`)

// ErrInvalidAddr is returned when a blob address fails validation (regex, traversal).
var ErrInvalidAddr = errors.New("invalid blob address")

// ErrBlobNotFound is returned when a valid address has no file on disk.
var ErrBlobNotFound = errors.New("blob not found")

// BlobRoot returns the blob directory: sibling to bigmem DB (~/.biggz/blobs).
// Mirrors oh-my-pi BlobStore but isolated at ~/.biggz (not ~/.omp).
// Empty $HOME returns "" without XDG_RUNTIME_DIR fallback (REQ-SD-B5/O3).
func BlobRoot() string {
	root := defaultBigmemRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(root), "blobs")
}

// IsBlobAddr reports whether s matches blob:sha256:<64hex>.
func IsBlobAddr(s string) bool {
	return blobAddrRe.MatchString(s)
}

// ValidateAddr validates a blob address, rejecting traversal.
// Returns hex without prefix on success.
func ValidateAddr(a string) (string, error) {
	if !blobAddrRe.MatchString(a) {
		return "", ErrInvalidAddr
	}
	if strings.Contains(a, "..") || strings.Contains(a, "/../") {
		return "", ErrInvalidAddr
	}
	hex := strings.TrimPrefix(a, BlobPrefix)
	// Extra guard: hex must be exactly 64 lower-hex chars (regex already ensures)
	if len(hex) != 64 {
		return "", ErrInvalidAddr
	}
	return hex, nil
}

// PutBlob hashes b with SHA-256, writes atomically to BlobRoot/<hex> via temp+rename
// with write-if-not-exists dedup, and returns blob:sha256:<hex>.
func PutBlob(b []byte) (string, error) {
	h := sha256.Sum256(b)
	hex := fmt.Sprintf("%x", h)
	addr := BlobPrefix + hex
	root := BlobRoot()
	if root == "" {
		return "", fmt.Errorf("home dir: not found — blob unavailable")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	dest := filepath.Join(root, hex)
	// Fast dedup: file already exists.
	if _, err := os.Stat(dest); err == nil {
		return addr, nil
	}
	// Atomic write via temp file in same directory.
	tmpFile, err := os.CreateTemp(root, ".tmp-blob-*")
	if err != nil {
		return "", err
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(b); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpName)
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	_ = os.Chmod(tmpName, 0644)
	// Recheck dedup after temp write (concurrent PutBlob).
	if _, err := os.Stat(dest); err == nil {
		_ = os.Remove(tmpName)
		return addr, nil
	}
	// Try rename; on Windows rename fails if dest exists (EEXIST), on POSIX it overwrites but
	// content is identical so no corruption. Handle exists case gracefully.
	if err := os.Rename(tmpName, dest); err != nil {
		if _, statErr := os.Stat(dest); statErr == nil {
			_ = os.Remove(tmpName)
			return addr, nil
		}
		_ = os.Remove(tmpName)
		return "", err
	}
	return addr, nil
}

// GetBlob validates addr and reads the blob file. Returns ErrInvalidAddr or ErrBlobNotFound.
// See docs/bigmem-DOCS.md for the blob lifecycle (Put→addr→Get→DoctorFixBlobs).
func GetBlob(addr string) ([]byte, error) {
	hex, err := ValidateAddr(addr)
	if err != nil {
		return nil, err
	}
	root := BlobRoot()
	if root == "" {
		return nil, fmt.Errorf("home dir: not found — blob unavailable")
	}
	// Guard path is inside BlobRoot (hex-only ensures no traversal).
	path := filepath.Join(root, hex)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBlobNotFound
		}
		return nil, err
	}
	return data, nil
}

// MissingBlobPrefix marks an in-memory missing-blob marker. Markers are
// display-only: never persisted, never fed back to GetBlob (threat-matrix:
// marker injection is N/A — helper checks IsBlobAddr first).
const MissingBlobPrefix = "[missing-blob "

// MissingBlobMarker returns the explicit miss marker embedding addr:
// "[missing-blob blob:sha256:<hex>]" (migration-safe, grep-able).
// The anchored IsBlobAddr regex never matches a marker, so no resolve loop.
func MissingBlobMarker(addr string) string {
	return MissingBlobPrefix + addr + "]"
}

// IsMissingBlobMarker reports whether s is exactly a missing-blob marker
// with a well-formed embedded addr (prefix + addr-shape + suffix check).
func IsMissingBlobMarker(s string) bool {
	if !strings.HasPrefix(s, MissingBlobPrefix) || !strings.HasSuffix(s, "]") {
		return false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, MissingBlobPrefix), "]")
	return IsBlobAddr(inner)
}

// ResolveBlobOrMarker resolves blob content or returns an explicit marker.
// Hit → raw bytes; miss (valid addr, no file) → marker + log; non-addr →
// passthrough untouched (literal marker text never reaches the filesystem).
// It never touches the DB: Get/Search mutate only the returned struct so a
// restored blob file self-heals on the next read.
func ResolveBlobOrMarker(content string) string {
	if !IsBlobAddr(content) {
		return content
	}
	data, err := GetBlob(content)
	if err == nil {
		return string(data)
	}
	log.Printf("[bigmem] blob missing: %s (%v) — returning marker, DB unmutated", content, err)
	return MissingBlobMarker(content)
}
