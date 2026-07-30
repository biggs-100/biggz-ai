package update

import _ "embed"

//go:embed minisign.pub
var minisignPublicKey []byte

// MinissignPublicKey returns the embedded minisign public key bytes.
func MinissignPublicKey() []byte {
	return minisignPublicKey
}
