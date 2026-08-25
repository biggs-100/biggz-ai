package install

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// DeployPiQuestionMouse writes internal/assets/pi/biggz-question-mouse.js to
// ~/.pi/agent/extensions/biggz-question-mouse.js via filemerge.WriteFileAtomic.
// It creates parent directories, is idempotent, supports TempDir via homeDir,
// and self-heals legacy stale extension variants. The extension adds SGR mouse
// parity to pi's ask_user_question (keyboard-only) matching opencode's question
// tool: left click focuses an option, second click confirms (single-select) or
// toggles checkbox (multi-select). Terminal mouse reporting is ESC[?1000h+1006h.
func DeployPiQuestionMouse(ctx context.Context, homeDir string) (filemerge.WriteResult, error) {
	_ = ctx
	extensionsDir := piExtensionsDir(homeDir)
	targetPath := filepath.Join(extensionsDir, "biggz-question-mouse.js")

	data, err := fs.ReadFile(assets.FS, "pi/biggz-question-mouse.js")
	if err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("read pi question mouse asset: %w", err)
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
		filepath.Join(extensionsDir, "biggz-question-mouse-legacy.js"),
		filepath.Join(extensionsDir, "question-mouse.js"),
	}
	for _, lp := range legacyPaths {
		_ = os.Remove(lp)
	}

	return result, nil
}
