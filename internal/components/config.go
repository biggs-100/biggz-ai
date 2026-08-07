package components

import (
	"context"
	"io/fs"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/plugin"
)

type configComponent struct {
	homeDir string
	fsys    fs.FS
}

// NewConfigComponent creates a Component that merges configuration overlays
// into the agent's settings file. The fsys parameter should be the embedded
// assets filesystem (use assets.FS in production).
func NewConfigComponent(homeDir string, fsys fs.FS) Component {
	return &configComponent{homeDir: homeDir, fsys: fsys}
}

func (c *configComponent) ID() string { return "config" }

func (c *configComponent) Deploy(ctx context.Context, adapter plugin.AgentAdapter) (*DeploymentResult, error) {
	settingsPath := adapter.SettingsPath(c.homeDir)

	var files []string
	// Check if overlay exists before deploying
	overlayData, err := fs.ReadFile(c.fsys, "opencode/sdd-overlay-multi.json")
	if err == nil {
		files = append(files, "opencode/sdd-overlay-multi.json")
	}
	_ = overlayData // used implicitly by install.DeployConfig

	merged, err := install.DeployConfig(settingsPath, c.fsys, false)
	if err != nil {
		return nil, err
	}

	if merged {
		files = append(files, settingsPath)
	}

	return &DeploymentResult{
		Changed: merged,
		Files:   files,
	}, nil
}

var _ Component = (*configComponent)(nil)
var _ = assets.FS
