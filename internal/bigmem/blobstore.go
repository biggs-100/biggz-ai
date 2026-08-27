package bigmem

import (
	"crypto/sha256"
	"errors"
	"fmt"
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
func BlobRoot() string {
	return filepath.Join(filepath.Dir(defaultBigmemRoot()), "blobs")
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
func GetBlob(addr string) ([]byte, error) {
	hex, err := ValidateAddr(addr)
	if err != nil {
		return nil, err
	}
	// Guard path is inside BlobRoot (hex-only ensures no traversal).
	path := filepath.Join(BlobRoot(), hex)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBlobNotFound
		}
		return nil, err
	}
	return data, nil
}
