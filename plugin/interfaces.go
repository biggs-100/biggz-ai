// Package plugin defines the interfaces for extending biggz-ai capabilities.
//
// LensPlugin provides domain-specific code analysis. AgentAdapter enables
// discovery and integration with AI coding agents (OpenCode, Claude, etc.)
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
	// ID returns a unique identifier for this agent adapter.
	ID() string

	// Name returns a human-readable name (e.g. "OpenCode", "Claude Code").
	Name() string

	// Detect checks whether this agent is installed on the system.
	// Returns the binary path if found, or an error if not detected.
	Detect(ctx context.Context) (binaryPath string, err error)

	// Capabilities returns the list of capabilities this agent supports
	// (e.g. "skills", "mcp", "sub_agents", "system_prompt").
	Capabilities() []Capability

	// DeployConfig installs or updates configuration for this agent.
	// This may include skills, MCP servers, system prompts, etc.
	DeployConfig(ctx context.Context, cfg AgentConfig) error

	// GlobalConfigDir returns the agent's global configuration directory
	// under the given home directory (e.g., ~/.config/opencode/).
	GlobalConfigDir(homeDir string) string

	// SkillsDir returns the subdirectory where the agent stores skills
	// (e.g., ~/.config/opencode/skills/).
	SkillsDir(homeDir string) string

	// SettingsPath returns the full path to the agent's settings
	// configuration file (e.g., ~/.config/opencode/opencode.jsonc).
	SettingsPath(homeDir string) string
}

// Capability represents a feature supported by an AI coding agent.
type Capability string

const (
	CapSkills       Capability = "skills"
	CapMCP          Capability = "mcp"
	CapSubAgents    Capability = "sub_agents"
	CapSystemPrompt Capability = "system_prompt"
	CapSlashCommands Capability = "slash_commands"
	CapWorkflows    Capability = "workflows"
)

// AgentConfig holds configuration to deploy to an AI coding agent.
type AgentConfig struct {
	SkillsDir     string            `json:"skills_dir,omitempty"`
	MCPServers    map[string]string `json:"mcp_servers,omitempty"`    // name → binary path
	SystemPrompt  string            `json:"system_prompt,omitempty"`
	SlashCommands []string          `json:"slash_commands,omitempty"`
}
