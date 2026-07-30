// Package model defines the core data types for biggz-ai.
//
// It contains the review data structures (ReviewState, FSM, hashing) as well
// as identity and configuration types for AI coding agent adapters.
//
// Agent types live here to avoid import cycles — both plugin/interfaces.go
// and internal/agents/ import model, while internal/agents/* adapters also
// import plugin.
package model

// AgentID is a typed string identifying an AI coding agent.
// It is used as a map key in registries and as the return type of
// AgentAdapter.ID(). Each known agent has a corresponding constant.
type AgentID string

// SupportTier indicates the level of support commitment for an agent adapter.
type SupportTier string

const (
	// TierFull indicates first-class, actively maintained support.
	TierFull SupportTier = "full"
	// TierFirst indicates a top-priority supported agent (future).
	TierFirst SupportTier = "first"
	// TierExtended indicates extended/community support (future).
	TierExtended SupportTier = "extended"
	// TierCommunity indicates community-contributed support (future).
	TierCommunity SupportTier = "community"
	// TierExperimental indicates experimental/unstable support (future).
	TierExperimental SupportTier = "experimental"
	// TierRetired indicates previously supported, now deprecated (future).
	TierRetired SupportTier = "retired"
)

// SystemPromptStrategy defines how the agent injects the system prompt
// (AGENTS.md, CLAUDE.md, etc.) into its context.
type SystemPromptStrategy int

const (
	// StrategyMarkdownSections reads sections from a markdown file.
	StrategyMarkdownSections SystemPromptStrategy = iota
	// StrategyFileReplace replaces the target file entirely.
	StrategyFileReplace
	// StrategyAppendToFile appends content to the target file.
	StrategyAppendToFile
	// StrategyInstructionsFile uses a dedicated instructions file.
	StrategyInstructionsFile
	// StrategyJinjaModules uses Jinja2-style module injection.
	StrategyJinjaModules
	// StrategySteeringFile uses a steering/override file.
	StrategySteeringFile
)

func (s SystemPromptStrategy) String() string {
	switch s {
	case StrategyMarkdownSections:
		return "markdown-sections"
	case StrategyFileReplace:
		return "file-replace"
	case StrategyAppendToFile:
		return "append-to-file"
	case StrategyInstructionsFile:
		return "instructions-file"
	case StrategyJinjaModules:
		return "jinja-modules"
	case StrategySteeringFile:
		return "steering-file"
	default:
		return "unknown"
	}
}

// MCPStrategy defines how the agent manages Model Context Protocol (MCP)
// server configuration.
type MCPStrategy int

const (
	// StrategySeparateMCPFiles stores each MCP server in its own file.
	StrategySeparateMCPFiles MCPStrategy = iota
	// StrategyMergeIntoSettings merges MCP config into the main settings file.
	StrategyMergeIntoSettings
	// StrategyMCPConfigFile uses a dedicated MCP config file.
	StrategyMCPConfigFile
	// StrategyTOMLFile uses a TOML-based config file.
	StrategyTOMLFile
	// StrategyMergeIntoYAML merges MCP config into a YAML settings file.
	StrategyMergeIntoYAML
)

func (s MCPStrategy) String() string {
	switch s {
	case StrategySeparateMCPFiles:
		return "separate-mcp-files"
	case StrategyMergeIntoSettings:
		return "merge-into-settings"
	case StrategyMCPConfigFile:
		return "mcp-config-file"
	case StrategyTOMLFile:
		return "toml-file"
	case StrategyMergeIntoYAML:
		return "merge-into-yaml"
	default:
		return "unknown"
	}
}
