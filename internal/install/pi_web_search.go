package install

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"

	// PR1 deps: pinned for PR2 TLS tier (chrome124/safari17) and Markdown extract.
	// Blank imports keep go.mod deps retained through `go mod tidy` before live use.
	_ "github.com/JohannesKaufmann/html-to-markdown"
	_ "github.com/bogdanfinn/tls-client"
	_ "github.com/go-shiori/dom"
	_ "github.com/go-shiori/go-readability"
	_ "github.com/refraction-networking/utls"
)

// DeployPiWebSearch writes internal/assets/pi/biggz-web-search.js to
// ~/.pi/agent/extensions/biggz-web-search.js via filemerge.WriteFileAtomic.
// It creates parent directories, is idempotent, supports TempDir via homeDir,
// and self-heals legacy stale extension variants.
func DeployPiWebSearch(ctx context.Context, homeDir string) (filemerge.WriteResult, error) {
	_ = ctx
	extensionsDir := piExtensionsDir(homeDir)
	targetPath := filepath.Join(extensionsDir, "biggz-web-search.js")

	data, err := fs.ReadFile(assets.FS, "pi/biggz-web-search.js")
	if err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("read pi web search asset: %w", err)
	}

	if err := os.MkdirAll(extensionsDir, 0755); err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("mkdir %s: %w", extensionsDir, err)
	}

	result, err := filemerge.WriteFileAtomic(targetPath, data, 0644)
	if err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("write %s: %w", targetPath, err)
	}

	// Self-heal: remove legacy deprecated variants if present (idempotent).
	legacyPaths := []string{
		filepath.Join(extensionsDir, "biggz-web-search-legacy.js"),
		filepath.Join(extensionsDir, "web-search.js"),
	}
	for _, lp := range legacyPaths {
		_ = os.Remove(lp)
	}

	return result, nil
}
