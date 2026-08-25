// Package capabilitymanifest provides canonical feature claims for every known
// AI coding agent. Each agent has a pre-defined set of feature claims that
// describe what the agent supports (skills, MCP, sub-agents, etc.).
//
// The featureClaimsByAgent map is the single source of truth — it contains
// exactly 16 entries covering all agents in the catalog. Implemented agents
// (opencode, claude-code, qwen-code) have realistic claims; the remaining 13
// are placeholders with zero-value claims.
package capabilitymanifest

import (
	"fmt"

	"github.com/biggs-100/biggz-ai/model"
)

// ContractPiReviewRelay is the Biggz host relay contract Pi requires for
// immutable receipt reviews. The gentle alias is kept for compat with a
// gentle-pi host that still exports gentle-pi.review-relay/v1. This mirrors
// gentle's ContractImmutableReviewExecutorV1 exposure for pi.
const ContractPiReviewRelay = "biggz-pi.review-relay/v1"

// ContractGentlePiReviewRelay is the gentle alias kept for compat.
const ContractGentlePiReviewRelay = "gentle-pi.review-relay/v1"

// AgentCapabilityManifest declares what features an AI coding agent supports.
type AgentCapabilityManifest struct {
	SchemaVersion string             `json:"schemaVersion"`
	AgentID       model.AgentID      `json:"agentId"`
	Features      AgentFeatureClaims `json:"features"`
}

// AgentFeatureClaims is a set of boolean flags describing agent capabilities.
type AgentFeatureClaims struct {
	AutoInstall   bool `json:"autoInstall"`
	OutputStyles  bool `json:"outputStyles"`
	SlashCommands bool `json:"slashCommands"`
	FileSubAgents bool `json:"fileSubAgents"`
	Skills        bool `json:"skills"`
	SystemPrompt  bool `json:"systemPrompt"`
	MCP           bool `json:"mcp"`
	Workflows     bool `json:"workflows"`
}

// featureClaimsByAgent is the canonical map of agent feature claims.
// It contains exactly 16 entries, one for every known agent.
// The first 3 (implemented agents) have realistic claims; the remaining 13
// are out-of-scope agents with empty (zero-value) claims.
var featureClaimsByAgent = map[model.AgentID]AgentFeatureClaims{
	// Implemented agents — realistic feature claims
	model.AgentID("opencode"): {
		AutoInstall:   true,
		OutputStyles:  false,
		SlashCommands: true,
		FileSubAgents: false,
		Skills:        true,
		SystemPrompt:  true,
		MCP:           true,
		Workflows:     true,
	},
	model.AgentID("claude-code"): {
		AutoInstall:   true,
		OutputStyles:  true,
		SlashCommands: true,
		FileSubAgents: true,
		Skills:        true,
		SystemPrompt:  true,
		MCP:           true,
		Workflows:     false,
	},
	model.AgentID("qwen-code"): {
		AutoInstall:   false,
		OutputStyles:  false,
		SlashCommands: true,
		FileSubAgents: false,
		Skills:        true,
		SystemPrompt:  true,
		MCP:           true,
		Workflows:     false,
	},
	// Implemented agents
	model.AgentID("cursor"): {
		AutoInstall:   true,
		OutputStyles:  false,
		SlashCommands: false,
		FileSubAgents: true,
		Skills:        true,
		SystemPrompt:  true,
		MCP:           true,
		Workflows:     false,
	},
	model.AgentID("windsurf"): {
		AutoInstall:   true,
		OutputStyles:  false,
		SlashCommands: false,
		FileSubAgents: false,
		Skills:        true,
		SystemPrompt:  true,
		MCP:           true,
		Workflows:     true,
	},
	model.AgentID("github-copilot"):   {},
	model.AgentID("cody"):             {},
	model.AgentID("aider"):            {},
	model.AgentID("continue"):         {},
	model.AgentID("codeium"):          {},
	model.AgentID("tabby"):            {},
	model.AgentID("marscode"):         {},
	model.AgentID("comate"):           {},
	model.AgentID("codegeex"):         {},
	model.AgentID("melo"):             {},
	model.AgentID("lingma"):           {},
	model.AgentID("gemini-cli"): {
		AutoInstall:   true,
		Skills:        true,
		SystemPrompt:  true,
	},
	model.AgentID("codex-cli"): {
		AutoInstall:   true,
		Skills:        true,
		SystemPrompt:  true,
	},
	model.AgentID("pi"): {
		AutoInstall:   true,
		MCP:           true,
		SystemPrompt:  true,
		FileSubAgents: true,
		Skills:        true,
	},
	model.AgentID("vscode-copilot"): {
		AutoInstall:   false,
		Skills:        false,
		SystemPrompt:  true,
		MCP:           true,
	},
	model.AgentID("kiro"): {
		AutoInstall:   true,
		Skills:        true,
		SystemPrompt:  true,
		MCP:           true,
		FileSubAgents: true,
	},
	model.AgentID("antigravity"): {Skills: true, SystemPrompt: true},
	model.AgentID("hermes"):      {Skills: true, SystemPrompt: true},
	model.AgentID("kimi"):        {Skills: true, SystemPrompt: true},
	model.AgentID("kilocode"):    {Skills: true, SystemPrompt: true},
	model.AgentID("trae-ide"):    {SystemPrompt: true},
	model.AgentID("openclaw"):    {Skills: true, SystemPrompt: true},
}

// ForAgent returns the canonical capability manifest for the given agent ID.
// Returns nil, false if the agent ID is unknown.
func ForAgent(id model.AgentID) (*AgentCapabilityManifest, bool) {
	claims, ok := featureClaimsByAgent[id]
	if !ok {
		return nil, false
	}
	return &AgentCapabilityManifest{
		SchemaVersion: "1.0",
		AgentID:       id,
		Features:      claims,
	}, true
}

// Validate checks that the given manifest matches the canonical entry for its
// AgentID. It returns an error if the agent is unknown or if the feature claims
// differ from the canonical values.
func Validate(manifest AgentCapabilityManifest) error {
	canonical, ok := featureClaimsByAgent[manifest.AgentID]
	if !ok {
		return fmt.Errorf("unknown agent ID: %s", manifest.AgentID)
	}
	if manifest.Features != canonical {
		return fmt.Errorf("feature claims for %s do not match canonical manifest", manifest.AgentID)
	}
	return nil
}

// Count returns the number of entries in the featureClaimsByAgent map.
// This is used by tests to verify the expected count (16).
func Count() int {
	return len(featureClaimsByAgent)
}

// AllAgentIDs returns all known agent IDs from the canonical map.
func AllAgentIDs() []model.AgentID {
	ids := make([]model.AgentID, 0, len(featureClaimsByAgent))
	for id := range featureClaimsByAgent {
		ids = append(ids, id)
	}
	return ids
}
