// Package assets provides embedded skill files and OpenCode configuration
// overlays that are deployed during agent installation.
package assets

import "embed"

//go:embed all:skills all:opencode all:biggz all:prompts all:prompts/review all:generic all:pi
var FS embed.FS
