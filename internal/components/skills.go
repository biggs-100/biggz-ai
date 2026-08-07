package components

import (
	"context"
	"io/fs"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/plugin"
)

type skillsComponent struct {
	homeDir string
	fsys    fs.FS
}

// NewSkillsComponent creates a Component that deploys embedded skill files
// to the agent's skills directory. The fsys parameter should be the embedded
// assets filesystem (use assets.FS in production).
func NewSkillsComponent(homeDir string, fsys fs.FS) Component {
	return &skillsComponent{homeDir: homeDir, fsys: fsys}
}

func (c *skillsComponent) ID() string { return "skills" }

func (c *skillsComponent) Deploy(ctx context.Context, adapter plugin.AgentAdapter) (*DeploymentResult, error) {
	skillsDir := adapter.SkillsDir(c.homeDir)

	// Collect file list from the embedded FS before deploying
	var files []string
	fs.WalkDir(c.fsys, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	count, err := install.DeploySkills(skillsDir, c.fsys, false)
	if err != nil {
		return nil, err
	}

	if count == 0 {
		files = nil
	}

	return &DeploymentResult{
		Changed: count > 0,
		Files:   files,
	}, nil
}

// Ensure compile-time compatibility with production use.
var _ Component = (*skillsComponent)(nil)
var _ = assets.FS // force import for production convenience
