package components

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/biggz-ai/biggz/internal/assets"
	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/plugin"
)

type promptsComponent struct {
	homeDir string
	fsys    fs.FS
}

// NewPromptsComponent creates a Component that deploys embedded SDD prompt
// files to the agent's prompts directory. The fsys parameter should be the
// embedded assets filesystem (use assets.FS in production).
func NewPromptsComponent(homeDir string, fsys fs.FS) Component {
	return &promptsComponent{homeDir: homeDir, fsys: fsys}
}

func (c *promptsComponent) ID() string { return "prompts" }

func (c *promptsComponent) Deploy(ctx context.Context, adapter plugin.AgentAdapter) (*DeploymentResult, error) {
	promptsDir := filepath.Join(adapter.GlobalConfigDir(c.homeDir), "prompts", "sdd")

	// Collect file list from the embedded FS before deploying
	var files []string
	fs.WalkDir(c.fsys, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	err := install.DeployPrompts(promptsDir, c.fsys, false)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return &DeploymentResult{Changed: false}, nil
	}

	return &DeploymentResult{
		Changed: true,
		Files:   files,
	}, nil
}

var _ Component = (*promptsComponent)(nil)
var _ = assets.FS
