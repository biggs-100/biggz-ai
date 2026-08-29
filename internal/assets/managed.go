package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// ManagedAssetHash computes SHA256 hex for the given data.
// It mirrors gentle-pi/lib/sdd-preflight.ts managedAssetHash.
func ManagedAssetHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ManagedAssetHashFile computes SHA256 hex for the file at path.
// Returns hex string or error if file cannot be read.
func ManagedAssetHashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return ManagedAssetHash(data), nil
}
