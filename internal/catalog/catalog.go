// Package catalog provides a metadata-driven registry of discoverable agents,
// components, and skills. All entries are hardcoded Go slices — no dynamic
// loading, no runtime init. Slices are returned by value to prevent caller
// mutation from corrupting the originals.
package catalog

import (
	"github.com/biggz-ai/biggz/internal/agents"
	"github.com/biggz-ai/biggz/plugin"
)

// SkillEntry extends CatalogEntry with platform and dependency metadata.
type SkillEntry struct {
	plugin.CatalogEntry
	Platforms []string `json:"platforms,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// ComponentEntry extends CatalogEntry with dependency metadata.
type ComponentEntry struct {
	plugin.CatalogEntry
	Dependencies []string `json:"dependencies,omitempty"`
}

// -- agents -------------------------------------------------------------------

var allAgents = []plugin.CatalogEntry{
	{
		ID:          string(agents.AgentOpenCode),
		Name:        "OpenCode",
		Description: "Open-source AI coding agent with skills, MCP, and sub-agent support",
		Tier:        "native",
		Type:        "agent",
	},
	{
		ID:          string(agents.AgentClaudeCode),
		Name:        "Claude Code",
		Description: "Anthropic's AI coding agent with skills and system prompt support",
		Tier:        "native",
		Type:        "agent",
	},
	{
		ID:          string(agents.AgentQwenCode),
		Name:        "Qwen Code",
		Description: "Alibaba's AI coding agent with basic skill and prompt support",
		Tier:        "community",
		Type:        "agent",
	},
}

// AllAgents returns a defensive copy of the built-in agent catalog.
func AllAgents() []plugin.CatalogEntry {
	out := make([]plugin.CatalogEntry, len(allAgents))
	copy(out, allAgents)
	return out
}

// GetAgent returns the agent metadata for the given ID, or nil if unknown.
func GetAgent(id string) *plugin.CatalogEntry {
	for _, a := range allAgents {
		if a.ID == id {
			return &a
		}
	}
	return nil
}

// IsSupportedAgent returns true if the given agent ID is in the built-in catalog.
func IsSupportedAgent(id string) bool {
	for _, a := range allAgents {
		if a.ID == id {
			return true
		}
	}
	return false
}

// -- components ---------------------------------------------------------------

var allComponents = []ComponentEntry{
	{
		CatalogEntry: plugin.CatalogEntry{
			ID:          "skills",
			Name:        "Skills",
			Description: "Custom slash commands and automation scripts for the agent",
			Tier:        "native",
			Type:        "component",
		},
		Dependencies: []string{},
	},
	{
		CatalogEntry: plugin.CatalogEntry{
			ID:          "config",
			Name:        "Configuration",
			Description: "Agent configuration files (opencode.jsonc, claude.jsonc, etc.)",
			Tier:        "native",
			Type:        "component",
		},
		Dependencies: []string{"skills"},
	},
	{
		CatalogEntry: plugin.CatalogEntry{
			ID:          "prompts",
			Name:        "Prompts",
			Description: "System prompt templates and custom prompt rules",
			Tier:        "native",
			Type:        "component",
		},
		Dependencies: []string{"config"},
	},
}

// AllComponents returns a defensive copy of the built-in component catalog.
func AllComponents() []ComponentEntry {
	out := make([]ComponentEntry, len(allComponents))
	copy(out, allComponents)
	return out
}

// ListComponents returns component entries filtered by the given tier.
func ListComponents(tier string) []ComponentEntry {
	if tier == "" {
		return AllComponents()
	}
	var out []ComponentEntry
	for _, c := range allComponents {
		if c.Tier == tier {
			out = append(out, c)
		}
	}
	return out
}

// -- skills -------------------------------------------------------------------

var allSkills = []SkillEntry{
	{
		CatalogEntry: plugin.CatalogEntry{
			ID:          "code-review",
			Name:        "Code Review",
			Description: "AI-powered code review automation",
			Tier:        "native",
			Type:        "skill",
		},
		Platforms: []string{"linux", "darwin", "windows"},
		DependsOn: []string{},
	},
	{
		CatalogEntry: plugin.CatalogEntry{
			ID:          "mcp-tools",
			Name:        "MCP Tools",
			Description: "Model Context Protocol tool integration",
			Tier:        "native",
			Type:        "skill",
		},
		Platforms: []string{"linux", "darwin"},
		DependsOn: []string{},
	},
	{
		CatalogEntry: plugin.CatalogEntry{
			ID:          "auto-fix",
			Name:        "Auto Fix",
			Description: "Automatic code fix suggestions",
			Tier:        "community",
			Type:        "skill",
		},
		Platforms: []string{"linux", "darwin", "windows"},
		DependsOn: []string{"code-review"},
	},
}

// AllSkills returns a defensive copy of the built-in skill catalog.
func AllSkills() []SkillEntry {
	out := make([]SkillEntry, len(allSkills))
	copy(out, allSkills)
	return out
}
