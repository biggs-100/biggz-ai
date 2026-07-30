// Package plugin defines the interfaces for extending biggz-ai capabilities.
//
// LensPlugin provides domain-specific code analysis. AgentAdapter enables
// discovery and integration with AI coding agents (OpenCode, Claude Code, etc.)
// that are installed on the system. Both are registered at build time via
// the registry package.
package plugin

import (
	"context"

	"github.com/biggz-ai/biggz/model"
)

// LensResult contains the findings produced by a LensPlugin after analysis.
type LensResult struct {
	LensID   string    `json:"lens_id"`
	Findings []Finding `json:"findings"`
}

// Finding represents a single observation from a lens analysis.
type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// Policy describes a policy that a lens enforces or recommends.
// Lenses return their associated policies via the Policies() method.
type Policy struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LensPlugin defines the interface for a code review analysis lens.
// Each lens is identified by a unique ID and can analyze a ReviewSubject
// to produce structured findings.
type LensPlugin interface {
	// ID returns the unique identifier for this lens.
	ID() string

	// Name returns a human-readable name for this lens.
	Name() string

	// Version returns the version string for this lens.
	Version() string

	// Analyze runs the lens analysis against the given subject.
	// It returns a LensResult containing findings, or an error if the
	// analysis could not be completed.
	Analyze(ctx context.Context, subject model.ReviewSubject) (*LensResult, error)

	// Policies returns the list of policies associated with this lens.
	Policies() []Policy
}

// AgentAdapter defines the interface for discovering and integrating with
// an AI coding agent installed on the system (e.g., OpenCode, Claude Code,
// Cursor). It does NOT call AI APIs — the agent itself is the AI runtime.
// biggz-ai is a harness that discovers what agent is available, what it
// supports, and configures it.
type AgentAdapter interface {
	// ID returns the unique typed identifier for this agent adapter.
	ID() model.AgentID

	// Name returns a human-readable name (e.g. "OpenCode", "Claude Code").
	Name() string

	// Tier returns the support commitment tier for this adapter.
	Tier() model.SupportTier

	// Detect checks whether this agent is installed on the system.
	// Returns installation status, binary path, config path, whether
	// auto-installation is capable, and any error.
	Detect(ctx context.Context, homeDir string) (installed bool, binaryPath string, configPath string, autoInstallCapable bool, err error)

	// InstallCommand returns the shell commands needed to install or
	// update this agent. Each element is a command + args slice.
	// Accepts an optional profile for version/channel customization.
	InstallCommand(profile interface{}) ([][]string, error)

	// Capabilities returns the list of capability strings this agent
	// supports (e.g. "skills", "mcp", "sub_agents", "system_prompt").
	Capabilities() []string

	// SupportsAutoInstall returns true if this agent supports automatic
	// binary download and setup without manual intervention.
	SupportsAutoInstall() bool

	// SupportsSkills returns true if this agent supports custom skills.
	SupportsSkills() bool

	// SupportsSystemPrompt returns true if this agent supports system
	// prompt injection (AGENTS.md, CLAUDE.md, etc.).
	SupportsSystemPrompt() bool

	// SupportsMCP returns true if this agent supports the Model Context
	// Protocol for tool/plugin integration.
	SupportsMCP() bool

	// SupportsOutputStyles returns true if this agent supports custom
	// output style/formatting configuration.
	SupportsOutputStyles() bool

	// SupportsSlashCommands returns true if this agent supports custom
	// slash commands.
	SupportsSlashCommands() bool

	// SupportsSubAgents returns true if this agent supports delegating
	// work to sub-agents.
	SupportsSubAgents() bool

	// SystemPromptStrategy returns the strategy the agent uses for
	// system prompt file injection.
	SystemPromptStrategy() model.SystemPromptStrategy

	// MCPStrategy returns the strategy the agent uses for MCP server
	// configuration.
	MCPStrategy() model.MCPStrategy

	// GlobalConfigDir returns the agent's global configuration directory
	// under the given home directory.
	GlobalConfigDir(homeDir string) string

	// SystemPromptDir returns the directory where the agent's system
	// prompt file lives.
	SystemPromptDir(homeDir string) string

	// SystemPromptFile returns the full path to the agent's system
	// prompt file (e.g., AGENTS.md, CLAUDE.md).
	SystemPromptFile(homeDir string) string

	// SkillsDir returns the subdirectory where the agent stores skills.
	SkillsDir(homeDir string) string

	// CommandsDir returns the subdirectory where the agent stores
	// custom slash commands.
	CommandsDir(homeDir string) string

	// SubAgentsDir returns the subdirectory where the agent stores
	// sub-agent definitions.
	SubAgentsDir(homeDir string) string

	// EmbeddedSubAgentsDir returns the relative path to embedded
	// sub-agents shipped with biggz-ai (no homeDir needed).
	EmbeddedSubAgentsDir() string

	// OutputStyleDir returns the subdirectory where the agent stores
	// output style definitions.
	OutputStyleDir(homeDir string) string

	// SettingsPath returns the full path to the agent's settings
	// configuration file.
	SettingsPath(homeDir string) string

	// MCPConfigPath returns the path where this agent stores MCP
	// server configuration for the given server name.
	MCPConfigPath(homeDir string, serverName string) string

	// DeployConfig installs or updates configuration for this agent.
	// This may include skills, MCP servers, system prompts, etc.
	DeployConfig(ctx context.Context, cfg AgentConfig) error
}

// Capability constants — used as string values by Capabilities().
const (
	CapSkills        = "skills"
	CapMCP           = "mcp"
	CapSubAgents     = "sub_agents"
	CapSystemPrompt  = "system_prompt"
	CapSlashCommands = "slash_commands"
	CapWorkflows     = "workflows"
)

// AgentConfig holds configuration to deploy to an AI coding agent.
type AgentConfig struct {
	SkillsDir     string            `json:"skills_dir,omitempty"`
	MCPServers    map[string]string `json:"mcp_servers,omitempty"`    // name → binary path
	SystemPrompt  string            `json:"system_prompt,omitempty"`
	SlashCommands []string          `json:"slash_commands,omitempty"`
}
