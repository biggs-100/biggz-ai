package update

import (
	"context"

	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/plugin"
)

// ReconcileResult describes what a reconcile pass deployed.
type ReconcileResult struct {
	Skills       int  // skill files deployed (canonical store + agent dir)
	Commands     int  // command files deployed
	Plugins      int  // plugin files deployed
	Prompts      int  // prompt files deployed
	ConfigMerged bool // settings overlay merged
	MCPDeployed  bool // MCP server binary + config deployed
}

// Reconcile re-deploys the full install-equivalent asset set for one
// detected agent: skills (canonical ~/.biggz/skills/ plus the agent's skills
// dir), prompts, commands, plugins, config overlay, persona/bigmem sections,
// MCP binary and MCP config. It reuses install.Run so the deployed set can
// never drift from a fresh install.
func Reconcile(ctx context.Context, adapter plugin.AgentAdapter, homeDir string) (*ReconcileResult, error) {
	r, err := install.Run(ctx, adapter, install.Config{HomeDir: homeDir})
	if err != nil {
		return nil, err
	}
	return &ReconcileResult{
		Skills:       r.SkillsDeployed,
		Commands:     r.CommandsWritten,
		Plugins:      r.PluginsDeployed,
		Prompts:      r.PromptsDeployed,
		ConfigMerged: r.ConfigMerged,
		MCPDeployed:  r.MCPDeployed,
	}, nil
}
