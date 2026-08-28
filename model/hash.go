package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	EvidenceDomain = "biggz-ai.review-evidence/v1"
	MerkleDomain   = "biggz-ai.review-merkle/v1"
)

// writeLengthPrefixed encodes fields with u32 BE length prefix.
func writeLengthPrefixed(fields ...[]byte) []byte {
	var buf bytes.Buffer
	for _, f := range fields {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(f)))
		buf.Write(l[:])
		buf.Write(f)
	}
	return buf.Bytes()
}

// domainHash computes sha256(domain + "\x00" + payload) with "sha256:" prefix.
func domainHash(domain string, payload []byte) string {
	h := sha256.Sum256(append([]byte(domain+"\x00"), payload...))
	return "sha256:" + hex.EncodeToString(h[:])
}

// evidenceHash computes the hash as domainHash with length-prefixed fields:
// Position, Timestamp, Kind, Payload, PrevHash.
func evidenceHash(e Evidence) string {
	payload := writeLengthPrefixed(
		[]byte(fmt.Sprintf("%d", e.Position)),
		[]byte(e.Timestamp.Format(time.RFC3339Nano)),
		[]byte(e.Kind),
		[]byte(e.Payload),
		[]byte(e.PrevHash),
	)
	return domainHash(EvidenceDomain, payload)
}

// AppendEvidence appends a new evidence entry to the chain, computing
// its PrevHash from the previous tail entry and its own Hash from its
// content fields. It returns a new slice; the original is not modified.
//
// The Kind classifies the evidence (e.g. "lens_result", "policy_verdict",
// "provider_response"). Payload is an arbitrary JSON string.
func AppendEvidence(chain []Evidence, kind, payload string) []Evidence {
	n := len(chain)
	var prevHash string
	if n > 0 {
		prevHash = chain[n-1].Hash
	}

	entry := Evidence{
		Position:  n + 1,
		Timestamp: time.Now(),
		Kind:      kind,
		Payload:   payload,
		PrevHash:  prevHash,
	}
	entry.Hash = evidenceHash(entry)

	return append(chain, entry)
}

// MerkleRoot returns the Merkle root as domainHash of the last entry's Hash.
func MerkleRoot(chain []Evidence) string {
	n := len(chain)
	if n == 0 {
		return ""
	}
	payload := writeLengthPrefixed([]byte(chain[n-1].Hash))
	return domainHash(MerkleDomain, payload)
}
