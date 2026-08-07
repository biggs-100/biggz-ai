// Package contracts embeds the frozen wire-envelope formalization layer
// (JSON Schemas + fixtures). The embed MUST live here, beside the data:
// go:embed patterns cannot reach parent directories, so the repo-root
// contracts/ tree is the only embeddable home for the canonical files.
// internal/contracts re-exports this FS and implements the validation
// engine on top of it.
package contracts

import "embed"

//go:embed all:review-integration all:sdd-integration
var FS embed.FS
