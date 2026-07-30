package update

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jedisct1/go-minisign"
)

// VerifyChecksum computes the SHA-256 digest of data and verifies that it
// appears in the checksumsContent. The checksumsContent is expected to follow
// the GoReleaser format: one line per entry with the hex digest followed by
// the filename (e.g., "sha256hash  filename.ext").
//
// Returns nil when the digest is found in one of the checksum entries.
func VerifyChecksum(data, checksumsContent []byte) error {
	sum := sha256.Sum256(data)
	expected := hex.EncodeToString(sum[:])

	scanner := bufio.NewScanner(bytes.NewReader(checksumsContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// GoReleaser format: "<hex_hash>  <filename>"
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] == expected {
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
