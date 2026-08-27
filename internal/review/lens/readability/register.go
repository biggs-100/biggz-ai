package readability

import "github.com/biggs-100/biggz-ai/internal/review/lens"

// Registrar is the minimal ExtensionAPI surface needed for lens registration.
// It matches extension.ExtensionAPI.RegisterLens without importing extension
// to avoid import cycle (readability -> extension -> lens).
type Registrar interface {
	RegisterLens(l lens.Lens)
}

// Register registers the readability lens via ExtensionAPI.
// Only this lens is migrated via ExtensionAPI; other lenses stay on legacy registry.
// Lens.Analyze remains pure and does not import internal/extension.
func Register(api Registrar) {
	api.RegisterLens(&Lens{})
}
