package agents

import (
	"fmt"
	"testing"
)

func TestAgentIDConstants_NotEmpty(t *testing.T) {
	ids := []AgentID{
		AgentOpenCode, AgentClaudeCode, AgentQwenCode,
		AgentCursor, AgentWindsurf, AgentGitHubCopilot,
		AgentCody, AgentAider, AgentContinue, AgentCodeium,
		AgentTabby, AgentMarsCode, AgentComate, AgentCodeGeeX,
		AgentMelo, AgentLingma,
	}
	if len(ids) != 16 {
		t.Fatalf("expected 27 AgentID constants, got %d", len(ids))
	}
	for _, id := range ids {
		if string(id) == "" {
			t.Error("found empty AgentID constant")
		}
	}
}

func TestAgentID_SpecificValues(t *testing.T) {
	tests := []struct {
		id   AgentID
		want string
	}{
		{AgentOpenCode, "opencode"},
		{AgentClaudeCode, "claude-code"},
		{AgentQwenCode, "qwen-code"},
		{AgentCursor, "cursor"},
		{AgentWindsurf, "windsurf"},
		{AgentGitHubCopilot, "github-copilot"},
		{AgentCody, "cody"},
		{AgentAider, "aider"},
		{AgentContinue, "continue"},
		{AgentCodeium, "codeium"},
		{AgentTabby, "tabby"},
		{AgentMarsCode, "marscode"},
		{AgentComate, "comate"},
		{AgentCodeGeeX, "codegeex"},
		{AgentMelo, "melo"},
		{AgentLingma, "lingma"},
	}
	for _, tt := range tests {
		if string(tt.id) != tt.want {
			t.Errorf("AgentID(%s) = %q, want %q", tt.want, string(tt.id), tt.want)
		}
	}
}

func TestAgentID_MapKey(t *testing.T) {
	m := map[AgentID]string{
		AgentOpenCode:   "opencode adapter",
		AgentClaudeCode: "claude-code adapter",
		AgentQwenCode:   "qwen-code adapter",
	}
	if m[AgentOpenCode] != "opencode adapter" {
		t.Error("AgentID map key lookup failed")
	}
	if _, ok := m[AgentCursor]; ok {
		t.Error("unregistered AgentID should not be found in the map")
	}
}

func TestSupportTier_Full(t *testing.T) {
	if TierFull != SupportTier("full") {
		t.Errorf("TierFull = %q, want %q", TierFull, "full")
	}
}

func TestSystemPromptStrategy_Values(t *testing.T) {
	tests := []struct {
		s    SystemPromptStrategy
		want string
	}{
		{StrategyMarkdownSections, "markdown-sections"},
		{StrategyFileReplace, "file-replace"},
		{StrategyAppendToFile, "append-to-file"},
		{StrategyInstructionsFile, "instructions-file"},
		{StrategyJinjaModules, "jinja-modules"},
		{StrategySteeringFile, "steering-file"},
	}
	for _, tt := range tests {
		got := tt.s.String()
		if got != tt.want {
			t.Errorf("SystemPromptStrategy(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestMCPStrategy_Values(t *testing.T) {
	tests := []struct {
		s    MCPStrategy
		want string
	}{
		{StrategySeparateMCPFiles, "separate-mcp-files"},
		{StrategyMergeIntoSettings, "merge-into-settings"},
		{StrategyMCPConfigFile, "mcp-config-file"},
		{StrategyTOMLFile, "toml-file"},
		{StrategyMergeIntoYAML, "merge-into-yaml"},
	}
	for _, tt := range tests {
		got := tt.s.String()
		if got != tt.want {
			t.Errorf("MCPStrategy(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestSystemPromptStrategy_DefaultString(t *testing.T) {
	if StrategyMarkdownSections.String() != "markdown-sections" {
		t.Fatalf("expected StrategyMarkdownSections (0) to produce the correct default")
	}
}

func TestMCPStrategy_DefaultString(t *testing.T) {
	if StrategySeparateMCPFiles.String() != "separate-mcp-files" {
		t.Fatalf("expected StrategySeparateMCPFiles (0) to produce the correct default")
	}
}

func TestSystemPromptStrategy_Unknown(t *testing.T) {
	var s SystemPromptStrategy = 99
	if s.String() != "unknown" {
		t.Errorf("SystemPromptStrategy(99).String() = %q, want %q", s.String(), "unknown")
	}
}

func TestMCPStrategy_Unknown(t *testing.T) {
	var s MCPStrategy = 99
	if s.String() != "unknown" {
		t.Errorf("MCPStrategy(99).String() = %q, want %q", s.String(), "unknown")
	}
}

func ExampleAgentID() {
	fmt.Println(string(AgentOpenCode))
	fmt.Println(string(AgentClaudeCode))
	// Output:
	// opencode
	// claude-code
}

func ExampleTierFull() {
	fmt.Println(string(TierFull))
	// Output:
	// full
}
