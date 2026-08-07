// Package contracts implements the wire-envelope formalization engine over
// the embedded contracts/ tree: JSON Schema compilation and validation that
// resolve ONLY from the embedded FS, never the network.
//
// Validation stance (inherited from gentle-ai): this package exists for
// CI-time conformance of emitted bytes and opt-in test helpers — it is NEVER
// a runtime path of the engine. Every function here is read-only: nothing
// mutates or rejects existing ledgers (additive-only rule).
package contracts

import (
	"embed"

	contractassets "github.com/biggs-100/biggz-ai/contracts"
)

// FS is the embedded contract tree (schemas + fixtures). The embed lives in
// the repo-root contracts package (go:embed patterns cannot reach parent
// directories); this package re-exports it so the engine and tests consume
// the canonical tree through one symbol.
var FS embed.FS = contractassets.FS
