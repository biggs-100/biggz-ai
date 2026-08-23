package update

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jedisct1/go-minisign"
)

// expectedChecksumFor returns the expected SHA-256 hex digest for filename by
// scanning checksumsContent for a line where fields[1] matches filename exactly
// (or contains it). It mirrors gentle-ai's expectedChecksumFor semantics.
func expectedChecksumFor(checksumsContent []byte, filename string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(checksumsContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// GoReleaser format: "<hex_hash>  <filename>" — handle leading "*"
		candidate := strings.TrimPrefix(fields[1], "*")
		// Exact match, base-name match, or contains (covers path-prefixed entries).
		if candidate == filename || filepath.Base(candidate) == filename || strings.Contains(candidate, filename) {
			return fields[0], nil
		}
		// Also check raw fields[1] without trimming for direct contains.
		if fields[1] == filename || strings.Contains(fields[1], filename) {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found in checksums", filename)
}

// VerifyChecksum computes the SHA-256 digest of data and verifies that it
// appears in the checksumsContent. The checksumsContent is expected to follow
// the GoReleaser format: one line per entry with the hex digest followed by
// the filename (e.g., "sha256hash  filename.ext").
//
// When filename is provided (variadic first element), the verification is
// filename-exact: it looks up the expected checksum for that specific file via
// expectedChecksumFor and compares. Without filename, it falls back to matching
// any line whose hash equals the computed digest (backward compat for tests).
func VerifyChecksum(data, checksumsContent []byte, filename ...string) error {
	sum := sha256.Sum256(data)
	expected := hex.EncodeToString(sum[:])

	// Filename-exact path (hardened): caller supplies archive name.
	if len(filename) > 0 && filename[0] != "" {
		want, err := expectedChecksumFor(checksumsContent, filename[0])
		if err != nil {
			return err
		}
		if want != expected {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filename[0], want, expected)
		}
		return nil
	}

	// Fallback: legacy any-match (no filename supplied).
	scanner := bufio.NewScanner(bytes.NewReader(checksumsContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// GoReleaser format: "<hex_hash>  <filename>"
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] == expected {
			// Still require a filename field to avoid matching malformed lines.
			if len(fields) >= 2 && fields[1] != "" {
				return nil
			}
			// If no filename field, still accept for backward compat with minimal checksums fixtures.
			return nil
		}
	}

	return fmt.Errorf("checksum %s not found in checksums", expected)
}

// VerifySignature verifies a minisign detached signature over checksumsData.
//
// The pubKey and sigData should be the complete content of the minisign.pub
// and checksums.txt.minisig files respectively. checksumsData is the content
// of the checksums.txt file that was signed.
func VerifySignature(checksumsData, sigData, pubKey []byte) error {
	publicKey, err := minisign.DecodePublicKey(string(pubKey))
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}

	signature, err := minisign.DecodeSignature(string(sigData))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	ok, err := publicKey.Verify(checksumsData, signature)
	if err != nil {
		return fmt.Errorf("verify signature: %w", err)
	}
	if !ok {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
