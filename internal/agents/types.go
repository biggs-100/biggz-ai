package agents

import "github.com/biggs-100/biggz-ai/model"

// AgentID is a type alias for model.AgentID, ensuring all agent adapters
// return the canonical typed identifier from model.
type AgentID = model.AgentID

// SupportTier is a type alias for model.SupportTier.
type SupportTier = model.SupportTier

// Known agent identifiers.
const (
	AgentOpenCode      AgentID = "opencode"
	AgentClaudeCode    AgentID = "claude-code"
	AgentQwenCode      AgentID = "qwen-code"
	AgentCursor        AgentID = "cursor"
	AgentWindsurf      AgentID = "windsurf"
	AgentGitHubCopilot AgentID = "github-copilot"
	AgentCody          AgentID = "cody"
	AgentAider         AgentID = "aider"
	AgentContinue      AgentID = "continue"
	AgentCodeium       AgentID = "codeium"
	AgentTabby         AgentID = "tabby"
	AgentMarsCode      AgentID = "marscode"
	AgentComate        AgentID = "comate"
	AgentCodeGeeX      AgentID = "codegeex"
	AgentMelo          AgentID = "melo"
	AgentLingma        AgentID = "lingma"
	AgentGeminiCLI     AgentID = "gemini-cli"
	AgentCodexCLI      AgentID = "codex-cli"
	AgentPi            AgentID = "pi"
	AgentVSCode        AgentID = "vscode-copilot"
	AgentKiro          AgentID = "kiro"
	AgentAntigravity   AgentID = "antigravity"
	AgentHermes        AgentID = "hermes"
	AgentKimi          AgentID = "kimi"
	AgentKiloCode      AgentID = "kilocode"
	AgentTrae          AgentID = "trae-ide"
	AgentOpenClaw      AgentID = "openclaw"
)

// Pre-defined support tier constants for convenience.
const (
	TierFull        = model.TierFull
	TierFirst       = model.TierFirst
	TierExtended    = model.TierExtended
	TierCommunity   = model.TierCommunity
	TierExperimental = model.TierExperimental
	TierRetired     = model.TierRetired
)
